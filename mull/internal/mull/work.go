package mull

import (
	"fmt"
	"slices"

	"mull/internal/model"
)

// ---- Plans ----

// PlanNew creates a plan.
func (m *Mull) PlanNew(slug, title, summary string) (*model.Plan, error) {
	if !model.ValidSlug(slug) {
		return nil, fmt.Errorf("invalid plan slug %q: use lowercase words joined by hyphens, e.g. taxogall or phase-two", slug)
	}
	if title == "" {
		return nil, fmt.Errorf("a plan needs a title")
	}
	var out *model.Plan
	err := m.update("plan new "+slug, func(s *state) error {
		if s.plans[slug] != nil {
			return fmt.Errorf("plan %q already exists", slug)
		}
		now := m.now()
		out = &model.Plan{Slug: slug, Title: title, State: model.PlanActive, Created: now, Updated: now, Summary: summary}
		return s.putPlan(out)
	})
	return out, err
}

// PlanView is a plan with its epics, items and progress.
type PlanView struct {
	model.Plan
	Progress Progress   `json:"progress"`
	Epics    []EpicView `json:"epics"`
	Items    []ItemView `json:"items"`
}

func (s *state) planView(p *model.Plan) PlanView {
	v := PlanView{Plan: *p, Epics: []EpicView{}, Items: []ItemView{}}
	var items []*model.Item
	for _, id := range sortedKeys(s.items) {
		if i := s.items[id]; i.Plan == p.Slug {
			items = append(items, i)
			v.Items = append(v.Items, s.itemView(i))
		}
	}
	v.Progress = s.progress(items)
	for _, id := range sortedKeys(s.epics) {
		if e := s.epics[id]; e.Plan == p.Slug {
			v.Epics = append(v.Epics, s.epicView(e))
		}
	}
	return v
}

// PlanList returns all plans with their progress (epics and items omitted).
func (m *Mull) PlanList() ([]PlanView, error) {
	var out []PlanView
	err := m.view(func(s *state) error {
		for _, slug := range sortedKeys(s.plans) {
			v := s.planView(s.plans[slug])
			v.Epics, v.Items = nil, nil
			out = append(out, v)
		}
		return nil
	})
	return out, err
}

// PlanShow returns one plan in full.
func (m *Mull) PlanShow(slug string) (PlanView, error) {
	var out PlanView
	err := m.view(func(s *state) error {
		p, err := s.plan(slug)
		if err != nil {
			return err
		}
		out = s.planView(p)
		return nil
	})
	return out, err
}

// PlanClose closes a plan.
func (m *Mull) PlanClose(slug string) (*model.Plan, error) {
	if slug == model.GeneralPlan {
		return nil, fmt.Errorf("the %s plan cannot be closed", model.GeneralPlan)
	}
	var out *model.Plan
	err := m.update("plan close "+slug, func(s *state) error {
		p, err := s.plan(slug)
		if err != nil {
			return err
		}
		p.State, p.Updated = model.PlanClosed, m.now()
		out = p
		return s.putPlan(p)
	})
	return out, err
}

// ---- Epics ----

// EpicAdd creates an epic in a plan.
func (m *Mull) EpicAdd(plan, title, summary string, needs []string) (*model.Epic, error) {
	if title == "" {
		return nil, fmt.Errorf("an epic needs a title")
	}
	var out *model.Epic
	err := m.update("epic add: "+title, func(s *state) error {
		if _, err := s.activePlan(plan); err != nil {
			return err
		}
		id, err := s.newID()
		if err != nil {
			return err
		}
		needs := dedupe(needs)
		if err := s.checkNeedTargets(id, needs); err != nil {
			return err
		}
		now := m.now()
		out = &model.Epic{ID: id, Plan: plan, Title: title, Needs: needs, Created: now, Updated: now, Summary: summary}
		if err := s.putEpic(out); err != nil {
			return err
		}
		return s.checkCycles()
	})
	return out, err
}

// EpicDetail is an epic with its items.
type EpicDetail struct {
	EpicView
	Items []ItemView `json:"items"`
}

// EpicList returns the epics of a plan, or of every plan when plan is "".
func (m *Mull) EpicList(plan string) ([]EpicView, error) {
	out := []EpicView{}
	err := m.view(func(s *state) error {
		for _, id := range sortedKeys(s.epics) {
			if e := s.epics[id]; plan == "" || e.Plan == plan {
				out = append(out, s.epicView(e))
			}
		}
		return nil
	})
	return out, err
}

// ---- Items ----

// NewItem holds the fields for creating an item.
type NewItem struct {
	Plan, Epic, Title, Notes string
	Needs, Docs              []string
}

// ItemAdd creates an item.
func (m *Mull) ItemAdd(in NewItem) (ItemView, error) {
	var out ItemView
	if in.Title == "" {
		return out, fmt.Errorf("an item needs a title")
	}
	err := m.update("add: "+in.Title, func(s *state) error {
		i, err := m.addItem(s, in)
		if err != nil {
			return err
		}
		out = s.itemView(i)
		return nil
	})
	return out, err
}

func (m *Mull) addItem(s *state, in NewItem) (*model.Item, error) {
	if _, err := s.activePlan(in.Plan); err != nil {
		return nil, err
	}
	if in.Epic != "" {
		e, err := s.epic(in.Epic)
		if err != nil {
			return nil, err
		}
		if e.Plan != in.Plan {
			return nil, fmt.Errorf("epic %s belongs to plan %q, not %q", e.ID, e.Plan, in.Plan)
		}
	}
	id, err := s.newID()
	if err != nil {
		return nil, err
	}
	needs := dedupe(in.Needs)
	if err := s.checkNeedTargets(id, needs); err != nil {
		return nil, err
	}
	now := m.now()
	i := &model.Item{
		ID: id, Plan: in.Plan, Epic: in.Epic, Title: in.Title, State: model.StateOpen,
		Needs: needs, Docs: dedupe(in.Docs), Created: now, Updated: now, Notes: in.Notes,
	}
	if err := s.putItem(i); err != nil {
		return nil, err
	}
	return i, s.checkCycles()
}

// ItemFilter selects items; empty fields match everything.
type ItemFilter struct {
	Plan, Epic, State string
	ReadyOnly         bool
}

// ItemList returns items matching f.
func (m *Mull) ItemList(f ItemFilter) ([]ItemView, error) {
	out := []ItemView{}
	err := m.view(func(s *state) error {
		for _, id := range sortedKeys(s.items) {
			i := s.items[id]
			if (f.Plan != "" && i.Plan != f.Plan) || (f.Epic != "" && i.Epic != f.Epic) || (f.State != "" && i.State != f.State) {
				continue
			}
			v := s.itemView(i)
			if f.ReadyOnly && !v.Ready {
				continue
			}
			out = append(out, v)
		}
		return nil
	})
	return out, err
}

// Show returns the plan, epic, item or inbox entry named by ref.
func (m *Mull) Show(ref string) (any, error) {
	var out any
	err := m.view(func(s *state) error {
		switch {
		case s.plans[ref] != nil:
			out = s.planView(s.plans[ref])
		case s.epics[ref] != nil:
			e := s.epics[ref]
			d := EpicDetail{EpicView: s.epicView(e), Items: []ItemView{}}
			for _, i := range s.epicItems(e.ID) {
				d.Items = append(d.Items, s.itemView(i))
			}
			out = d
		case s.items[ref] != nil:
			out = s.itemView(s.items[ref])
		case s.entries[ref] != nil:
			out = s.entries[ref]
		default:
			return fmt.Errorf("nothing called %q", ref)
		}
		return nil
	})
	return out, err
}

// Set changes one field of a plan, epic or item.
//
//	plan: title, summary
//	epic: title, summary
//	item: title, notes, epic ("" to unfile), doc (adds a path)
func (m *Mull) Set(ref, field, value string) (any, error) {
	var out any
	err := m.update(fmt.Sprintf("set %s %s", ref, field), func(s *state) error {
		now := m.now()
		switch {
		case s.plans[ref] != nil:
			p := s.plans[ref]
			switch field {
			case "title":
				p.Title = value
			case "summary":
				p.Summary = value
			default:
				return fmt.Errorf("a plan's settable fields are title and summary")
			}
			p.Updated, out = now, p
			return s.putPlan(p)
		case s.epics[ref] != nil:
			e := s.epics[ref]
			switch field {
			case "title":
				e.Title = value
			case "summary":
				e.Summary = value
			default:
				return fmt.Errorf("an epic's settable fields are title and summary")
			}
			e.Updated, out = now, e
			return s.putEpic(e)
		case s.items[ref] != nil:
			i := s.items[ref]
			switch field {
			case "title":
				i.Title = value
			case "notes":
				i.Notes = value
			case "doc":
				i.Docs = dedupe(append(i.Docs, value))
			case "epic":
				if value != "" {
					e, err := s.epic(value)
					if err != nil {
						return err
					}
					if e.Plan != i.Plan {
						return fmt.Errorf("epic %s belongs to plan %q, not %q", e.ID, e.Plan, i.Plan)
					}
				}
				i.Epic = value
			default:
				return fmt.Errorf("an item's settable fields are title, notes, epic and doc")
			}
			i.Updated = now
			if err := s.putItem(i); err != nil {
				return err
			}
			out = s.itemView(i)
			return s.checkCycles()
		}
		return fmt.Errorf("no plan, epic or item %q", ref)
	})
	return out, err
}

// ---- Dependencies ----

// Need records that ref (an item or epic) needs the targets.
func (m *Mull) Need(ref string, targets []string) (any, error) {
	return m.changeNeeds(ref, targets, true)
}

// Unneed removes needs.
func (m *Mull) Unneed(ref string, targets []string) (any, error) {
	return m.changeNeeds(ref, targets, false)
}

func (m *Mull) changeNeeds(ref string, targets []string, add bool) (any, error) {
	targets = dedupe(targets)
	verb := "need"
	if !add {
		verb = "unneed"
	}
	var out any
	err := m.update(fmt.Sprintf("%s %s %v", verb, ref, targets), func(s *state) error {
		apply := func(needs []string) ([]string, error) {
			if add {
				if err := s.checkNeedTargets(ref, targets); err != nil {
					return nil, err
				}
				return dedupe(append(needs, targets...)), nil
			}
			return slices.DeleteFunc(needs, func(n string) bool { return slices.Contains(targets, n) }), nil
		}
		now := m.now()
		switch {
		case s.items[ref] != nil:
			i := s.items[ref]
			needs, err := apply(i.Needs)
			if err != nil {
				return err
			}
			i.Needs, i.Updated = needs, now
			if err := s.putItem(i); err != nil {
				return err
			}
			out = s.itemView(i)
		case s.epics[ref] != nil:
			e := s.epics[ref]
			needs, err := apply(e.Needs)
			if err != nil {
				return err
			}
			e.Needs, e.Updated = needs, now
			if err := s.putEpic(e); err != nil {
				return err
			}
			out = s.epicView(e)
		default:
			return fmt.Errorf("no item or epic %q", ref)
		}
		return s.checkCycles()
	})
	return out, err
}

// ---- Claims ----

// StartResult is a claimed item plus any warnings.
type StartResult struct {
	Item     ItemView `json:"item"`
	Warnings []string `json:"warnings,omitempty"`
}

// Start claims an item for this worktree and marks it active. Starting a
// blocked item is allowed but warned about.
func (m *Mull) Start(id string) (StartResult, error) {
	var out StartResult
	wt, branch := m.Worktree(), m.repo.Branch()
	err := m.update("start "+id, func(s *state) error {
		out = StartResult{}
		i, err := s.item(id)
		if err != nil {
			return err
		}
		switch i.State {
		case model.StateOpen:
		case model.StateActive:
			if i.Claim != nil && i.Claim.Worktree == wt {
				out.Item = s.itemView(i)
				return nil // already ours
			}
			who := "another worktree"
			if i.Claim != nil {
				who = fmt.Sprintf("%s (branch %s, since %s)", i.Claim.Worktree, i.Claim.Branch, i.Claim.At)
			}
			return fmt.Errorf("item %s is already claimed by %s", id, who)
		default:
			return fmt.Errorf("item %s is %s and cannot be started", id, i.State)
		}
		if blocked := s.blockedBy(s.requirements(i)); len(blocked) > 0 {
			out.Warnings = append(out.Warnings, fmt.Sprintf("started while blocked by %v", blocked))
		}
		now := m.now()
		i.State, i.Updated = model.StateActive, now
		i.Claim = &model.Claim{Worktree: wt, Branch: branch, At: now}
		if err := s.putItem(i); err != nil {
			return err
		}
		out.Item = s.itemView(i)
		return nil
	})
	return out, err
}

// Release gives back a claim: the item returns to open.
func (m *Mull) Release(id string) (ItemView, error) {
	var out ItemView
	err := m.update("release "+id, func(s *state) error {
		i, err := s.item(id)
		if err != nil {
			return err
		}
		if i.State != model.StateActive {
			return fmt.Errorf("item %s is %s, not active", id, i.State)
		}
		i.State, i.Claim, i.Updated = model.StateOpen, nil, m.now()
		if err := s.putItem(i); err != nil {
			return err
		}
		out = s.itemView(i)
		return nil
	})
	return out, err
}
