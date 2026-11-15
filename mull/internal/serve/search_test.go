package serve

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"mull/internal/mull"
)

// Search covers every project and names each hit's project.
func TestSearchEndpoint(t *testing.T) {
	a, b := onMull(t), onMull(t)
	for _, dir := range []string{a, b} {
		m, err := mull.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := m.ItemAdd(mull.NewItem{Plan: "none", Title: "Charge cards in " + filepath.Base(dir)}); err != nil {
			t.Fatal(err)
		}
	}
	h := Handler(func() ([]string, error) { return []string{a, b, filepath.Join(t.TempDir(), "gone")}, nil })
	rec := get(t, h, "/api/search?q=charge")
	if rec.Code != 200 {
		t.Fatalf("GET /api/search: %d %s", rec.Code, rec.Body)
	}
	var res SearchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	projects := map[string]bool{}
	for _, h := range res.Hits {
		projects[h.Project] = true
		if h.Type != mull.RecordItem {
			t.Errorf("hit %+v", h)
		}
	}
	if res.Query != "charge" || res.Status != mull.SearchOpen || res.Project != "" || len(res.Hits) != 2 || len(projects) != 2 || res.Open != 2 {
		t.Fatalf("search = %+v", res)
	}
}

// project= searches one project, and status= picks open, finished or all
// records; the counts cover every status, so the page can say what the
// filter left out.
func TestSearchEndpointScopeAndStatus(t *testing.T) {
	a, b := onMull(t), onMull(t)
	add := func(dir, title string) (*mull.Mull, string) {
		t.Helper()
		m, err := mull.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		it, err := m.ItemAdd(mull.NewItem{Plan: "none", Title: title})
		if err != nil {
			t.Fatal(err)
		}
		return m, it.ID
	}
	add(a, "Charge cards")
	mb, dropped := add(b, "Charge the old cards")
	if _, err := mb.Drop([]string{dropped}, "not needed", mull.SettleOpts{OnRequest: true}); err != nil {
		t.Fatal(err)
	}
	add(b, "Charge new cards")
	h := Handler(func() ([]string, error) { return []string{a, b}, nil })
	ids := projectIDs([]string{a, b})
	search := func(query string) SearchResult {
		t.Helper()
		rec := get(t, h, "/api/search?q=charge"+query)
		if rec.Code != 200 {
			t.Fatalf("GET /api/search?q=charge%s: %d %s", query, rec.Code, rec.Body)
		}
		return decode[SearchResult](t, rec)
	}
	where := func(res SearchResult) map[string]int {
		n := map[string]int{}
		for _, h := range res.Hits {
			n[h.Project]++
		}
		return n
	}

	if res := search(""); len(res.Hits) != 2 || res.Open != 2 || res.Finished != 1 {
		t.Errorf("open search = %+v; want the 2 open items, 1 finished left out", res)
	}
	if res := search("&status=finished"); res.Status != mull.SearchFinished || len(res.Hits) != 1 || res.Hits[0].ID != dropped || !res.Hits[0].Finished || res.Open != 2 {
		t.Errorf("finished search = %+v; want the dropped item", res)
	}
	if res := search("&status=all"); len(res.Hits) != 3 {
		t.Errorf("all search = %+v; want 3", res)
	}
	res := search("&status=all&project=" + ids[1])
	if n := where(res); res.Project != ids[1] || len(res.Hits) != 2 || n[ids[1]] != 2 || res.Open != 1 || res.Finished != 1 {
		t.Errorf("search of %s = %+v; want its 2 items, counted there only", ids[1], res)
	}
	if res := search("&project=" + ids[0]); len(res.Hits) != 1 || res.Hits[0].Project != ids[0] || res.Finished != 0 {
		t.Errorf("search of %s = %+v", ids[0], res)
	}
	wantMullError(t, "an unknown project", get(t, h, "/api/search?q=charge&project=nope"), 404, mull.CodeNotFound)
	wantMullError(t, "an unknown status", get(t, h, "/api/search?q=charge&status=archive"), 400, mull.CodeInvalid)
}
