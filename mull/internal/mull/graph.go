package mull

import (
	"strings"

	"mull/internal/model"
)

// A need is satisfied when its target is finished. Unreviewed work is
// incomplete work, so an item counts only once accepted (or dropped), and an
// epic only once every item in it does.

func finished(i *model.Item) bool {
	return i.State == model.StateAccepted || i.State == model.StateDropped
}

func (s *state) epicItems(epicID string) []*model.Item {
	var out []*model.Item
	for _, id := range sortedKeys(s.items) {
		if s.items[id].Epic == epicID {
			out = append(out, s.items[id])
		}
	}
	return out
}

func (s *state) epicComplete(epicID string) bool {
	items := s.epicItems(epicID)
	if len(items) == 0 {
		return false
	}
	for _, i := range items {
		if !finished(i) {
			return false
		}
	}
	return true
}

func (s *state) satisfied(id string) bool {
	if i := s.items[id]; i != nil {
		return finished(i)
	}
	if s.epics[id] != nil {
		return s.epicComplete(id)
	}
	return true // a missing target cannot block; doctor-style checks can flag it later
}

// requirements are what an item waits for: its own needs plus its epic's.
func (s *state) requirements(i *model.Item) []string {
	reqs := append([]string{}, i.Needs...)
	if e := s.epics[i.Epic]; e != nil {
		reqs = append(reqs, e.Needs...)
	}
	return dedupe(reqs)
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

// ItemView is an item with its derived readiness.
type ItemView struct {
	model.Item
	Ready     bool     `json:"ready"`
	BlockedBy []string `json:"blocked_by,omitempty"`
}

func (s *state) itemView(i *model.Item) ItemView {
	v := ItemView{Item: *i}
	if i.State == model.StateOpen {
		v.BlockedBy = s.blockedBy(s.requirements(i))
		v.Ready = len(v.BlockedBy) == 0
	}
	return v
}

// Progress counts an epic's or plan's items by state.
type Progress struct {
	Total    int `json:"total"`
	Open     int `json:"open"`
	Active   int `json:"active"`
	Done     int `json:"done_unreviewed"`
	Accepted int `json:"accepted"`
	Dropped  int `json:"dropped"`
	Ready    int `json:"ready"`
}

func (s *state) progress(items []*model.Item) Progress {
	var p Progress
	for _, i := range items {
		p.Total++
		switch i.State {
		case model.StateOpen:
			p.Open++
			if s.itemView(i).Ready {
				p.Ready++
			}
		case model.StateActive:
			p.Active++
		case model.StateDone:
			p.Done++
		case model.StateAccepted:
			p.Accepted++
		case model.StateDropped:
			p.Dropped++
		}
	}
	return p
}

// EpicView is an epic with its derived progress.
type EpicView struct {
	model.Epic
	Progress  Progress `json:"progress"`
	Complete  bool     `json:"complete"`
	BlockedBy []string `json:"blocked_by,omitempty"`
}

func (s *state) epicView(e *model.Epic) EpicView {
	return EpicView{
		Epic:      *e,
		Progress:  s.progress(s.epicItems(e.ID)),
		Complete:  s.epicComplete(e.ID),
		BlockedBy: s.blockedBy(e.Needs),
	}
}

// checkNeedTargets validates that every target exists and is not the record itself.
func (s *state) checkNeedTargets(self string, targets []string) error {
	for _, t := range targets {
		if t == self {
			return errf(CodeInvalid, "%s cannot need itself", self)
		}
		if s.items[t] == nil && s.epics[t] == nil {
			return errf(CodeNotFound, "no item or epic %q", t)
		}
	}
	return nil
}

// checkCycles refuses a dependency graph with a cycle. Edges point from a
// record to what it waits for: an item waits for its needs and its epic's
// needs; an epic waits for its needs and for its own items.
func (s *state) checkCycles() error {
	edges := map[string][]string{}
	for id, i := range s.items {
		edges[id] = s.requirements(i)
	}
	for id, e := range s.epics {
		deps := append([]string{}, e.Needs...)
		for _, i := range s.epicItems(id) {
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
