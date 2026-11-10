package mull

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
)

// counterCheck returns a check command that appends a line to a file outside
// the repository, and a function reporting how many times it ran.
func counterCheck(t *testing.T, exit int) (string, func() int) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "runs")
	cmd := "echo run >> '" + file + "'"
	if exit != 0 {
		cmd += "; echo boom; exit " + itoa(exit)
	}
	return cmd, func() int {
		b, _ := os.ReadFile(file)
		return strings.Count(string(b), "run")
	}
}

func startWork(t *testing.T, m *Mull, dir, title string) ItemView {
	t.Helper()
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: title}))
	must[StartResult](t)(m.Start(it.ID))
	return it
}

func TestDoneHappyPathCacheAndTour(t *testing.T) {
	m, dir := setup(t)
	check, runs := counterCheck(t, 0)
	must[model.Config](t)(m.ConfigChecks([]string{check}))
	must[model.Config](t)(m.ConfigLayer("ui", []string{"web/**"}))
	must[model.Config](t)(m.ConfigLayer("tests", []string{"test/**", "**/*_test.go"}))

	it := startWork(t, m, dir, "Invoices page")
	gittest.Commit(t, dir, "web/invoices.html", "<h1>Invoices</h1>\n", "Add invoices page", "Mull: "+it.ID)
	gittest.Commit(t, dir, "test/invoices_test.go", "package x\n", "Test it", "Mull: "+it.ID)
	gittest.Commit(t, dir, "lib/billing/invoice.go", "package billing\n", "Model", "Mull: "+it.ID)

	_, err := m.Done(it.ID, DoneInput{Summary: "Adds the invoices page.", Design: "A static page."})
	wantErr(t, err, "UI files changed")
	if runs() != 1 {
		t.Fatalf("checks ran %d times, want 1", runs())
	}

	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{
		Summary: "Adds the invoices page.",
		Tour:    []string{"Open /invoices", "See the empty state"},
		Design:  "A static page.",
	}))
	if runs() != 1 {
		t.Fatalf("the cached pass for the same tree was not reused: %d runs", runs())
	}
	if res.Item.State != model.StateInReview || res.Review == "" || !res.Report.Passed || res.Report.N != 1 {
		t.Fatalf("done = %+v", res)
	}
	layers := map[string]int{}
	for _, lc := range res.Report.Changes {
		layers[lc.Layer] = len(lc.Files)
	}
	if layers["ui"] != 1 || layers["tests"] != 1 || layers["other"] != 1 {
		t.Fatalf("layers = %v", layers)
	}

	text, rep, err := m.ReportText(it.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Design != "A static page." {
		t.Errorf("report design = %q", rep.Design)
	}
	for _, s := range []string{"Status:** PASSED", "## What was built", "1. Open /invoices", "## How it was built",
		"### Design notes\n\nA static page.", "### Decisions made in the session\n\nNone recorded.",
		"**ui**: 1 files", "A web/invoices.html (+1 −0)"} {
		if !strings.Contains(text, s) {
			t.Errorf("report lacks %q:\n%s", s, text)
		}
	}
	if rep.Commit != gittest.Git(t, dir, "rev-parse", "HEAD") {
		t.Errorf("report commit = %s", rep.Commit)
	}

	review := must[[]*model.Entry](t)(m.Inbox(InboxFilter{Kind: model.KindReview}))
	if len(review) != 1 || review[0].Item != it.ID || review[0].Source != res.ReportPath {
		t.Fatalf("review entry = %+v", review)
	}
}

func TestDoneFailureKeepsItemActive(t *testing.T) {
	m, dir := setup(t)
	failing, _ := counterCheck(t, 3)
	must[model.Config](t)(m.ConfigChecks([]string{failing}))
	it := startWork(t, m, dir, "Webhook")
	gittest.Commit(t, dir, "hook.go", "package x\n", "Webhook", "Mull: "+it.ID)

	_, err := m.Done(it.ID, DoneInput{Summary: "Webhook handler.", Design: "One handler."})
	var failed *ErrChecksFailed
	if !errors.As(err, &failed) || failed.Report != model.ReportPath(it.ID, 1) {
		t.Fatalf("got %v, want ErrChecksFailed for report 1", err)
	}
	if st := item(t, m, it.ID).State; st != model.StateActive {
		t.Fatalf("item is %s after failed checks, want active", st)
	}
	text, rep, err := m.ReportText(it.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Passed || !strings.Contains(text, "FAILED") || !strings.Contains(text, "boom") {
		t.Fatalf("failed report:\n%s", text)
	}
	v := must[View](t)(m.Board(""))
	if probs := problemsByItem(v.Plans[0]); probs[it.ID] != "failed-done" {
		t.Fatalf("problems = %v", probs)
	}
	if n := len(must[[]*model.Entry](t)(m.Inbox(InboxFilter{Kind: model.KindReview}))); n != 0 {
		t.Fatal("a failed done filed a review")
	}

	// Fix the check; a new command list means the old result is not reused.
	must[model.Config](t)(m.ConfigChecks([]string{"true"}))
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Webhook handler.", Design: "One handler."}))
	if res.Report.N != 2 || !res.Report.Passed {
		t.Fatalf("second attempt = %+v", res.Report)
	}
}

func TestDoneGuards(t *testing.T) {
	m, dir := setup(t)
	_, err := m.Done("abcd", DoneInput{})
	wantErr(t, err, "needs a summary")
	for _, design := range []string{"", "  ", "none", "None."} {
		_, err = m.Done("abcd", DoneInput{Summary: "x", Design: design})
		wantCode(t, err, CodeDesignRequired)
	}

	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A"}))
	_, err = m.Done(it.ID, DoneInput{Summary: "x", Design: "y"})
	wantErr(t, err, "not started")

	must[StartResult](t)(m.Start(it.ID))
	_, err = m.Done(it.ID, DoneInput{Summary: "x", Design: "y"})
	wantErr(t, err, "no hard checks configured")

	must[model.Config](t)(m.ConfigChecks([]string{"true"}))
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = m.Done(it.ID, DoneInput{Summary: "x", Design: "y"})
	wantErr(t, err, "uncommitted or untracked")
	gittest.Git(t, dir, "add", "dirty.txt")
	gittest.Git(t, dir, "commit", "-q", "-m", "commit it")

	wt := gittest.AddWorktree(t, dir, "other")
	other := openAt(t, wt)
	_, err = other.Done(it.ID, DoneInput{Summary: "x", Design: "y"})
	wantErr(t, err, "claimed by "+dir)
}

func TestDoneOverrideWithoutChecks(t *testing.T) {
	m, dir := setup(t)
	it := startWork(t, m, dir, "Docs tweak")
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Docs only.", Design: "Prose only.", Override: "no checks in this repo yet"}))
	if res.Item.State != model.StateInReview || res.Report.Passed || res.Report.Override == "" {
		t.Fatalf("override done = %+v", res)
	}
	text, _, _ := m.ReportText(it.ID, 0)
	if !strings.Contains(text, "OVERRIDDEN") {
		t.Fatalf("report:\n%s", text)
	}
}

func decision(t *testing.T, m *Mull, item, text string) *model.Entry {
	t.Helper()
	return must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindDecision, Text: text, Item: item, Detail: "why " + text}))
}

// sameIDs is ids in a canonical order: entries made in the same second have
// no defined order.
func sameIDs(ids ...string) string {
	s := append([]string{}, ids...)
	sort.Strings(s)
	return strings.Join(s, " ")
}

func entry(t *testing.T, m *Mull, id string) *model.Entry {
	t.Helper()
	v := must[Shown](t)(m.Show(id))
	if v.Entry == nil {
		t.Fatalf("%s is a %s, not an entry", id, v.Type)
	}
	return v.Entry
}

func TestReviewCarriesDecisions(t *testing.T) {
	m, dir := setup(t)
	must[model.Config](t)(m.ConfigChecks([]string{"true"}))
	it := startWork(t, m, dir, "Webhook")
	other := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "Other"}))
	d1 := decision(t, m, it.ID, "Store raw payloads")
	d2 := decision(t, m, it.ID, "Retry three times")
	d3 := decision(t, m, it.ID, "Log to stderr")
	parked := decision(t, m, it.ID, "Use a queue")
	must[*model.Entry](t)(m.Park(parked.ID, "later", SettleOpts{}))
	elsewhere := decision(t, m, other.ID, "Not this item's")
	gittest.Commit(t, dir, "hook.go", "package x\n", "Webhook", "Mull: "+it.ID)

	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Webhook handler.", Design: "One handler, retries in a loop."}))
	want := sameIDs(d1.ID, d2.ID, d3.ID)
	var got []string
	for _, d := range res.Report.Decisions {
		got = append(got, d.ID)
		if d.Detail != "why "+d.Text {
			t.Errorf("report decision %+v lacks its detail", d)
		}
	}
	if sameIDs(got...) != want {
		t.Fatalf("report decisions = %+v, want %v", res.Report.Decisions, want)
	}
	review := entry(t, m, res.Review)
	if sameIDs(review.Decisions...) != want {
		t.Fatalf("review carries %v, want %v", review.Decisions, want)
	}
	text, _, _ := m.ReportText(it.ID, 0)
	for _, s := range []string{"One handler, retries in a loop.", "**" + d2.ID + "** Retry three times", "Why: why Retry three times"} {
		if !strings.Contains(text, s) {
			t.Errorf("report lacks %q:\n%s", s, text)
		}
	}

	// Decide one differently, then accept the review: the rest are ratified with it.
	must[*model.Entry](t)(m.Decide(d2.ID, "Retry five times", SettleOpts{}))
	acc := must[AcceptResult](t)(m.Accept([]string{it.ID}, "", SettleOpts{OnRequest: true}))
	settled := map[string]bool{}
	for _, e := range acc.Entries {
		settled[e.ID] = true
	}
	if len(acc.Entries) != 3 || !settled[res.Review] || !settled[d1.ID] || !settled[d3.ID] {
		t.Fatalf("accept settled %v, want the review, %s and %s", settled, d1.ID, d3.ID)
	}
	if s := entry(t, m, d1.ID).Settled; s == nil || s.Action != model.ActAccept || !s.OnRequest {
		t.Fatalf("carried decision settled as %+v", s)
	}
	if s := entry(t, m, d2.ID).Settled; s.Action != model.ActDecide || s.Note != "Retry five times" {
		t.Fatalf("overridden decision settled as %+v", s)
	}
	if s := entry(t, m, parked.ID).Settled; s.Action != model.ActPark {
		t.Fatalf("parked decision settled as %+v", s)
	}
	if entry(t, m, elsewhere.ID).Settled != nil {
		t.Fatal("another item's decision was settled")
	}
	log := map[string]string{}
	for _, d := range must[[]Decision](t)(m.Decisions("")) {
		log[d.ID] = d.Decision
	}
	if log[d1.ID] != "Store raw payloads" || log[d2.ID] != "Retry five times" {
		t.Fatalf("decision log = %v", log)
	}
}

func TestSentBackDecisionsStayOpen(t *testing.T) {
	m, dir := setup(t)
	must[model.Config](t)(m.ConfigChecks([]string{"true"}))
	it := startWork(t, m, dir, "Webhook")
	d1 := decision(t, m, it.ID, "Store raw payloads")
	first := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Webhook handler.", Design: "One handler."}))
	must[DoResult](t)(m.Do(DoInput{ID: first.Review, Reason: "No tests"}))
	if entry(t, m, d1.ID).Settled != nil {
		t.Fatal("sending the review back settled its decision")
	}

	must[StartResult](t)(m.Start(it.ID))
	d2 := decision(t, m, it.ID, "Table-driven tests")
	second := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Webhook handler, tested.", Design: "One handler."}))
	if got := entry(t, m, second.Review).Decisions; sameIDs(got...) != sameIDs(d1.ID, d2.ID) {
		t.Fatalf("second review carries %v", got)
	}
	// Naming a carried decision alongside its review is not an error.
	acc := must[AcceptResult](t)(m.Accept([]string{second.Review, d1.ID}, "", SettleOpts{OnRequest: true}))
	if len(acc.Entries) != 3 || entry(t, m, d2.ID).Settled.Note != "with review "+second.Review {
		t.Fatalf("accept = %+v", acc.Entries)
	}
}

func TestCheckRecordsAndReuses(t *testing.T) {
	m, _ := setup(t)
	check, runs := counterCheck(t, 0)
	must[model.Config](t)(m.ConfigChecks([]string{check}))
	first := must[CheckOutcome](t)(m.Check())
	second := must[CheckOutcome](t)(m.Check())
	if !first.Passed || first.Cached || !second.Cached || runs() != 1 {
		t.Fatalf("first %+v, second %+v, runs %d", first, second, runs())
	}
}

func TestGlobs(t *testing.T) {
	cases := []struct {
		glob, path string
		want       bool
	}{
		{"lib/web/**", "lib/web/live/page.ex", true},
		{"lib/web/**", "lib/webby/x", false},
		{"**/*_test.go", "a/b/c_test.go", true},
		{"**/*_test.go", "c_test.go", true},
		{"assets/*.css", "assets/app.css", true},
		{"assets/*.css", "assets/sub/app.css", false},
	}
	for _, c := range cases {
		re, err := globRegexp(c.glob)
		if err != nil {
			t.Fatal(err)
		}
		if got := re.MatchString(c.path); got != c.want {
			t.Errorf("%s ~ %s = %v, want %v", c.glob, c.path, got, c.want)
		}
	}
}
