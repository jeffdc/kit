package core

import (
	"fmt"
	"sort"
	"strings"
)

// ---- Plans ----

// PlanNew creates a plan, shaping until the human agrees it. Its work lands
// on target: "" or "own" is plan/<slug>, "none" the integration branch.
func (m *Mull) PlanNew(slug, title, summary, target string, onRequest bool) (*Plan, error) {
	var out *Plan
	err := m.update("plan new "+slug, func(s *state) error {
		switch {
		case !validSlug(slug):
			return errf(CodeInvalid, "a plan slug is lowercase words joined by hyphens, not %q", slug)
		case strings.TrimSpace(title) == "":
			return errf(CodeInvalid, "a plan needs a title")
		case s.plans[slug] != nil:
			return errf(CodeExists, "plan %q exists", slug)
		case !m.asked(onRequest):
			return errf(CodeNotAllowed, "only the human creates a plan: pass --on-request when they've asked")
		}
		switch target {
		case "", "own":
			target = "plan/" + slug
		case NoPlan:
			target = ""
		}
		now := m.now()
		out = &Plan{Slug: slug, Title: title, State: Shaping, Target: target, Created: now, Updated: now, Summary: summary}
		return s.putPlan(out)
	})
	return out, err
}

// PlanState agrees (active) or closes (done) a plan: the human's call.
func (m *Mull) PlanState(slug, to string, onRequest bool) (*Plan, error) {
	var out *Plan
	err := m.update("plan "+to+" "+slug, func(s *state) error {
		p, err := s.plan(slug)
		if err != nil {
			return err
		}
		if !m.asked(onRequest) {
			return errf(CodeNotAllowed, "only the human agrees or closes a plan: pass --on-request when they've asked")
		}
		if to != PlanActive && to != PlanDone {
			return errf(CodeInvalid, "a plan is agreed (active) or closed (done)")
		}
		if to == PlanActive && p.State != Shaping {
			return errf(CodeState, "plan %q is %s, not shaping", slug, p.State)
		}
		p.State, p.Updated = to, m.now()
		out = p
		return s.putPlan(p)
	})
	return out, err
}

// PlanSet changes a plan's title, summary, target, preview or doc.
func (m *Mull) PlanSet(slug, field, value string) (*Plan, error) {
	var out *Plan
	err := m.update("plan set "+slug+" "+field, func(s *state) error {
		p, err := s.plan(slug)
		if err != nil {
			return err
		}
		switch field {
		case "title":
			p.Title = value
		case "summary":
			p.Summary = value
		case "target":
			p.Target = strings.TrimPrefix(strings.TrimSpace(value), "refs/heads/")
			if p.Target == "mull" {
				return errf(CodeInvalid, "mull's own branch can't be a target")
			}
		case "preview":
			p.Preview = strings.TrimSpace(value)
		case "doc":
			p.Doc = strings.TrimSpace(value)
			s.tx.Put(planDocPath(slug), []byte(p.Doc+"\n"))
		default:
			return errf(CodeInvalid, "a plan's fields are title, summary, target, preview and doc")
		}
		p.Updated = m.now()
		out = p
		return s.putPlan(p)
	})
	return out, err
}

// ---- Phases ----

// PhaseAdd adds a phase to an agreed or shaping plan.
func (m *Mull) PhaseAdd(plan, title, summary string, needs []string) (*Phase, error) {
	var out *Phase
	err := m.update("phase add: "+title, func(s *state) error {
		if _, err := s.plan(plan); err != nil {
			return err
		}
		id, err := s.newID()
		if err != nil {
			return err
		}
		now := m.now()
		out = &Phase{ID: id, Plan: plan, Title: title, Created: now, Updated: now, Summary: summary}
		if err := s.putPhase(out); err != nil {
			return err
		}
		return s.setNeeds(out, needs)
	})
	return out, err
}

// PhaseSet changes a phase's title, summary, doc or needs (phase IDs,
// comma-separated; "" for none).
func (m *Mull) PhaseSet(id, field, value string) (*Phase, error) {
	var out *Phase
	err := m.update("phase set "+id+" "+field, func(s *state) error {
		p, err := s.phase(id)
		if err != nil {
			return err
		}
		switch field {
		case "title":
			p.Title = value
		case "summary":
			p.Summary = value
		case "doc":
			p.Doc = strings.TrimSpace(value)
			s.tx.Put(phaseDocPath(id), []byte(p.Doc+"\n"))
		case "needs":
			if err := s.setNeeds(p, strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' })); err != nil {
				return err
			}
		default:
			return errf(CodeInvalid, "a phase's fields are title, summary, doc and needs")
		}
		p.Updated = m.now()
		out = p
		return s.putPhase(p)
	})
	return out, err
}

func (s *state) setNeeds(p *Phase, needs []string) error {
	needs = dedupe(needs)
	for _, n := range needs {
		q, err := s.phase(n)
		if err != nil {
			return err
		}
		if q.Plan != p.Plan || n == p.ID {
			return errf(CodeInvalid, "phase %s can't need %s: needs are between phases of one plan", p.ID, n)
		}
	}
	p.Needs = needs
	seen := map[string]int{} // 1 visiting, 2 done
	var visit func(id string) bool
	visit = func(id string) bool {
		switch seen[id] {
		case 1:
			return false
		case 2:
			return true
		}
		seen[id] = 1
		if ph := s.phases[id]; ph != nil {
			for _, n := range ph.Needs {
				if !visit(n) {
					return false
				}
			}
		}
		seen[id] = 2
		return true
	}
	if !visit(p.ID) {
		return errf(CodeCycle, "those needs would make a cycle")
	}
	return s.putPhase(p)
}

// phaseItems are a phase's items, in ID order.
func (s *state) phaseItems(id string) []*Item {
	var out []*Item
	for _, k := range sortedKeys(s.items) {
		if i := s.items[k]; i.Phase == id {
			out = append(out, i)
		}
	}
	return out
}

// complete reports whether a phase is done: all its committed work done or
// dropped, at least one done. A phase whose work was all dropped (cut back
// to an outline) isn't.
func (s *state) complete(id string) bool {
	done := 0
	for _, i := range s.phaseItems(id) {
		switch i.State {
		case Captured, Dropped:
		case Done:
			done++
		default:
			return false
		}
	}
	return done > 0
}

// blockedBy are a phase's needs that aren't complete.
func (s *state) blockedBy(id string) []string {
	var out []string
	if p := s.phases[id]; p != nil {
		for _, n := range p.Needs {
			if !s.complete(n) {
				out = append(out, n)
			}
		}
	}
	return out
}

// Progress counts items by stage; Total is committed work, every stage but
// captured.
type Progress struct {
	Total     int `json:"total"`
	Queued    int `json:"queued"`
	Committed int `json:"committed"`
	Active    int `json:"active"`
	InReview  int `json:"in_review"`
	Done      int `json:"done"`
	Dropped   int `json:"dropped"`
}

func progress(items []*Item) Progress {
	var p Progress
	for _, i := range items {
		switch i.State {
		case Captured:
			p.Queued++
			continue
		case Committed:
			p.Committed++
		case Active:
			p.Active++
		case InReview:
			p.InReview++
		case Done:
			p.Done++
		case Dropped:
			p.Dropped++
		}
		p.Total++
	}
	return p
}

func (p Progress) String() string {
	parts := []string{fmt.Sprintf("%d/%d done", p.Done, p.Total-p.Dropped)}
	for _, c := range []struct {
		n    int
		what string
	}{{p.Active, "active"}, {p.InReview, "in review"}, {p.Committed, "to do"}, {p.Queued, "queued"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.what))
		}
	}
	return strings.Join(parts, " · ")
}

// ---- Items ----

// NewItem is an item to add: committed work, or (Kind set) queued work.
type NewItem struct {
	Plan, Phase, Title, Notes, Area, From string
	Kind, Source                          string // bug or idea, and where it came from: queued
}

// Add adds an item. With Kind (bug or idea) it's queued: captured, the
// agents' to pick up; otherwise it's committed work.
func (m *Mull) Add(in NewItem) (*Item, error) {
	var out *Item
	err := m.update("add: "+in.Title, func(s *state) error {
		var err error
		out, err = m.add(s, in)
		return err
	})
	return out, err
}

func (m *Mull) add(s *state, in NewItem) (*Item, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, errf(CodeInvalid, "an item needs a title")
	}
	if in.Kind != "" && in.Kind != Bug && in.Kind != Idea {
		return nil, errf(CodeInvalid, "a queued item is a bug or an idea, not %q", in.Kind)
	}
	plan, err := s.resolve(in.Plan, in.Phase)
	if err != nil {
		return nil, err
	}
	if in.From != "" {
		if _, err := s.item(in.From); err != nil {
			return nil, err
		}
	}
	stage := Committed
	if in.Kind != "" {
		stage = Captured
	} else if p := s.plans[plan]; p != nil && p.State == Shaping {
		return nil, errf(CodeState, "plan %q is shaping: it takes queued items, not committed work, until the human agrees it", plan)
	}
	id, err := s.newID()
	if err != nil {
		return nil, err
	}
	now := m.now()
	i := &Item{ID: id, Plan: plan, Phase: in.Phase, Title: strings.TrimSpace(in.Title), State: stage, Kind: in.Kind,
		Area: strings.TrimSpace(in.Area), Source: in.Source, From: in.From, Created: now, Updated: now, Notes: in.Notes}
	return i, s.putItem(i)
}

// resolve checks a plan and phase given together, returning the plan:
// a phase implies its plan, and "none" or "" is unplanned.
func (s *state) resolve(plan, phase string) (string, error) {
	if plan == NoPlan {
		plan = ""
	}
	if phase != "" {
		p, err := s.phase(phase)
		if err != nil {
			return "", err
		}
		if plan != "" && plan != p.Plan {
			return "", errf(CodeInvalid, "phase %s is in plan %q, not %q", phase, p.Plan, plan)
		}
		return p.Plan, nil
	}
	if plan != "" {
		p, err := s.plan(plan)
		if err != nil {
			return "", err
		}
		if p.State == PlanDone {
			return "", errf(CodeState, "plan %q is done", plan)
		}
	}
	return plan, nil
}

// Set changes an item's title, notes, area, kind, plan or phase. Moving
// is a move, for work nobody's on yet.
func (m *Mull) Set(id, field, value string) (*Item, error) {
	var out *Item
	err := m.update("set "+id+" "+field, func(s *state) error {
		i, err := s.item(id)
		if err != nil {
			return err
		}
		switch field {
		case "title":
			i.Title = value
		case "notes":
			i.Notes = value
		case "area":
			i.Area = strings.TrimSpace(value)
		case "kind":
			if i.State != Captured || (value != Bug && value != Idea) {
				return errf(CodeInvalid, "only a queued item has a kind: bug or idea")
			}
			i.Kind = value
		case "plan", "phase":
			if i.State != Captured && i.State != Committed {
				return errf(CodeState, "item %s is %s; only queued or committed work moves", id, i.State)
			}
			plan, phase := value, ""
			if field == "phase" {
				plan, phase = "", value
			}
			if i.Plan, err = s.resolve(plan, phase); err != nil {
				return err
			}
			i.Phase = phase
		default:
			return errf(CodeInvalid, "an item's fields are title, notes, area, kind, plan and phase")
		}
		i.Updated = m.now()
		out = i
		return s.putItem(i)
	})
	return out, err
}

// Commit turns a queued item into work, optionally moving it.
func (m *Mull) Commit(id, plan, phase string) (*Item, error) {
	var out *Item
	err := m.update("commit "+id, func(s *state) error {
		i, err := s.item(id)
		if err != nil {
			return err
		}
		if i.State != Captured {
			return errf(CodeState, "item %s is %s, not queued", id, i.State)
		}
		if plan != "" || phase != "" {
			if i.Plan, err = s.resolve(plan, phase); err != nil {
				return err
			}
			i.Phase = phase
		}
		i.State, i.Updated = Committed, m.now()
		out = i
		return s.putItem(i)
	})
	return out, err
}

// DropItems drops items that aren't done, with a reason.
func (m *Mull) DropItems(ids []string, reason string) ([]*Item, error) {
	var out []*Item
	err := m.update("drop "+strings.Join(ids, " "), func(s *state) error {
		for _, id := range dedupe(ids) {
			i, err := s.item(id)
			if err != nil {
				return err
			}
			if i.State == Done {
				return errf(CodeState, "item %s is done", id)
			}
			i.State, i.Claim, i.Updated = Dropped, nil, m.now()
			if reason != "" {
				i.Notes = joinNotes(i.Notes, "Dropped: "+reason)
			}
			if err := s.putItem(i); err != nil {
				return err
			}
			out = append(out, i)
		}
		return nil
	})
	return out, err
}

// Filter selects items.
type Filter struct {
	Plan, Phase, State, Kind string
}

// List lists items, plan by plan (unplanned last), oldest first.
func (m *Mull) List(f Filter) ([]*Item, error) {
	out := []*Item{}
	err := m.view(func(s *state) error {
		for _, id := range sortedKeys(s.items) {
			i := s.items[id]
			if (f.Plan == NoPlan && i.Plan != "") || (f.Plan != "" && f.Plan != NoPlan && i.Plan != f.Plan) ||
				(f.Phase != "" && i.Phase != f.Phase) || (f.State != "" && i.State != f.State) || (f.Kind != "" && i.Kind != f.Kind) {
				continue
			}
			out = append(out, i)
		}
		return nil
	})
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Plan != out[b].Plan {
			return out[b].Plan == "" || (out[a].Plan != "" && out[a].Plan < out[b].Plan)
		}
		return out[a].Created < out[b].Created
	})
	return out, err
}

// Shown is any record shown by ID or slug.
type Shown struct {
	Type    string   `json:"type"`
	Plan    *PlanOut `json:"plan,omitempty"`
	Phase   *Phase   `json:"phase,omitempty"`
	Items   []*Item  `json:"items,omitempty"`
	Item    *Item    `json:"item,omitempty"`
	Reports []Report `json:"reports,omitempty"`
	Entry   *Entry   `json:"entry,omitempty"`
}

// PlanOut is a plan with its phases on the hill and its items.
type PlanOut struct {
	Plan
	Phases []PhaseHill `json:"phases"`
	Items  []*Item     `json:"items"`
}

// Show returns a plan, phase, item (with its reports) or inbox entry.
func (m *Mull) Show(ref string) (Shown, error) {
	var out Shown
	err := m.view(func(s *state) error {
		switch {
		case s.plans[ref] != nil:
			p := s.plans[ref]
			po := &PlanOut{Plan: *p, Phases: []PhaseHill{}, Items: []*Item{}}
			for _, ph := range s.planPhases(ref) {
				po.Phases = append(po.Phases, s.hill(ph))
			}
			for _, id := range sortedKeys(s.items) {
				if i := s.items[id]; i.Plan == ref {
					po.Items = append(po.Items, i)
				}
			}
			out = Shown{Type: "plan", Plan: po}
		case s.phases[ref] != nil:
			out = Shown{Type: "phase", Phase: s.phases[ref], Items: s.phaseItems(ref)}
		case s.items[ref] != nil:
			out = Shown{Type: "item", Item: s.items[ref], Reports: s.reports(ref)}
		case s.entries[ref] != nil:
			out = Shown{Type: "entry", Entry: s.entries[ref]}
		default:
			return errf(CodeNotFound, "no plan, phase, item or entry %q", ref)
		}
		return nil
	})
	return out, err
}

// planPhases are a plan's phases, needs first, then oldest.
func (s *state) planPhases(slug string) []*Phase {
	var out []*Phase
	for _, id := range sortedKeys(s.phases) {
		if p := s.phases[id]; p.Plan == slug {
			out = append(out, p)
		}
	}
	depth := map[string]int{}
	var d func(id string) int
	d = func(id string) int {
		if v, ok := depth[id]; ok {
			return v
		}
		depth[id] = 0
		best := 0
		if p := s.phases[id]; p != nil {
			for _, n := range p.Needs {
				best = max(best, d(n)+1)
			}
		}
		depth[id] = best
		return best
	}
	sort.SliceStable(out, func(a, b int) bool {
		if da, db := d(out[a].ID), d(out[b].ID); da != db {
			return da < db
		}
		return out[a].Created < out[b].Created
	})
	return out
}

// PhaseHill is a phase's place on the hill: uphill is being worked out
// (nothing started), downhill is executing.
type PhaseHill struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Hill      string   `json:"hill"` // uphill, downhill, waiting, done
	Progress  Progress `json:"progress"`
	BlockedBy []string `json:"blocked_by,omitempty"`
}

func (s *state) hill(p *Phase) PhaseHill {
	h := PhaseHill{ID: p.ID, Title: p.Title, Progress: progress(s.phaseItems(p.ID)), BlockedBy: s.blockedBy(p.ID)}
	switch {
	case s.complete(p.ID):
		h.Hill = "done"
	case len(h.BlockedBy) > 0:
		h.Hill = "waiting"
	case h.Progress.Done+h.Progress.Active+h.Progress.InReview == 0:
		h.Hill = "uphill"
	default:
		h.Hill = "downhill"
	}
	return h
}

// Ready lists committed work whose phase can start, plan by plan.
func (m *Mull) Ready() ([]*Item, error) {
	out := []*Item{}
	err := m.view(func(s *state) error {
		for _, id := range sortedKeys(s.items) {
			if i := s.items[id]; s.ready(i) {
				out = append(out, i)
			}
		}
		return nil
	})
	return out, err
}

func (s *state) ready(i *Item) bool {
	if i.State != Committed {
		return false
	}
	if p := s.plans[i.Plan]; p != nil && p.State != PlanActive {
		return false
	}
	return i.Phase == "" || len(s.blockedBy(i.Phase)) == 0
}
