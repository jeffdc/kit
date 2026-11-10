package mull

import (
	"strings"

	"mull/internal/model"
)

// A need is satisfied when its target is finished. Unreviewed work is
// incomplete work, so an item counts only once done (reviewed) or dropped,
// and a phase only once every item committed to it does. Captured items are
// backlog: tagged to a phase, they neither count toward it nor hold it open.

func finished(i *model.Item) bool {
	return i.State == model.StateDone || i.State == model.StateDropped
}

func (s *state) phaseItems(phaseID string) []*model.Item {
	var out []*model.Item
	for _, id := range sortedKeys(s.items) {
		if s.items[id].Phase == phaseID {
			out = append(out, s.items[id])
		}
	}
	return out
}

func (s *state) phaseComplete(phaseID string) bool {
	committed := 0
	for _, i := range s.phaseItems(phaseID) {
		if i.State == model.StateCaptured {
			continue
		}
		if !finished(i) {
			return false
		}
		committed++
	}
	return committed > 0
}

func (s *state) satisfied(id string) bool {
	if i := s.items[id]; i != nil {
		return finished(i)
	}
	if s.phases[id] != nil {
		return s.phaseComplete(id)
	}
	return true // a missing target cannot block; doctor-style checks can flag it later
}

// requirements are what an item waits for: its phase's needs. Needs exist
// between phases only, and items inside a phase are unordered. An item's own
// needs, left from before Model v2, are kept and shown but ignored until
// mull migrate converts them.
func (s *state) requirements(i *model.Item) []string {
	if e := s.phases[i.Phase]; e != nil {
		return dedupe(e.Needs)
	}
	return nil
}

func (s *state) blockedBy(reqs []string) []string {
	var out []string
	for _, r := range reqs {
		if !s.satisfied(r) {
			out = append(out, r)
		}
	}
	return out
}

// ItemView is an item with its derived readiness: committed, with its phase's
// needs done. Items with no phase wait on nothing.
type ItemView struct {
	model.Item
	Ready     bool     `json:"ready"`
	BlockedBy []string `json:"blocked_by,omitempty"`
}

func (s *state) itemView(i *model.Item) ItemView {
	v := ItemView{Item: *i}
	if i.State == model.StateCommitted {
		v.BlockedBy = s.blockedBy(s.requirements(i))
		v.Ready = len(v.BlockedBy) == 0
	}
	return v
}

// Progress counts a phase's or plan's items by stage. Total counts committed
// work only; captured items are counted on their own.
type Progress struct {
	Total     int `json:"total"`
	Captured  int `json:"captured"`
	Committed int `json:"committed"`
	Active    int `json:"active"`
	InReview  int `json:"in_review"`
	Done      int `json:"done"`
	Dropped   int `json:"dropped"`
	Ready     int `json:"ready"`
}

func (s *state) progress(items []*model.Item) Progress {
	var p Progress
	for _, i := range items {
		if i.State == model.StateCaptured {
			p.Captured++
			continue
		}
		p.Total++
		switch i.State {
		case model.StateCommitted:
			p.Committed++
			if s.itemView(i).Ready {
				p.Ready++
			}
		case model.StateActive:
			p.Active++
		case model.StateInReview:
			p.InReview++
		case model.StateDone:
			p.Done++
		case model.StateDropped:
			p.Dropped++
		}
	}
	return p
}

// PhaseView is a phase with its derived progress.
type PhaseView struct {
	model.Phase
	Progress  Progress `json:"progress"`
	Complete  bool     `json:"complete"`
	BlockedBy []string `json:"blocked_by,omitempty"`
}

func (s *state) phaseView(e *model.Phase) PhaseView {
	return PhaseView{
		Phase:     *e,
		Progress:  s.progress(s.phaseItems(e.ID)),
		Complete:  s.phaseComplete(e.ID),
		BlockedBy: s.blockedBy(e.Needs),
	}
}

// checkNeedTargets validates that every target is a phase other than the
// phase itself: phases need phases, nothing else. (A phase's needs on items,
// left from before Model v2, still count until mull migrate converts them.)
func (s *state) checkNeedTargets(self string, targets []string) error {
	for _, t := range targets {
		if t == self {
			return errf(CodeInvalid, "%s cannot need itself", self)
		}
		if s.items[t] != nil {
			return errf(CodeNotAllowed, "%s is an item: phases need phases only", t)
		}
		if s.phases[t] == nil {
			return errf(CodeNotFound, "no phase %q", t)
		}
	}
	return nil
}

// checkCycles refuses a dependency graph with a cycle. Edges point from a
// record to what it waits for: an item waits for its phase's needs; a phase
// waits for its needs and for its own items.
func (s *state) checkCycles() error {
	edges := map[string][]string{}
	for id, i := range s.items {
		edges[id] = s.requirements(i)
	}
	for id, e := range s.phases {
		deps := append([]string{}, e.Needs...)
		for _, i := range s.phaseItems(id) {
			deps = append(deps, i.ID)
		}
		edges[id] = deps
	}
	const (
		unseen = iota
		onPath
		finishedVisit
	)
	mark := map[string]int{}
	var path []string
	var visit func(id string) error
	visit = func(id string) error {
		switch mark[id] {
		case onPath:
			start := 0
			for k, p := range path {
				if p == id {
					start = k
				}
			}
			cycle := append(append([]string{}, path[start:]...), id)
			return errf(CodeCycle, "dependency cycle: %s", strings.Join(cycle, " → "))
		case finishedVisit:
			return nil
		}
		mark[id] = onPath
		path = append(path, id)
		for _, d := range edges[id] {
			if err := visit(d); err != nil {
				return err
			}
		}
		path = path[:len(path)-1]
		mark[id] = finishedVisit
		return nil
	}
	for _, id := range sortedKeys(edges) {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}
