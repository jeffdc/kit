package mull

import (
	"strings"
	"testing"
	"time"

	"mull/internal/gittest"
	"mull/internal/model"
)

func TestSurface(t *testing.T) {
	m, dir := riskSetup(t)
	gittest.Commit(t, dir, "RULES.md", rulebook, "Rules")
	gittest.Commit(t, dir, ".mull/ratchets/R1.txt", "web/old.html\n", "Baseline")
	gittest.Commit(t, dir, "web/old.html", `<div class="page-local">`, "Old page")

	// A high-risk item waits; a question has a default; a cluster forms.
	hi := startWork(t, m, dir, "Touch the rules")
	gittest.Commit(t, dir, "RULES.md", rulebook+"\n## R9: New\n", "R9", "Mull: "+hi.ID)
	must[DoneResult](t)(m.Done(hi.ID, DoneInput{Summary: "Adds R9.", Rules: "R9: new rule", Slowed: "the checks ran twice"}))
	q := must[any](t)(m.Exec("ask", &CaptureIn{kind: model.KindQuestion, Text: "Centre prose?", Plan: "general", Default: "Centred", By: "1h"})).(*model.Entry)
	for _, title := range []string{"a", "b", "c"} {
		must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: title, Area: "web/footer"}))
	}

	since := m.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	v := must[Surface](t)(m.Surface(since))
	if len(v.Calls) != 2 || v.Calls[0].Kind != "review" || v.Calls[0].Item != hi.ID || v.Calls[1].Entry != q.ID || v.Calls[1].Default != "Centred" {
		t.Fatalf("calls = %+v", v.Calls)
	}
	if len(v.Rules) != 2 || v.Rules[0].ID != "R1" && v.Rules[0].ID != "R2" {
		t.Fatalf("rules = %+v", v.Rules)
	}
	if len(v.Advisory) != 2 || !strings.HasPrefix(v.Advisory[0], "R3") {
		t.Errorf("advisory = %v", v.Advisory)
	}
	if len(v.Clusters) != 1 || len(v.Flow.Landed) != 1 || len(v.Flow.Slowed) != 1 || v.Flow.Slowed[0].Text != "the checks ran twice" {
		t.Errorf("clusters %+v, flow %+v", v.Clusters, v.Flow)
	}
	text := v.Text()
	for _, s := range []string{"NEEDS YOU (2)", "default: Centred", "3 queued in web/footer", "slowed " + hi.ID} {
		if !strings.Contains(text, s) {
			t.Errorf("text lacks %q:\n%s", s, text)
		}
	}

	// Past its due time, the default stands.
	m.Now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if v = must[Surface](t)(m.Surface(since)); !v.Calls[1].Lapsed || !strings.Contains(v.Text(), "default stands: Centred") {
		t.Fatalf("calls = %+v", v.Calls)
	}
}
