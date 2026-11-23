package core

import (
	"strings"
	"testing"
	"time"

	"mull/internal/gittest"
)

func TestSurface(t *testing.T) {
	m, dir := riskSetup(t)
	gittest.Commit(t, dir, "RULES.md", rulebook, "Rules")
	gittest.Commit(t, dir, ".mull/ratchets/R1.txt", "web/old.html\n", "Baseline")
	gittest.Commit(t, dir, "web/old.html", `<div class="page-local">`, "Old page")
	hi := startWork(t, m, "Touch the rules")
	gittest.Commit(t, dir, "RULES.md", rulebook+"\n## R9: New\n", "R9", "Mull: "+hi.ID)
	must[DoneOut](t)(m.Done(hi.ID, DoneInput{Summary: "Adds R9.", Rules: "R9: new rule", Slowed: "the checks ran twice"}))
	q := must[*Entry](t)(m.Ask("Centre prose?", "", "general", "Centred", "1h", ""))

	v := must[Surface](t)(m.Surface(m.Now().UTC().Add(-time.Hour).Format(time.RFC3339)))
	if len(v.Calls) != 2 || v.Calls[0].Kind != "review" || v.Calls[0].Item != hi.ID || v.Calls[1].Entry != q.ID {
		t.Fatalf("calls = %+v", v.Calls)
	}
	if len(v.Rules) != 2 || len(v.Advisory) != 2 || len(v.Flow.Landed) != 1 || len(v.Flow.Slowed) != 1 {
		t.Fatalf("rules %+v advisory %v flow %+v", v.Rules, v.Advisory, v.Flow)
	}
	if len(v.Plans) != 1 || v.Plans[0].Slug != "general" {
		t.Fatalf("plans = %+v", v.Plans)
	}
	text := v.Text()
	for _, s := range []string{"NEEDS YOU (2)", "default: Centred", "slowed " + hi.ID} {
		if !strings.Contains(text, s) {
			t.Errorf("text lacks %q:\n%s", s, text)
		}
	}
	sum := must[Summary](t)(m.Summarize())
	if sum.Calls != 2 || len(sum.Plans) != 1 {
		t.Fatalf("summary = %+v", sum)
	}
}
