package mull

import (
	"testing"

	"mull/internal/model"
)

func TestSearch(t *testing.T) {
	m, _ := setup(t)
	agreedPlan(t, m, "launch", "Public launch", "")
	phase := must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "", nil))
	cards := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Phase: phase.ID, Title: "Charge cards"}))
	recharge := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Title: "Recharge failed payments"}))
	q := entryIn(t, m, model.KindQuestion, "Which card processor?", cards.ID, "", "")

	ids := func(hits []Hit) []string {
		var out []string
		for _, h := range hits {
			out = append(out, h.Type+":"+h.ID)
		}
		return out
	}
	// An ID finds its record first.
	hits := must[Found](t)(m.Search(cards.ID, 10, SearchOpen)).Hits
	if len(hits) == 0 || hits[0].ID != cards.ID || hits[0].Rank != 0 {
		t.Fatalf("search %s = %v", cards.ID, ids(hits))
	}
	// Words: a title whose words start with them comes before one that only
	// contains them; every word must match.
	hits = must[Found](t)(m.Search("charge", 10, SearchOpen)).Hits
	if got := ids(hits); len(got) != 2 || got[0] != "item:"+cards.ID || got[1] != "item:"+recharge.ID {
		t.Fatalf("search charge = %v", got)
	}
	if got := ids(must[Found](t)(m.Search("card processor", 10, SearchOpen)).Hits); len(got) != 1 || got[0] != "entry:"+q.ID {
		t.Fatalf("search card processor = %v", got)
	}
	// Plans by slug, phases by title; an entry carries its item and plan.
	if got := ids(must[Found](t)(m.Search("launch", 10, SearchOpen)).Hits); len(got) != 1 || got[0] != "plan:launch" {
		t.Fatalf("search launch = %v", got)
	}
	if got := ids(must[Found](t)(m.Search("billing", 10, SearchOpen)).Hits); len(got) != 1 || got[0] != "phase:"+phase.ID {
		t.Fatalf("search billing = %v", got)
	}
	e := must[Found](t)(m.Search(q.ID, 1, SearchOpen)).Hits[0]
	if e.Item != cards.ID || e.Plan != "launch" || e.Kind != model.KindQuestion || e.State != "open" {
		t.Errorf("entry hit = %+v", e)
	}
	// Nothing to find, and the limit.
	if got := must[Found](t)(m.Search("  ", 10, SearchOpen)).Hits; len(got) != 0 {
		t.Errorf("blank search = %v", got)
	}
	if got := must[Found](t)(m.Search("a", 2, SearchOpen)).Hits; len(got) != 2 {
		t.Errorf("limit 2 gave %d", len(got))
	}
}

// A closed plan is in the archive, with everything in it: search leaves it
// out, counted, unless asked to search the archive too.
func TestSearchArchive(t *testing.T) {
	m, _ := setup(t)
	agreedPlan(t, m, "launch", "Public launch", "")
	agreedPlan(t, m, "pilot", "Pilot launch", "")
	old := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "pilot", Title: "Launch the pilot"}))
	must[*model.Plan](t)(m.PlanClose("pilot"))

	open := must[Found](t)(m.Search("launch", 10, SearchOpen))
	if len(open.Hits) != 1 || open.Hits[0].ID != "launch" || open.Finished != 2 {
		t.Fatalf("search launch = %+v; want the open plan, and 2 in the archive", open)
	}
	all := must[Found](t)(m.Search("launch", 10, SearchAll))
	got := map[string]bool{}
	for _, h := range all.Hits {
		got[h.ID] = h.Archived
	}
	if len(all.Hits) != 3 || !got["pilot"] || !got[old.ID] || got["launch"] || all.Open != 1 || all.Finished != 2 {
		t.Fatalf("search launch with the archive = %+v", all)
	}
}

// The status filter: Open is live work (and an entry still waiting, parked
// ones too), Finished is everything done, dropped, settled or archived, and
// All is both. Found counts each status whichever is searched.
func TestSearchStatus(t *testing.T) {
	m, _ := setup(t)
	agreedPlan(t, m, "launch", "Public launch", "")
	billing := must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "", nil))
	refunds := must[*model.Phase](t)(m.PhaseAdd("launch", "Refunds", "", nil))
	add := func(plan, phase, title string) string {
		t.Helper()
		return must[ItemView](t)(m.ItemAdd(NewItem{Plan: plan, Phase: phase, Title: title})).ID
	}
	charge := add("launch", billing.ID, "Charge cards")
	refund := add("launch", refunds.ID, "Refund cards")
	finish(t, m, refund)
	fees := add("launch", billing.ID, "Card fees")
	must[DropResult](t)(m.Drop([]string{fees}, "not needed", SettleOpts{OnRequest: true}))
	asked := entryIn(t, m, model.KindQuestion, "Which card processor?", charge, "", "")
	decided := entryIn(t, m, model.KindDecision, "Cards go through Stripe", charge, "", "")
	must[AcceptResult](t)(m.Accept([]string{decided.ID}, "", SettleOpts{OnRequest: true}))
	parked := entryIn(t, m, model.KindQuestion, "Card limits?", charge, "", "")
	must[*model.Entry](t)(m.Park(parked.ID, "later", SettleOpts{OnRequest: true}))
	agreedPlan(t, m, "pilot", "Card pilot", "")
	pilot := add("pilot", "", "Pilot cards") // committed, but its plan closes
	must[*model.Plan](t)(m.PlanClose("pilot"))

	search := func(status string) (Found, map[string]Hit) {
		t.Helper()
		f := must[Found](t)(m.Search("card", 50, status))
		by := map[string]Hit{}
		for _, h := range f.Hits {
			by[h.ID] = h
		}
		return f, by
	}
	open, openHits := search(SearchOpen)
	if _, ok := openHits[charge]; len(open.Hits) != 3 || !ok {
		t.Fatalf("open search = %+v; want the committed work, the open question and the parked one", open)
	}
	if _, ok := openHits[asked.ID]; !ok {
		t.Errorf("open search has no open question: %+v", open)
	}
	if _, ok := openHits[parked.ID]; !ok {
		t.Errorf("open search has no parked question: %+v", open)
	}
	fin, finHits := search(SearchFinished)
	for _, id := range []string{refund, fees, decided.ID, pilot, "pilot"} {
		if h, ok := finHits[id]; !ok || !h.Finished {
			t.Errorf("finished search has no %s: %+v", id, fin)
		}
	}
	for id := range openHits {
		if _, ok := finHits[id]; ok {
			t.Errorf("%s is both open and finished", id)
		}
	}
	all, allHits := search(SearchAll)
	if len(all.Hits) != len(open.Hits)+len(fin.Hits) {
		t.Errorf("all found %d; want open's %d and finished's %d", len(all.Hits), len(open.Hits), len(fin.Hits))
	}
	// The counts are the same whichever status is searched.
	for _, f := range []Found{open, fin, all} {
		if f.Open != len(open.Hits) || f.Finished != len(fin.Hits) {
			t.Errorf("counts %d open, %d finished; want %d and %d", f.Open, f.Finished, len(open.Hits), len(fin.Hits))
		}
	}
	// Archived is the closed plan and what's in it; done and dropped work
	// in an open plan is finished, not archived, and carries its stage.
	for id, want := range map[string]bool{pilot: true, "pilot": true, refund: false, fees: false, charge: false} {
		if allHits[id].Archived != want {
			t.Errorf("%s archived = %v, want %v", id, allHits[id].Archived, want)
		}
	}
	if allHits[refund].State != model.StateDone || allHits[fees].State != model.StateDropped || allHits[parked.ID].State != model.ActPark {
		t.Errorf("states: refund %q, fees %q, parked %q", allHits[refund].State, allHits[fees].State, allHits[parked.ID].State)
	}
	// "" is Open; anything else is refused.
	if f := must[Found](t)(m.Search("card", 50, "")); len(f.Hits) != len(open.Hits) {
		t.Errorf("search with no status = %+v; want the open search", f)
	}
	_, err := m.Search("card", 50, "archived")
	wantCode(t, err, CodeInvalid)

	// A phase carries its state: done once its work is, and finished then.
	b := must[Found](t)(m.Search(billing.ID, 1, SearchAll)).Hits
	r := must[Found](t)(m.Search(refunds.ID, 1, SearchAll)).Hits
	if len(b) != 1 || b[0].State != PhaseNow || b[0].Finished || len(r) != 1 || r[0].State != PhaseDone || !r[0].Finished {
		t.Errorf("phases: billing %+v, refunds %+v", b, r)
	}
}
