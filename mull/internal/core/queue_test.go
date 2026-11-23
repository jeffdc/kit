package core

import (
	"strings"
	"testing"

	"mull/internal/gittest"
)

func TestAreas(t *testing.T) {
	for _, c := range []struct {
		area, path string
		want       bool
	}{
		{"lib/web", "lib/web/page.ex", true},
		{"lib/web", "lib/webhook.ex", false},
		{"lib/**/live/*.ex", "lib/web/live/x.ex", true},
		{"", "x", false},
	} {
		if got := areaMatches(c.area, c.path); got != c.want {
			t.Errorf("areaMatches(%q, %q) = %v", c.area, c.path, got)
		}
	}
	if !areasOverlap("lib/web/**", "lib/web/live") || areasOverlap("lib/web", "lib/db") {
		t.Error("areasOverlap")
	}
}

func TestQueueByArea(t *testing.T) {
	m, dir := riskSetup(t)
	var q []*Item
	for _, title := range []string{"Footer link 404s", "Footer wraps", "Footer year"} {
		q = append(q, must[*Item](t)(m.Add(NewItem{Kind: Bug, Title: title, Area: "web/footer"})))
	}
	elsewhere := must[*Item](t)(m.Add(NewItem{Kind: Bug, Title: "Slow import", Area: "lib/import"}))
	if v := must[Surface](t)(m.Surface("")); v.Flow.Queue != 4 || len(v.Clusters) != 1 || v.Clusters[0].Count != 3 {
		t.Fatalf("queue %d, clusters %+v", v.Flow.Queue, v.Clusters)
	}

	it := must[*Item](t)(m.Add(NewItem{Plan: "general", Title: "Footer rework", Area: "web/footer"}))
	must[StartResult](t)(m.Start(it.ID))
	brief := must[Context](t)(m.Brief(it.ID))
	got := map[string]bool{}
	for _, b := range brief.Item.Queue {
		got[b.ID] = true
	}
	if len(got) != 3 || got[elsewhere.ID] {
		t.Fatalf("brief queue = %+v", brief.Item.Queue)
	}

	gittest.Commit(t, dir, "web/footer/footer.html", "<footer>\n", "Footer", "Mull: "+it.ID)
	res := must[DoneOut](t)(m.Done(it.ID, DoneInput{Summary: "Footer reworked.", Rules: "none", Covers: []string{q[0].ID, q[1].ID}}))
	if strings.Join(res.Report.Covered, ",") != q[0].ID+","+q[1].ID || strings.Join(res.Report.Nearby, ",") != q[2].ID {
		t.Fatalf("covered %v, nearby %v", res.Report.Covered, res.Report.Nearby)
	}
	if c := item(t, m, q[0].ID); c.State != Done || c.CoveredBy != it.ID {
		t.Fatalf("covered item = %+v", c)
	}
}
