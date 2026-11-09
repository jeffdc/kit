package mull

import (
	"errors"
	"os"
	"path/filepath"
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

	_, err := m.Done(it.ID, DoneInput{Summary: "Adds the invoices page."})
	wantErr(t, err, "UI files changed")
	if runs() != 1 {
		t.Fatalf("checks ran %d times, want 1", runs())
	}

	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{
		Summary: "Adds the invoices page.",
		Tour:    []string{"Open /invoices", "See the empty state"},
	}))
	if runs() != 1 {
		t.Fatalf("the cached pass for the same tree was not reused: %d runs", runs())
	}
	if res.Item.State != model.StateDone || res.Review == "" || !res.Report.Passed || res.Report.N != 1 {
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
	for _, s := range []string{"Status:** PASSED", "1. Open /invoices", "**ui**: 1 files", "A web/invoices.html (+1 −0)"} {
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

	_, err := m.Done(it.ID, DoneInput{Summary: "Webhook handler."})
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
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Webhook handler."}))
	if res.Report.N != 2 || !res.Report.Passed {
		t.Fatalf("second attempt = %+v", res.Report)
	}
}

func TestDoneGuards(t *testing.T) {
	m, dir := setup(t)
	_, err := m.Done("abcd", DoneInput{})
	wantErr(t, err, "needs a summary")

	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A"}))
	_, err = m.Done(it.ID, DoneInput{Summary: "x"})
	wantErr(t, err, "not started")

	must[StartResult](t)(m.Start(it.ID))
	_, err = m.Done(it.ID, DoneInput{Summary: "x"})
	wantErr(t, err, "no hard checks configured")

	must[model.Config](t)(m.ConfigChecks([]string{"true"}))
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = m.Done(it.ID, DoneInput{Summary: "x"})
	wantErr(t, err, "uncommitted or untracked")
	gittest.Git(t, dir, "add", "dirty.txt")
	gittest.Git(t, dir, "commit", "-q", "-m", "commit it")

	wt := gittest.AddWorktree(t, dir, "other")
	other := openAt(t, wt)
	_, err = other.Done(it.ID, DoneInput{Summary: "x"})
	wantErr(t, err, "claimed by "+dir)
}

func TestDoneOverrideWithoutChecks(t *testing.T) {
	m, dir := setup(t)
	it := startWork(t, m, dir, "Docs tweak")
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Docs only.", Override: "no checks in this repo yet"}))
	if res.Item.State != model.StateDone || res.Report.Passed || res.Report.Override == "" {
		t.Fatalf("override done = %+v", res)
	}
	text, _, _ := m.ReportText(it.ID, 0)
	if !strings.Contains(text, "OVERRIDDEN") {
		t.Fatalf("report:\n%s", text)
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
