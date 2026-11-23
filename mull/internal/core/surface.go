package core

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The surface is the human's one view: what changed since they last
// looked, in the order they act on it. Calls that need them; what's
// trending worse; where each plan stands; how the work moved. Every number
// comes from mull's records and git, never from what an agent wrote.

// Surface is the human's view of a project since a time.
type Surface struct {
	Since     string      `json:"since"`
	Generated string      `json:"generated"`
	Calls     []Call      `json:"calls"`
	Rules     []RuleTrend `json:"rules"`
	Advisory  []string    `json:"advisory"`
	Clusters  []Cluster   `json:"clusters"`
	Problems  []Problem   `json:"problems"`
	NewEdges  []string    `json:"new_edges"`
	Plans     []PlanHill  `json:"plans"`
	Flow      Flow        `json:"flow"`
	Audit     *Audited    `json:"audit,omitempty"`
}

// Call is one thing for the human to act on.
type Call struct {
	Kind    string `json:"kind"` // review (high risk, waiting), question, look (landed)
	Entry   string `json:"entry"`
	Item    string `json:"item,omitempty"`
	Plan    string `json:"plan,omitempty"`
	Text    string `json:"text"`
	Why     string `json:"why,omitempty"`
	Default string `json:"default,omitempty"`
	Due     string `json:"due,omitempty"`
	Lapsed  bool   `json:"lapsed,omitempty"` // past due: the default stands
	Created string `json:"created"`
}

// RuleTrend is a checked rule's count at the last look and now.
type RuleTrend struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Then    int    `json:"then"`
	Now     int    `json:"now"`
	Failing bool   `json:"failing,omitempty"`
	Trend   string `json:"trend"` // worse, better, same, new, unrun
}

// Problem is something wrong with work in flight.
type Problem struct {
	Item  string `json:"item"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// PlanHill is a plan's phases as a hill.
type PlanHill struct {
	Slug    string      `json:"slug"`
	Title   string      `json:"title"`
	State   string      `json:"state"`
	Target  string      `json:"target"`
	Preview string      `json:"preview,omitempty"`
	ToShip  int         `json:"to_ship"` // commits on the target the release branch lacks
	Phases  []PhaseHill `json:"phases"`
}

// Flow is how work moved since the last look.
type Flow struct {
	Active        int       `json:"active"`
	Parallel      int       `json:"parallel"`
	Queue         int       `json:"queue"`
	Landed        []Landing `json:"landed"`
	Slowed        []Slowed  `json:"slowed"`
	Rework        int       `json:"rework"`
	MedianMinutes int       `json:"median_minutes,omitempty"`
}

// Landing is work finished since the last look.
type Landing struct {
	Item    string `json:"item"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Risk    string `json:"risk"`
	Sampled bool   `json:"sampled,omitempty"`
	At      string `json:"at"`
}

// Slowed is an agent's line on what slowed its work.
type Slowed struct {
	Item string `json:"item"`
	Text string `json:"text"`
}

// Audited is the latest audit's date and first lines.
type Audited struct {
	At      string `json:"at"`
	Summary string `json:"summary"`
	Stale   bool   `json:"stale,omitempty"`
}

// Surface reads the human's view since a time (RFC 3339; "" is a week ago).
func (m *Mull) Surface(since string) (Surface, error) {
	now := m.Now().UTC()
	if since == "" {
		since = now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	}
	v := Surface{Since: since, Generated: now.Format(time.RFC3339), Calls: []Call{}, Rules: []RuleTrend{},
		Advisory: []string{}, Clusters: []Cluster{}, Problems: []Problem{}, NewEdges: []string{}, Plans: []PlanHill{},
		Flow: Flow{Landed: []Landing{}, Slowed: []Slowed{}}}
	rules, err := m.readRules()
	if err != nil {
		return v, err
	}
	err = m.view(func(s *state) error {
		v.Calls = s.calls(now)
		var reports []Report
		for _, p := range s.tx.List("reports/") {
			if r, ok := s.report(p); ok {
				reports = append(reports, r)
			}
		}
		sort.SliceStable(reports, func(a, b int) bool { return reports[a].At < reports[b].At })
		v.Rules = ruleTrends(rules, reports, since)
		for _, r := range rules {
			if r.Advisory {
				v.Advisory = append(v.Advisory, r.ID+": "+r.Title)
			}
		}
		if c := s.clusters(); c != nil {
			v.Clusters = c
		}
		v.Problems = m.problems(s, reports, now)
		for _, r := range reports {
			if r.At > since {
				v.NewEdges = append(v.NewEdges, r.NewEdges...)
			}
		}
		if v.NewEdges = dedupe(v.NewEdges); v.NewEdges == nil {
			v.NewEdges = []string{}
		}
		for _, slug := range sortedKeys(s.plans) {
			p := s.plans[slug]
			if p.State == PlanDone {
				continue
			}
			h := PlanHill{Slug: slug, Title: p.Title, State: p.State, Target: s.targetOf(slug), Preview: p.Preview,
				ToShip: m.ahead(s.release(), s.targetOf(slug)), Phases: []PhaseHill{}}
			for _, ph := range s.planPhases(slug) {
				h.Phases = append(h.Phases, s.hill(ph))
			}
			v.Plans = append(v.Plans, h)
		}
		v.Flow = s.flow(reports, since)
		if b, ok := s.tx.Get(auditPath); ok {
			text := strings.TrimSpace(string(b))
			a := &Audited{At: auditDate(text), Summary: auditSummary(text)}
			if t, ok := parseTime(a.At); ok && now.Sub(t) > 14*24*time.Hour {
				a.Stale = true
			}
			v.Audit = a
		}
		return nil
	})
	return v, err
}

// calls: reviews of high-risk work, then questions (lapsed last), then
// looks at landed work.
func (s *state) calls(now time.Time) []Call {
	var reviews, questions, looks []Call
	for _, id := range sortedKeys(s.entries) {
		e := s.entries[id]
		if e.Settled != nil {
			continue
		}
		c := Call{Entry: e.ID, Item: e.Item, Plan: e.Plan, Text: e.Text, Created: e.Created}
		switch e.Kind {
		case KindReview:
			if e.Item == "" {
				continue // an old review of a phase
			}
			if e.Landed != "" {
				c.Kind, c.Why = "look", e.Landed
				looks = append(looks, c)
				continue
			}
			if r := s.latestReport(e.Item); r != nil {
				c.Why = strings.Join(r.RiskWhy, "; ")
			}
			c.Kind = "review"
			reviews = append(reviews, c)
		case KindQuestion:
			c.Kind, c.Default, c.Due = "question", e.Default, e.Due
			if t, ok := parseTime(e.Due); ok && now.After(t) {
				c.Lapsed = true
			}
			questions = append(questions, c)
		}
	}
	sort.SliceStable(questions, func(a, b int) bool { return !questions[a].Lapsed && questions[b].Lapsed })
	return append(append(append([]Call{}, reviews...), questions...), looks...)
}

// problems: claims gone quiet, reviews waiting days, and finished work
// that isn't on its target.
func (m *Mull) problems(s *state, reports []Report, now time.Time) []Problem {
	out := []Problem{}
	trailers := map[string]map[string]bool{} // branch: items with a recent trailer on it
	onBranch := func(branch, item, commit string) bool {
		if trailers[branch] == nil {
			trailers[branch] = m.recentTrailers(branch)
		}
		return trailers[branch][item] || m.landed(branch, commit)
	}
	latest := map[string]Report{}
	for _, r := range reports {
		latest[r.Item] = r
	}
	for _, id := range sortedKeys(s.items) {
		i := s.items[id]
		r, reported := latest[id]
		switch i.State {
		case Active:
			if t, ok := parseTime(i.Claim.At); ok && now.Sub(t) > 24*time.Hour && !reported {
				out = append(out, Problem{id, i.Title, fmt.Sprintf("active for %s with nothing finished", ago(now, t))})
			}
		case InReview:
			if t, ok := parseTime(i.Updated); ok && now.Sub(t) > 3*24*time.Hour {
				out = append(out, Problem{id, i.Title, fmt.Sprintf("waiting for review for %s", ago(now, t))})
			}
		case Done:
			if !reported || r.Commit == "" || i.CoveredBy != "" {
				continue
			}
			if t, ok := parseTime(r.At); !ok || now.Sub(t) > 14*24*time.Hour {
				continue // old work: what shipped shipped
			}
			if target := s.targetOf(i.Plan); !onBranch(target, id, r.Commit) && !onBranch(s.release(), id, r.Commit) {
				out = append(out, Problem{id, i.Title, fmt.Sprintf("done, but not on %s: mull land %s", target, id)})
			}
		}
	}
	return out
}

// landed reports whether commit is on branch.
func (m *Mull) landed(branch, commit string) bool {
	tip, err := m.repo.RefCommit("refs/heads/" + branch)
	return err == nil && tip != "" && (tip == commit || m.isAncestor(commit, tip))
}

// recentTrailers are the items whose "Mull: <id>" trailer is on a commit
// of branch from the last month: their work landed, rebased or squashed
// on the way or not. One git call.
func (m *Mull) recentTrailers(branch string) map[string]bool {
	out := map[string]bool{}
	log, err := m.repo.Output(nil, nil, "log", "--since=30.days", "--format=%(trailers:key=Mull,valueonly)", "refs/heads/"+branch)
	if err != nil {
		return out
	}
	for _, l := range strings.Split(log, "\n") {
		if l = strings.TrimSpace(l); IsID(l) {
			out[l] = true
		}
	}
	return out
}

// ahead counts target's commits that release lacks.
func (m *Mull) ahead(release, target string) int {
	if release == target {
		return 0
	}
	out, err := m.repo.Output(nil, nil, "rev-list", "--count", "refs/heads/"+release+"..refs/heads/"+target)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(out)
	return n
}

func ago(now, then time.Time) string {
	d := now.Sub(then)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func ruleTrends(rules []Rule, reports []Report, since string) []RuleTrend {
	out := []RuleTrend{}
	for _, r := range rules {
		if r.Advisory {
			continue
		}
		t := RuleTrend{ID: r.ID, Title: r.Title}
		before, after := false, false
		for _, rep := range reports {
			for _, rc := range rep.RuleChecks {
				if rc.ID != r.ID {
					continue
				}
				if rep.At <= since {
					t.Then, before = rc.Count, true
				}
				t.Now, t.Failing, after = rc.Count, !rc.Passed, true
			}
		}
		switch {
		case !after:
			t.Trend = "unrun"
		case !before:
			t.Trend = "new"
		case t.Now > t.Then:
			t.Trend = "worse"
		case t.Now < t.Then:
			t.Trend = "better"
		default:
			t.Trend = "same"
		}
		out = append(out, t)
	}
	rank := func(t RuleTrend) int {
		switch {
		case t.Failing:
			return 0
		case t.Trend == "worse":
			return 1
		case t.Trend == "better":
			return 3
		}
		return 2
	}
	sort.SliceStable(out, func(a, b int) bool { return rank(out[a]) < rank(out[b]) })
	return out
}

func (s *state) flow(reports []Report, since string) Flow {
	f := Flow{Parallel: s.parallel(), Landed: []Landing{}, Slowed: []Slowed{}}
	for _, i := range s.items {
		switch {
		case i.State == Active:
			f.Active++
		case queued(i):
			f.Queue++
		}
		if i.Created > since && i.From != "" {
			f.Rework++
		}
	}
	var minutes []int
	for _, r := range reports {
		i := s.items[r.Item]
		if r.At <= since || !r.Passed || i == nil || (i.State != Done && i.State != InReview) {
			continue
		}
		f.Landed = append(f.Landed, Landing{Item: r.Item, Title: i.Title, Summary: r.Summary, Risk: r.Risk, Sampled: r.Sampled, At: r.At})
		if sl := strings.TrimSpace(r.Slowed); sl != "" && !noRules(sl) && !strings.EqualFold(sl, "nothing") {
			f.Slowed = append(f.Slowed, Slowed{Item: r.Item, Text: sl})
		}
		if i.Claim != nil {
			a, ok1 := parseTime(i.Claim.At)
			b, ok2 := parseTime(r.At)
			if ok1 && ok2 && b.After(a) {
				minutes = append(minutes, int(b.Sub(a).Minutes()))
			}
		}
	}
	if len(minutes) > 0 {
		sort.Ints(minutes)
		f.MedianMinutes = minutes[len(minutes)/2]
	}
	return f
}

func auditSummary(text string) string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l == "" || strings.HasPrefix(l, "Audited:") || strings.HasPrefix(l, "#") {
			continue
		}
		if out = append(out, l); len(out) == 3 {
			break
		}
	}
	return strings.Join(out, "\n")
}

// Text renders the surface for a terminal.
func (v Surface) Text() string {
	var b strings.Builder
	since := v.Since
	if t, ok := parseTime(v.Since); ok {
		since = t.Format("2006-01-02 15:04")
	}
	fmt.Fprintf(&b, "SINCE %s\n", since)
	if len(v.Calls) == 0 {
		b.WriteString("\nNothing needs you.\n")
	} else {
		fmt.Fprintf(&b, "\nNEEDS YOU (%d)\n", len(v.Calls))
		for _, c := range v.Calls {
			line := fmt.Sprintf("  %-8s %s  %s", c.Kind, c.Entry, c.Text)
			switch {
			case c.Kind == "question" && c.Default == "":
				line += "  (no default given)"
			case c.Kind == "question" && c.Lapsed:
				line += "  (default stands: " + c.Default + ")"
			case c.Kind == "question":
				line += "  (default: " + c.Default + ")"
			case c.Why != "":
				line += "  (" + c.Why + ")"
			}
			b.WriteString(line + "\n")
		}
	}
	var worse []string
	for _, r := range v.Rules {
		switch {
		case r.Failing:
			worse = append(worse, fmt.Sprintf("  %s failing: %s (%d known)", r.ID, r.Title, r.Now))
		case r.Trend == "worse":
			worse = append(worse, fmt.Sprintf("  %s worse: %s (%d to %d)", r.ID, r.Title, r.Then, r.Now))
		}
	}
	for _, c := range v.Clusters {
		worse = append(worse, fmt.Sprintf("  %d queued in %s: the design there may be wrong", c.Count, c.Area))
	}
	for _, e := range v.NewEdges {
		worse = append(worse, "  new connection "+e)
	}
	for _, p := range v.Problems {
		worse = append(worse, fmt.Sprintf("  %s %s: %s", p.Item, p.Title, p.Text))
	}
	if len(worse) > 0 {
		b.WriteString("\nTRENDING WORSE\n" + strings.Join(worse, "\n") + "\n")
	}
	if len(v.Advisory) > 0 {
		fmt.Fprintf(&b, "\n%d rules have no check: %s\n", len(v.Advisory), strings.Join(v.Advisory, "; "))
	}
	if len(v.Plans) > 0 {
		b.WriteString("\nPLANS\n")
		for _, p := range v.Plans {
			fmt.Fprintf(&b, "  %s (%s) on %s", p.Title, p.State, p.Target)
			if p.ToShip > 0 {
				fmt.Fprintf(&b, ", %d commits to ship", p.ToShip)
			}
			b.WriteString("\n")
			for _, h := range p.Phases {
				if h.Hill != "done" {
					fmt.Fprintf(&b, "    %-8s %s  %s\n", h.Hill, h.Title, h.Progress)
				}
			}
		}
	}
	if a := v.Audit; a != nil {
		stale := ""
		if a.Stale {
			stale = ", over two weeks old"
		}
		fmt.Fprintf(&b, "\nAUDIT %.10s%s\n  %s\n", a.At, stale, strings.ReplaceAll(a.Summary, "\n", "\n  "))
	}
	f := v.Flow
	fmt.Fprintf(&b, "\nFLOW  %d landed, %d active of %d, %d queued", len(f.Landed), f.Active, f.Parallel, f.Queue)
	if f.MedianMinutes > 0 {
		fmt.Fprintf(&b, ", median %dm start to done", f.MedianMinutes)
	}
	if f.Rework > 0 {
		fmt.Fprintf(&b, ", %d came back", f.Rework)
	}
	b.WriteString("\n")
	for _, sl := range f.Slowed {
		fmt.Fprintf(&b, "  slowed %s: %s\n", sl.Item, sl.Text)
	}
	return strings.TrimRight(b.String(), "\n")
}

// Summary is a project's line in mull serve's project list.
type Summary struct {
	Calls  int        `json:"calls"`
	Active int        `json:"active"`
	Queue  int        `json:"queue"`
	Plans  []PlanLine `json:"plans"`
}

// PlanLine is a plan's name and state.
type PlanLine struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	State string `json:"state"`
}

// Summarize counts what needs the human, what's active and what's queued,
// and lists the open plans.
func (m *Mull) Summarize() (Summary, error) {
	out := Summary{Plans: []PlanLine{}}
	err := m.view(func(s *state) error {
		out.Calls = len(s.calls(m.Now().UTC()))
		for _, i := range s.items {
			switch {
			case i.State == Active:
				out.Active++
			case queued(i):
				out.Queue++
			}
		}
		for _, slug := range sortedKeys(s.plans) {
			if p := s.plans[slug]; p.State != PlanDone {
				out.Plans = append(out.Plans, PlanLine{slug, p.Title, p.State})
			}
		}
		return nil
	})
	return out, err
}
