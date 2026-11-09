package mull

import (
	"fmt"
	"slices"

	"mull/internal/model"
)

// Every write has an inner form that runs on a transaction's state (used by
// batch, which runs many in one transaction) and a public wrapper that runs
// it in its own transaction.

// ---- Plans ----

// PlanNew creates a plan.
func (m *Mull) PlanNew(slug, title, summary string) (*model.Plan, error) {
	var out *model.Plan
	err := m.update("plan new "+slug, func(s *state) error {
		var err error
		out, err = m.planNew(s, slug, title, summary)
		return err
	})
	return out, err
}

func (m *Mull) planNew(s *state, slug, title, summary string) (*model.Plan, error) {
	if !model.ValidSlug(slug) {
		return nil, errf(CodeInvalid, "invalid plan slug %q: use lowercase words joined by hyphens, e.g. taxogall or phase-two", slug)
	}
	if title == "" {
		return nil, errf(CodeInvalid, "a plan needs a title")
	}
	if s.plans[slug] != nil {
		return nil, errf(CodeExists, "plan %q already exists", slug)
	}
	now := m.now()
	p := &model.Plan{Slug: slug, Title: title, State: model.PlanActive, Created: now, Updated: now, Summary: summary}
	return p, s.putPlan(p)
}

// PlanView is a plan with its epics, items and progress.
type PlanView struct {
	model.Plan
	Progress Progress   `json:"progress"`
	Epics    []EpicView `json:"epics"`
	Items    []ItemView `json:"items"`
}

// PlanSummary is a plan with its progress, as listed by plan list.
type PlanSummary struct {
	model.Plan
	Progress Progress `json:"progress"`
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

// PlanList returns all plans with their progress.
func (m *Mull) PlanList() ([]PlanSummary, error) {
	out := []PlanSummary{}
	err := m.view(func(s *state) error {
		for _, slug := range sortedKeys(s.plans) {
			v := s.planView(s.plans[slug])
			out = append(out, PlanSummary{Plan: v.Plan, Progress: v.Progress})
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
	var out *model.Plan
	err := m.update("plan close "+slug, func(s *state) error {
		var err error
		out, err = m.planClose(s, slug)
		return err
	})
	return out, err
}

func (m *Mull) planClose(s *state, slug string) (*model.Plan, error) {
	if slug == model.GeneralPlan {
		return nil, errf(CodeNotAllowed, "the %s plan cannot be closed", model.GeneralPlan)
	}
	p, err := s.plan(slug)
	if err != nil {
		return nil, err
	}
	p.State, p.Updated = model.PlanClosed, m.now()
	return p, s.putPlan(p)
}

// ---- Epics ----

// EpicAdd creates an epic in a plan.
func (m *Mull) EpicAdd(plan, title, summary string, needs []string) (*model.Epic, error) {
	var out *model.Epic
	err := m.update("epic add: "+title, func(s *state) error {
		var err error
		out, err = m.epicAdd(s, plan, title, summary, needs, "")
		return err
	})
	return out, err
}

func (m *Mull) epicAdd(s *state, plan, title, summary string, needs []string, from string) (*model.Epic, error) {
	if title == "" {
		return nil, errf(CodeInvalid, "an epic needs a title")
	}
	if plan == "" {
		return nil, errf(CodeInvalid, "an epic needs a plan")
	}
	if _, err := s.activePlan(plan); err != nil {
		return nil, err
	}
	id, err := s.newID()
	if err != nil {
		return nil, err
	}
	needs = dedupe(needs)
	if err := s.checkNeedTargets(id, needs); err != nil {
		return nil, err
	}
	now := m.now()
	e := &model.Epic{ID: id, Plan: plan, Title: title, Needs: needs, From: from, Created: now, Updated: now, Summary: summary}
	if err := s.putEpic(e); err != nil {
		return nil, err
	}
	return e, s.checkCycles()
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
		if plan != "" {
			if _, err := s.plan(plan); err != nil {
				return err
			}
		}
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

// NewItem holds the fields for creating an item. Plan may be left empty
// when Epic is given: the epic's plan is used.
type NewItem struct {
	Plan, Epic, Title, Notes string
	Needs, Docs              []string
	From                     string // the inbox entry it was made from
}

// ItemAdd creates an item.
func (m *Mull) ItemAdd(in NewItem) (ItemView, error) {
	var out ItemView
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

// resolvePlan works out an item's plan from plan and epic: the epic implies
// its plan, and the two must agree when both are given.
func (s *state) resolvePlan(plan, epic string) (string, error) {
	if epic != "" {
		e, err := s.epic(epic)
		if err != nil {
			return "", err
		}
		if plan != "" && plan != e.Plan {
			return "", errf(CodeInvalid, "epic %s belongs to plan %q, not %q", e.ID, e.Plan, plan)
		}
		return e.Plan, nil
	}
	if plan == "" {
		return "", errf(CodeScope, "say which plan the item belongs to: give --plan or --epic")
	}
	return plan, nil
}

func (m *Mull) addItem(s *state, in NewItem) (*model.Item, error) {
	if in.Title == "" {
		return nil, errf(CodeInvalid, "an item needs a title")
	}
	plan, err := s.resolvePlan(in.Plan, in.Epic)
	if err != nil {
		return nil, err
	}
	if _, err := s.activePlan(plan); err != nil {
		return nil, err
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
		ID: id, Plan: plan, Epic: in.Epic, Title: in.Title, State: model.StateOpen,
		Needs: needs, Docs: dedupe(in.Docs), From: in.From, Created: now, Updated: now, Notes: in.Notes,
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
		if f.Plan != "" {
			if _, err := s.plan(f.Plan); err != nil {
				return err
			}
		}
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

// Shown is any record returned by show, tagged with its type.
type Shown struct {
	Type  string       `json:"type"` // plan, epic, item, entry
	Plan  *PlanView    `json:"plan,omitempty"`
	Epic  *EpicDetail  `json:"epic,omitempty"`
	Item  *ItemView    `json:"item,omitempty"`
	Entry *model.Entry `json:"entry,omitempty"`
}

// Show returns the plan, epic, item or inbox entry named by ref.
func (m *Mull) Show(ref string) (Shown, error) {
	var out Shown
	err := m.view(func(s *state) error {
		switch {
		case s.plans[ref] != nil:
			v := s.planView(s.plans[ref])
			out = Shown{Type: "plan", Plan: &v}
		case s.epics[ref] != nil:
			e := s.epics[ref]
			d := EpicDetail{EpicView: s.epicView(e), Items: []ItemView{}}
			for _, i := range s.epicItems(e.ID) {
				d.Items = append(d.Items, s.itemView(i))
			}
			out = Shown{Type: "epic", Epic: &d}
		case s.items[ref] != nil:
			v := s.itemView(s.items[ref])
			out = Shown{Type: "item", Item: &v}
		case s.entries[ref] != nil:
			out = Shown{Type: "entry", Entry: s.entries[ref]}
		default:
			return errf(CodeNotFound, "nothing called %q", ref)
		}
		return nil
	})
	return out, err
}

// SetResult is the record set changed, tagged with its type.
type SetResult struct {
	Type string      `json:"type"` // plan, epic, item
	Plan *model.Plan `json:"plan,omitempty"`
	Epic *EpicView   `json:"epic,omitempty"`
	Item *ItemView   `json:"item,omitempty"`
}

// Set changes one field of a plan, epic or item.
//
//	plan: title, summary
//	epic: title, summary
//	item: title, notes, epic ("" to unfile), doc (adds a path)
func (m *Mull) Set(ref, field, value string) (SetResult, error) {
	var out SetResult
	err := m.update(fmt.Sprintf("set %s %s", ref, field), func(s *state) error {
		var err error
		out, err = m.set(s, ref, field, value)
		return err
	})
	return out, err
}

func (m *Mull) set(s *state, ref, field, value string) (SetResult, error) {
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
			return SetResult{}, errf(CodeInvalid, "a plan's settable fields are title and summary")
		}
		p.Updated = now
		return SetResult{Type: "plan", Plan: p}, s.putPlan(p)
	case s.epics[ref] != nil:
		e := s.epics[ref]
		switch field {
		case "title":
			e.Title = value
		case "summary":
			e.Summary = value
		default:
			return SetResult{}, errf(CodeInvalid, "an epic's settable fields are title and summary")
		}
		e.Updated = now
		if err := s.putEpic(e); err != nil {
			return SetResult{}, err
		}
		v := s.epicView(e)
		return SetResult{Type: "epic", Epic: &v}, nil
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
					return SetResult{}, err
				}
				if e.Plan != i.Plan {
					return SetResult{}, errf(CodeInvalid, "epic %s belongs to plan %q, not %q", e.ID, e.Plan, i.Plan)
				}
			}
			i.Epic = value
		default:
			return SetResult{}, errf(CodeInvalid, "an item's settable fields are title, notes, epic and doc")
		}
		i.Updated = now
		if err := s.putItem(i); err != nil {
			return SetResult{}, err
		}
		if err := s.checkCycles(); err != nil {
			return SetResult{}, err
		}
		v := s.itemView(i)
		return SetResult{Type: "item", Item: &v}, nil
	}
	return SetResult{}, errf(CodeNotFound, "no plan, epic or item %q", ref)
}

// ---- Dependencies ----

// NeedResult is the item or epic whose needs changed, tagged with its type.
type NeedResult struct {
	Type string    `json:"type"` // item, epic
	Epic *EpicView `json:"epic,omitempty"`
	Item *ItemView `json:"item,omitempty"`
}

// Need records that ref (an item or epic) needs the targets.
func (m *Mull) Need(ref string, targets []string) (NeedResult, error) {
	return m.changeNeedsTx(ref, targets, true)
}

// Unneed removes needs.
func (m *Mull) Unneed(ref string, targets []string) (NeedResult, error) {
	return m.changeNeedsTx(ref, targets, false)
}

func (m *Mull) changeNeedsTx(ref string, targets []string, add bool) (NeedResult, error) {
	var out NeedResult
	verb := "need"
	if !add {
		verb = "unneed"
	}
	err := m.update(fmt.Sprintf("%s %s %v", verb, ref, targets), func(s *state) error {
		var err error
		out, err = m.changeNeeds(s, ref, targets, add)
		return err
	})
	return out, err
}

func (m *Mull) changeNeeds(s *state, ref string, targets []string, add bool) (NeedResult, error) {
	targets = dedupe(targets)
	if len(targets) == 0 {
		return NeedResult{}, errf(CodeInvalid, "give at least one item or epic it needs")
	}
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
	var out NeedResult
	switch {
	case s.items[ref] != nil:
		i := s.items[ref]
		needs, err := apply(i.Needs)
		if err != nil {
			return out, err
		}
		i.Needs, i.Updated = needs, now
		if err := s.putItem(i); err != nil {
			return out, err
		}
		if err := s.checkCycles(); err != nil {
			return out, err
		}
		v := s.itemView(i)
		out = NeedResult{Type: "item", Item: &v}
	case s.epics[ref] != nil:
		e := s.epics[ref]
		needs, err := apply(e.Needs)
		if err != nil {
			return out, err
		}
		e.Needs, e.Updated = needs, now
		if err := s.putEpic(e); err != nil {
			return out, err
		}
		if err := s.checkCycles(); err != nil {
			return out, err
		}
		v := s.epicView(e)
		out = NeedResult{Type: "epic", Epic: &v}
	default:
		return out, errf(CodeNotFound, "no item or epic %q", ref)
	}
	return out, nil
}

// ---- Claims ----

// StartResult is a claimed item plus any warnings.
type StartResult struct {
	Item     ItemView `json:"item"`
	Warnings []string `json:"warnings"`
}

// Start claims an item for this worktree and marks it active. Starting a
// blocked item is allowed but warned about.
func (m *Mull) Start(id string) (StartResult, error) {
	var out StartResult
	err := m.update("start "+id, func(s *state) error {
		var err error
		out, err = m.start(s, id)
		return err
	})
	return out, err
}

func (m *Mull) start(s *state, id string) (StartResult, error) {
	wt, branch := m.Worktree(), m.repo.Branch()
	out := StartResult{Warnings: []string{}}
	i, err := s.item(id)
	if err != nil {
		return out, err
	}
	switch i.State {
	case model.StateOpen:
	case model.StateActive:
		if i.Claim != nil && i.Claim.Worktree == wt {
			out.Item = s.itemView(i)
			return out, nil // already ours
		}
		who := "another worktree"
		if i.Claim != nil {
			who = fmt.Sprintf("%s (branch %s, since %s)", i.Claim.Worktree, i.Claim.Branch, i.Claim.At)
		}
		return out, errf(CodeClaimed, "item %s is already claimed by %s", id, who)
	default:
		return out, errf(CodeState, "item %s is %s and cannot be started", id, i.State)
	}
	if blocked := s.blockedBy(s.requirements(i)); len(blocked) > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("started while blocked by %v", blocked))
	}
	base, _ := m.repo.RefCommit("HEAD")
	now := m.now()
	i.State, i.Updated = model.StateActive, now
	i.Claim = &model.Claim{Worktree: wt, Branch: branch, At: now, Base: base}
	if err := s.putItem(i); err != nil {
		return out, err
	}
	out.Item = s.itemView(i)
	return out, nil
}

// Release gives back a claim: the item returns to open.
func (m *Mull) Release(id string) (ItemView, error) {
	var out ItemView
	err := m.update("release "+id, func(s *state) error {
		var err error
		out, err = m.release(s, id)
		return err
	})
	return out, err
}

func (m *Mull) release(s *state, id string) (ItemView, error) {
	i, err := s.item(id)
	if err != nil {
		return ItemView{}, err
	}
	if i.State != model.StateActive {
		return ItemView{}, errf(CodeState, "item %s is %s, not active", id, i.State)
	}
	i.State, i.Claim, i.Updated = model.StateOpen, nil, m.now()
	if err := s.putItem(i); err != nil {
		return ItemView{}, err
	}
	return s.itemView(i), nil
}
