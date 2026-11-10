package mull

import (
	"sort"

	"mull/internal/model"
)

// What "now" and "next" mean is derived from phases and their needs only:
//
//   - a phase is done once it is complete (every committed item done or
//     dropped, and at least one);
//   - otherwise it can start now when every phase it needs is done (and its
//     plan isn't shaping), and it waits otherwise;
//   - next are the waiting phases one step away: some phase they need can
//     start now;
//   - go live is the phase a plan ends with: the one phase deepest in its
//     needs, when that is deeper than the first step. A plan whose phases
//     need nothing, or that ends in several phases at once, has none.
const (
	PhaseDone  = "done"
	PhaseNow   = "now"
	PhaseWaits = "waits"
)

func (s *state) phaseState(e *model.Phase) string {
	switch {
	case s.phaseComplete(e.ID):
		return PhaseDone
	case len(s.blockedBy(e.Needs)) > 0:
		return PhaseWaits
	case s.plans[e.Plan] != nil && s.plans[e.Plan].State == model.PlanShaping:
		return PhaseWaits
	}
	return PhaseNow
}

// depth is how many steps of needs lie below a phase: 0 for one that needs
// nothing. With unfinished, only needs not yet done count, so depth 0 is
// exactly what can start now. Needs never form a cycle (checkCycles).
func (s *state) depth(id string, unfinished bool, memo map[string]int) int {
	if d, ok := memo[id]; ok {
		return d
	}
	memo[id] = 0
	d := 0
	if e := s.phases[id]; e != nil {
		for _, n := range dedupe(e.Needs) {
			if s.phases[n] == nil || (unfinished && s.satisfied(n)) {
				continue
			}
			d = max(d, 1+s.depth(n, unfinished, memo))
		}
	}
	memo[id] = d
	return d
}

// planPhases returns a plan's phases, oldest first.
func (s *state) planPhases(slug string) []*model.Phase {
	var out []*model.Phase
	for _, id := range sortedKeys(s.phases) {
		if e := s.phases[id]; e.Plan == slug {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Created < out[b].Created })
	return out
}

// goLive returns the phase the plan ends with, or nil (see above).
func (s *state) goLive(slug string) *model.Phase {
	memo := map[string]int{}
	var best *model.Phase
	deepest, ties := 0, 0
	for _, e := range s.planPhases(slug) {
		switch d := s.depth(e.ID, false, memo); {
		case d > deepest:
			best, deepest, ties = e, d, 1
		case d == deepest:
			ties++
		}
	}
	if deepest == 0 || ties > 1 {
		return nil
	}
	return best
}

// PhaseRef names a phase, with its state.
type PhaseRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	State string `json:"state"` // done, now or waits
}

func (s *state) phaseRef(id string) PhaseRef {
	if e := s.phases[id]; e != nil {
		return PhaseRef{ID: id, Title: e.Title, State: s.phaseState(e)}
	}
	if i := s.items[id]; i != nil { // a need on an item, from before Model v2
		st := PhaseWaits
		if finished(i) {
			st = PhaseDone
		}
		return PhaseRef{ID: id, Title: i.Title, State: st}
	}
	return PhaseRef{ID: id, State: PhaseDone} // a missing need can't block
}

func (s *state) phaseRefs(ids []string) []PhaseRef {
	out := []PhaseRef{}
	for _, id := range ids {
		out = append(out, s.phaseRef(id))
	}
	return out
}

// PhaseRow is a phase in a plan's schedule.
type PhaseRow struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Summary   string     `json:"summary,omitempty"`
	State     string     `json:"state"` // done, now or waits
	GoLive    bool       `json:"go_live,omitempty"`
	Progress  Progress   `json:"progress"`
	Questions int        `json:"questions"` // open questions on its items
	Blocking  int        `json:"blocking"`  // of them, blocking work
	Needs     []string   `json:"needs"`
	WaitsFor  []PhaseRef `json:"waits_for"` // needs not done yet
}

func (s *state) phaseRow(e *model.Phase, goLive *model.Phase) PhaseRow {
	r := PhaseRow{ID: e.ID, Title: e.Title, Summary: e.Summary, State: s.phaseState(e),
		GoLive: goLive != nil && goLive.ID == e.ID, Progress: s.progress(s.phaseItems(e.ID)),
		Needs: append([]string{}, e.Needs...), WaitsFor: s.phaseRefs(s.blockedBy(dedupe(e.Needs)))}
	for _, x := range s.entries {
		if x.Settled == nil && x.Kind == model.KindQuestion && s.entryPhase(x) == e.ID {
			r.Questions++
			if s.blocking(x) {
				r.Blocking++
			}
		}
	}
	return r
}

// ScheduleGroup is a step of a plan's schedule: the done phases, then the
// unfinished ones in the order their needs allow, a step per depth of
// needs not yet done. Key is done, now (the first step: can start now),
// then, or go_live (the step holding the go-live phase).
type ScheduleGroup struct {
	Key    string     `json:"key"`
	Title  string     `json:"title"`
	Step   int        `json:"step"` // depth of unfinished needs; -1 for done
	Phases []PhaseRow `json:"phases"`
}

// schedule orders a plan's phases into steps.
func (s *state) schedule(slug string) []ScheduleGroup {
	goLive := s.goLive(slug)
	memo := map[string]int{}
	var done []PhaseRow
	steps := map[int][]PhaseRow{}
	for _, e := range s.planPhases(slug) {
		r := s.phaseRow(e, goLive)
		if r.State == PhaseDone {
			done = append(done, r)
			continue
		}
		d := s.depth(e.ID, true, memo)
		steps[d] = append(steps[d], r)
	}
	out := []ScheduleGroup{}
	if len(done) > 0 {
		out = append(out, ScheduleGroup{Key: "done", Title: "Done", Step: -1, Phases: done})
	}
	var ds []int
	for d := range steps {
		ds = append(ds, d)
	}
	sort.Ints(ds)
	for _, d := range ds {
		g := ScheduleGroup{Key: "then", Title: "Then", Step: d, Phases: steps[d]}
		if d == 0 && steps[d][0].State == PhaseNow {
			g.Key, g.Title = "now", "Can start now"
		}
		for _, r := range steps[d] {
			if r.GoLive && d > 0 {
				g.Key, g.Title = "go_live", "Go live"
			}
		}
		out = append(out, g)
	}
	return out
}

// AgentRow is an item someone is working on: active, with its claim and
// what git says about its commits.
type AgentRow struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Plan         string       `json:"plan,omitempty"`
	Phase        string       `json:"phase,omitempty"`
	PhaseTitle   string       `json:"phase_title,omitempty"`
	Claim        *model.Claim `json:"claim,omitempty"`
	Commits      int          `json:"commits"` // linked commits, landed or not
	Branches     []string     `json:"branches,omitempty"`
	LastActivity string       `json:"last_activity,omitempty"`
}

func (s *state) agentRow(i *model.Item, g *gitState) AgentRow {
	r := AgentRow{ID: i.ID, Title: i.Title, Plan: i.Plan, Phase: i.Phase, Claim: i.Claim}
	if ph := s.phases[i.Phase]; ph != nil {
		r.PhaseTitle = ph.Title
	}
	if ig := g.Items[i.ID]; ig != nil {
		r.Commits, r.Branches, r.LastActivity = ig.Landed+ig.Unmerged, ig.Branches, ig.LastActivity
	}
	return r
}

// PlanStatus is a plan's block on the project overview: where it stands and
// what's next.
type PlanStatus struct {
	Slug      string     `json:"slug"`
	Title     string     `json:"title"`
	State     string     `json:"state"`
	Summary   string     `json:"summary,omitempty"`
	Progress  Progress   `json:"progress"`
	Phases    int        `json:"phases"`
	Built     int        `json:"built"`     // phases done
	Questions int        `json:"questions"` // open questions anywhere in the plan
	Blocking  int        `json:"blocking"`
	Now       []PhaseRow `json:"now"`     // phases that can start now
	Next      []PhaseRow `json:"next"`    // waiting phases one step away (not go live)
	GoLive    *PhaseRow  `json:"go_live"` // the phase the plan ends with, if it has one
	Agents    []AgentRow `json:"agents"`
}

func (s *state) planStatus(p *model.Plan, g *gitState) PlanStatus {
	out := PlanStatus{Slug: p.Slug, Title: p.Title, State: p.State, Summary: p.Summary,
		Now: []PhaseRow{}, Next: []PhaseRow{}, Agents: []AgentRow{}}
	var items []*model.Item
	for _, id := range sortedKeys(s.items) {
		if i := s.items[id]; i.Plan == p.Slug {
			items = append(items, i)
			if i.State == model.StateActive {
				out.Agents = append(out.Agents, s.agentRow(i, g))
			}
		}
	}
	out.Progress = s.progress(items)
	goLive := s.goLive(p.Slug)
	for _, e := range s.planPhases(p.Slug) {
		out.Phases++
		r := s.phaseRow(e, goLive)
		switch {
		case r.State == PhaseDone:
			out.Built++
		case r.State == PhaseNow:
			out.Now = append(out.Now, r)
		case r.GoLive:
		default:
			for _, w := range r.WaitsFor {
				if w.State == PhaseNow {
					out.Next = append(out.Next, r)
					break
				}
			}
		}
		if r.GoLive {
			out.GoLive = &r
		}
	}
	for _, e := range s.entries {
		if e.Settled == nil && e.Kind == model.KindQuestion && e.Plan == p.Slug {
			out.Questions++
			if s.blocking(e) {
				out.Blocking++
			}
		}
	}
	return out
}
