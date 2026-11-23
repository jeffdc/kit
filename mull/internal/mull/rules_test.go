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

const rulebook = `# Rules

The rules for this code. Each one has a check unless it says it's advisory.

## R1: Pages use only library components

- scope: web/**
- check: grep -rl 'class="page-local' web || true
- ratchet: yes

Five page agents built five versions of the same parts.

## R2: No SQL by hand

- check: ! grep -rq 'DROP TABLE' web

## R3: Keep the prose Jeff's

Agents never write it.

## Not a rule

Prose under another heading.
`

func TestParseRules(t *testing.T) {
	rules := must[[]Rule](t)(ParseRules(rulebook))
	if len(rules) != 3 {
		t.Fatalf("rules = %+v", rules)
	}
	r1, r2, r3 := rules[0], rules[1], rules[2]
	if r1.ID != "R1" || r1.Title != "Pages use only library components" || len(r1.Scope) != 1 || r1.Scope[0] != "web/**" ||
		r1.Ratchet != ".mull/ratchets/R1.txt" || r1.Advisory || !strings.HasPrefix(r1.Why, "Five page agents") {
		t.Errorf("R1 = %+v", r1)
	}
	if r2.Check != "! grep -rq 'DROP TABLE' web" || r2.Ratchet != "" || r2.Advisory {
		t.Errorf("R2 = %+v", r2)
	}
	if !r3.Advisory || r3.Why != "Agents never write it." {
		t.Errorf("R3 = %+v", r3)
	}
	_, err := ParseRules("## R1: a\n## R1: b\n")
	wantErr(t, err, "twice")
}

func TestRatchetOnlyShrinks(t *testing.T) {
	m, dir := riskSetup(t)
	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("RULES.md", rulebook)
	write("web/a.html", `<div class="page-local">`)
	write("web/b.html", `<div class="page-local">`)

	// No baseline yet: only the human sets one.
	_, err := m.Ratchet("R1", false, false)
	wantCode(t, err, CodeNotAllowed)
	base := must[RatchetResult](t)(m.Ratchet("R1", true, true))
	if base.After != 2 {
		t.Fatalf("baseline = %+v", base)
	}
	out := must[RuleCheckOut](t)(m.CheckRules())
	if !out.Passed || len(out.Results) != 2 || out.Results[0].Count != 2 {
		t.Fatalf("check = %+v", out)
	}

	// A new violation fails, and can't be ratcheted in by an agent.
	write("web/c.html", `<div class="page-local">`)
	out = must[RuleCheckOut](t)(m.CheckRules())
	if out.Passed || len(out.Results[0].New) != 1 || out.Results[0].New[0] != "web/c.html" {
		t.Fatalf("check = %+v", out)
	}
	_, err = m.Ratchet("R1", false, false)
	wantCode(t, err, CodeNotAllowed)
	if err := os.Remove(filepath.Join(dir, "web/c.html")); err != nil {
		t.Fatal(err)
	}

	// A fixed one fails until the ratchet shrinks, which anyone may do.
	if err := os.Remove(filepath.Join(dir, "web/b.html")); err != nil {
		t.Fatal(err)
	}
	out = must[RuleCheckOut](t)(m.CheckRules())
	if out.Passed || out.Results[0].Fixed != 1 || !strings.Contains(out.Results[0].Tail, "mull rules ratchet R1") {
		t.Fatalf("check = %+v", out)
	}
	shrunk := must[RatchetResult](t)(m.Ratchet("R1", false, false))
	if shrunk.Before != 2 || shrunk.After != 1 {
		t.Fatalf("shrink = %+v", shrunk)
	}
	if out = must[RuleCheckOut](t)(m.CheckRules()); !out.Passed {
		t.Fatalf("check = %+v", out)
	}
}

func TestDoneRunsTheRulebook(t *testing.T) {
	m, dir := riskSetup(t)
	it := startWork(t, m, dir, "Pages")
	gittest.Commit(t, dir, "RULES.md", rulebook, "Rules")
	gittest.Commit(t, dir, ".mull/ratchets/R1.txt", "# baseline\n", "Empty baseline")
	gittest.Commit(t, dir, "web/a.html", `<div class="page-local">`, "Page", "Mull: "+it.ID)

	_, err := m.Done(it.ID, DoneInput{Summary: "A page.", Rules: "none"})
	var failed *ErrChecksFailed
	if !asErr(err, &failed) {
		t.Fatalf("done = %v, want the rule check to fail", err)
	}
	rep := must[ReportOut](t)(m.ReportText(it.ID, 0))
	if rep.Report.Passed || len(rep.Report.RuleChecks) != 2 || rep.Report.RuleChecks[0].Passed || !strings.Contains(rep.Text, "Rule R1 failed") {
		t.Fatalf("report = %+v\n%s", rep.Report, rep.Text)
	}
	if item(t, m, it.ID).State != model.StateActive {
		t.Fatal("the item isn't active")
	}

	gittest.Commit(t, dir, "web/a.html", `<div class="card">`, "Use the library", "Mull: "+it.ID)
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "A page.", Rules: "none"}))
	if !res.Report.Passed || len(res.Report.InScope) != 1 || res.Report.InScope[0] != "R1" {
		t.Fatalf("report = %+v", res.Report)
	}
	// RULES.md and the ratchet changed: that's high risk.
	if res.Report.Risk != model.RiskHigh {
		t.Errorf("risk = %s", res.Report.Risk)
	}

	ctx := must[Context](t)(m.Context(it.ID))
	if len(ctx.Rules) != 3 || !strings.Contains(strings.Join(ctx.Protocol, " "), "RULES.md") {
		t.Fatalf("context rules = %+v", ctx.Rules)
	}
}

func asErr[T error](err error, target *T) bool { return errors.As(err, target) }
