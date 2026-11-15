package mull

import (
	"testing"
	"time"

	"mull/internal/model"
)

// Done work in a plan more than doneShownFor old leaves the sheet index,
// unless something on it is still open; it stays on its phase's sheet.
// (Finished unplanned work doesn't wait: see archive_test.go.)
func TestDoneWorkAgesOut(t *testing.T) {
	m, _ := setup(t)
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	m.Now = func() time.Time { return base }
	agreedPlan(t, m, "launch", "Launch", "")
	phase := must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "", nil))
	add := func(title string) string {
		t.Helper()
		return must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Phase: phase.ID, Title: title})).ID
	}
	oldPlanned := add("Old billing work")
	finish(t, m, oldPlanned)
	oldOpen := add("Old work with a decision open")
	finish(t, m, oldOpen)
	entryIn(t, m, model.KindDecision, "Bill monthly", oldOpen, "", "")

	m.Now = func() time.Time { return base.Add(doneShownFor + 24*time.Hour) }
	recent := add("Recent billing work")
	finish(t, m, recent)

	tree := must[Tree](t)(m.TreePage())
	listed := map[string]bool{}
	for _, p := range tree.Plans {
		for _, ph := range p.Phases {
			for _, it := range ph.Items {
				listed[it.ID] = true
			}
		}
	}
	if listed[oldPlanned] || !listed[oldOpen] || !listed[recent] {
		t.Errorf("tree lists %v; want the recent work and the old work with an open decision, not the old done work", listed)
	}
	sheet := must[PhasePage](t)(m.PhasePage(phase.ID))
	if got := rowIDs(sheet.Items); !got[oldPlanned] || !got[oldOpen] || !got[recent] {
		t.Errorf("the phase sheet lists %v; want every item", got)
	}
	// Done work in an open plan isn't archived, however old.
	if found := must[Found](t)(m.Search("billing", 10, false)); len(found.Hits) != 5 || found.Archived != 0 {
		t.Errorf("search billing = %+v; want the phase, its done items and their reviews, none archived", found)
	}
}

// finish takes item id through review to done.
func finish(t *testing.T, m *Mull, id string) {
	t.Helper()
	must[StartResult](t)(m.Start(id))
	markInReview(t, m, id)
	must[AcceptResult](t)(m.Accept([]string{id}, "", SettleOpts{OnRequest: true}))
}

func rowIDs(rows []ItemRow) map[string]bool {
	out := map[string]bool{}
	for _, r := range rows {
		out[r.ID] = true
	}
	return out
}
