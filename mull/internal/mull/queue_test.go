package mull

import (
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
)

func TestAreas(t *testing.T) {
	for _, c := range []struct {
		area, path string
		want       bool
	}{
		{"lib/web", "lib/web/page.ex", true},
		{"lib/web/", "lib/web/live/x.ex", true},
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
	q1 := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Footer link is 404", Area: "web/footer"}))
	q2 := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Footer wraps at 390px", Area: "web/footer/**"}))
	q3 := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Footer copyright year", Area: "web/footer"}))
	elsewhere := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Slow import", Area: "lib/import"}))

	// Three in one area is a cluster.
	v := must[View](t)(m.Board(""))
	if v.Queue != 4 || len(v.Clusters) != 1 || v.Clusters[0].Area != "web/footer" || v.Clusters[0].Count != 3 {
		t.Fatalf("queue %d, clusters %+v", v.Queue, v.Clusters)
	}

	// Work in the area gets the queue there in its brief.
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "Footer rework", Area: "web/footer"}))
	must[StartResult](t)(m.Start(it.ID))
	ctx := must[Context](t)(m.Context(it.ID))
	got := map[string]bool{}
	for _, b := range ctx.Item.Queue {
		got[b.ID] = true
	}
	if !got[q1.ID] || !got[q2.ID] || !got[q3.ID] || got[elsewhere.ID] {
		t.Fatalf("brief queue = %+v", ctx.Item.Queue)
	}

	// It fixes two and says so; the third is listed as nearby.
	gittest.Commit(t, dir, "web/footer/footer.html", "<footer>\n", "Footer", "Mull: "+it.ID)
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Footer reworked.", Rules: "none", Covers: []string{q1.ID, q2.ID}}))
	if strings.Join(res.Report.Covered, ",") != q1.ID+","+q2.ID || strings.Join(res.Report.Nearby, ",") != q3.ID {
		t.Fatalf("covered %v, nearby %v", res.Report.Covered, res.Report.Nearby)
	}
	if c := item(t, m, q1.ID); c.State != model.StateDone || c.CoveredBy != it.ID {
		t.Fatalf("covered item = %+v", c)
	}
	if v = must[View](t)(m.Board("")); v.Queue != 2 || len(v.Clusters) != 0 {
		t.Fatalf("after: queue %d, clusters %+v", v.Queue, v.Clusters)
	}
}

func TestStartingQueuedWorkKeepsAPhaseComplete(t *testing.T) {
	m, _ := setup(t)
	ph := must[*model.Phase](t)(m.PhaseAdd("general", "Shell", "", nil))
	done := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "Masthead"}))
	must[StartResult](t)(m.Start(done.ID))
	markInReview(t, m, done.ID)
	must[AcceptResult](t)(m.Accept([]string{done.ID}, "", SettleOpts{OnRequest: true}))

	bug := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Masthead wraps", Phase: ph.ID}))
	res := must[StartResult](t)(m.Start(bug.ID))
	if res.Item.Phase != "" || res.Item.Plan != "general" || !strings.Contains(res.Item.Notes, "About phase "+ph.ID) || len(res.Warnings) == 0 {
		t.Fatalf("started = %+v", res)
	}
	if !must[Shown](t)(m.Show(ph.ID)).Phase.Complete {
		t.Fatal("the phase reopened")
	}
}

func TestMovingIsAMove(t *testing.T) {
	m, _ := setup(t)
	agreedPlan(t, m, "launch", "Launch", "")
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "Pricing page"}))
	moved := must[SetResult](t)(m.Set(it.ID, "plan", "launch"))
	if moved.Item.Plan != "launch" || moved.Item.ID != it.ID {
		t.Fatalf("moved = %+v", moved.Item)
	}
	must[StartResult](t)(m.Start(it.ID))
	_, err := m.Set(it.ID, "plan", "general")
	wantCode(t, err, CodeState)
}
