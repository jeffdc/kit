package serve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mull/internal/model"
	"mull/internal/mull"
	"mull/internal/seen"
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
	ok(m.CaptureItem(mull.NewCapture{Kind: model.ItemBug, Title: "Footer", Plan: "launch", Area: "web/footer/**"}))
	ok(m.ItemAdd(mull.NewItem{Plan: model.NoPlan, Title: "Unplanned fix"}))
	dropped, err := m.ItemAdd(mull.NewItem{Plan: model.NoPlan, Title: "Unplanned fix, not needed"})
	ok(dropped, err)
	ok(m.Drop([]string{dropped.ID}, "not needed", mull.SettleOpts{OnRequest: true}))

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
	if len(un.Items) != 1 || un.Archived != 1 {
		t.Errorf("unplanned = %+v", un)
	}
	// Finished unplanned work is archived: the wire shape of Unplanned's
	// count, the tree's Archive and the item's sheet.
	var raw struct {
		Archived          int `json:"archived"`
		ArchivedUnplanned []struct {
			ID, Title, State, Finished string
		} `json:"archived_unplanned"`
	}
	decode("/api/projects/"+id+"/unplanned", &raw)
	decode("/api/projects/"+id+"/tree", &raw)
	if a := raw.ArchivedUnplanned; raw.Archived != 1 || len(a) != 1 || a[0].ID != dropped.ID || a[0].State != model.StateDropped || a[0].Finished == "" {
		t.Errorf("archived = %d, archived_unplanned = %+v", raw.Archived, a)
	}
	var sheet struct {
		Archived bool `json:"archived"`
	}
	decode("/api/projects/"+id+"/items/"+dropped.ID, &sheet)
	if !sheet.Archived {
		t.Errorf("the dropped unplanned item's sheet isn't archived")
	}
	decode("/api/projects/"+id+"/unplanned?done=all", &un)
	if len(un.Items) != 2 || un.Archived != 0 {
		t.Errorf("unplanned with done=all = %+v", un)
	}
	var bl mull.Backlog
	decode("/api/projects/"+id+"/backlog", &bl)
	if len(bl.Items) != 1 || bl.Bugs != 1 || bl.Items[0].Area != "web/footer/**" {
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
		"/api/projects/gone":                         {500, mull.CodeNotRepo},
		"/api/projects/gone/tree":                    {500, mull.CodeNotRepo},
	} {
		rec := get(t, h, path)
		var e mull.Error
		_ = json.Unmarshal(rec.Body.Bytes(), &e)
		if rec.Code != want.status || e.Code != want.code {
			t.Errorf("GET %s: %d %s, want %d %s\n%s", path, rec.Code, e.Code, want.status, want.code, rec.Body)
		}
	}
}

// The surface reads from the human's last look, or ?since=; the rulebook
// is RULES.md in the repository, advisory rules (no check) counted apart.
func TestAPISurfaceAndRules(t *testing.T) {
	dir := onMull(t)
	m, err := mull.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.PlanNew("launch", "Public launch", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.PlanAgree("launch", true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Capture(mull.CaptureInput{Kind: model.KindQuestion, Text: "Which processor?", Plan: "launch", Default: "Stripe", By: "2d"}); err != nil {
		t.Fatal(err)
	}
	rules := "# Rules\n\n## R1: Pages use the components\n\n- scope: web/**\n- check: true\n\nOne look.\n\n## R2: Keep it small\n\nA hope.\n"
	if err := os.WriteFile(filepath.Join(dir, "RULES.md"), []byte(rules), 0o644); err != nil {
		t.Fatal(err)
	}
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	id := filepath.Base(dir)
	decode := func(path string, v any) {
		t.Helper()
		rec := get(t, h, path)
		if rec.Code != 200 {
			t.Fatalf("GET %s: %d\n%s", path, rec.Code, rec.Body)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
	}

	look := "2026-01-02T03:04:05Z"
	if err := seen.Mark(dir, seen.Look{At: look}); err != nil {
		t.Fatal(err)
	}
	var sf mull.Surface
	decode("/api/projects/"+id+"/surface", &sf)
	if sf.Since != look {
		t.Errorf("surface since = %q, want the last look %q", sf.Since, look)
	}
	if len(sf.Calls) != 1 || sf.Calls[0].Kind != "question" || sf.Calls[0].Default != "Stripe" || sf.Calls[0].Due == "" || sf.Calls[0].Lapsed {
		t.Errorf("surface calls = %+v", sf.Calls)
	}
	if len(sf.Rules) != 1 || sf.Rules[0].ID != "R1" || len(sf.Advisory) != 1 {
		t.Errorf("surface rules = %+v, advisory = %v", sf.Rules, sf.Advisory)
	}
	if len(sf.Plans) != 1 || sf.Plans[0].Slug != "launch" {
		t.Errorf("surface plans = %+v", sf.Plans)
	}
	decode("/api/projects/"+id+"/surface?since=2026-02-01T00:00:00Z", &sf)
	if sf.Since != "2026-02-01T00:00:00Z" {
		t.Errorf("surface since with ?since = %q", sf.Since)
	}

	var rb mull.RulesOut
	decode("/api/projects/"+id+"/rules", &rb)
	if len(rb.Rules) != 2 || rb.Checked != 1 || rb.Advisory != 1 || !rb.Rules[1].Advisory || rb.Rules[0].Check != "true" {
		t.Errorf("rules = %+v", rb)
	}
}
