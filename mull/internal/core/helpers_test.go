package core

import (
	"errors"
	"strings"
	"testing"

	"mull/internal/gittest"
)

// setup is a repository on mull whose work lands on main (the release
// branch is another, so the runner may land there), with an agreed plan,
// general, and room for many active items.
func setup(t *testing.T) (*Mull, string) {
	t.Helper()
	dir := gittest.NewRepo(t)
	m, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	must[Config](t)(m.Init("main", "release"))
	must[Config](t)(m.ConfigSet("parallel", "50", true))
	must[*Plan](t)(m.PlanNew("general", "General", "", "none", true))
	must[*Plan](t)(m.PlanState("general", PlanActive, true))
	return m, dir
}

// riskSetup is setup with a passing check and no sampling.
func riskSetup(t *testing.T) (*Mull, string) {
	t.Helper()
	m, dir := setup(t)
	must[Config](t)(m.ConfigChecks([]string{"true"}, true))
	must[Config](t)(m.ConfigRisk(RiskInput{Sample: -1, OnRequest: true}))
	return m, dir
}

// startWork adds an item to general and claims it here.
func startWork(t *testing.T, m *Mull, title string) *Item {
	t.Helper()
	i := must[*Item](t)(m.Add(NewItem{Plan: "general", Title: title}))
	must[StartResult](t)(m.Start(i.ID))
	return i
}

func item(t *testing.T, m *Mull, id string) *Item {
	t.Helper()
	return must[Shown](t)(m.Show(id)).Item
}

func entry(t *testing.T, m *Mull, id string) *Entry {
	t.Helper()
	return must[Shown](t)(m.Show(id)).Entry
}

func must[T any](t *testing.T) func(T, error) T {
	return func(v T, err error) T {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("got %v, want code %s", err, code)
	}
}

func wantErr(t *testing.T, err error, contains string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), contains) {
		t.Fatalf("got error %v, want one containing %q", err, contains)
	}
}

// reviewRules makes done score work high risk, so it waits for review.
const reviewRules = "R0 (test: wait for review)"

func commitFile(t *testing.T, dir, path, item string) {
	t.Helper()
	gittest.Commit(t, dir, path, "x "+path+"\n", "Change "+path, "Mull: "+item)
}
