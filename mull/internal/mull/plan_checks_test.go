package mull

import (
	"errors"
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
)

func TestPlanChecksRunAfterConfig(t *testing.T) {
	m, dir := setup(t)
	cfgCheck, cfgRuns := counterCheck(t, 0)
	planCheck, planRuns := counterCheck(t, 0)
	must[model.Config](t)(m.ConfigChecks([]string{cfgCheck}, true))
	// Blanks are dropped and commands trimmed; a copy of config's is kept
	// but never run twice.
	p := must[*model.Plan](t)(m.PlanChecks("general", []string{" " + planCheck + " ", "", cfgCheck}, true))
	if len(p.Checks) != 2 || p.Checks[0] != planCheck || p.Checks[1] != cfgCheck {
		t.Fatalf("plan checks = %q", p.Checks)
	}

	it := startWork(t, m, dir, "Webhook")
	gittest.Commit(t, dir, "lib/hook.go", "package lib\n", "Hook", "Mull: "+it.ID)
	// Without an item, check runs config's only.
	only := must[CheckOutcome](t)(m.Check(""))
	if len(only.Results) != 1 || planRuns() != 0 {
		t.Fatalf("check without an item = %+v", only)
	}
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Adds the hook.", Design: "One handler."}))
	got := res.Report.Checks.Results
	if len(got) != 2 || got[0].Command != cfgCheck || got[1].Command != planCheck || !res.Report.Passed {
		t.Fatalf("done ran %+v", got)
	}
	// The config-only pass on the same tree has a different command list,
	// so done ran them all; a check for the item now reuses done's pass.
	if cfgRuns() != 2 || planRuns() != 1 {
		t.Fatalf("runs: config %d, plan %d", cfgRuns(), planRuns())
	}
	if again := must[CheckOutcome](t)(m.Check(it.ID)); !again.Cached || planRuns() != 1 {
		t.Fatalf("check for the item after done = %+v", again)
	}
	text, _, err := m.ReportText(it.ID, 0)
	if err != nil || !strings.Contains(text, "`"+planCheck+"`: pass") {
		t.Errorf("report text lacks the plan's check (%v):\n%s", err, text)
	}
}

func TestPlanChecksOnlyForThePlansItems(t *testing.T) {
	m, dir := setup(t)
	failing, _ := counterCheck(t, 3)
	must[model.Config](t)(m.ConfigChecks([]string{"true"}, true))
	must[*model.Plan](t)(m.PlanNew("other", "Other", "", true))
	must[*model.Plan](t)(m.PlanAgree("other", true))
	must[*model.Plan](t)(m.PlanChecks("other", []string{failing}, true))

	// An item in general, and one with no plan, don't run other's checks.
	it := startWork(t, m, dir, "Webhook")
	gittest.Commit(t, dir, "lib/hook.go", "package lib\n", "Hook", "Mull: "+it.ID)
	if res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Hook.", Design: "One handler."})); len(res.Report.Checks.Results) != 1 {
		t.Fatalf("general item ran %+v", res.Report.Checks.Results)
	}
	loose := must[ItemView](t)(m.ItemAdd(NewItem{Plan: model.NoPlan, Title: "Typo"}))
	must[StartResult](t)(m.Start(loose.ID))
	if out := must[CheckOutcome](t)(m.Check(loose.ID)); len(out.Results) != 1 {
		t.Fatalf("unplanned item ran %+v", out.Results)
	}

	// An item in other fails on its plan's check and stays active.
	ot := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "other", Title: "Page"}))
	must[StartResult](t)(m.Start(ot.ID))
	gittest.Commit(t, dir, "lib/page.go", "package lib\n", "Page", "Mull: "+ot.ID)
	res, err := m.Done(ot.ID, DoneInput{Summary: "Page.", Design: "A page."})
	var failed *ErrChecksFailed
	if !errors.As(err, &failed) || res.Report.Passed || len(res.Report.Checks.Results) != 2 || res.Report.Checks.Results[1].Exit != 3 {
		t.Fatalf("done in other = %v, %+v", err, res.Report.Checks)
	}
	if st := item(t, m, ot.ID).State; st != model.StateActive {
		t.Fatalf("item %s is %s after failing its plan's check", ot.ID, st)
	}

	// With no config checks, a plan's own are enough to prove done.
	must[model.Config](t)(m.ConfigChecks(nil, true))
	must[*model.Plan](t)(m.PlanChecks("other", []string{"true"}, true))
	if res := must[DoneResult](t)(m.Done(ot.ID, DoneInput{Summary: "Page.", Design: "A page."})); !res.Report.Passed || len(res.Report.Checks.Results) != 1 {
		t.Fatalf("done on plan checks only = %+v", res.Report)
	}
	_, err = m.Check("")
	wantCode(t, err, CodeNoChecks)
	wantErr(t, err, "mull plan checks")
}

func TestPlanChecksWrites(t *testing.T) {
	m, dir := setup(t)
	_, err := m.PlanChecks("nope", []string{"true"}, true)
	wantCode(t, err, CodeNotFound)

	seen := head(t, dir)
	results := must[[]BatchResult](t)(m.Batch(strings.NewReader(`{"op": "plan checks", "slug": "general", "commands": "make e2e", "on-request": true}`+"\n"), false))
	if p := results[0].Result.(*model.Plan); len(p.Checks) != 1 || p.Checks[0] != "make e2e" || results[0].ID != "general" {
		t.Fatalf("batch plan checks = %+v", results[0])
	}
	c, ok := byRecord(must[Revision](t)(m.Revision(seen)).Changes)["plan:general"]
	if f, has := fieldOf(c, "checks"); !ok || !has || f.From != "" || f.To != "" {
		t.Fatalf("change = %+v", c)
	}
	// The same list again writes nothing; no commands removes them.
	tip := head(t, dir)
	must[*model.Plan](t)(m.PlanChecks("general", []string{"make e2e"}, true))
	if head(t, dir) != tip {
		t.Errorf("unchanged plan checks wrote a commit")
	}
	if p := must[*model.Plan](t)(m.PlanChecks("general", nil, true)); p.Checks != nil {
		t.Errorf("cleared plan checks = %q", p.Checks)
	}
	if p := must[PlanView](t)(m.PlanShow("general")); p.Checks != nil {
		t.Errorf("plan show checks = %q", p.Checks)
	}
}

// The checks judge agents' work, so an agent changes them, config's or a
// plan's, only on the human's word (on-request); the human, as mull serve
// writes, needs no flag.
func TestChecksAreTheHumans(t *testing.T) {
	m, dir := setup(t)
	tip := head(t, dir)
	_, err := m.ConfigChecks([]string{"true"}, false)
	wantCode(t, err, CodeNotAllowed)
	wantErr(t, err, "--on-request")
	_, err = m.PlanChecks("general", []string{"true"}, false)
	wantCode(t, err, CodeNotAllowed)
	wantErr(t, err, "--on-request")
	// Clearing them is a change too.
	_, err = m.ConfigChecks(nil, false)
	wantCode(t, err, CodeNotAllowed)
	for _, line := range []string{
		`{"op": "config checks", "commands": "true"}`,
		`{"op": "plan checks", "slug": "general", "commands": "true"}`,
	} {
		_, err = m.Batch(strings.NewReader(line+"\n"), false)
		if e := batchErr(t, err); len(e.Lines) != 1 || e.Lines[0].Code != CodeNotAllowed {
			t.Errorf("batch %s = %+v", line, e.Lines)
		}
	}
	if head(t, dir) != tip {
		t.Fatalf("a refused checks change wrote a commit")
	}

	h := human(t, m, dir)
	if c := must[model.Config](t)(h.ConfigChecks([]string{"true"}, false)); len(c.DoneChecks) != 1 {
		t.Errorf("the human's config checks = %q", c.DoneChecks)
	}
	if p := must[*model.Plan](t)(h.PlanChecks("general", []string{"exit 0"}, false)); len(p.Checks) != 1 {
		t.Errorf("the human's plan checks = %q", p.Checks)
	}
}
