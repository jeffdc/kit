package mull

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"mull/internal/model"
)

// captureSimilar captures a bug through the bug op, as mull bug does, and
// returns the IDs of the items it lists as similar.
func captureSimilar(t *testing.T, m *Mull, title string) (string, []string) {
	t.Helper()
	results := must[[]BatchResult](t)(m.Batch(strings.NewReader(jsonl(t, map[string]any{"op": "bug", "text": title})), false))
	v := results[0].Result.(CaptureView)
	ids := []string{}
	for _, s := range v.Similar {
		ids = append(ids, s.ID)
	}
	return v.ID, ids
}

func TestTitleWords(t *testing.T) {
	for title, want := range map[string]string{
		"Flaky login test":                 "flaky login test",
		"The login tests are FLAKY!":       "flaky login test",
		"mull serve: Jeff's tree scrolls":  "jeff mull scroll serve tree",
		"Two h1s on the home page":         "h1s home page two",
		"Crashes when it fixes boxes":      "box crash fix",
		"Class names in the 2 CSS classes": "2 class css name",
	} {
		got := []string{}
		for w := range titleWords(title) {
			got = append(got, w)
		}
		slices.Sort(got)
		if s := strings.Join(got, " "); s != want {
			t.Errorf("titleWords(%q) = %q, want %q", title, s, want)
		}
	}
}

func TestCaptureListsSimilarOpenItems(t *testing.T) {
	m, _ := setup(t)
	flaky, sim := captureSimilar(t, m, "Flaky login test")
	if len(sim) != 0 {
		t.Fatalf("first capture: similar %v", sim)
	}
	dark, _ := captureSimilar(t, m, "Dark mode")
	work := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "Webhook handler retries"}))

	// Same words in another order, a plural, filler words: similar.
	if _, sim := captureSimilar(t, m, "The login tests are flaky"); !slices.Equal(sim, []string{flaky}) {
		t.Errorf("login: similar %v, want [%s]", sim, flaky)
	}
	// Committed work counts too.
	if _, sim := captureSimilar(t, m, "Webhook handler retries twice"); !slices.Equal(sim, []string{work.ID}) {
		t.Errorf("webhook: similar %v, want [%s]", sim, work.ID)
	}
	// One shared word isn't enough between titles of two words or more.
	if _, sim := captureSimilar(t, m, "Dark theme"); len(sim) != 0 {
		t.Errorf("dark theme: similar %v", sim)
	}
	if _, sim := captureSimilar(t, m, "Dark mode toggle"); !slices.Contains(sim, dark) {
		t.Errorf("dark mode toggle: similar %v, want %s", sim, dark)
	}
	// A one-word title can match on its one word.
	one, _ := captureSimilar(t, m, "Crash")
	if _, sim := captureSimilar(t, m, "crashes"); !slices.Equal(sim, []string{one}) {
		t.Errorf("crashes: similar %v, want [%s]", sim, one)
	}

	// Active, in-review, done and dropped items aren't listed.
	must[StartResult](t)(m.Start(work.ID))
	if _, sim := captureSimilar(t, m, "Webhook handler retries forever"); slices.Contains(sim, work.ID) {
		t.Errorf("active item %s listed as similar: %v", work.ID, sim)
	}
	must[DropResult](t)(m.Drop([]string{flaky}, "", SettleOpts{OnRequest: true}))
	if _, sim := captureSimilar(t, m, "Login test flaky again"); slices.Contains(sim, flaky) {
		t.Errorf("dropped item %s listed as similar: %v", flaky, sim)
	}
}

func TestSimilarDiscountsCommonWords(t *testing.T) {
	m, _ := setup(t)
	scroll, _ := captureSimilar(t, m, "mull serve: tree scroll jumps")
	for _, title := range []string{"mull serve: inbox badge", "mull serve: backlog filter", "mull serve: plan colours",
		"mull serve: report fonts", "mull serve: phase arrows"} {
		captureSimilar(t, m, title)
	}
	// Sharing the project's common prefix and one word isn't similar ...
	if _, sim := captureSimilar(t, m, "mull serve: tree collapses on reload"); len(sim) != 0 {
		t.Errorf("collapses: similar %v", sim)
	}
	// ... but sharing the rarer words is.
	if _, sim := captureSimilar(t, m, "mull serve: tree scroll jumps on reload"); !slices.Contains(sim, scroll) {
		t.Errorf("scroll: similar %v, want %s", sim, scroll)
	}
}

func TestSimilarLongTitles(t *testing.T) {
	m, _ := setup(t)
	for _, title := range []string{"Backlog view: sort by column", "Backlog view: triage handling",
		"Backlog view: add new items", "Report fonts too small on phones"} {
		captureSimilar(t, m, title)
	}
	docs, _ := captureSimilar(t, m, "Revision changes for docs could name the changed sections, so the doc view can cloud paragraphs as the prototype does")
	// Short titles sharing only two words aren't similar ...
	if _, sim := captureSimilar(t, m, "Backlog view: filtering"); len(sim) != 0 {
		t.Errorf("filtering: similar %v", sim)
	}
	// ... but long ones sharing many are, though each has many of its own.
	if _, sim := captureSimilar(t, m, "Revision schedule: say which sections of a plan or phase doc changed, not just that the doc changed"); !slices.Equal(sim, []string{docs}) {
		t.Errorf("sections: similar %v, want [%s]", sim, docs)
	}
}

func TestSimilarIsCappedAndAlwaysAList(t *testing.T) {
	m, _ := setup(t)
	for k := range 7 {
		captureSimilar(t, m, fmt.Sprintf("Footer links broken %d", k))
	}
	if _, sim := captureSimilar(t, m, "Footer links broken"); len(sim) != maxSimilar {
		t.Errorf("similar %v, want %d", sim, maxSimilar)
	}
	results := must[[]BatchResult](t)(m.Batch(strings.NewReader(jsonl(t, map[string]any{"op": "idea", "text": "Something else entirely"})), false))
	data, err := json.Marshal(results[0].Result)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if s, ok := out["similar"].([]any); !ok || len(s) != 0 {
		t.Errorf("similar = %#v, want []", out["similar"])
	}
	if out["kind"] != model.ItemIdea || out["state"] != model.StateCaptured {
		t.Errorf("idea printed as %v", out)
	}
}
