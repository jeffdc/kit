package mull

import (
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
)

func TestPlanSharedFlaggedByDone(t *testing.T) {
	m, dir := setup(t)
	must[model.Config](t)(m.ConfigChecks([]string{"true"}, true))
	must[model.Config](t)(m.ConfigLayer("styles", []string{"**/*.css"}))
	// Blanks and repeats are dropped, and globs trimmed.
	p := must[*model.Plan](t)(m.PlanShared("general", []string{" app/layout.css ", "", "lib/shared/**", "app/layout.css"}))
	if len(p.Shared) != 2 || p.Shared[0] != "app/layout.css" || p.Shared[1] != "lib/shared/**" {
		t.Fatalf("plan shared = %q", p.Shared)
	}

	it := startWork(t, m, dir, "Account page")
	gittest.Commit(t, dir, "app/layout.css", "/* account */\n", "Layout", "Mull: "+it.ID)
	gittest.Commit(t, dir, "lib/shared/button/button.go", "package button\n", "Button", "Mull: "+it.ID)
	gittest.Commit(t, dir, "app/account.css", "/* account */\n", "Account", "Mull: "+it.ID)
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Rules: reviewRules, Summary: "The account page.", Design: "One page."}))
	want := []model.SharedChange{
		{Status: "A", Path: "app/layout.css", Glob: "app/layout.css"},
		{Status: "A", Path: "lib/shared/button/button.go", Glob: "lib/shared/**"},
	}
	if got := res.Report.Shared; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("report shared = %+v", got)
	}
	out, err := m.ReportText(it.ID, 0)
	text, rep := out.Text, out.Report
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Shared) != 2 {
		t.Errorf("stored report shared = %+v", rep.Shared)
	}
	for _, s := range []string{"**Shared files:** 2 changed", "app/layout.css, lib/shared/button/button.go"} {
		if !strings.Contains(text, s) {
			t.Errorf("report text lacks %q:\n%s", s, text)
		}
	}
}

func TestPlanSharedUnflagged(t *testing.T) {
	m, dir := setup(t)
	must[model.Config](t)(m.ConfigChecks([]string{"true"}, true))
	// With no shared globs, or none matching, the report flags nothing.
	it := startWork(t, m, dir, "Webhook")
	gittest.Commit(t, dir, "lib/hook.go", "package lib\n", "Hook", "Mull: "+it.ID)
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Rules: reviewRules, Summary: "The hook.", Design: "One handler."}))
	if res.Report.Shared != nil {
		t.Fatalf("no globs: shared = %+v", res.Report.Shared)
	}
	must[*model.Plan](t)(m.PlanShared("general", []string{"app/**"}))
	it2 := startWork(t, m, dir, "Retry")
	gittest.Commit(t, dir, "lib/retry.go", "package lib\n", "Retry", "Mull: "+it2.ID)
	res = must[DoneResult](t)(m.Done(it2.ID, DoneInput{Rules: reviewRules, Summary: "Retries.", Design: "A loop."}))
	if res.Report.Shared != nil {
		t.Fatalf("no match: shared = %+v", res.Report.Shared)
	}
	if out, _ := m.ReportText(it2.ID, 0); strings.Contains(out.Text, "Shared files") {
		t.Errorf("report text mentions shared files:\n%s", out.Text)
	}
}

func TestPlanSharedSetAndClear(t *testing.T) {
	m, dir := setup(t)
	_, err := m.PlanShared("general", []string{"/app/layout.css"})
	wantCode(t, err, CodeInvalid)
	_, err = m.PlanShared("general", []string{"./app"})
	wantCode(t, err, CodeInvalid)
	_, err = m.PlanShared("nope", []string{"app/**"})
	wantCode(t, err, CodeNotFound)
	seen := head(t, dir)
	results := must[[]BatchResult](t)(m.Batch(strings.NewReader(`{"op": "plan shared", "slug": "general", "globs": ["app/**", "lib/ui/*.go"]}`+"\n"), false))
	if p := results[0].Result.(*model.Plan); len(p.Shared) != 2 || p.Shared[1] != "lib/ui/*.go" || results[0].ID != "general" {
		t.Fatalf("batch plan shared = %+v", results[0])
	}
	// The change feed gives the lists, as short fields.
	c, ok := byRecord(must[Revision](t)(m.Revision(seen)).Changes)["plan:general"]
	if f, has := fieldOf(c, "shared"); !ok || !has || f.From != "" || f.To != "app/** lib/ui/*.go" {
		t.Fatalf("change = %+v", c)
	}
	tip := head(t, dir)
	must[*model.Plan](t)(m.PlanShared("general", []string{"app/**", "lib/ui/*.go"}))
	if head(t, dir) != tip {
		t.Errorf("an unchanged list wrote a commit")
	}
	p := must[*model.Plan](t)(m.PlanShared("general", nil))
	if p.Shared != nil {
		t.Fatalf("cleared = %q", p.Shared)
	}
	if got := must[*model.Plan](t)(m.PlanShared("general", nil)); got.Shared != nil {
		t.Fatalf("cleared again = %q", got.Shared)
	}
}

// context --item hands the worker its plan's shared globs.
func TestContextHandsOverShared(t *testing.T) {
	m, _ := setup(t)
	must[*model.Plan](t)(m.PlanShared("general", []string{"app/**", "lib/ui/*.go"}))
	i := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A page"}))
	loose := must[ItemView](t)(m.ItemAdd(NewItem{Plan: model.NoPlan, Title: "Loose"}))
	if c := must[Context](t)(m.Context(i.ID)); strings.Join(c.Item.Shared, " ") != "app/** lib/ui/*.go" {
		t.Errorf("shared = %q", c.Item.Shared)
	}
	if c := must[Context](t)(m.Context(loose.ID)); c.Item.Shared != nil {
		t.Errorf("unplanned shared = %q, want none", c.Item.Shared)
	}
}
