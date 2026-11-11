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
	if res.Query != "charge" || len(res.Hits) != 2 || len(projects) != 2 {
		t.Fatalf("search = %+v", res)
	}
}
