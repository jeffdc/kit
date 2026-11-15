package mull

import (
	"testing"
	"time"

	"mull/internal/model"
)

// Finished unplanned work is archived right away: done or dropped, with
// nothing open on it, it leaves Unplanned for the Archive, default search
// leaves it out (and the entries on it), and its sheet says so. Something
// open on it keeps it out of the Archive until it's settled.
func TestUnplannedWorkArchives(t *testing.T) {
	m, _ := setup(t)
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	m.Now = func() time.Time { return base }
	add := func(title string) string {
		t.Helper()
		return must[ItemView](t)(m.ItemAdd(NewItem{Plan: model.NoPlan, Title: title})).ID
	}
	done := add("Fix the export")
	finish(t, m, done)
	decision := entryIn(t, m, model.KindDecision, "Export as CSV", done, "", "")
	must[AcceptResult](t)(m.Accept([]string{decision.ID}, "", SettleOpts{OnRequest: true}))

	m.Now = func() time.Time { return base.Add(time.Hour) }
	dropped := add("Fix the export twice")
	must[DropResult](t)(m.Drop([]string{dropped}, "a duplicate", SettleOpts{OnRequest: true}))
	asked := add("Fix the export header")
	finish(t, m, asked)
	question := entryIn(t, m, model.KindQuestion, "Which export header?", asked, "", "")
	parked := add("Fix the export footer")
	finish(t, m, parked)
	park := entryIn(t, m, model.KindQuestion, "Which export footer?", parked, "", "")
	must[*model.Entry](t)(m.Park(park.ID, "later", SettleOpts{OnRequest: true}))
	open := add("Fix the export again") // committed: never archived

	un := must[Unplanned](t)(m.UnplannedPage(false))
	if got := rowIDs(un.Items); got[done] || got[dropped] || !got[asked] || !got[parked] || !got[open] || un.Archived != 2 {
		t.Errorf("unplanned lists %v, archived %d; want the open work and the finished work with an entry on it, 2 archived", got, un.Archived)
	}
	all := must[Unplanned](t)(m.UnplannedPage(true))
	if got := rowIDs(all.Items); !got[done] || !got[dropped] || len(got) != 5 || all.Archived != 0 {
		t.Errorf("unplanned with all lists %v, archived %d; want all 5", got, all.Archived)
	}
	tree := must[Tree](t)(m.TreePage())
	if tree.Counts.Unplanned != len(un.Items) {
		t.Errorf("the tree counts %d unplanned, the list shows %d", tree.Counts.Unplanned, len(un.Items))
	}
	if a := tree.ArchivedUnplanned; len(a) != 2 || a[0].ID != dropped || a[0].State != model.StateDropped || a[1].ID != done || a[1].Finished == "" {
		t.Errorf("the tree's archived unplanned = %+v; want %s then %s, newest first", a, dropped, done)
	}
	if len(tree.Archive) != 0 {
		t.Errorf("archived plans = %+v; want none", tree.Archive)
	}
	for id, want := range map[string]bool{done: true, dropped: true, asked: false, parked: false, open: false} {
		if got := must[ItemPage](t)(m.ItemPage(id)).Archived; got != want {
			t.Errorf("item %s archived = %v, want %v", id, got, want)
		}
	}

	// Search's Open filter leaves out the archived work and the decision on
	// it, and the finished work that isn't archived too: just the live work
	// and what's open on finished work.
	found := must[Found](t)(m.Search("export", 20, SearchOpen))
	hits := map[string]bool{}
	for _, h := range found.Hits {
		hits[h.ID] = true
	}
	if len(found.Hits) != 3 || !hits[open] || !hits[question.ID] || !hits[park.ID] || found.Open != 3 || found.Finished != 8 {
		t.Errorf("search export = %+v; want the committed work, the open question and the parked one; 8 finished", found)
	}
	found = must[Found](t)(m.Search("export", 20, SearchAll))
	archived := map[string]bool{}
	for _, h := range found.Hits {
		archived[h.ID] = h.Archived
	}
	if len(found.Hits) != 11 || !archived[done] || !archived[dropped] || !archived[decision.ID] || archived[asked] || archived[question.ID] {
		t.Errorf("search export with the archive = %+v", found)
	}

	// Answering the question archives the work it was holding.
	must[*model.Entry](t)(m.Decide(question.ID, "The plain one", SettleOpts{OnRequest: true}))
	tree = must[Tree](t)(m.TreePage())
	if a := tree.ArchivedUnplanned; len(a) != 3 || a[2].ID != done || (a[0].ID != asked && a[1].ID != asked) {
		t.Errorf("after answering, archived unplanned = %+v; want %s among them", a, asked)
	}
	if got := rowIDs(must[Unplanned](t)(m.UnplannedPage(false)).Items); got[asked] {
		t.Errorf("after answering, unplanned still lists %s", asked)
	}
}

// A captured item dropped with no plan isn't backlog any more, so it counts
// as unplanned (bug 522d): finished, it's archived with the rest, and leaves
// Unplanned at once rather than after doneShownFor.
func TestDroppedCaptureArchives(t *testing.T) {
	m, _ := setup(t)
	bug := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "A stray bug"}))
	must[DropResult](t)(m.Drop([]string{bug.ID}, "not a bug", SettleOpts{OnRequest: true}))
	if un := must[Unplanned](t)(m.UnplannedPage(false)); len(un.Items) != 0 || un.Archived != 1 {
		t.Errorf("unplanned = %+v; want nothing listed, 1 archived", un)
	}
	if a := must[Tree](t)(m.TreePage()).ArchivedUnplanned; len(a) != 1 || a[0].ID != bug.ID {
		t.Errorf("archived unplanned = %+v", a)
	}
}

// An item in a closed plan is in the Archive, as its plan is.
func TestClosedPlanItemArchived(t *testing.T) {
	m, _ := setup(t)
	agreedPlan(t, m, "pilot", "Pilot", "")
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "pilot", Title: "Pilot work"}))
	if must[ItemPage](t)(m.ItemPage(it.ID)).Archived {
		t.Fatal("an item in an open plan is archived")
	}
	must[*model.Plan](t)(m.PlanClose("pilot"))
	if !must[ItemPage](t)(m.ItemPage(it.ID)).Archived {
		t.Error("an item in a closed plan isn't archived")
	}
}
