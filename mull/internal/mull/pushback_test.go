package mull

import (
	"strings"
	"testing"

	"mull/internal/model"
)

// Deciding one of a review's decisions differently sends the work back:
// it can't be accepted until an agent takes the decision in and files it
// for review again. The other decisions can still be approved one by one.
func TestDecidingDifferentlySendsReviewBack(t *testing.T) {
	m, _ := setup(t)
	v := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "Name the human"}))
	must[StartResult](t)(m.Start(v.ID))
	d1 := entryIn(t, m, model.KindDecision, "Store the name in repo config", v.ID, "", "")
	d2 := entryIn(t, m, model.KindDecision, "Add a guide test", v.ID, "", "")
	markInReview(t, m, v.ID)
	o := SettleOpts{OnRequest: true}
	must[*model.Entry](t)(m.Decide(d1.ID, "Keep it per machine", o))
	i := item(t, m, v.ID)
	if i.State != model.StateCommitted || !strings.Contains(i.Notes, "decided differently: Keep it per machine") {
		t.Fatalf("after deciding differently, item = %s, notes %q; want committed, with the decision in its notes", i.State, i.Notes)
	}
	if _, err := m.Accept([]string{v.ID}, "", o); err == nil {
		t.Fatal("the work was accepted after a decision was decided differently")
	}
	got := must[AcceptResult](t)(m.Accept([]string{d2.ID}, "", o))
	if len(got.Entries) != 1 || got.Entries[0].Settled == nil || got.Entries[0].Settled.Action != model.ActAccept {
		t.Fatalf("approving the other decision = %+v", got)
	}
	if item(t, m, v.ID).State != model.StateCommitted {
		t.Fatal("approving the other decision changed the item")
	}
}

// Deciding a decision on work that isn't in review sends nothing back.
func TestDecidingOutsideReviewSendsNothingBack(t *testing.T) {
	m, _ := setup(t)
	v := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "Active work"}))
	must[StartResult](t)(m.Start(v.ID))
	d := entryIn(t, m, model.KindDecision, "Use sqlite", v.ID, "", "")
	must[*model.Entry](t)(m.Decide(d.ID, "Use postgres", SettleOpts{OnRequest: true}))
	if i := item(t, m, v.ID); i.State != model.StateActive {
		t.Fatalf("item = %s, want still active", i.State)
	}
}
