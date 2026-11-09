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
var allowed = map[string][]string{
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

// Capture files an inbox entry. The entry's plan comes from its item, then
// from plan, then defaults to the general plan.
func (m *Mull) Capture(kind, text, item, plan, source, detail string) (*model.Entry, error) {
	switch kind {
	case model.KindDecision, model.KindQuestion, model.KindLoose:
	default:
		return nil, fmt.Errorf("cannot capture an entry of kind %q", kind)
	}
	if text == "" {
		return nil, fmt.Errorf("an inbox entry needs text")
	}
	var out *model.Entry
	err := m.update(fmt.Sprintf("%s: %s", kind, text), func(s *state) error {
		e, err := m.newEntry(s, kind, text, item, plan, source, detail)
		out = e
		return err
	})
	return out, err
}

func (m *Mull) newEntry(s *state, kind, text, item, plan, source, detail string) (*model.Entry, error) {
	if item != "" {
		i, err := s.item(item)
		if err != nil {
			return nil, err
		}
		if plan != "" && plan != i.Plan {
			return nil, fmt.Errorf("item %s belongs to plan %q, not %q", item, i.Plan, plan)
		}
		plan = i.Plan
	}
	if plan == "" {
		plan = model.GeneralPlan
	}
	if _, err := s.plan(plan); err != nil {
		return nil, err
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

// InboxFilter selects inbox entries. By default only unsettled entries.
type InboxFilter struct {
	Plan, Kind string
	Parked     bool // include parked entries
	All        bool // include every settled entry
}

// Inbox lists entries, oldest first.
func (m *Mull) Inbox(f InboxFilter) ([]*model.Entry, error) {
	out := []*model.Entry{}
	err := m.view(func(s *state) error {
		for _, id := range sortedKeys(s.entries) {
			e := s.entries[id]
			if (f.Plan != "" && e.Plan != f.Plan) || (f.Kind != "" && e.Kind != f.Kind) {
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

// SettleOpts are the common options for settling.
type SettleOpts struct {
	// OnRequest records that an agent settled this on the human's explicit instruction.
	OnRequest bool
}

func (m *Mull) settle(s *state, e *model.Entry, action, note, item string, o SettleOpts) error {
	if e.Settled != nil && e.Settled.Action != model.ActPark {
		return fmt.Errorf("inbox entry %s is already settled (%s)", e.ID, e.Settled.Action)
	}
	if !allows(e.Kind, action) {
		return fmt.Errorf("a %s cannot be settled with %q (allowed: %v)", e.Kind, action, allowed[e.Kind])
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
	Items    []ItemView     `json:"items,omitempty"`
	FollowUp *ItemView      `json:"follow_up,omitempty"`
}

// Accept ratifies decisions and accepts reviewed work. refs may be decision
// or review entries, or done items (whose review is settled with them).
// followUp, if set, creates a follow-up item for an accepted review; it
// applies only when accepting a single review.
func (m *Mull) Accept(refs []string, followUp string, o SettleOpts) (AcceptResult, error) {
	var out AcceptResult
	err := m.update(fmt.Sprintf("accept %v", refs), func(s *state) error {
		out = AcceptResult{}
		reviews := 0
		for _, ref := range dedupe(refs) {
			var entry *model.Entry
			var item *model.Item
			switch {
			case s.entries[ref] != nil:
				entry = s.entries[ref]
				if entry.Kind == model.KindReview {
					var err error
					if item, err = s.item(entry.Item); err != nil {
						return err
					}
				}
			case s.items[ref] != nil:
				item = s.items[ref]
				entry = s.openReview(ref)
			default:
				return fmt.Errorf("no inbox entry or item %q", ref)
			}
			if item != nil {
				reviews++
				if err := m.acceptItem(s, item, o); err != nil {
					return err
				}
				out.Items = append(out.Items, s.itemView(item))
			}
			if entry != nil {
				if err := m.settle(s, entry, model.ActAccept, "", "", o); err != nil {
					return err
				}
				out.Entries = append(out.Entries, entry)
			}
		}
		if followUp != "" {
			if reviews != 1 || len(out.Items) != 1 {
				return fmt.Errorf("--follow-up applies when accepting exactly one reviewed item")
			}
			src := s.items[out.Items[0].ID]
			i, err := m.addItem(s, NewItem{Plan: src.Plan, Epic: src.Epic, Title: followUp,
				Notes: fmt.Sprintf("Follow-up from the review of %s.", src.ID)})
			if err != nil {
				return err
			}
			v := s.itemView(i)
			out.FollowUp = &v
		}
		return nil
	})
	return out, err
}

// acceptItem marks done work accepted. The worker that claimed the item may
// not accept it from its own worktree unless the human asked it to.
func (m *Mull) acceptItem(s *state, i *model.Item, o SettleOpts) error {
	if i.State != model.StateDone {
		return fmt.Errorf("item %s is %s; only done items can be accepted", i.ID, i.State)
	}
	if i.Claim != nil && i.Claim.Worktree == m.Worktree() && !o.OnRequest {
		return fmt.Errorf("item %s was done from this worktree; a worker cannot accept its own work unless the human asks (use --on-request)", i.ID)
	}
	i.State, i.Updated = model.StateAccepted, m.now()
	return s.putItem(i)
}

// Decide settles an entry with the human's decision, which joins the decision log.
func (m *Mull) Decide(id, decision string, o SettleOpts) (*model.Entry, error) {
	if decision == "" {
		return nil, fmt.Errorf("decide needs the decision text")
	}
	var out *model.Entry
	err := m.update("decide "+id, func(s *state) error {
		e, err := s.entry(id)
		if err != nil {
			return err
		}
		out = e
		return m.settle(s, e, model.ActDecide, decision, "", o)
	})
	return out, err
}

// DoResult reports what do did.
type DoResult struct {
	Entry *model.Entry `json:"entry"`
	Item  ItemView     `json:"item"`
}

// Do turns a question or loose end into an item, or sends reviewed work back
// (the item reopens for another session, with the reason added to its notes).
func (m *Mull) Do(id, plan, epic, reason string, o SettleOpts) (DoResult, error) {
	var out DoResult
	err := m.update("do "+id, func(s *state) error {
		e, err := s.entry(id)
		if err != nil {
			return err
		}
		if e.Kind == model.KindReview {
			i, err := s.item(e.Item)
			if err != nil {
				return err
			}
			if i.State != model.StateDone {
				return fmt.Errorf("item %s is %s; only done items can be sent back", i.ID, i.State)
			}
			if reason == "" {
				return fmt.Errorf("say why the work is sent back (--reason)")
			}
			i.State, i.Claim, i.Updated = model.StateOpen, nil, m.now()
			i.Notes = joinNotes(i.Notes, fmt.Sprintf("Sent back on review (%s): %s", m.now(), reason))
			if err := s.putItem(i); err != nil {
				return err
			}
			out = DoResult{Entry: e, Item: s.itemView(i)}
			return m.settle(s, e, model.ActDo, reason, i.ID, o)
		}
		if plan == "" {
			plan = e.Plan
		}
		notes := fmt.Sprintf("From inbox %s (%s).", e.ID, e.Kind)
		if e.Detail != "" {
			notes = joinNotes(notes, e.Detail)
		}
		i, err := m.addItem(s, NewItem{Plan: plan, Epic: epic, Title: e.Text, Notes: notes})
		if err != nil {
			return err
		}
		out = DoResult{Entry: e, Item: s.itemView(i)}
		return m.settle(s, e, model.ActDo, reason, i.ID, o)
	})
	return out, err
}

func joinNotes(a, b string) string {
	if a == "" {
		return b
	}
	return a + "\n\n" + b
}

// Park sets an entry aside with a reason. It stays listed as parked.
func (m *Mull) Park(id, reason string, o SettleOpts) (*model.Entry, error) {
	if reason == "" {
		return nil, fmt.Errorf("park needs a reason")
	}
	var out *model.Entry
	err := m.update("park "+id, func(s *state) error {
		e, err := s.entry(id)
		if err != nil {
			return err
		}
		out = e
		return m.settle(s, e, model.ActPark, reason, "", o)
	})
	return out, err
}

// DropResult reports what drop did.
type DropResult struct {
	Entries []*model.Entry `json:"entries,omitempty"`
	Items   []ItemView     `json:"items,omitempty"`
}

// Drop drops inbox entries (gone for good, recorded) and items (state dropped).
func (m *Mull) Drop(refs []string, reason string, o SettleOpts) (DropResult, error) {
	var out DropResult
	err := m.update(fmt.Sprintf("drop %v", refs), func(s *state) error {
		out = DropResult{}
		for _, ref := range dedupe(refs) {
			switch {
			case s.entries[ref] != nil:
				e := s.entries[ref]
				if err := m.settle(s, e, model.ActDrop, reason, "", o); err != nil {
					return err
				}
				out.Entries = append(out.Entries, e)
			case s.items[ref] != nil:
				i := s.items[ref]
				if i.State == model.StateAccepted {
					return fmt.Errorf("item %s is accepted and cannot be dropped", i.ID)
				}
				i.State, i.Updated = model.StateDropped, m.now()
				if reason != "" {
					i.Notes = joinNotes(i.Notes, "Dropped: "+reason)
				}
				if err := s.putItem(i); err != nil {
					return err
				}
				out.Items = append(out.Items, s.itemView(i))
			default:
				return fmt.Errorf("no inbox entry or item %q", ref)
			}
		}
		return nil
	})
	return out, err
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
