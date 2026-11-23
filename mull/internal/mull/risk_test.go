package mull

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
)

// riskSetup is a repository with a passing check and no sampling, so a test
// controls what it sees.
func riskSetup(t *testing.T) (*Mull, string) {
	t.Helper()
	m, dir := setup(t)
	must[model.Config](t)(m.ConfigChecks([]string{"true"}, true))
	must[model.Config](t)(m.ConfigRisk(RiskInput{Sample: -1, OnRequest: true}))
	return m, dir
}

func TestLowRiskLandsWithoutReview(t *testing.T) {
	m, dir := riskSetup(t)
	it := startWork(t, m, dir, "Footer wording")
	gittest.Commit(t, dir, "web/footer.html", "<p>Oak Compendium</p>\n", "Footer", "Mull: "+it.ID)
	noteDecision(t, m, it.ID, "Kept the old class name")

	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Footer says Oak Compendium.", Rules: "none", Slowed: "nothing"}))
	if !res.Landed || res.Item.State != model.StateDone || res.Review != "" {
		t.Fatalf("done = %+v", res)
	}
	if r := res.Report; r.Risk != model.RiskLow || r.Sampled || r.Rules != "none" || r.Slowed != "nothing" || len(r.Decisions) != 1 {
		t.Fatalf("report = %+v", r)
	}
	if n := len(must[[]*model.Entry](t)(m.Inbox(InboxFilter{}))); n != 0 {
		t.Fatalf("inbox has %d entries, want none: low-risk work and noted decisions need no one", n)
	}
	text := must[ReportOut](t)(m.ReportText(it.ID, 0)).Text
	for _, s := range []string{"Footer says Oak Compendium.", "low risk", "**Rules:** none", "**Slowed by:** nothing", "- Kept the old class name"} {
		if !strings.Contains(text, s) {
			t.Errorf("report lacks %q:\n%s", s, text)
		}
	}
}

func TestHighRiskPathsWaitForReview(t *testing.T) {
	m, dir := riskSetup(t)
	for _, path := range []string{"RULES.md", "go.mod", "priv/repo/migrations/1_add.exs", ".github/workflows/ci.yml"} {
		it := startWork(t, m, dir, "Touch "+path)
		gittest.Commit(t, dir, path, "x "+path+"\n", "Touch", "Mull: "+it.ID)
		res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Touched.", Rules: "none"}))
		if res.Landed || res.Item.State != model.StateInReview || res.Review == "" || res.Report.Risk != model.RiskHigh {
			t.Fatalf("%s: done = %+v", path, res)
		}
		if len(res.Report.RiskWhy) != 1 || !strings.Contains(res.Report.RiskWhy[0], path) {
			t.Errorf("%s: why = %v", path, res.Report.RiskWhy)
		}
	}

	// The list is the human's to grow and shrink.
	_, err := m.ConfigRisk(RiskInput{Remove: []string{"go.mod"}})
	wantCode(t, err, CodeNotAllowed)
	cfg := must[model.Config](t)(m.ConfigRisk(RiskInput{Remove: []string{"go.mod", "**/go.mod"}, Add: []string{"lib/oaks/accounts/**"}, OnRequest: true}))
	if slices.Contains(cfg.Risk.High, "go.mod") || !slices.Contains(cfg.Risk.High, "lib/oaks/accounts/**") {
		t.Fatalf("high = %v", cfg.Risk.High)
	}
	it := startWork(t, m, dir, "Go module again")
	gittest.Commit(t, dir, "go.mod", "module y\n", "Module", "Mull: "+it.ID)
	if res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Module.", Rules: "none"})); !res.Landed {
		t.Fatalf("go.mod is off the list, but done = %+v", res)
	}
}

func TestTouchingARuleIsHighRisk(t *testing.T) {
	m, dir := riskSetup(t)
	it := startWork(t, m, dir, "Page widths")
	gittest.Commit(t, dir, "web/page.css", "main{}\n", "Widths", "Mull: "+it.ID)
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Widths.", Rules: "R7: a page sets its own width, as an exception"}))
	if res.Landed || res.Report.Risk != model.RiskHigh || !strings.Contains(res.Report.RiskWhy[0], "R7") {
		t.Fatalf("done = %+v", res)
	}
}

func TestMediumRiskLandsAndIsShown(t *testing.T) {
	m, dir := riskSetup(t)
	must[model.Config](t)(m.ConfigRisk(RiskInput{MaxFiles: 2, OnRequest: true}))
	it := startWork(t, m, dir, "Three pages")
	for k := 1; k <= 3; k++ {
		gittest.Commit(t, dir, fmt.Sprintf("web/p%d.html", k), "x\n", "Page", "Mull: "+it.ID)
	}
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Three pages.", Rules: "none"}))
	if !res.Landed || res.Item.State != model.StateDone || res.Report.Risk != model.RiskMedium || res.Review == "" {
		t.Fatalf("done = %+v", res)
	}
	look := entry(t, m, res.Review)
	if look.Landed != model.RiskMedium || !strings.HasPrefix(look.Text, "Look: ") {
		t.Fatalf("look entry = %+v", look)
	}

	// Sending landed work back files a follow-up; the item stays done.
	_, err := m.Do(DoInput{ID: look.ID})
	wantErr(t, err, "--reason")
	do := must[DoResult](t)(m.Do(DoInput{ID: look.ID, Reason: "p3 has no title", SettleOpts: SettleOpts{OnRequest: true}}))
	if do.Item == nil || do.Item.ID == it.ID || do.Item.From != it.ID || do.Item.State != model.StateCommitted {
		t.Fatalf("send back = %+v", do.Item)
	}
	if item(t, m, it.ID).State != model.StateDone {
		t.Fatal("the landed item reopened")
	}
}

func TestLandedLookAcceptedLeavesItemDone(t *testing.T) {
	m, dir := riskSetup(t)
	must[model.Config](t)(m.ConfigRisk(RiskInput{Sample: 100, OnRequest: true}))
	it := startWork(t, m, dir, "Sampled")
	gittest.Commit(t, dir, "web/a.html", "x\n", "A", "Mull: "+it.ID)
	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "A.", Rules: "none"}))
	if !res.Landed || !res.Report.Sampled || res.Review == "" || entry(t, m, res.Review).Landed != "sampled" {
		t.Fatalf("done = %+v", res)
	}
	acc := must[AcceptResult](t)(m.Accept([]string{res.Review}, "", SettleOpts{OnRequest: true}))
	if len(acc.Items) != 0 || len(acc.Entries) != 1 || item(t, m, it.ID).State != model.StateDone {
		t.Fatalf("accept = %+v", acc)
	}
}

func TestSampleDrawIsFixed(t *testing.T) {
	hits := 0
	for k := 0; k < 1000; k++ {
		key := fmt.Sprintf("%04x-commit", k)
		if drawn(key, 20) != drawn(key, 20) {
			t.Fatal("the same work drew differently")
		}
		if drawn(key, 20) {
			hits++
		}
	}
	if hits < 150 || hits > 250 {
		t.Fatalf("20%% sampled %d of 1000", hits)
	}
	if drawn("x", 0) {
		t.Fatal("0% sampled something")
	}
}

func TestAskNeedsADefault(t *testing.T) {
	m, _ := setup(t)
	_, err := m.Exec("ask", &CaptureIn{kind: model.KindQuestion, Text: "Center prose?", Plan: "general", By: "soon"})
	wantErr(t, err, "--by")
	e := must[any](t)(m.Exec("ask", &CaptureIn{kind: model.KindQuestion, Text: "Center prose?", Plan: "general", Default: "Centred", By: "2d"})).(*model.Entry)
	if e.Default != "Centred" || e.Due == "" {
		t.Fatalf("question = %+v", e)
	}
}

// noteDecision notes an agent decision on an item.
func noteDecision(t *testing.T, m *Mull, item, text string) *model.Entry {
	t.Helper()
	out, err := m.Exec("decided", &DecidedIn{Text: text, Item: item})
	if err != nil {
		t.Fatal(err)
	}
	return out.(*model.Entry)
}
