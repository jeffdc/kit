package core

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The inbox holds only what needs the human: questions, which carry the
// agent's default and when it stands, and reviews of high-risk work (or
// looks at landed work). An agent's decisions are noted, never inboxed.

// Ask files a question for the human. def is what the agent does if
// nobody answers; by is when that stands (36h, 2d, or a date).
func (m *Mull) Ask(text, item, plan, def, by, detail string) (*Entry, error) {
	var out *Entry
	err := m.update("ask: "+text, func(s *state) error {
		if strings.TrimSpace(def) == "" {
			return errf(CodeInvalid, "a question needs --default: what you'll do if nobody answers. Ask only about a rule conflict or something hard to undo")
		}
		due, err := m.parseDue(by)
		if err != nil {
			return err
		}
		e, err := m.newEntry(s, KindQuestion, text, item, plan, detail)
		if err != nil {
			return err
		}
		e.Default, e.Due = strings.TrimSpace(def), due
		out = e
		return s.putEntry(e)
	})
	return out, err
}

// Decided notes a decision. An agent's own goes into its report and the
// log, never to the human; with human (on request), it's the human's own,
// made in conversation.
func (m *Mull) Decided(text, item, plan, why string, human, onRequest bool) (*Entry, error) {
	var out *Entry
	err := m.update("decided: "+text, func(s *state) error {
		if human && !m.asked(onRequest) {
			return errf(CodeNotAllowed, "only the human makes their own decisions: pass --on-request when they decided this")
		}
		e, err := m.newEntry(s, KindDecision, text, item, plan, why)
		if err != nil {
			return err
		}
		act := ActNote
		if human {
			act = ActDecide
		}
		e.Settled = &Settlement{Action: act, Note: text, By: m.by(), Human: human, OnRequest: onRequest, At: e.Created}
		out = e
		return s.putEntry(e)
	})
	return out, err
}

func (m *Mull) newEntry(s *state, kind, text, item, plan, detail string) (*Entry, error) {
	if strings.TrimSpace(text) == "" {
		return nil, errf(CodeInvalid, "an entry needs text")
	}
	if item != "" {
		i, err := s.item(item)
		if err != nil {
			return nil, err
		}
		plan = i.Plan
	} else if plan != "" && plan != NoPlan {
		if _, err := s.plan(plan); err != nil {
			return nil, err
		}
	} else {
		plan = ""
	}
	id, err := s.newID()
	if err != nil {
		return nil, err
	}
	return &Entry{ID: id, Kind: kind, Text: strings.TrimSpace(text), Plan: plan, Item: item, By: m.by(), Created: m.now(), Detail: detail}, nil
}

// parseDue turns --by into a time: a duration (36h, 2d) or a date.
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
	return "", errf(CodeInvalid, "--by %q: a duration (36h, 2d) or a date (2026-12-01)", by)
}

// Inbox lists the open entries, oldest first.
func (m *Mull) Inbox() ([]*Entry, error) {
	out := []*Entry{}
	err := m.view(func(s *state) error {
		for _, id := range sortedKeys(s.entries) {
			if e := s.entries[id]; e.Settled == nil {
				out = append(out, e)
			}
		}
		return nil
	})
	sort.SliceStable(out, func(a, b int) bool { return out[a].Created < out[b].Created })
	return out, err
}

func (m *Mull) settle(s *state, e *Entry, action, note, item string, onRequest bool) error {
	if e.Settled != nil {
		return errf(CodeSettled, "entry %s is settled (%s)", e.ID, e.Settled.Action)
	}
	e.Settled = &Settlement{Action: action, Note: note, Item: item, By: m.by(), Human: m.human != "", OnRequest: onRequest && m.human == "", At: m.now()}
	return s.putEntry(e)
}

// Decide answers a question (or settles an old open decision) with the
// human's decision.
func (m *Mull) Decide(id, decision string, onRequest bool) (*Entry, error) {
	var out *Entry
	err := m.update("decide "+id, func(s *state) error {
		e, err := s.entry(id)
		if err != nil {
			return err
		}
		if e.Kind == KindReview {
			return errf(CodeInvalid, "a review is accepted or sent back, not decided")
		}
		if strings.TrimSpace(decision) == "" {
			return errf(CodeInvalid, "decide needs the decision")
		}
		if !m.asked(onRequest) {
			return errf(CodeNotAllowed, "only the human decides: pass --on-request when they did")
		}
		out = e
		return m.settle(s, e, ActDecide, decision, "", onRequest)
	})
	return out, err
}

// AcceptResult is what accept did.
type AcceptResult struct {
	Entries  []*Entry `json:"entries"`
	Warnings []string `json:"warnings,omitempty"`
}

// Accept settles reviews (by entry or item ID): high-risk work is done and
// lands on its target; a look at landed work just says it looked fine. It
// also takes a question's default.
func (m *Mull) Accept(refs []string, onRequest bool) (AcceptResult, error) {
	out := AcceptResult{Entries: []*Entry{}}
	type landing struct {
		id, target, release, commit string
		edges                       []string
	}
	var lands []landing
	err := m.update("accept "+strings.Join(refs, " "), func(s *state) error {
		out.Entries, lands = []*Entry{}, nil
		if !m.asked(onRequest) {
			return errf(CodeNotAllowed, "only the human accepts: pass --on-request when they did")
		}
		for _, ref := range dedupe(refs) {
			e := s.entries[ref]
			if e == nil {
				e = s.openReview(ref)
			}
			if e == nil {
				return errf(CodeNotFound, "no open review or entry %q", ref)
			}
			switch {
			case e.Kind == KindQuestion:
				if e.Default == "" {
					return errf(CodeInvalid, "question %s has no default to take: decide it", e.ID)
				}
				if err := m.settle(s, e, ActDecide, e.Default, "", onRequest); err != nil {
					return err
				}
			case e.Kind == KindReview && e.Landed != "":
				if err := m.settle(s, e, ActAccept, "", "", onRequest); err != nil {
					return err
				}
			case e.Kind == KindReview:
				i, err := s.item(e.Item)
				if err != nil {
					return err
				}
				if i.State != InReview {
					return errf(CodeState, "item %s is %s, not in review", i.ID, i.State)
				}
				i.State, i.Updated = Done, m.now()
				if err := s.putItem(i); err != nil {
					return err
				}
				if err := m.settle(s, e, ActAccept, "", "", onRequest); err != nil {
					return err
				}
				if rep := s.latestReport(i.ID); rep != nil {
					lands = append(lands, landing{i.ID, s.targetOf(i.Plan), s.release(), rep.Commit, rep.NewEdges})
				}
			default:
				if err := m.settle(s, e, ActAccept, "", "", onRequest); err != nil {
					return err
				}
			}
			out.Entries = append(out.Entries, e)
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	land := func() {
		for _, l := range lands {
			if ok, why := m.landItem(l.target, l.release, l.commit); !ok {
				out.Warnings = append(out.Warnings, fmt.Sprintf("%s is accepted but not on %s yet: %s", l.id, l.target, why))
			}
			if err := m.addEdges(l.edges); err != nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("recording %s's new connections: %v", l.id, err))
			}
		}
	}
	if m.after != nil { // in a batch: land once it commits, never on a dry run
		*m.after = append(*m.after, land)
		return out, nil
	}
	land()
	return out, nil
}

// openReview is an item's open review.
func (s *state) openReview(itemID string) *Entry {
	for _, id := range sortedKeys(s.entries) {
		if e := s.entries[id]; e.Kind == KindReview && e.Item == itemID && e.Settled == nil {
			return e
		}
	}
	return nil
}

// SendBack returns reviewed work with a reason: work waiting for review
// reopens for another session; landed work gets a follow-up item.
func (m *Mull) SendBack(ref, reason string, onRequest bool) (*Item, error) {
	var out *Item
	err := m.update("send back "+ref, func(s *state) error {
		if strings.TrimSpace(reason) == "" {
			return errf(CodeInvalid, "say why (--reason)")
		}
		if !m.asked(onRequest) {
			return errf(CodeNotAllowed, "only the human sends work back: pass --on-request when they did")
		}
		e := s.entries[ref]
		if e == nil {
			e = s.openReview(ref)
		}
		if e == nil || e.Kind != KindReview {
			return errf(CodeNotFound, "no open review %q", ref)
		}
		i, err := s.item(e.Item)
		if err != nil {
			return err
		}
		if e.Landed != "" {
			f, err := m.add(s, NewItem{Plan: i.Plan, Phase: i.Phase, Area: i.Area, From: i.ID, Title: "Fix " + i.ID + ": " + reason,
				Notes: fmt.Sprintf("Sent back from the look at %s: %s", i.ID, reason)})
			if err != nil {
				return err
			}
			out = f
			return m.settle(s, e, ActDo, reason, f.ID, onRequest)
		}
		i.State, i.Claim, i.Updated = Committed, nil, m.now()
		i.Notes = joinNotes(i.Notes, "Sent back: "+reason)
		if err := s.putItem(i); err != nil {
			return err
		}
		out = i
		return m.settle(s, e, ActDo, reason, i.ID, onRequest)
	})
	return out, err
}

// DropEntries drops inbox entries, gone for good.
func (m *Mull) DropEntries(ids []string, reason string, onRequest bool) ([]*Entry, error) {
	var out []*Entry
	err := m.update("drop "+strings.Join(ids, " "), func(s *state) error {
		if !m.asked(onRequest) {
			return errf(CodeNotAllowed, "only the human drops inbox entries: pass --on-request when they did")
		}
		for _, id := range dedupe(ids) {
			e, err := s.entry(id)
			if err != nil {
				return err
			}
			if err := m.settle(s, e, ActDrop, reason, "", onRequest); err != nil {
				return err
			}
			out = append(out, e)
		}
		return nil
	})
	return out, err
}

// Decision is one line of the decision log.
type Decision struct {
	ID       string `json:"id"`
	Item     string `json:"item,omitempty"`
	Plan     string `json:"plan,omitempty"`
	Asked    string `json:"asked,omitempty"` // the question, when it answers one
	Decision string `json:"decision"`
	Why      string `json:"why,omitempty"`
	By       string `json:"by"`
	Human    bool   `json:"human,omitempty"`
	At       string `json:"at"`
}

// Decisions is the log, newest first: the human's decisions and answers,
// and agents' noted choices (Human false).
func (m *Mull) Decisions() ([]Decision, error) {
	out := []Decision{}
	err := m.view(func(s *state) error {
		for _, e := range s.entries {
			st := e.Settled
			if st == nil || (st.Action != ActDecide && st.Action != ActNote && !(st.Action == ActAccept && e.Kind == KindDecision)) {
				continue
			}
			d := Decision{ID: e.ID, Item: e.Item, Plan: e.Plan, Decision: e.Text, Why: e.Detail, By: st.By, Human: st.Human || st.Action == ActDecide, At: st.At}
			if e.Kind == KindQuestion {
				d.Asked, d.Decision = e.Text, st.Note
			}
			out = append(out, d)
		}
		return nil
	})
	sort.SliceStable(out, func(a, b int) bool { return out[a].At > out[b].At })
	return out, err
}
