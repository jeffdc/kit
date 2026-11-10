package mull

import (
	"fmt"
	"sort"

	"mull/internal/model"
)

// Which settle actions each entry kind allows.
//
//	decision:  accept (ratify), decide (overrule), drop, park
//	question:  decide, do, drop, park
//	loose-end: decide, do, drop, park
//	review:    accept (optionally with a follow-up item), do (send back: reopen the item), park
//	idea:      do (promote to an item or phase; only on the human's say-so), drop, park
var allowed = map[string][]string{
	model.KindIdea:     {model.ActDo, model.ActDrop, model.ActPark},
	model.KindDecision: {model.ActAccept, model.ActDecide, model.ActDrop, model.ActPark},
	model.KindQuestion: {model.ActDecide, model.ActDo, model.ActDrop, model.ActPark},
	model.KindLoose:    {model.ActDecide, model.ActDo, model.ActDrop, model.ActPark},
	model.KindReview:   {model.ActAccept, model.ActDo, model.ActPark},
}

func allows(kind, action string) bool {
	for _, a := range allowed[kind] {
		if a == action {
			return true
		}
	}
	return false
}

// CaptureInput is an inbox entry an agent files.
type CaptureInput struct {
	Kind   string // decision, question, loose-end or idea
	Text   string // for an idea, its title
	Item   string // the item it concerns; implies the plan (not for ideas)
	Plan   string // the plan, when there is no item (optional for ideas)
	Source string // where it came from
	Detail string // more detail: for a decision, why
}

// Capture files an inbox entry.
func (m *Mull) Capture(in CaptureInput) (*model.Entry, error) {
	var out *model.Entry
	err := m.update(fmt.Sprintf("%s: %s", in.Kind, in.Text), func(s *state) error {
		var err error
		out, err = m.capture(s, in)
		return err
	})
	return out, err
}

func (m *Mull) capture(s *state, in CaptureInput) (*model.Entry, error) {
	switch in.Kind {
	case model.KindDecision, model.KindQuestion:
	case model.KindLoose, model.KindIdea:
		return nil, errf(CodeRetired, "%s", retiredCapture)
	default:
		return nil, errf(CodeInvalid, "cannot capture an entry of kind %q", in.Kind)
	}
	if in.Text == "" {
		return nil, errf(CodeInvalid, "an inbox entry needs text")
	}
	if in.Item == "" && in.Plan == "" && s.openPlans() > 1 {
		return nil, errf(CodeScope, "this repository has more than one plan: say what the entry is about with --item or --plan")
	}
	return m.newEntry(s, in.Kind, in.Text, in.Item, in.Plan, in.Source, in.Detail)
}

// retiredCapture points from mull loose and inbox ideas to captures.
const retiredCapture = "loose ends and inbox ideas are retired: capture it as an item with " +
	"mull bug \"...\" or mull idea \"...\" (--source says where it came from; --plan or --phase tag it)"

// openPlans counts the plans not done: shaping or active.
func (s *state) openPlans() int {
	n := 0
	for _, p := range s.plans {
		if p.State != model.PlanDone {
			n++
		}
	}
	return n
}

func (m *Mull) newEntry(s *state, kind, text, item, plan, source, detail string) (*model.Entry, error) {
	if item != "" {
		i, err := s.item(item)
		if err != nil {
			return nil, err
		}
		if plan != "" && plan != i.Plan {
			return nil, errf(CodeInvalid, "item %s belongs to plan %q, not %q", item, i.Plan, plan)
		}
		plan = i.Plan
	}
	if plan != "" {
		if _, err := s.plan(plan); err != nil {
			return nil, err
		}
	}
	id, err := s.newID()
	if err != nil {
		return nil, err
	}
	e := &model.Entry{
		ID: id, Kind: kind, Text: text, Plan: plan, Item: item, Source: source,
		By: m.Worktree(), Created: m.now(), Detail: detail,
	}
	return e, s.putEntry(e)
}

// InboxFilter selects inbox entries. By default only unsettled entries,
// and never the retired kinds (loose ends, ideas) unless Kind names one:
// needs you is questions, decisions and reviews. Plan "none" selects
// entries with no plan.
type InboxFilter struct {
	Plan, Kind string
	Parked     bool // include parked entries
	All        bool // include every settled entry
}

// Inbox lists entries, oldest first.
func (m *Mull) Inbox(f InboxFilter) ([]*model.Entry, error) {
	out := []*model.Entry{}
	err := m.view(func(s *state) error {
		plan := f.Plan
		if plan == model.NoPlan {
			plan = ""
		} else if plan != "" {
			if _, err := s.plan(plan); err != nil {
				return err
			}
		}
		for _, id := range sortedKeys(s.entries) {
			e := s.entries[id]
			if (f.Plan != "" && e.Plan != plan) || (f.Kind != "" && e.Kind != f.Kind) {
				continue
			}
			if retiredKind(e.Kind) && f.Kind != e.Kind {
				continue
			}
			parked := e.Settled != nil && e.Settled.Action == model.ActPark
			if e.Settled != nil && !f.All && !(f.Parked && parked) {
				continue
			}
			out = append(out, e)
		}
		return nil
	})
	sort.SliceStable(out, func(a, b int) bool { return out[a].Created < out[b].Created })
	return out, err
}

func retiredKind(kind string) bool { return kind == model.KindLoose || kind == model.KindIdea }

// SettleOpts are the common options for settling.
type SettleOpts struct {
	// OnRequest records that an agent settled this on the human's explicit instruction.
	OnRequest bool
}

func (m *Mull) settle(s *state, e *model.Entry, action, note, item string, o SettleOpts) error {
	if e.Settled != nil && e.Settled.Action != model.ActPark {
		return errf(CodeSettled, "inbox entry %s is already settled (%s)", e.ID, e.Settled.Action)
	}
	if !allows(e.Kind, action) {
		return errf(CodeNotAllowed, "a %s cannot be settled with %q (allowed: %v)", e.Kind, action, allowed[e.Kind])
	}
	e.Settled = &model.Settlement{
		Action: action, Note: note, Item: item, By: m.Worktree(), OnRequest: o.OnRequest, At: m.now(),
	}
	return s.putEntry(e)
}

// openReview returns the item's unsettled (or parked) review entry, if any.
func (s *state) openReview(itemID string) *model.Entry {
	for _, id := range sortedKeys(s.entries) {
		e := s.entries[id]
		if e.Kind == model.KindReview && e.Item == itemID &&
			(e.Settled == nil || e.Settled.Action == model.ActPark) {
			return e
		}
	}
	return nil
}

// AcceptResult reports what accept did.
type AcceptResult struct {
	Entries  []*model.Entry `json:"entries"`
	Items    []ItemView     `json:"items"`
	FollowUp *ItemView      `json:"follow_up"`
}

// Accept ratifies decisions and accepts reviewed work. refs may be decision
// or review entries, or items in review (whose review is settled with them).
// followUp, if set, creates a follow-up item for an accepted review; it
// applies only when accepting a single review.
func (m *Mull) Accept(refs []string, followUp string, o SettleOpts) (AcceptResult, error) {
	var out AcceptResult
	err := m.update(fmt.Sprintf("accept %v", refs), func(s *state) error {
		var err error
		out, err = m.accept(s, refs, followUp, o)
		return err
	})
	return out, err
}

func (m *Mull) accept(s *state, refs []string, followUp string, o SettleOpts) (AcceptResult, error) {
	out := AcceptResult{Entries: []*model.Entry{}, Items: []ItemView{}}
	refs = dedupe(refs)
	if len(refs) == 0 {
		return out, errf(CodeInvalid, "give the entries or items to accept")
	}
	for _, ref := range refs {
		var entry *model.Entry
		var item *model.Item
		switch {
		case s.entries[ref] != nil:
			entry = s.entries[ref]
			if entry.Kind == model.KindReview {
				var err error
				if item, err = s.item(entry.Item); err != nil {
					return out, err
				}
			}
		case s.items[ref] != nil:
			item = s.items[ref]
			entry = s.openReview(ref)
		default:
			return out, errf(CodeNotFound, "no inbox entry or item %q", ref)
		}
		if item != nil {
			if err := m.acceptItem(s, item, o); err != nil {
				return out, err
			}
			out.Items = append(out.Items, s.itemView(item))
		}
		if entry != nil {
			if err := m.settle(s, entry, model.ActAccept, "", "", o); err != nil {
				return out, err
			}
			out.Entries = append(out.Entries, entry)
		}
	}
	if followUp != "" {
		if len(out.Items) != 1 {
			return out, errf(CodeInvalid, "follow-up applies when accepting exactly one reviewed item")
		}
		src := s.items[out.Items[0].ID]
		plan := src.Plan
		if plan == "" {
			plan = model.NoPlan
		}
		i, err := m.addItem(s, NewItem{Plan: plan, Phase: src.Phase, Title: followUp,
			Notes: fmt.Sprintf("Follow-up from the review of %s.", src.ID)})
		if err != nil {
			return out, err
		}
		v := s.itemView(i)
		out.FollowUp = &v
	}
	return out, nil
}

// acceptItem marks work in review done. The worker that claimed the item may
// not accept it from its own worktree unless the human asked it to.
func (m *Mull) acceptItem(s *state, i *model.Item, o SettleOpts) error {
	if i.State != model.StateInReview {
		return errf(CodeState, "item %s is %s; only items in review can be accepted", i.ID, i.State)
	}
	if i.Claim != nil && i.Claim.Worktree == m.Worktree() && !o.OnRequest {
		return errf(CodeNotAllowed, "item %s was done from this worktree; a worker cannot accept its own work unless the human asks (use --on-request)", i.ID)
	}
	i.State, i.Updated = model.StateDone, m.now()
	return s.putItem(i)
}

// Decide settles an entry with the human's decision, which joins the decision log.
func (m *Mull) Decide(id, decision string, o SettleOpts) (*model.Entry, error) {
	var out *model.Entry
	err := m.update("decide "+id, func(s *state) error {
		var err error
		out, err = m.decide(s, id, decision, o)
		return err
	})
	return out, err
}

func (m *Mull) decide(s *state, id, decision string, o SettleOpts) (*model.Entry, error) {
	if decision == "" {
		return nil, errf(CodeInvalid, "decide needs the decision text")
	}
	e, err := s.entry(id)
	if err != nil {
		return nil, err
	}
	return e, m.settle(s, e, model.ActDecide, decision, "", o)
}

// DoResult reports what do did: the settled entry and the item or phase it
// became (or, for a review, the reopened item).
type DoResult struct {
	Entry *model.Entry `json:"entry"`
	Item  *ItemView    `json:"item,omitempty"`
	Phase *PhaseView   `json:"phase,omitempty"`
}

// DoInput says what to do with a question, loose end, idea or review.
type DoInput struct {
	ID, Plan, Phase, Reason string
	// As is "item" (the default) or "phase": what the entry becomes.
	As string
	SettleOpts
}

// Do turns a question, loose end or idea into an item (or a phase, with As
// "phase"), linked back to it; or sends reviewed work back (the item reopens
// for another session, with the reason added to its notes). Only the human
// promotes ideas: an agent must have been asked, and say so with OnRequest.
func (m *Mull) Do(in DoInput) (DoResult, error) {
	var out DoResult
	err := m.update("do "+in.ID, func(s *state) error {
		var err error
		out, err = m.do(s, in)
		return err
	})
	return out, err
}

func (m *Mull) do(s *state, in DoInput) (DoResult, error) {
	e, err := s.entry(in.ID)
	if err != nil {
		return DoResult{}, err
	}
	as := in.As
	if as == "" {
		as = "item"
	}
	if as != "item" && as != "phase" {
		return DoResult{}, errf(CodeInvalid, "--as must be item or phase, not %q", as)
	}
	if e.Kind == model.KindReview {
		if as != "item" {
			return DoResult{}, errf(CodeInvalid, "a review sends its item back; it can't become a phase")
		}
		i, err := s.item(e.Item)
		if err != nil {
			return DoResult{}, err
		}
		if i.State != model.StateInReview {
			return DoResult{}, errf(CodeState, "item %s is %s; only items in review can be sent back", i.ID, i.State)
		}
		if in.Reason == "" {
			return DoResult{}, errf(CodeInvalid, "say why the work is sent back (--reason)")
		}
		if err := m.settle(s, e, model.ActDo, in.Reason, i.ID, in.SettleOpts); err != nil {
			return DoResult{}, err
		}
		i.State, i.Claim, i.Updated = model.StateCommitted, nil, m.now()
		i.Notes = joinNotes(i.Notes, fmt.Sprintf("Sent back on review (%s): %s", m.now(), in.Reason))
		if err := s.putItem(i); err != nil {
			return DoResult{}, err
		}
		v := s.itemView(i)
		return DoResult{Entry: e, Item: &v}, nil
	}
	if !allows(e.Kind, model.ActDo) {
		return DoResult{}, errf(CodeNotAllowed, "a %s cannot be settled with %q (allowed: %v)", e.Kind, model.ActDo, allowed[e.Kind])
	}
	if e.Kind == model.KindIdea && !in.OnRequest {
		return DoResult{}, errf(CodeNotAllowed, "only the human promotes ideas: pass --on-request when the human has asked for idea %s to become work", e.ID)
	}
	if e.Settled != nil && e.Settled.Action != model.ActPark {
		return DoResult{}, errf(CodeSettled, "inbox entry %s is already settled (%s)", e.ID, e.Settled.Action)
	}
	plan := in.Plan
	if plan == "" && in.Phase == "" {
		plan = e.Plan
	}
	if plan == "" && in.Phase == "" {
		return DoResult{}, errf(CodeScope, "%s %s has no plan: give --plan (or --phase), or --plan none for unplanned work", e.Kind, e.ID)
	}
	notes := fmt.Sprintf("From inbox %s (%s).", e.ID, e.Kind)
	if e.Detail != "" {
		notes = joinNotes(notes, e.Detail)
	}
	if e.Source != "" {
		notes = joinNotes(notes, "Source: "+e.Source)
	}
	if as == "phase" {
		if in.Phase != "" {
			return DoResult{}, errf(CodeInvalid, "a phase can't go inside a phase: drop --phase")
		}
		ep, err := m.phaseAdd(s, plan, e.Text, notes, nil, e.ID)
		if err != nil {
			return DoResult{}, err
		}
		if err := m.settleDo(s, e, in, "", ep.ID); err != nil {
			return DoResult{}, err
		}
		v := s.phaseView(ep)
		return DoResult{Entry: e, Phase: &v}, nil
	}
	i, err := m.addItem(s, NewItem{Plan: plan, Phase: in.Phase, Title: e.Text, Notes: notes, From: e.ID})
	if err != nil {
		return DoResult{}, err
	}
	if err := m.settleDo(s, e, in, i.ID, ""); err != nil {
		return DoResult{}, err
	}
	v := s.itemView(i)
	return DoResult{Entry: e, Item: &v}, nil
}

func (m *Mull) settleDo(s *state, e *model.Entry, in DoInput, item, phase string) error {
	if err := m.settle(s, e, model.ActDo, in.Reason, item, in.SettleOpts); err != nil {
		return err
	}
	e.Settled.Phase = phase
	return s.putEntry(e)
}

func joinNotes(a, b string) string {
	if a == "" {
		return b
	}
	return a + "\n\n" + b
}

// Park sets an entry aside with a reason. It stays listed as parked.
func (m *Mull) Park(id, reason string, o SettleOpts) (*model.Entry, error) {
	var out *model.Entry
	err := m.update("park "+id, func(s *state) error {
		var err error
		out, err = m.park(s, id, reason, o)
		return err
	})
	return out, err
}

func (m *Mull) park(s *state, id, reason string, o SettleOpts) (*model.Entry, error) {
	if reason == "" {
		return nil, errf(CodeInvalid, "park needs a reason")
	}
	e, err := s.entry(id)
	if err != nil {
		return nil, err
	}
	return e, m.settle(s, e, model.ActPark, reason, "", o)
}

// DropResult reports what drop did.
type DropResult struct {
	Entries []*model.Entry `json:"entries"`
	Items   []ItemView     `json:"items"`
}

// Drop drops inbox entries (gone for good, recorded) and items (state dropped).
func (m *Mull) Drop(refs []string, reason string, o SettleOpts) (DropResult, error) {
	var out DropResult
	err := m.update(fmt.Sprintf("drop %v", refs), func(s *state) error {
		var err error
		out, err = m.drop(s, refs, reason, o)
		return err
	})
	return out, err
}

func (m *Mull) drop(s *state, refs []string, reason string, o SettleOpts) (DropResult, error) {
	out := DropResult{Entries: []*model.Entry{}, Items: []ItemView{}}
	refs = dedupe(refs)
	if len(refs) == 0 {
		return out, errf(CodeInvalid, "give the entries or items to drop")
	}
	for _, ref := range refs {
		switch {
		case s.entries[ref] != nil:
			e := s.entries[ref]
			if err := m.settle(s, e, model.ActDrop, reason, "", o); err != nil {
				return out, err
			}
			out.Entries = append(out.Entries, e)
		case s.items[ref] != nil:
			i := s.items[ref]
			if i.State == model.StateDone {
				return out, errf(CodeState, "item %s is done and cannot be dropped", i.ID)
			}
			i.State, i.Updated = model.StateDropped, m.now()
			if reason != "" {
				i.Notes = joinNotes(i.Notes, "Dropped: "+reason)
			}
			if err := s.putItem(i); err != nil {
				return out, err
			}
			out.Items = append(out.Items, s.itemView(i))
		default:
			return out, errf(CodeNotFound, "no inbox entry or item %q", ref)
		}
	}
	return out, nil
}

// Decision is one entry in the decision log.
type Decision struct {
	ID        string `json:"id"`
	Plan      string `json:"plan"`
	Item      string `json:"item,omitempty"`
	Kind      string `json:"kind"`
	Context   string `json:"context"`  // what was asked or decided by the agent
	Decision  string `json:"decision"` // what holds now
	By        string `json:"by"`
	OnRequest bool   `json:"on_request,omitempty"`
	At        string `json:"at"`
}

// Decisions returns the decision log, newest first: ratified agent decisions
// and the human's own decisions.
func (m *Mull) Decisions(plan string) ([]Decision, error) {
	out := []Decision{}
	err := m.view(func(s *state) error {
		if plan != "" {
			if _, err := s.plan(plan); err != nil {
				return err
			}
		}
		for _, id := range sortedKeys(s.entries) {
			e := s.entries[id]
			if e.Settled == nil || (plan != "" && e.Plan != plan) {
				continue
			}
			d := Decision{ID: e.ID, Plan: e.Plan, Item: e.Item, Kind: e.Kind, Context: e.Text,
				By: e.Settled.By, OnRequest: e.Settled.OnRequest, At: e.Settled.At}
			switch {
			case e.Settled.Action == model.ActDecide:
				d.Decision = e.Settled.Note
			case e.Settled.Action == model.ActAccept && e.Kind == model.KindDecision:
				d.Decision = e.Text
			default:
				continue
			}
			out = append(out, d)
		}
		return nil
	})
	sort.SliceStable(out, func(a, b int) bool { return out[a].At > out[b].At })
	return out, err
}
