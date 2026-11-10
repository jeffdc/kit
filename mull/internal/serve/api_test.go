package serve

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"mull/internal/model"
	"mull/internal/mull"
)

func TestAPIPages(t *testing.T) {
	dir := onMull(t)
	m, err := mull.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ok := func(_ any, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ok(m.PlanNew("launch", "Public launch", "", true))
	ok(m.PlanAgree("launch", true))
	phase, err := m.PhaseAdd("launch", "Billing", "", nil)
	ok(phase, err)
	item, err := m.ItemAdd(mull.NewItem{Plan: "launch", Phase: phase.ID, Title: "Charge cards"})
	ok(item, err)
	ok(m.Capture(mull.CaptureInput{Kind: model.KindQuestion, Text: "Which processor?", Item: item.ID}))
	ok(m.CaptureItem(mull.NewCapture{Kind: model.ItemBug, Title: "Footer", Plan: "launch"}))
	ok(m.ItemAdd(mull.NewItem{Plan: model.NoPlan, Title: "Unplanned fix"}))

	gone := filepath.Join(t.TempDir(), "gone")
	repos := []string{dir, gone}
	h := Handler(func() ([]string, error) { return repos, nil })
	id := filepath.Base(dir)

	decode := func(path string, v any) {
		t.Helper()
		rec := get(t, h, path)
		if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("GET %s: %d %s\n%s", path, rec.Code, rec.Header().Get("Content-Type"), rec.Body)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
	}

	var ov mull.Overview
	decode("/api/projects/"+id, &ov)
	if ov.Summary.Blocking != 1 || len(ov.Plans) != 1 || len(ov.Plans[0].Now) != 1 || ov.NeedsYou.Tiers[0].Key != mull.TierBlocking {
		t.Errorf("overview = %+v", ov)
	}
	var tree mull.Tree
	decode("/api/projects/"+id+"/tree", &tree)
	if len(tree.Plans) != 1 || len(tree.Plans[0].Phases) != 1 || len(tree.Plans[0].Phases[0].Items) != 1 {
		t.Errorf("tree = %+v", tree)
	}
	var plan mull.PlanPage
	decode("/api/projects/"+id+"/plans/launch", &plan)
	if plan.Plan.Slug != "launch" || len(plan.Schedule) != 1 || plan.Schedule[0].Key != "now" || plan.Backlog != 1 {
		t.Errorf("plan = %+v", plan)
	}
	var ph mull.PhasePage
	decode("/api/projects/"+id+"/phases/"+phase.ID, &ph)
	if ph.Phase.ID != phase.ID || len(ph.Items) != 1 {
		t.Errorf("phase = %+v", ph)
	}
	var it mull.ItemPage
	decode("/api/projects/"+id+"/items/"+item.ID, &it)
	if it.Item.ID != item.ID || it.PhaseTitle != "Billing" || it.NeedsYou.Open != 1 || it.Review != nil {
		t.Errorf("item = %+v", it)
	}
	var un mull.Unplanned
	decode("/api/projects/"+id+"/unplanned", &un)
	if len(un.Items) != 1 {
		t.Errorf("unplanned = %+v", un)
	}
	var bl mull.Backlog
	decode("/api/projects/"+id+"/backlog", &bl)
	if len(bl.Items) != 1 || bl.Bugs != 1 {
		t.Errorf("backlog = %+v", bl)
	}
	decode("/api/projects/"+id+"/backlog?plan=none", &bl)
	if len(bl.Items) != 0 {
		t.Errorf("untagged backlog = %+v", bl)
	}

	for path, want := range map[string]struct {
		status int
		code   string
	}{
		"/api/projects/nope":                         {404, mull.CodeNotFound},
		"/api/projects/" + id + "/items/ffff":        {404, mull.CodeNotFound},
		"/api/projects/" + id + "/plans/nope":        {404, mull.CodeNotFound},
		"/api/projects/" + id + "/phases/ffff":       {404, mull.CodeNotFound},
		"/api/projects/" + id + "/backlog?plan=nope": {404, mull.CodeNotFound},
		"/api/projects/gone":                         {409, mull.CodeNotRepo},
		"/api/projects/gone/tree":                    {409, mull.CodeNotRepo},
	} {
		rec := get(t, h, path)
		var e mull.Error
		_ = json.Unmarshal(rec.Body.Bytes(), &e)
		if rec.Code != want.status || e.Code != want.code {
			t.Errorf("GET %s: %d %s, want %d %s\n%s", path, rec.Code, e.Code, want.status, want.code, rec.Body)
		}
	}
}
