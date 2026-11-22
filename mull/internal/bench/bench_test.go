package bench

import (
	"path/filepath"
	"testing"
	"time"

	"mull/internal/gittest"
	"mull/internal/model"
	"mull/internal/mull"
)

var end = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func seedSmall(t *testing.T, seed uint64) string {
	t.Helper()
	gittest.Isolate(t)
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	dir = filepath.Join(dir, "repo")
	if err := Seed(dir, Sizes["small"], seed, end); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A seeded repository reads as mull's own writes would have left it: every
// stage and plan state present, done work found on main, and writable.
func TestSeedReadsBack(t *testing.T) {
	dir := seedSmall(t, 1)
	m, err := mull.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.Now = func() time.Time { return end }
	items, err := m.ItemList(mull.ItemFilter{})
	if err != nil {
		t.Fatal(err)
	}
	stages := map[string]int{}
	for _, i := range items {
		stages[i.State]++
	}
	for _, s := range model.Stages {
		if stages[s] == 0 {
			t.Errorf("no %s items (have %v)", s, stages)
		}
	}
	plans, err := m.PlanList()
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]bool{}
	for _, p := range plans {
		states[p.State] = true
	}
	for _, s := range []string{model.PlanShaping, model.PlanActive, model.PlanDone} {
		if !states[s] {
			t.Errorf("no %s plan", s)
		}
	}
	view, err := m.Board("")
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range view.Plans {
		for _, p := range b.Problems {
			if p.Kind == "done-not-in-git" {
				t.Errorf("%s: %s %s", b.Slug, p.Item, p.Text)
			}
		}
	}
	if _, err := m.PlanNew("extra", "Extra", "", true); err != nil {
		t.Fatalf("seeded repository isn't writable: %v", err)
	}
}

// The same seed and end give the same records.
func TestSeedIsDeterministic(t *testing.T) {
	a, b := seedSmall(t, 7), seedSmall(t, 7)
	tree := func(dir string) string {
		return gittest.Git(t, dir, "rev-parse", "refs/heads/mull")
	}
	if tree(a) != tree(b) {
		t.Fatalf("two seeds with the same inputs differ: %s, %s", tree(a), tree(b))
	}
}

// Targets gives active plans a target branch of their own, cut from main.
func TestSeedTargets(t *testing.T) {
	gittest.Isolate(t)
	dir := filepath.Join(t.TempDir(), "repo")
	size := Sizes["small"]
	size.Targets = 1
	if err := Seed(dir, size, 1, end); err != nil {
		t.Fatal(err)
	}
	m, err := mull.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	view, err := m.Board("")
	if err != nil {
		t.Fatal(err)
	}
	own := 0
	for _, b := range view.Plans {
		if b.Target.From == mull.TargetFromPlan {
			own++
			if !b.Target.Found || b.Target.Branch != b.Slug+"-next" {
				t.Errorf("%s: target %+v", b.Slug, b.Target)
			}
		}
	}
	if own != 1 {
		t.Errorf("%d plans with their own target, want 1", own)
	}
}

// Run times every target in-process and over HTTP without errors.
func TestRun(t *testing.T) {
	dir := seedSmall(t, 1)
	rep, err := Run([]string{dir}, Options{Runs: 1, Back: []int{5, 100000}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Shapes) != 1 || rep.Shapes[0].Items != Sizes["small"].Items {
		t.Fatalf("shapes = %+v", rep.Shapes)
	}
	seen := map[string]bool{}
	for _, r := range rep.Results {
		seen[r.Over+" "+r.Target] = true
		if r.Over == "in-process" && r.Git <= 0 {
			t.Errorf("%s: counted %d git calls", r.Target, r.Git)
		}
	}
	for _, want := range []string{"in-process overview", "http overview", "in-process tree", "http backlog",
		"http projects (summary)", "in-process revision, 5 back"} {
		if !seen[want] {
			t.Errorf("no result for %q", want)
		}
	}
	var plans, phases, items int
	for k := range seen {
		switch {
		case len(k) > 10 && k[:10] == "http plan ":
			plans++
		case len(k) > 11 && k[:11] == "http phase ":
			phases++
		case len(k) > 10 && k[:10] == "http item ":
			items++
		}
	}
	if plans != 2 || phases != 1 || items != 3 {
		t.Errorf("timed %d plans, %d phases, %d items; want 2, 1, 3", plans, phases, items)
	}
}
