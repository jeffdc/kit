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
	hits := must[Found](t)(m.Search(cards.ID, 10, false)).Hits
	if len(hits) == 0 || hits[0].ID != cards.ID || hits[0].Rank != 0 {
		t.Fatalf("search %s = %v", cards.ID, ids(hits))
	}
	// Words: a title whose words start with them comes before one that only
	// contains them; every word must match.
	hits = must[Found](t)(m.Search("charge", 10, false)).Hits
	if got := ids(hits); len(got) != 2 || got[0] != "item:"+cards.ID || got[1] != "item:"+recharge.ID {
		t.Fatalf("search charge = %v", got)
	}
	if got := ids(must[Found](t)(m.Search("card processor", 10, false)).Hits); len(got) != 1 || got[0] != "entry:"+q.ID {
		t.Fatalf("search card processor = %v", got)
	}
	// Plans by slug, phases by title; an entry carries its item and plan.
	if got := ids(must[Found](t)(m.Search("launch", 10, false)).Hits); len(got) != 1 || got[0] != "plan:launch" {
		t.Fatalf("search launch = %v", got)
	}
	if got := ids(must[Found](t)(m.Search("billing", 10, false)).Hits); len(got) != 1 || got[0] != "phase:"+phase.ID {
		t.Fatalf("search billing = %v", got)
	}
	e := must[Found](t)(m.Search(q.ID, 1, false)).Hits[0]
	if e.Item != cards.ID || e.Plan != "launch" || e.Kind != model.KindQuestion || e.State != "open" {
		t.Errorf("entry hit = %+v", e)
	}
	// Nothing to find, and the limit.
	if got := must[Found](t)(m.Search("  ", 10, false)).Hits; len(got) != 0 {
		t.Errorf("blank search = %v", got)
	}
	if got := must[Found](t)(m.Search("a", 2, false)).Hits; len(got) != 2 {
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

	open := must[Found](t)(m.Search("launch", 10, false))
	if len(open.Hits) != 1 || open.Hits[0].ID != "launch" || open.Archived != 2 {
		t.Fatalf("search launch = %+v; want the open plan, and 2 in the archive", open)
	}
	all := must[Found](t)(m.Search("launch", 10, true))
	got := map[string]bool{}
	for _, h := range all.Hits {
		got[h.ID] = h.Archived
	}
	if len(all.Hits) != 3 || !got["pilot"] || !got[old.ID] || got["launch"] || all.Archived != 0 {
		t.Fatalf("search launch with the archive = %+v", all)
	}
}
