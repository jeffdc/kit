package mull

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"mull/internal/model"
)

// The surface is the human's one view: what changed since they last looked,
// in the order they act on it. Calls that need them come first, each with
// what happens if they don't answer; then what's trending worse (rules,
// clusters, problems); then where each plan stands; then the rest, folded.
// Every number comes from mull's records and git, never from what an agent
// wrote about its work.

// Surface is the human's view of one project since a time.
type Surface struct {
	Since     string `json:"since"`
	Generated string `json:"generated"`
	// Calls need the human: high-risk work waiting for review, questions
	// (with the agent's default and when it stands), and landed work to
	// look at (medium risk, or sampled).
	Calls []Call `json:"calls"`
	// Trends are what's getting worse or better: each checked rule's
	// violations then and now, the rules with no check, where queued work
	// clusters, and the problems with work.
	Rules    []RuleTrend  `json:"rules"`
	Advisory []string     `json:"advisory"` // rules with no check: hopes, not guards
	Clusters []Cluster    `json:"clusters"`
	Problems []ProblemRow `json:"problems"`
	// Plans are where each plan stands, phase by phase.
	Plans []PlanHill `json:"plans"`
	// Flow is how the work moved since: what landed, what slowed it, what
	// came back.
	Flow Flow `json:"flow"`
	// NewEdges are connections between parts that work added since; Audit
	// is the latest audit, and Stale says it's over two weeks old.
	NewEdges []string `json:"new_edges"`
	Audit    *Audited `json:"audit,omitempty"`
}

// Audited is the latest audit, as the surface shows it: when, and its
// first lines.
type Audited struct {
	At      string `json:"at"`
	Summary string `json:"summary"`
	Stale   bool   `json:"stale,omitempty"`
}

// Call is one thing the human acts on.
type Call struct {
	Kind    string `json:"kind"` // review (high risk, waiting), question, look (landed: medium or sampled)
	Entry   string `json:"entry"`
	Item    string `json:"item,omitempty"`
	Plan    string `json:"plan,omitempty"`
	Text    string `json:"text"`
	Why     string `json:"why,omitempty"`     // a review's risk reasons; a look's band
	Default string `json:"default,omitempty"` // a question's: what happens if nobody answers
	Due     string `json:"due,omitempty"`
	Lapsed  bool   `json:"lapsed,omitempty"` // its due time passed: the default stands
	Created string `json:"created"`
}

// RuleTrend is one checked rule's violations at the last look and now, as
// the latest done reports counted them.
type RuleTrend struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Then    int    `json:"then"`
	Now     int    `json:"now"`
	Failing bool   `json:"failing,omitempty"` // its latest run failed
	Trend   string `json:"trend"`             // worse, better, same, or new (no run before the last look)
}

// PlanHill is a plan's phases as a hill: uphill is still being worked out
// (nothing cut, or nothing started), downhill is executing.
type PlanHill struct {
	Slug           string      `json:"slug"`
	Title          string      `json:"title"`
	Target         string      `json:"target"`
	AheadOfRelease int         `json:"ahead_of_release"` // commits waiting to ship
	Phases         []PhaseHill `json:"phases"`
}

// PhaseHill is one phase's place on the hill.
type PhaseHill struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Hill     string   `json:"hill"` // uphill, downhill, done, waiting (blocked)
	Progress Progress `json:"progress"`
}

// Flow is how work moved since the last look.
type Flow struct {
	Active   int       `json:"active"`
	Parallel int       `json:"parallel"`
	Landed   []Landing `json:"landed"`
	Slowed   []Slowed  `json:"slowed"`
	// Rework counts work that came back: follow-ups and fixes filed since.
	Rework int `json:"rework"`
	// MedianMinutes is the median time from start to done of what landed.
	MedianMinutes int `json:"median_minutes,omitempty"`
}

// Landing is a finished item since the last look.
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

// MainWorktree is the repository's main working tree: what the registry
// and the human's last look are keyed by.
func (m *Mull) MainWorktree() string { return m.repo.MainWorktree() }

// Surface reads the human's view since a time (RFC 3339; empty is a week
// ago).
func (m *Mull) Surface(since string) (Surface, error) {
	now := m.Now().UTC()
	if since == "" {
		since = now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	}
	out := Surface{Since: since, Generated: now.Format(time.RFC3339), Calls: []Call{}, Rules: []RuleTrend{},
		Advisory: []string{}, Clusters: []Cluster{}, Problems: []ProblemRow{}, Plans: []PlanHill{},
		Flow: Flow{Landed: []Landing{}, Slowed: []Slowed{}}, NewEdges: []string{}}
	rules, err := m.readRules()
	if err != nil {
		return out, err
	}
	err = m.view(func(s *state) error {
		out.Calls = s.calls(now)
		reports := s.allReports()
		out.Rules = ruleTrends(rules, reports, since)
		for _, r := range rules {
			if r.Advisory {
				out.Advisory = append(out.Advisory, r.ID+": "+r.Title)
			}
		}
		out.Clusters = s.clusters()
		if out.Clusters == nil {
			out.Clusters = []Cluster{}
		}
		g, err := m.readGit(s)
		if err != nil {
			return err
		}
		var slugs []string
		for _, slug := range sortedKeys(s.plans) {
			p := s.plans[slug]
			if p.State == model.PlanDone {
				continue
			}
			slugs = append(slugs, slug)
			t := s.target(slug, g.Targets)
			ph := PlanHill{Slug: slug, Title: p.Title, Target: t.Branch, AheadOfRelease: t.AheadOfRelease, Phases: []PhaseHill{}}
			for _, e := range s.planPhases(slug) {
				ph.Phases = append(ph.Phases, s.phaseHill(e))
			}
			out.Plans = append(out.Plans, ph)
		}
		if rows := s.problemRows(g, now, append(slugs, "")...); rows != nil {
			out.Problems = rows
		}
		out.Flow = s.flow(reports, since)
		for _, r := range reports {
			if r.At > since {
				out.NewEdges = append(out.NewEdges, r.NewEdges...)
			}
		}
		out.NewEdges = dedupe(out.NewEdges)
		if b, ok := s.tx.Get(model.AuditPath); ok {
			text := strings.TrimSpace(string(b))
			a := &Audited{At: auditDate(text), Summary: auditSummary(text)}
			if t, ok := parseTime(a.At); ok && now.Sub(t) > 14*24*time.Hour {
				a.Stale = true
			}
			out.Audit = a
		}
		return nil
	})
	return out, err
}

// calls lists what needs the human: reviews of high-risk work first, then
// questions (lapsed last: their defaults stand), then looks at landed work.
func (s *state) calls(now time.Time) []Call {
	var reviews, questions, looks []Call
	for _, id := range sortedKeys(s.entries) {
		e := s.entries[id]
		if e.Settled != nil {
			continue
		}
		c := Call{Entry: e.ID, Item: e.Item, Plan: e.Plan, Text: e.Text, Created: e.Created}
		switch e.Kind {
		case model.KindReview:
			if e.Phase != "" {
				continue // a retired integration review
			}
			if rep := s.latestReport(e.Item); rep != nil {
				c.Why = strings.Join(rep.RiskWhy, "; ")
			}
			if e.Landed != "" {
				c.Kind, c.Why = "look", e.Landed
				looks = append(looks, c)
				continue
			}
			c.Kind = "review"
			reviews = append(reviews, c)
		case model.KindQuestion:
			c.Kind, c.Default, c.Due = "question", e.Default, e.Due
			if t, ok := parseTime(e.Due); ok && now.After(t) {
				c.Lapsed = true
			}
			questions = append(questions, c)
		}
	}
	sort.SliceStable(questions, func(a, b int) bool { return !questions[a].Lapsed && questions[b].Lapsed })
	return append(append(reviews, questions...), looks...)
}

// allReports decodes every done report, oldest first.
func (s *state) allReports() []model.Report {
	var out []model.Report
	for _, p := range s.tx.List("reports/") {
		b, ok := s.tx.Get(p)
		if !ok {
			continue
		}
		var r model.Report
		if _, err := model.Decode(b, &r); err == nil {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].At < out[b].At })
	return out
}

// ruleTrends compares each checked rule's count at the last look (the
// latest run before since) with its latest run.
func ruleTrends(rules []Rule, reports []model.Report, since string) []RuleTrend {
	out := []RuleTrend{}
	for _, r := range rules {
		if r.Advisory {
			continue
		}
		t := RuleTrend{ID: r.ID, Title: r.Title, Trend: "new"}
		seenBefore, seenAfter := false, false
		for _, rep := range reports {
			for _, rc := range rep.RuleChecks {
				if rc.ID != r.ID {
					continue
				}
				if rep.At <= since {
					t.Then, seenBefore = rc.Count, true
				}
				t.Now, t.Failing, seenAfter = rc.Count, !rc.Passed, true
			}
		}
		switch {
		case !seenAfter:
			t.Trend = "unrun"
		case !seenBefore:
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
	sort.SliceStable(out, func(a, b int) bool { return trendRank(out[a]) < trendRank(out[b]) })
	return out
}

func trendRank(t RuleTrend) int {
	switch {
	case t.Failing:
		return 0
	case t.Trend == "worse":
		return 1
	case t.Trend == "new" || t.Trend == "unrun":
		return 2
	case t.Trend == "better":
		return 3
	}
	return 4
}

// phaseHill places a phase on the hill.
func (s *state) phaseHill(e *model.Phase) PhaseHill {
	v := s.phaseView(e)
	h := PhaseHill{ID: e.ID, Title: e.Title, Progress: v.Progress}
	switch {
	case v.Complete:
		h.Hill = "done"
	case len(v.BlockedBy) > 0:
		h.Hill = "waiting"
	case v.Progress.Done+v.Progress.Active+v.Progress.InReview == 0:
		h.Hill = "uphill"
	default:
		h.Hill = "downhill"
	}
	return h
}

// flow is how the work moved since.
func (s *state) flow(reports []model.Report, since string) Flow {
	f := Flow{Parallel: s.parallel(), Landed: []Landing{}, Slowed: []Slowed{}}
	for _, i := range s.items {
		if i.State == model.StateActive {
			f.Active++
		}
		if i.Created > since && (i.From != "" && s.items[i.From] != nil) {
			f.Rework++ // a follow-up or fix of done work
		}
	}
	var minutes []int
	for _, r := range reports {
		if r.At <= since || r.Item == "" || !r.Passed {
			continue
		}
		i := s.items[r.Item]
		if i == nil || (i.State != model.StateDone && i.State != model.StateInReview) {
			continue
		}
		f.Landed = append(f.Landed, Landing{Item: r.Item, Title: i.Title, Summary: r.Summary, Risk: r.Risk, Sampled: r.Sampled, At: r.At})
		if r.Slowed != "" && !noRules(r.Slowed) && !strings.EqualFold(strings.TrimSpace(r.Slowed), "nothing") {
			f.Slowed = append(f.Slowed, Slowed{Item: r.Item, Text: r.Slowed})
		}
		if i.Claim != nil {
			if a, ok := parseTime(i.Claim.At); ok {
				if b, ok := parseTime(r.At); ok && b.After(a) {
					minutes = append(minutes, int(b.Sub(a).Minutes()))
				}
			}
		}
	}
	if len(minutes) > 0 {
		sort.Ints(minutes)
		f.MedianMinutes = minutes[len(minutes)/2]
	}
	return f
}

// Text renders the surface for a terminal: short, in the order it's acted
// on, with nothing that's fine.
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
			case c.Kind == "question" && c.Lapsed:
				line += fmt.Sprintf("  (default stands: %s)", c.Default)
			case c.Kind == "question" && c.Default == "":
				line += "  (no default given)"
			case c.Kind == "question":
				line += fmt.Sprintf("  (default: %s)", c.Default)
			case c.Why != "":
				line += "  (" + c.Why + ")"
			}
			b.WriteString(line + "\n")
		}
	}
	var trend []string
	for _, r := range v.Rules {
		switch {
		case r.Failing:
			trend = append(trend, fmt.Sprintf("  %s failing: %s (%d known)", r.ID, r.Title, r.Now))
		case r.Trend == "worse":
			trend = append(trend, fmt.Sprintf("  %s worse: %s (%d to %d)", r.ID, r.Title, r.Then, r.Now))
		}
	}
	for _, c := range v.Clusters {
		trend = append(trend, fmt.Sprintf("  %d queued in %s: the design there may be wrong", c.Count, c.Area))
	}
	for _, p := range v.Problems {
		trend = append(trend, fmt.Sprintf("  %s %s: %s", p.Item, p.Title, p.Text))
	}
	if len(trend) > 0 {
		b.WriteString("\nTRENDING WORSE\n" + strings.Join(trend, "\n") + "\n")
	}
	for _, e := range v.NewEdges {
		trend = append(trend, "  new connection "+e)
	}
	if len(v.Advisory) > 0 {
		fmt.Fprintf(&b, "\n%d rules have no check: %s\n", len(v.Advisory), strings.Join(v.Advisory, "; "))
	}
	if len(v.Plans) > 0 {
		b.WriteString("\nPLANS\n")
		for _, p := range v.Plans {
			fmt.Fprintf(&b, "  %s  on %s", p.Title, p.Target)
			if p.AheadOfRelease > 0 {
				fmt.Fprintf(&b, ", %d commits to ship", p.AheadOfRelease)
			}
			b.WriteString("\n")
			for _, h := range p.Phases {
				if h.Hill == "done" {
					continue
				}
				fmt.Fprintf(&b, "    %-8s %s  %s\n", h.Hill, h.Title, progressText(h.Progress))
			}
		}
	}
	if a := v.Audit; a != nil {
		at := a.At
		if t, ok := parseTime(a.At); ok {
			at = t.Format("2006-01-02")
		}
		stale := ""
		if a.Stale {
			stale = ", over two weeks old"
		}
		fmt.Fprintf(&b, "\nAUDIT %s%s\n  %s\n", at, stale, strings.ReplaceAll(a.Summary, "\n", "\n  "))
	}
	f := v.Flow
	fmt.Fprintf(&b, "\nFLOW  %d landed, %d active of %d", len(f.Landed), f.Active, f.Parallel)
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

// auditSummary is an audit's first few lines of findings, after its date.
func auditSummary(text string) string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "Audited:") || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, l)
		if len(out) == 3 {
			break
		}
	}
	return strings.Join(out, "\n")
}
