package mull

import (
	"strings"
	"testing"

	"mull/internal/model"
)

func decidedEntry(t *testing.T, r any) *model.Entry {
	t.Helper()
	e, ok := r.(*model.Entry)
	if !ok {
		t.Fatalf("decided returned %T", r)
	}
	return e
}

// The human's own decision, recorded by an agent on request, is settled at
// once as decided: in the decision log, never in the inbox or a review.
func TestDecidedHuman(t *testing.T) {
	m, _ := setup(t)
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "Checkout"}))

	// Only on request, and --on-request only with --human.
	_, err := m.Exec("decided", &DecidedIn{Text: "Monthly plans only", Item: it.ID, Human: true})
	wantCode(t, err, CodeNotAllowed)
	_, err = m.Exec("decided", &DecidedIn{Text: "Monthly plans only", Item: it.ID, OnRequest: true})
	wantCode(t, err, CodeInvalid)
	// The usual capture rules still hold.
	_, err = m.Exec("decided", &DecidedIn{Human: true, OnRequest: true})
	wantCode(t, err, CodeInvalid)

	e := decidedEntry(t, must[any](t)(m.Exec("decided", &DecidedIn{Text: "Monthly plans only", Item: it.ID,
		Detail: "Annual later", Source: "conversation", Human: true, OnRequest: true})))
	st := e.Settled
	if e.Kind != model.KindDecision || e.Plan != "general" || e.Detail != "Annual later" || st == nil {
		t.Fatalf("entry = %+v", e)
	}
	if st.Action != model.ActDecide || st.Note != "Monthly plans only" || !st.OnRequest || st.Human || st.By != m.Worktree() {
		t.Fatalf("settled = %+v", st)
	}
	if inbox := must[[]*model.Entry](t)(m.Inbox(InboxFilter{})); len(inbox) != 0 {
		t.Fatalf("inbox = %+v", inbox)
	}
	ds := must[[]Decision](t)(m.Decisions(""))
	if len(ds) != 1 || ds[0].ID != e.ID || ds[0].Decision != "Monthly plans only" || ds[0].Item != it.ID || !ds[0].OnRequest {
		t.Fatalf("decision log = %+v", ds)
	}
	// It's already settled, so a review filed for the item doesn't carry it.
	if err := m.view(func(s *state) error {
		if open := s.openDecisions(it.ID); len(open) != 0 {
			t.Fatalf("open decisions = %+v", open)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Settled is settled.
	_, err = m.Accept([]string{e.ID}, "", SettleOpts{OnRequest: true})
	wantCode(t, err, CodeSettled)

	// An agent's own decision is unchanged: open, in the inbox.
	own := decidedEntry(t, must[any](t)(m.Exec("decided", &DecidedIn{Text: "Retry three times", Item: it.ID})))
	if own.Settled != nil {
		t.Fatalf("agent decision settled: %+v", own.Settled)
	}
}

// From mull serve (as the human) it's theirs by name, with no --on-request.
func TestDecidedHumanAsHuman(t *testing.T) {
	m, dir := setup(t)
	h := human(t, m, dir)
	e := decidedEntry(t, must[any](t)(h.Exec("decided", &DecidedIn{Text: "Ship Friday", Plan: "general", Human: true})))
	if st := e.Settled; st == nil || !st.Human || st.By != "Jeff" || st.OnRequest {
		t.Fatalf("settled = %+v", e.Settled)
	}
}

func TestDecidedHumanInBatch(t *testing.T) {
	m, _ := setup(t)
	input := jsonl(t,
		map[string]any{"op": "add", "ref": "it", "title": "Checkout", "plan": "general"},
		map[string]any{"op": "decided", "item": "@it", "text": "Monthly plans only", "human": true, "on-request": true},
		map[string]any{"op": "decided", "item": "@it", "text": "Retry three times"},
	)
	results := must[[]BatchResult](t)(m.Batch(strings.NewReader(input), false))
	if st := results[1].Result.(*model.Entry).Settled; st == nil || st.Action != model.ActDecide || !st.OnRequest {
		t.Fatalf("human decision = %+v", results[1].Result)
	}
	if st := results[2].Result.(*model.Entry).Settled; st != nil {
		t.Fatalf("agent decision settled: %+v", st)
	}

	// Without on-request the line fails, and nothing is written.
	_, err := m.Batch(strings.NewReader(jsonl(t,
		map[string]any{"op": "decided", "plan": "general", "text": "x", "human": true})), false)
	if e := batchErr(t, err); len(e.Lines) != 1 || e.Lines[0].Code != CodeNotAllowed {
		t.Fatalf("batch error = %+v", e)
	}
	// ask takes neither field.
	_, err = m.Batch(strings.NewReader(jsonl(t,
		map[string]any{"op": "ask", "plan": "general", "text": "x", "human": true})), false)
	batchErr(t, err)
}
