package mull

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mull/internal/model"
)

// Thresholds for problems.
const (
	idleClaimAfter   = 24 * time.Hour
	notInGitAfter    = 3 * 24 * time.Hour
	recentActivity   = 7 * 24 * time.Hour
	olderInboxAfter  = 7 * 24 * time.Hour
	unlinkedListSize = 5
)

// View is the whole-repository board.
type View struct {
	Generated        string      `json:"generated"`
	Integration      string      `json:"integration"`
	IntegrationFound bool        `json:"integration_found"`
	Release          string      `json:"release"`
	AheadOfRelease   int         `json:"ahead_of_release"`
	Plans            []PlanBoard `json:"plans"`
	Unplanned        *PlanBoard  `json:"unplanned,omitempty"` // items with no plan, and what needs you about them
	UnlinkedCount    int         `json:"unlinked_commits_this_week"`
	UnlinkedRecent   []string    `json:"unlinked_recent,omitempty"`
	Legacy           *Legacy     `json:"legacy,omitempty"`
}

// Legacy counts open entries of the retired kinds, kept until the Model v2
// migration converts them to captured items. They are not in needs_you.
type Legacy struct {
	LooseEnds int `json:"loose_ends"`
	Ideas     int `json:"ideas"`
}

// PlanBoard is one plan's block in the view.
type PlanBoard struct {
	Slug     string       `json:"slug"`
	Title    string       `json:"title"`
	State    string       `json:"state,omitempty"` // the plan's state; none for the unplanned board
	Progress Progress     `json:"progress"`
	Phases   []PhaseLine  `json:"phases"`
	Unfiled  Progress     `json:"unfiled"`
	Problems []Problem    `json:"problems"`
	InFlight []FlightLine `json:"in_flight"`
	NeedsYou NeedsYou     `json:"needs_you"`
}

// PhaseLine is a phase's progress line.
type PhaseLine struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Progress  Progress `json:"progress"`
	Complete  bool     `json:"complete"`
	BlockedBy []string `json:"blocked_by,omitempty"`
}

// FlightLine is an item being worked on.
type FlightLine struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	State        string   `json:"state"`
	Worktree     string   `json:"worktree,omitempty"`
	Branches     []string `json:"branches,omitempty"`
	Commits      int      `json:"commits"`
	LastActivity string   `json:"last_activity,omitempty"`
}

// Problem is something wrong the human should see first.
type Problem struct {
	Kind string `json:"kind"` // idle-claim, done-not-in-git, unclaimed-work
	Item string `json:"item"`
	Text string `json:"text"`
}

// NeedsYou counts the plan's open inbox: questions, decisions and reviews.
type NeedsYou struct {
	Reviews   int `json:"reviews"`
	Decisions int `json:"decisions"`
	Questions int `json:"questions"`
	Older     int `json:"older_than_a_week"`
	Parked    int `json:"parked"`
}

func (n NeedsYou) total() int { return n.Reviews + n.Decisions + n.Questions }

// Board computes the view, for one plan or every plan not done: active
// plans first, then shaping ones.
func (m *Mull) Board(plan string) (View, error) {
	var v View
	err := m.view(func(s *state) error {
		if plan != "" {
			if _, err := s.plan(plan); err != nil {
				return err
			}
		}
		g, err := m.readGit(s)
		if err != nil {
			return err
		}
		now := m.Now().UTC()
		v = View{
			Generated: now.Format(time.RFC3339), Integration: g.Integration, IntegrationFound: g.IntegrationFound,
			Release: s.config.Release, AheadOfRelease: g.AheadOfRelease,
			UnlinkedCount: len(g.Unlinked), Plans: []PlanBoard{},
		}
		if n := len(g.Unlinked); n > 0 {
			v.UnlinkedRecent = g.Unlinked[:min(n, unlinkedListSize)]
		}
		var legacy Legacy
		for _, e := range s.entries {
			if e.Settled != nil {
				continue
			}
			switch e.Kind {
			case model.KindLoose:
				legacy.LooseEnds++
			case model.KindIdea:
				legacy.Ideas++
			}
		}
		if legacy != (Legacy{}) {
			v.Legacy = &legacy
		}
		var shaping []PlanBoard
		for _, slug := range sortedKeys(s.plans) {
			p := s.plans[slug]
			if plan != "" && slug != plan {
				continue
			}
			if plan == "" && p.State == model.PlanDone {
				continue
			}
			if b := s.board(p, g, now); p.State == model.PlanShaping {
				shaping = append(shaping, b)
			} else {
				v.Plans = append(v.Plans, b)
			}
		}
		v.Plans = append(v.Plans, shaping...)
		if plan == "" {
			b := s.board(&model.Plan{Title: "Unplanned"}, g, now)
			if b.Progress.Total+b.Progress.Captured > 0 || b.NeedsYou.total() > 0 {
				v.Unplanned = &b
			}
		}
		return nil
	})
	return v, err
}

func parseTime(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	return t, err == nil
}

func (s *state) board(p *model.Plan, g *gitState, now time.Time) PlanBoard {
	b := PlanBoard{Slug: p.Slug, Title: p.Title, State: p.State, Phases: []PhaseLine{}, Problems: []Problem{}, InFlight: []FlightLine{}}
	var items, unfiled []*model.Item
	for _, id := range sortedKeys(s.items) {
		if i := s.items[id]; i.Plan == p.Slug {
			items = append(items, i)
			if i.Phase == "" {
				unfiled = append(unfiled, i)
			}
		}
	}
	b.Progress = s.progress(items)
	b.Unfiled = s.progress(unfiled)

	var phases []*model.Phase
	for _, id := range sortedKeys(s.phases) {
		if e := s.phases[id]; e.Plan == p.Slug {
			phases = append(phases, e)
		}
	}
	// Ready and in-progress phases first, then blocked, then complete; by age within.
	rank := func(e *model.Phase) int {
		switch {
		case s.phaseComplete(e.ID):
			return 2
		case len(s.blockedBy(e.Needs)) > 0:
			return 1
		}
		return 0
	}
	sort.SliceStable(phases, func(a, c int) bool {
		if ra, rc := rank(phases[a]), rank(phases[c]); ra != rc {
			return ra < rc
		}
		return phases[a].Created < phases[c].Created
	})
	for _, e := range phases {
		ev := s.phaseView(e)
		b.Phases = append(b.Phases, PhaseLine{ID: e.ID, Title: e.Title, Progress: ev.Progress, Complete: ev.Complete, BlockedBy: ev.BlockedBy})
	}

	for _, i := range items {
		ig := g.Items[i.ID]
		if ig == nil {
			ig = &ItemGit{}
		}
		last, hasLast := parseTime(ig.LastActivity)
		recent := hasLast && now.Sub(last) <= recentActivity
		switch i.State {
		case model.StateActive:
			f := FlightLine{ID: i.ID, Title: i.Title, State: i.State, Branches: ig.Branches,
				Commits: ig.Unmerged + ig.Landed, LastActivity: ig.LastActivity}
			if i.Claim != nil {
				f.Worktree = filepath.Base(i.Claim.Worktree)
				if at, ok := parseTime(i.Claim.At); ok && now.Sub(at) > idleClaimAfter &&
					(!hasLast || now.Sub(last) > idleClaimAfter) {
					b.Problems = append(b.Problems, Problem{Kind: "idle-claim", Item: i.ID,
						Text: fmt.Sprintf("claimed %s ago by %s, no linked commits for a day", ago(now, at), f.Worktree)})
				}
			}
			if rep := s.latestReport(i.ID); rep != nil && !rep.Passed && rep.Override == "" {
				b.Problems = append(b.Problems, Problem{Kind: "failed-done", Item: i.ID,
					Text: fmt.Sprintf("done attempt %d failed its checks (%s)", rep.N, model.ReportPath(i.ID, rep.N))})
			}
			b.InFlight = append(b.InFlight, f)
		case model.StateCaptured, model.StateCommitted:
			if ig.Unmerged > 0 && recent {
				b.InFlight = append(b.InFlight, FlightLine{ID: i.ID, Title: i.Title, State: i.State,
					Branches: ig.Branches, Commits: ig.Unmerged, LastActivity: ig.LastActivity})
				b.Problems = append(b.Problems, Problem{Kind: "unclaimed-work", Item: i.ID,
					Text: "has recent commits but nobody has claimed it"})
			}
		case model.StateInReview, model.StateDone:
			if !ig.SeenInGit {
				if up, ok := parseTime(i.Updated); ok && now.Sub(up) > notInGitAfter {
					b.Problems = append(b.Problems, Problem{Kind: "done-not-in-git", Item: i.ID,
						Text: fmt.Sprintf("%s %s ago, but none of its work is on %s", i.State, ago(now, up), g.Integration)})
				}
			}
		}
	}

	for _, id := range sortedKeys(s.entries) {
		e := s.entries[id]
		if e.Plan != p.Slug {
			continue
		}
		if retiredKind(e.Kind) {
			continue // counted in the view's legacy
		}
		if e.Settled != nil {
			if e.Settled.Action == model.ActPark {
				b.NeedsYou.Parked++
			}
			continue
		}
		switch e.Kind {
		case model.KindReview:
			b.NeedsYou.Reviews++
		case model.KindDecision:
			b.NeedsYou.Decisions++
		case model.KindQuestion:
			b.NeedsYou.Questions++
		}
		if c, ok := parseTime(e.Created); ok && now.Sub(c) > olderInboxAfter {
			b.NeedsYou.Older++
		}
	}
	return b
}

// latestReport returns the item's highest-numbered done report, if any.
func (s *state) latestReport(id string) *model.Report {
	var best *model.Report
	for _, p := range s.tx.List("reports/" + id + "-") {
		var r model.Report
		b, _ := s.tx.Get(p)
		if _, err := model.Decode(b, &r); err != nil {
			continue
		}
		if best == nil || r.N > best.N {
			rr := r
			best = &rr
		}
	}
	return best
}

// ago renders a duration compactly: 5m, 3h, 2d, 3w.
func ago(now, then time.Time) string {
	d := now.Sub(then)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 21*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	}
}

func bar(p Progress) string {
	if p.Total == 0 {
		return "·"
	}
	width := min(p.Total, 10)
	filled := (p.Done + p.Dropped) * width / p.Total
	return strings.Repeat("▰", filled) + strings.Repeat("▱", width-filled)
}

func progressText(p Progress) string {
	if p.Total == 0 && p.Captured == 0 {
		return "no items"
	}
	parts := []string{fmt.Sprintf("%d/%d done", p.Done, p.Total-p.Dropped)}
	if p.InReview > 0 {
		parts = append(parts, fmt.Sprintf("%d in review", p.InReview))
	}
	if p.Active > 0 {
		parts = append(parts, fmt.Sprintf("%d active", p.Active))
	}
	if p.Ready > 0 {
		parts = append(parts, fmt.Sprintf("%d ready", p.Ready))
	}
	if p.Captured > 0 {
		parts = append(parts, fmt.Sprintf("%d captured", p.Captured))
	}
	return strings.Join(parts, " · ")
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// Text renders the view for a terminal.
func (v View) Text(now time.Time) string {
	var b strings.Builder
	header := "integration " + v.Integration
	if !v.IntegrationFound {
		header += " (branch not found)"
	} else if v.AheadOfRelease > 0 {
		header += fmt.Sprintf(" · %d ahead of %s", v.AheadOfRelease, v.Release)
	}
	fmt.Fprintf(&b, "%s · %s\n", header, now.Format("2006-01-02"))
	if len(v.Plans) == 0 && v.Unplanned == nil {
		b.WriteString("\nNo open plans with work.\n")
	}
	for _, p := range v.Plans {
		state := ""
		switch p.State {
		case model.PlanShaping:
			state = "  (shaping: not agreed yet)"
		case model.PlanDone:
			state = "  (done)"
		}
		fmt.Fprintf(&b, "\n%s  %s%s\n", strings.ToUpper(p.Slug), p.Title, state)
		writeBoard(&b, p, now)
	}
	if v.Unplanned != nil {
		b.WriteString("\nUNPLANNED\n")
		writeBoard(&b, *v.Unplanned, now)
	}
	if l := v.Legacy; l != nil {
		var parts []string
		if l.LooseEnds > 0 {
			parts = append(parts, plural(l.LooseEnds, "loose end")+" (mull inbox --kind loose-end)")
		}
		if l.Ideas > 0 {
			parts = append(parts, plural(l.Ideas, "inbox idea")+" (mull inbox --kind idea)")
		}
		fmt.Fprintf(&b, "\nLEGACY  %s, kept until mull migrate turns them into captured items\n", strings.Join(parts, " · "))
	}
	if v.UnlinkedCount > 0 {
		verb := "carry"
		if v.UnlinkedCount == 1 {
			verb = "carries"
		}
		fmt.Fprintf(&b, "\nUNLINKED  %s on %s this week %s no item link", plural(v.UnlinkedCount, "commit"), v.Integration, verb)
		if len(v.UnlinkedRecent) > 0 {
			b.WriteString(", latest: " + strings.Join(v.UnlinkedRecent, "; "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// writeBoard renders one plan's (or the unplanned work's) block.
func writeBoard(b *strings.Builder, p PlanBoard, now time.Time) {
	fmt.Fprintf(b, "  %s %s\n", bar(p.Progress), progressText(p.Progress))
	if len(p.Phases) > 0 || p.Unfiled.Total > 0 {
		b.WriteString("  PHASES\n")
		for _, e := range p.Phases {
			status := progressText(e.Progress)
			if e.Complete {
				status = "complete"
			} else if len(e.BlockedBy) > 0 {
				status += " · blocked by " + strings.Join(e.BlockedBy, ", ")
			}
			fmt.Fprintf(b, "    %s  %-28s %-10s %s\n", e.ID, truncate(e.Title, 28), bar(e.Progress), status)
		}
		if p.Unfiled.Total > 0 {
			fmt.Fprintf(b, "    ----  %-28s %-10s %s\n", "(no phase)", bar(p.Unfiled), progressText(p.Unfiled))
		}
	}
	if len(p.Problems) > 0 {
		fmt.Fprintf(b, "  PROBLEMS (%d)\n", len(p.Problems))
		for _, pr := range p.Problems {
			fmt.Fprintf(b, "    %s  %s\n", pr.Item, pr.Text)
		}
	}
	if len(p.InFlight) > 0 {
		b.WriteString("  IN FLIGHT\n")
		for _, f := range p.InFlight {
			var bits []string
			if f.Worktree != "" {
				bits = append(bits, f.Worktree)
			}
			if len(f.Branches) > 0 {
				bits = append(bits, strings.Join(f.Branches, ","))
			}
			bits = append(bits, plural(f.Commits, "commit"))
			if t, ok := parseTime(f.LastActivity); ok {
				bits = append(bits, "last "+ago(now, t))
			}
			fmt.Fprintf(b, "    %s  %-36s %s\n", f.ID, truncate(f.Title, 36), strings.Join(bits, " · "))
		}
	}
	n := p.NeedsYou
	if n.total() > 0 {
		var parts []string
		for _, c := range []struct {
			n    int
			noun string
		}{{n.Reviews, "review"}, {n.Decisions, "decision"}, {n.Questions, "question"}} {
			if c.n > 0 {
				parts = append(parts, plural(c.n, c.noun))
			}
		}
		line := strings.Join(parts, " · ")
		if n.Older > 0 {
			line += fmt.Sprintf(" (%d older than a week)", n.Older)
		}
		fmt.Fprintf(b, "  NEEDS YOU  %s\n", line)
	} else {
		b.WriteString("  NEEDS YOU  nothing\n")
	}
}
