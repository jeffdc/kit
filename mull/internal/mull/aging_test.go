package mull

import (
	"testing"
	"time"

	"mull/internal/model"
)

// Done work more than doneShownFor old leaves the sheet index and the
// Unplanned list, unless something on it is still open; it stays on its
// phase's sheet, and Unplanned shows it when asked for all.
func TestDoneWorkAgesOut(t *testing.T) {
	m, _ := setup(t)
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	m.Now = func() time.Time { return base }
	agreedPlan(t, m, "launch", "Launch", "")
	phase := must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "", nil))
	finish := func(id string) {
		t.Helper()
		must[StartResult](t)(m.Start(id))
		markInReview(t, m, id)
		must[AcceptResult](t)(m.Accept([]string{id}, "", SettleOpts{OnRequest: true}))
	}
	add := func(plan, phase, title string) string {
		t.Helper()
		return must[ItemView](t)(m.ItemAdd(NewItem{Plan: plan, Phase: phase, Title: title})).ID
	}
	oldUnplanned := add(model.NoPlan, "", "Old unplanned fix")
	finish(oldUnplanned)
	oldPlanned := add("launch", phase.ID, "Old billing work")
	finish(oldPlanned)
	oldOpen := add("launch", phase.ID, "Old work with a decision open")
	finish(oldOpen)
	entryIn(t, m, model.KindDecision, "Bill monthly", oldOpen, "", "")
	dropped := add(model.NoPlan, "", "Old dropped fix")
	must[DropResult](t)(m.Drop([]string{dropped}, "not needed", SettleOpts{OnRequest: true}))

	m.Now = func() time.Time { return base.Add(doneShownFor + 24*time.Hour) }
	recent := add(model.NoPlan, "", "Recent unplanned fix")
	finish(recent)
	committed := add(model.NoPlan, "", "Old but still open") // committed, never ages out
	_ = committed

	ids := func(rows []ItemRow) map[string]bool {
		out := map[string]bool{}
		for _, r := range rows {
			out[r.ID] = true
		}
		return out
	}
	un := must[Unplanned](t)(m.UnplannedPage(false))
	got := ids(un.Items)
	if got[oldUnplanned] || got[dropped] || !got[recent] || !got[committed] {
		t.Errorf("unplanned lists %v; want the recent and open work, not the old done or dropped", got)
	}
	if un.AgedOut != 2 || un.DoneDays != 14 {
		t.Errorf("aged_out = %d, done_shown_days = %d; want 2, 14", un.AgedOut, un.DoneDays)
	}
	if tree := must[Tree](t)(m.TreePage()); tree.Counts.Unplanned != len(un.Items) {
		t.Errorf("the tree counts %d unplanned, the list shows %d", tree.Counts.Unplanned, len(un.Items))
	}
	all := must[Unplanned](t)(m.UnplannedPage(true))
	if got := ids(all.Items); !got[oldUnplanned] || !got[dropped] || all.AgedOut != 0 {
		t.Errorf("unplanned with all lists %v, aged_out %d", got, all.AgedOut)
	}

	tree := must[Tree](t)(m.TreePage())
	listed := map[string]bool{}
	for _, p := range tree.Plans {
		for _, ph := range p.Phases {
			for _, it := range ph.Items {
				listed[it.ID] = true
			}
		}
	}
	if listed[oldPlanned] || !listed[oldOpen] {
		t.Errorf("tree lists %v; want the old work with an open decision, not the old done work", listed)
	}
	sheet := must[PhasePage](t)(m.PhasePage(phase.ID))
	if got := ids(sheet.Items); !got[oldPlanned] || !got[oldOpen] {
		t.Errorf("the phase sheet lists %v; want every item", got)
	}
}
