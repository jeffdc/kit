package mull

import (
	"reflect"
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
)

func TestTourParts(t *testing.T) {
	const pv = "http://localhost:4000"
	cases := []struct {
		step, preview string
		want          []TourPart
	}{
		{"Open /trees/quercus-alba and see the map", pv, []TourPart{
			{Text: "Open "}, {Text: "/trees/quercus-alba", URL: pv + "/trees/quercus-alba"}, {Text: " and see the map"}}},
		// Sentence punctuation ends the path; brackets and quotes open one.
		{"Go to /trees.", pv, []TourPart{{Text: "Go to "}, {Text: "/trees", URL: pv + "/trees"}, {Text: "."}}},
		{"Open (/a?x=1#top), then `/b`", pv, []TourPart{
			{Text: "Open ("}, {Text: "/a?x=1#top", URL: pv + "/a?x=1#top"}, {Text: "), then `"}, {Text: "/b", URL: pv + "/b"}, {Text: "`"}}},
		{"/", pv, []TourPart{{Text: "/", URL: pv + "/"}}},
		// A path inside a URL, a word with a slash, and // are not paths.
		{"See http://example.com/x and and/or //cdn", pv, []TourPart{{Text: "See http://example.com/x and and/or //cdn"}}},
		// No preview: nothing resolves.
		{"Open /trees", "", []TourPart{{Text: "Open /trees"}}},
		// A preview under a path keeps it; its trailing slash, query and
		// fragment are dropped.
		{"Open /trees", "https://example.com/app/?v=1#x", []TourPart{{Text: "Open "}, {Text: "/trees", URL: "https://example.com/app/trees"}}},
		{"Open /#/demo/plans/x", "http://localhost:6899/#/demo", []TourPart{{Text: "Open "}, {Text: "/#/demo/plans/x", URL: "http://localhost:6899/#/demo/plans/x"}}},
	}
	for _, c := range cases {
		if got := tourParts(c.step, c.preview); !reflect.DeepEqual(got, c.want) {
			t.Errorf("tourParts(%q, %q) = %+v, want %+v", c.step, c.preview, got, c.want)
		}
	}
	if got := resolveTourStep("Open /trees, then /maps.", pv); got != "Open http://localhost:4000/trees, then http://localhost:4000/maps." {
		t.Errorf("resolveTourStep = %q", got)
	}
}

func TestTourWarnings(t *testing.T) {
	tour := []string{
		"Open /trees",                            // a path: fine
		"Open http://localhost:4001/trees/x?y=1", // another local server
		"Open http://localhost:4000/maps",        // the preview: fine
		"See https://staging.example.com/a",      // not local: fine
		"Open 127.0.0.1:5000/b.",                 // scheme-less, with a port
	}
	w := tourWarnings(tour, "http://localhost:4000")
	if len(w) != 2 {
		t.Fatalf("warnings = %q, want two", w)
	}
	if !strings.Contains(w[0], "tour step 2 gives http://localhost:4001/trees/x?y=1") || !strings.Contains(w[0], "isn't the plan's preview (http://localhost:4000)") || !strings.Contains(w[0], "give the path (/trees/x?y=1)") {
		t.Errorf("warning 1 = %q", w[0])
	}
	if !strings.Contains(w[1], "tour step 5 gives 127.0.0.1:5000/b,") || !strings.Contains(w[1], "(/b)") {
		t.Errorf("warning 2 = %q", w[1])
	}
	// With no preview, every local URL warns, and says the plan has none.
	w = tourWarnings(tour, "")
	if len(w) != 3 || !strings.Contains(w[1], "http://localhost:4000/maps, a local URL, and the item's plan has no preview: give the path (/maps)") {
		t.Errorf("no preview: %q", w)
	}
	if w := tourWarnings([]string{"Open http://app.localhost:3000/", "Open http://[::1]:3000/"}, ""); len(w) != 2 {
		t.Errorf("app.localhost and ::1 are local: %q", w)
	}
}

// A tour that gives local URLs other than the preview: done warns, but
// finishes, and the report carries the warning. report's text and mull
// serve's packet resolve the tour's paths against the preview as it is
// when they're read; the report keeps the steps as written.
func TestDoneTourAgainstPreview(t *testing.T) {
	m, dir := setup(t)
	check, _ := counterCheck(t, 0)
	must[model.Config](t)(m.ConfigChecks([]string{check}, true))
	must[model.Config](t)(m.ConfigLayer("ui", []string{"web/**"}))
	agreedPlan(t, m, "site", "The site", "")
	must[SetResult](t)(m.Set("site", "preview", "http://localhost:4000"))

	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "site", Title: "Tree page"}))
	must[StartResult](t)(m.Start(it.ID))
	gittest.Commit(t, dir, "web/tree.html", "<h1>Tree</h1>\n", "Tree page", "Mull: "+it.ID)

	tour := []string{"Open /trees/quercus-alba", "Then http://localhost:4001/maps: see the pin"}
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Adds the tree page.", Tour: tour, Design: "A template."}))
	if res.Item.State != model.StateInReview {
		t.Fatalf("a warning stopped done: %+v", res.Item)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "tour step 2 gives http://localhost:4001/maps,") {
		t.Fatalf("done warnings = %q", res.Warnings)
	}
	if !reflect.DeepEqual(res.Report.Warnings, res.Warnings) || !reflect.DeepEqual(res.Report.Tour, tour) {
		t.Errorf("report warnings %q, tour %q", res.Report.Warnings, res.Report.Tour)
	}

	out := must[ReportOut](t)(m.ReportText(it.ID, 0))
	if out.Preview != "http://localhost:4000" || !reflect.DeepEqual(out.Report.Tour, tour) {
		t.Errorf("report preview %q, tour %q", out.Preview, out.Report.Tour)
	}
	for _, s := range []string{
		"1. Open http://localhost:4000/trees/quercus-alba\n",
		"2. Then http://localhost:4001/maps: see the pin\n",
		"**Warning:** tour step 2 gives http://localhost:4001/maps,",
		"### Checks (run by mull)",
	} {
		if !strings.Contains(out.Text, s) {
			t.Errorf("report text lacks %q:\n%s", s, out.Text)
		}
	}

	page := must[ItemPage](t)(m.ItemPage(it.ID))
	want := [][]TourPart{
		{{Text: "Open "}, {Text: "/trees/quercus-alba", URL: "http://localhost:4000/trees/quercus-alba"}},
		{{Text: "Then http://localhost:4001/maps: see the pin"}},
	}
	if page.Review == nil || page.Review.Preview != "http://localhost:4000" || !reflect.DeepEqual(page.Review.Tour, want) {
		t.Fatalf("review packet = %+v", page.Review)
	}

	// A new preview: the same report resolves against it.
	must[SetResult](t)(m.Set("site", "preview", "https://site.example.com/"))
	out = must[ReportOut](t)(m.ReportText(it.ID, 0))
	if !strings.Contains(out.Text, "1. Open https://site.example.com/trees/quercus-alba\n") {
		t.Errorf("report text after the preview changed:\n%s", out.Text)
	}
	// None: the steps read as written.
	must[SetResult](t)(m.Set("site", "preview", ""))
	out = must[ReportOut](t)(m.ReportText(it.ID, 0))
	if out.Preview != "" || !strings.Contains(out.Text, "1. Open /trees/quercus-alba\n") {
		t.Errorf("report text with no preview:\n%s", out.Text)
	}
	if page := must[ItemPage](t)(m.ItemPage(it.ID)); page.Review.Tour[0][0].URL != "" || len(page.Review.Tour[0]) != 1 {
		t.Errorf("packet tour with no preview = %+v", page.Review.Tour)
	}
}

// An item with no plan has no preview: its tour reads as written.
func TestReportTextUnplanned(t *testing.T) {
	m, dir := setup(t)
	check, _ := counterCheck(t, 0)
	must[model.Config](t)(m.ConfigChecks([]string{check}, true))
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "none", Title: "Loose"}))
	must[StartResult](t)(m.Start(it.ID))
	gittest.Commit(t, dir, "a.txt", "a\n", "A", "Mull: "+it.ID)
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "S.", Tour: []string{"Open /a"}, Design: "D."}))
	if len(res.Warnings) != 0 {
		t.Errorf("warnings = %q", res.Warnings)
	}
	out := must[ReportOut](t)(m.ReportText(it.ID, 0))
	if out.Preview != "" || !strings.Contains(out.Text, "1. Open /a\n") {
		t.Errorf("report:\n%s", out.Text)
	}
}
