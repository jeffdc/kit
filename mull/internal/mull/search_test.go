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
	hits := must[[]Hit](t)(m.Search(cards.ID, 10))
	if len(hits) == 0 || hits[0].ID != cards.ID || hits[0].Rank != 0 {
		t.Fatalf("search %s = %v", cards.ID, ids(hits))
	}
	// Words: a title whose words start with them comes before one that only
	// contains them; every word must match.
	hits = must[[]Hit](t)(m.Search("charge", 10))
	if got := ids(hits); len(got) != 2 || got[0] != "item:"+cards.ID || got[1] != "item:"+recharge.ID {
		t.Fatalf("search charge = %v", got)
	}
	if got := ids(must[[]Hit](t)(m.Search("card processor", 10))); len(got) != 1 || got[0] != "entry:"+q.ID {
		t.Fatalf("search card processor = %v", got)
	}
	// Plans by slug, phases by title; an entry carries its item and plan.
	if got := ids(must[[]Hit](t)(m.Search("launch", 10))); len(got) != 1 || got[0] != "plan:launch" {
		t.Fatalf("search launch = %v", got)
	}
	if got := ids(must[[]Hit](t)(m.Search("billing", 10))); len(got) != 1 || got[0] != "phase:"+phase.ID {
		t.Fatalf("search billing = %v", got)
	}
	e := must[[]Hit](t)(m.Search(q.ID, 1))[0]
	if e.Item != cards.ID || e.Plan != "launch" || e.Kind != model.KindQuestion || e.State != "open" {
		t.Errorf("entry hit = %+v", e)
	}
	// Nothing to find, and the limit.
	if got := must[[]Hit](t)(m.Search("  ", 10)); len(got) != 0 {
		t.Errorf("blank search = %v", got)
	}
	if got := must[[]Hit](t)(m.Search("a", 2)); len(got) != 2 {
		t.Errorf("limit 2 gave %d", len(got))
	}
}
