package mull

import (
	"testing"
	"time"

	"mull/internal/gittest"
	"mull/internal/model"
)

func TestSummary(t *testing.T) {
	m, dir := setup(t)
	must[model.Config](t)(m.ConfigSet("release", "release"))
	gittest.Git(t, dir, "branch", "release")
	gittest.Commit(t, dir, "a", "a\n", "one")
	gittest.Commit(t, dir, "b", "b\n", "two")

	must[*model.Plan](t)(m.PlanNew("launch", "Public launch", "", true))
	must[*model.Plan](t)(m.PlanNew("later", "Later", "", true)) // shaping
	agreedPlan(t, m, "old", "Old", "")
	must[*model.Plan](t)(m.PlanClose("old"))

	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "Working"}))
	must[StartResult](t)(m.Start(a.ID))
	must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Footer"}))
	must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "Which?", Item: a.ID}))
	must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindDecision, Text: "Chose X", Item: a.ID}))
	must[*model.Entry](t)(m.captureLegacy(CaptureInput{Kind: model.KindLoose, Text: "Old loose end"}))

	m.Now = func() time.Time { return time.Now().Add(8 * 24 * time.Hour) }
	sum := must[Summary](t)(m.Summary())

	if sum.Integration != "main" || !sum.IntegrationFound || sum.Release != "release" || sum.AheadOfRelease != 2 {
		t.Errorf("refs: %+v", sum)
	}
	want := []SummaryPlan{{"general", "General", "active"}, {"later", "Later", "shaping"}, {"launch", "Public launch", "shaping"}}
	if len(sum.Plans) != len(want) {
		t.Fatalf("plans = %+v, want %+v (active first, done ones left out)", sum.Plans, want)
	}
	for i := range want {
		if sum.Plans[i] != want[i] {
			t.Errorf("plans[%d] = %+v, want %+v", i, sum.Plans[i], want[i])
		}
	}
	if n := sum.NeedsYou; n.Questions != 1 || n.Decisions != 1 || n.Reviews != 0 || n.Older != 2 {
		t.Errorf("needs you = %+v (the legacy loose end is not counted)", n)
	}
	if sum.Agents != 1 || sum.Untriaged != 1 {
		t.Errorf("agents = %d, untriaged = %d", sum.Agents, sum.Untriaged)
	}
	tip := gittest.Git(t, dir, "rev-parse", "refs/heads/mull")
	if sum.Commit != tip || sum.LastChange != gittest.Git(t, dir, "log", "-1", "--format=%cI", tip) {
		t.Errorf("commit = %q at %q, want the mull branch tip %q", sum.Commit, sum.LastChange, tip)
	}
	if sum.Format != model.Format || sum.MigrationRequired {
		t.Errorf("format = %d, migration required = %v", sum.Format, sum.MigrationRequired)
	}
}
