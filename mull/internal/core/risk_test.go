package core

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"mull/internal/gittest"
)

func TestLowRiskLandsWithoutReview(t *testing.T) {
	m, dir := riskSetup(t)
	it := startWork(t, m, "Footer wording")
	gittest.Commit(t, dir, "web/footer.html", "<p>Oak Compendium</p>\n", "Footer", "Mull: "+it.ID)
	must[*Entry](t)(m.Decided("Kept the old class name", it.ID, "", "", false, false))

	_, err := m.Done(it.ID, DoneInput{Summary: "x"})
	wantCode(t, err, CodeRulesRequired)
	res := must[DoneOut](t)(m.Done(it.ID, DoneInput{Summary: "Footer says Oak Compendium.", Rules: "none", Slowed: "nothing"}))
	if !res.Landed || !res.Merged || res.Item.State != Done || res.Review != "" {
		t.Fatalf("done = %+v", res)
	}
	if r := res.Report; r.Risk != RiskLow || r.Sampled || len(r.Decisions) != 1 || !strings.Contains(r.Text, "low risk") {
		t.Fatalf("report = %+v", r)
	}
	if n := len(must[[]*Entry](t)(m.Inbox())); n != 0 {
		t.Fatalf("inbox has %d entries: low-risk work and noted decisions need no one", n)
	}
	if d := must[[]Decision](t)(m.Decisions()); len(d) != 1 || d[0].Human {
		t.Fatalf("log = %+v", d)
	}
}

func TestHighRiskWaitsAndLandsOnAccept(t *testing.T) {
	m, dir := riskSetup(t)
	for _, path := range []string{"RULES.md", "go.mod", "priv/repo/migrations/1.exs", ".github/workflows/ci.yml"} {
		it := startWork(t, m, "Touch "+path)
		gittest.Commit(t, dir, path, "x "+path+"\n", "Touch", "Mull: "+it.ID)
		res := must[DoneOut](t)(m.Done(it.ID, DoneInput{Summary: "Touched.", Rules: "none"}))
		if res.Landed || res.Item.State != InReview || res.Review == "" || !strings.Contains(res.Report.RiskWhy[0], path) {
			t.Fatalf("%s: done = %+v", path, res)
		}
		_, err := m.Accept([]string{it.ID}, false)
		wantCode(t, err, CodeNotAllowed)
		acc := must[AcceptResult](t)(m.Accept([]string{it.ID}, true))
		if len(acc.Warnings) != 0 || item(t, m, it.ID).State != Done {
			t.Fatalf("accept = %+v", acc)
		}
	}
	// The list is the human's to grow and shrink.
	_, err := m.ConfigRisk(RiskInput{Remove: []string{"go.mod"}})
	wantCode(t, err, CodeNotAllowed)
	cfg := must[Config](t)(m.ConfigRisk(RiskInput{Remove: []string{"go.mod", "**/go.mod"}, Add: []string{"lib/accounts/**"}, OnRequest: true}))
	if slices.Contains(cfg.Risk.High, "go.mod") || !slices.Contains(cfg.Risk.High, "lib/accounts/**") {
		t.Fatalf("high = %v", cfg.Risk.High)
	}
}

func TestTouchingARuleIsHighRisk(t *testing.T) {
	m, dir := riskSetup(t)
	it := startWork(t, m, "Widths")
	gittest.Commit(t, dir, "web/page.css", "main{}\n", "Widths", "Mull: "+it.ID)
	res := must[DoneOut](t)(m.Done(it.ID, DoneInput{Summary: "Widths.", Rules: "R7: a page sets its own width"}))
	if res.Landed || !strings.Contains(res.Report.RiskWhy[0], "R7") {
		t.Fatalf("done = %+v", res)
	}
}

func TestMediumLandsWithALook(t *testing.T) {
	m, dir := riskSetup(t)
	must[Config](t)(m.ConfigRisk(RiskInput{MaxFiles: 2, OnRequest: true}))
	it := startWork(t, m, "Three pages")
	for k := 1; k <= 3; k++ {
		gittest.Commit(t, dir, fmt.Sprintf("web/p%d.html", k), "x\n", "Page", "Mull: "+it.ID)
	}
	res := must[DoneOut](t)(m.Done(it.ID, DoneInput{Summary: "Three pages.", Rules: "none"}))
	if !res.Landed || res.Report.Risk != RiskMedium || res.Review == "" || entry(t, m, res.Review).Landed != RiskMedium {
		t.Fatalf("done = %+v", res)
	}
	// Sending landed work back files a follow-up; the item stays done.
	_, err := m.SendBack(res.Review, "", true)
	wantErr(t, err, "--reason")
	f := must[*Item](t)(m.SendBack(res.Review, "p3 has no title", true))
	if f.ID == it.ID || f.From != it.ID || f.State != Committed || item(t, m, it.ID).State != Done {
		t.Fatalf("follow-up = %+v", f)
	}
}

func TestSampledLookAndDraw(t *testing.T) {
	m, dir := riskSetup(t)
	must[Config](t)(m.ConfigRisk(RiskInput{Sample: 100, OnRequest: true}))
	it := startWork(t, m, "Sampled")
	gittest.Commit(t, dir, "web/a.html", "x\n", "A", "Mull: "+it.ID)
	res := must[DoneOut](t)(m.Done(it.ID, DoneInput{Summary: "A.", Rules: "none"}))
	if !res.Report.Sampled || entry(t, m, res.Review).Landed != "sampled" {
		t.Fatalf("done = %+v", res)
	}
	must[AcceptResult](t)(m.Accept([]string{res.Review}, true))
	if item(t, m, it.ID).State != Done {
		t.Fatal("a look changed the item")
	}
	hits := 0
	for k := range 1000 {
		key := fmt.Sprintf("%04x", k)
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
}

func TestFailedChecksKeepItActive(t *testing.T) {
	m, dir := setup(t)
	must[Config](t)(m.ConfigChecks([]string{"echo broken; exit 3"}, true))
	it := startWork(t, m, "Broken")
	gittest.Commit(t, dir, "a.txt", "a\n", "A", "Mull: "+it.ID)
	_, err := m.Done(it.ID, DoneInput{Summary: "A.", Rules: "none"})
	wantCode(t, err, CodeChecksFailed)
	r := must[Report](t)(m.ReportOf(it.ID, 0))
	if r.Passed || !strings.Contains(r.Text, "broken") || item(t, m, it.ID).State != Active {
		t.Fatalf("report = %+v", r)
	}
}
