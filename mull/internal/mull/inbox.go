package mull

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"mull/internal/model"
)

// Which settle actions each entry kind allows.
//
//	decision:  accept (ratify), decide (overrule), drop, park
//	question:  decide, do, drop, park
//	loose-end: decide, do, drop, park
//	review:    accept (optionally with a follow-up item), do (send back: reopen the item,
//	           or for a phase's integration review add a fix-up item to the phase), park
//	idea:      do (promote to an item or phase; only on the human's say-so), drop, park
var allowed = map[string][]string{
	model.KindIdea:     {model.ActDo, model.ActDrop, model.ActPark},
	model.KindDecision: {model.ActAccept, model.ActDecide, model.ActDrop, model.ActPark},
	model.KindQuestion: {model.ActDecide, model.ActDo, model.ActDrop, model.ActPark},
	model.KindLoose:    {model.ActDecide, model.ActDo, model.ActDrop, model.ActPark},
	model.KindReview:   {model.ActAccept, model.ActDo, model.ActDrop, model.ActPark},
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
	Kind   string // decision or question (loose-end and idea are retired: bugs and ideas are captured items)
	Text   string // for an idea, its title
	Item   string // the item it concerns; implies the plan (not for ideas)
	Plan   string // the plan, when there is no item (optional for ideas)
	Source string // where it came from
	Detail string // more detail: for a decision, why
	// Default, on a question (required), is what the agent does if the
	// human doesn't answer; By is when that default stands: a duration
	// from now (36h, 2d) or a date (2026-12-01).
	Default, By string
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
	var due string
	if in.Kind == model.KindQuestion {
		var err error
		if due, err = m.parseDue(in.By); err != nil {
			return nil, err
		}
	}
	e, err := m.newEntry(s, in.Kind, in.Text, in.Item, in.Plan, in.Source, in.Detail)
	if err != nil || in.Kind != model.KindQuestion {
		return e, err
	}
	e.Default, e.Due = strings.TrimSpace(in.Default), due
	return e, s.putEntry(e)
}

// parseDue turns --by into an RFC 3339 time: a duration from now (36h, 2d)
// or a date (2026-12-01, the end of that day UTC). Empty is no deadline.
func (m *Mull) parseDue(by string) (string, error) {
	by = strings.TrimSpace(by)
	if by == "" {
		return "", nil
	}
	now := m.Now().UTC()
	if n, ok := strings.CutSuffix(by, "d"); ok {
		if d, err := strconv.Atoi(n); err == nil && d > 0 {
			return now.Add(time.Duration(d) * 24 * time.Hour).Format(time.RFC3339), nil
		}
	}
	if d, err := time.ParseDuration(by); err == nil && d > 0 {
		return now.Add(d).Format(time.RFC3339), nil
	}
	if t, err := time.Parse("2006-01-02", by); err == nil {
		return t.Add(24*time.Hour - time.Second).Format(time.RFC3339), nil
	}
	if t, err := time.Parse(time.RFC3339, by); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	return "", errf(CodeInvalid, "--by %q: give a duration (36h, 2d) or a date (2026-12-01)", by)
}

// decided records a decision. An agent's own (human false) is noted at once:
// it goes into its item's done report and the decision log, never to the
// human's inbox (anything that needs the human is a question: mull ask). The human's own (human
// true), made in conversation and recorded by an agent on request, is settled
// at once as decided: straight into the decision log, never in the inbox and
// never carried by a review.
func (m *Mull) decided(s *state, in CaptureInput, human, onRequest bool) (*model.Entry, error) {
	if !human {
		if onRequest {
			return nil, errf(CodeInvalid, "--on-request goes with --human, for a decision the human made; an agent's own decision needs neither")
		}
		e, err := m.capture(s, in)
		if err != nil {
			return nil, err
		}
		e.Settled = &model.Settlement{Action: model.ActNote, By: m.Worktree(), At: e.Created}
		return e, s.putEntry(e)
	}
	if !m.asked(onRequest) {
		return nil, errf(CodeNotAllowed, "only the human makes their own decisions: pass --on-request when the human decided this in conversation (for a choice you made, drop --human)")
	}
	e, err := m.capture(s, in)
	if err != nil {
		return nil, err
	}
	return e, m.settle(s, e, model.ActDecide, e.Text, "", SettleOpts{OnRequest: onRequest})
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
			out = append(out, copyEntry(e))
		}
		return nil
	})
	sort.SliceStable(out, func(a, b int) bool { return out[a].Created < out[b].Created })
	return out, err
}

// copyEntry returns a copy of e that shares nothing with it: reads hand out
// copies, never the cached state's own records.
func copyEntry(e *model.Entry) *model.Entry {
	c := *e
	if e.Settled != nil {
		st := *e.Settled
		c.Settled = &st
	}
	c.Decisions = slices.Clone(e.Decisions)
	c.Items = slices.Clone(e.Items)
	return &c
}

// sendBack settles review r as sent back and reopens its item i for another
// session, with the reason added to its notes. Its decisions stay open.
func (m *Mull) sendBack(s *state, r *model.Entry, i *model.Item, reason string, o SettleOpts) error {
	if err := m.settle(s, r, model.ActDo, reason, i.ID, o); err != nil {
		return err
	}
	i.State, i.Claim, i.Updated = model.StateCommitted, nil, m.now()
	i.Notes = joinNotes(i.Notes, fmt.Sprintf("Sent back on review (%s): %s", m.now(), reason))
	return s.putItem(i)
}

// sendBackLanded settles a review of work that already landed (medium risk,
// or sampled) by filing a follow-up item: the landed item stays done, and
// the follow-up, in its plan and phase, carries the reason.
func (m *Mull) sendBackLanded(s *state, r *model.Entry, i *model.Item, in DoInput) (DoResult, error) {
	if in.Reason == "" {
		return DoResult{}, errf(CodeInvalid, "say what's wrong with the landed work (--reason); it becomes a follow-up item")
	}
	if r.Settled != nil && r.Settled.Action != model.ActPark {
		return DoResult{}, errf(CodeSettled, "inbox entry %s is already settled (%s)", r.ID, r.Settled.Action)
	}
	f, err := m.addItem(s, NewItem{FollowUp: i.ID, Title: "Fix " + i.ID + ": " + in.Reason,
		Notes: fmt.Sprintf("Sent back from the %s review of landed work %s (%s): %s", r.Landed, i.ID, r.ID, in.Reason)})
	if err != nil {
		return DoResult{}, err
	}
	if err := m.settle(s, r, model.ActDo, in.Reason, f.ID, in.SettleOpts); err != nil {
		return DoResult{}, err
	}
	v := s.itemView(f)
	return DoResult{Entry: r, Item: &v}, nil
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
	e.Settled = &model.Settlement{Action: action, Note: note, Item: item, By: m.Worktree(), OnRequest: o.OnRequest, At: m.now()}
	if m.human != "" {
		e.Settled.By, e.Settled.Human, e.Settled.OnRequest = m.human, true, false
	}
	return s.putEntry(e)
}

// openReview returns the item's unsettled (or parked) review entry, if any.
func (s *state) openReview(itemID string) *model.Entry {
	for _, e := range s.itemEntries(itemID) {
		if e.Kind == model.KindReview &&
			(e.Settled == nil || e.Settled.Action == model.ActPark) {
			return e
		}
	}
	return nil
}

// sessionDecisions returns the decisions an agent noted for the item since
// its claim (since), oldest first: what its done report lists.
func (s *state) sessionDecisions(itemID, since string) []*model.Entry {
	var out []*model.Entry
	for _, e := range s.itemEntries(itemID) {
		if e.Kind == model.KindDecision && e.Settled != nil && e.Settled.Action == model.ActNote && e.Created >= since {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Created < out[b].Created })
	return out
}

// openDecisions returns the item's unsettled decision entries, oldest first:
// what a review filed now would carry. Parked decisions are left out. Only
// decisions from before the redesign are open: an agent's are noted.
func (s *state) openDecisions(itemID string) []*model.Entry {
	var out []*model.Entry
	for _, e := range s.itemEntries(itemID) {
		if e.Kind == model.KindDecision && e.Settled == nil {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Created < out[b].Created })
	return out
}

// AcceptResult reports what accept did.
type AcceptResult struct {
	Entries  []*model.Entry `json:"entries"`
	Items    []ItemView     `json:"items"`
	FollowUp *ItemView      `json:"follow_up"`
	Warnings []string       `json:"warnings,omitempty"`
}

// Accept ratifies decisions and accepts reviewed work. refs may be decision
// or review entries, items in review (whose review is settled with them), or
// phases with an open integration review (which completes them). Accepting
// a review also ratifies the decisions it carries that are still unsettled.
// followUp, if set, creates a follow-up item for an accepted review; it
// applies only when accepting a single reviewed item.
func (m *Mull) Accept(refs []string, followUp string, o SettleOpts) (AcceptResult, error) {
	var out AcceptResult
	type landing struct {
		id, target, release, commit string
		edges                       []string
	}
	var lands []landing
	err := m.update(fmt.Sprintf("accept %v", refs), func(s *state) error {
		var err error
		lands = nil
		out, err = m.accept(s, refs, followUp, o)
		for _, v := range out.Items {
			if rep := s.latestReport(v.ID); rep != nil && rep.Passed {
				target, _ := s.targetOf(v.Plan)
				lands = append(lands, landing{v.ID, target, s.release(), rep.Commit, rep.NewEdges})
			}
		}
		return err
	})
	if err != nil {
		return out, err
	}
	// Accepted work lands on its target now, as done lands low-risk work.
	for _, l := range lands {
		if ok, why := m.landItem(l.target, l.release, l.commit); !ok {
			out.Warnings = append(out.Warnings, fmt.Sprintf("%s is accepted but not on %s yet: %s", l.id, l.target, why))
		}
		// The new connections it made are part of the map now.
		if err := m.addEdges(l.edges); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("recording %s's new connections: %v", l.id, err))
		}
	}
	return out, nil
}

func (m *Mull) accept(s *state, refs []string, followUp string, o SettleOpts) (AcceptResult, error) {
	out := AcceptResult{Entries: []*model.Entry{}, Items: []ItemView{}}
	refs = dedupe(refs)
	if len(refs) == 0 {
		return out, errf(CodeInvalid, "give the entries or items to accept")
	}
	named := map[string]bool{}
	for _, ref := range refs {
		named[ref] = true
	}
	for _, ref := range refs {
		var entry *model.Entry
		var item *model.Item
		switch {
		case s.entries[ref] != nil && s.entries[ref].Phase != "":
			return out, errf(CodeRetired, "entry %s is an integration review of a phase: those are retired (drop it)", ref)
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
		if entry != nil && entry.Landed != "" {
			item = nil // the work landed already: accepting says it looked fine
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
			carried, err := m.acceptCarried(s, entry, named, o)
			if err != nil {
				return out, err
			}
			out.Entries = append(out.Entries, carried...)
		}
	}
	if followUp != "" {
		if len(out.Items) != 1 {
			return out, errf(CodeInvalid, "follow-up applies when accepting exactly one reviewed item")
		}
		src := s.items[out.Items[0].ID]
		i, err := m.addItem(s, NewItem{FollowUp: src.ID, Title: followUp,
			Notes: fmt.Sprintf("Follow-up from the review of %s.", src.ID)})
		if err != nil {
			return out, err
		}
		v := s.itemView(i)
		out.FollowUp = &v
	}
	return out, nil
}

// acceptCarried ratifies the decisions an accepted review carries. Any
// settled or parked since done are left as they are (a decide overrides one),
// and so are any named in the same accept, which settle on their own.
func (m *Mull) acceptCarried(s *state, review *model.Entry, named map[string]bool, o SettleOpts) ([]*model.Entry, error) {
	var out []*model.Entry
	for _, id := range review.Decisions {
		d := s.entries[id]
		if d == nil || d.Settled != nil || named[id] {
			continue
		}
		if err := m.settle(s, d, model.ActAccept, "with review "+review.ID, "", o); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// acceptItem marks work in review done. The worker that claimed the item may
// not accept it from its own worktree unless the human asked it to.
func (m *Mull) acceptItem(s *state, i *model.Item, o SettleOpts) error {
	if i.State != model.StateInReview {
		return errf(CodeState, "item %s is %s; only items in review can be accepted", i.ID, i.State)
	}
	if i.Claim != nil && i.Claim.Worktree == m.Worktree() && !m.asked(o.OnRequest) {
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
	if err := m.settle(s, e, model.ActDecide, decision, "", o); err != nil {
		return nil, err
	}
	// A decision decided differently while its work waits for review means
	// the work isn't what the human accepts: it goes back, so an agent
	// takes the decision in, decides how to make the change, and files the
	// work for review again. Accepting can't happen meanwhile.
	if e.Kind == model.KindDecision && e.Item != "" {
		if i := s.items[e.Item]; i != nil && i.State == model.StateInReview {
			if r := s.openReview(i.ID); r != nil {
				reason := fmt.Sprintf("decision %s (%s) was decided differently: %s. Take it in (mull decided, saying how you'll make the change), make it, and finish again for review.", e.ID, e.Text, decision)
				if err := m.sendBack(s, r, i, reason, o); err != nil {
					return nil, err
				}
			}
		}
	}
	return e, nil
}

// DoResult reports what do did: the settled entry and the item or phase it
// became (or, for a review, the reopened item; for an integration review,
// the fix-up item it added to the phase).
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

// Do turns a question (or a loose end or idea filed before Model v2) into an
// item (or a phase, with As "phase"), linked back to it; or sends reviewed work back (the item reopens
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
	if e.Kind == model.KindReview && e.Phase != "" {
		return DoResult{}, errf(CodeRetired, "entry %s is an integration review of a phase: those are retired (drop it)", e.ID)
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
		if e.Landed != "" {
			return m.sendBackLanded(s, e, i, in)
		}
		if i.State != model.StateInReview {
			return DoResult{}, errf(CodeState, "item %s is %s; only items in review can be sent back", i.ID, i.State)
		}
		if in.Reason == "" {
			return DoResult{}, errf(CodeInvalid, "say why the work is sent back (--reason)")
		}
		if err := m.sendBack(s, e, i, in.Reason, in.SettleOpts); err != nil {
			return DoResult{}, err
		}
		v := s.itemView(i)
		return DoResult{Entry: e, Item: &v}, nil
	}
	if !allows(e.Kind, model.ActDo) {
		return DoResult{}, errf(CodeNotAllowed, "a %s cannot be settled with %q (allowed: %v)", e.Kind, model.ActDo, allowed[e.Kind])
	}
	if e.Kind == model.KindIdea && !m.asked(in.OnRequest) {
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
				by := ""
				if m.human != "" {
					by = " by " + m.human // the human dropped it, from the page
				}
				i.Notes = joinNotes(i.Notes, "Dropped"+by+": "+reason)
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
	Human     bool   `json:"human,omitempty"` // the human settled it themselves; By is their name
	OnRequest bool   `json:"on_request,omitempty"`
	At        string `json:"at"`
	// Noted means an agent's own choice, noted as made and never put to
	// the human; Why is the agent's reason.
	Noted bool   `json:"noted,omitempty"`
	Why   string `json:"why,omitempty"`
}

// Decisions returns the decision log, newest first: the human's own
// decisions, agent decisions they ratified, and agents' noted choices.
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
				By: e.Settled.By, Human: e.Settled.Human, OnRequest: e.Settled.OnRequest, At: e.Settled.At}
			switch {
			case e.Settled.Action == model.ActDecide:
				d.Decision = e.Settled.Note
			case e.Settled.Action == model.ActAccept && e.Kind == model.KindDecision:
				d.Decision = e.Text
			case e.Settled.Action == model.ActNote && e.Kind == model.KindDecision:
				d.Decision, d.Noted, d.Why = e.Text, true, e.Detail
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
