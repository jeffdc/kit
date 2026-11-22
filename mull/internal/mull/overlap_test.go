package mull

import (
	"reflect"
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
)

// startIn adds a worktree of dir on branch, opens it and starts id there,
// so the claim records the branch and its base.
func startIn(t *testing.T, dir, branch, id string) (*Mull, string) {
	t.Helper()
	wt := gittest.AddWorktree(t, dir, branch)
	w, err := Open(wt)
	if err != nil {
		t.Fatal(err)
	}
	must[StartResult](t)(w.Start(id))
	return w, wt
}

func overlapPairs(os []Overlap) map[string][]string {
	out := map[string][]string{}
	for _, o := range os {
		out[o.Items[0].ID+"+"+o.Items[1].ID] = o.Files
	}
	return out
}

func TestOverlap(t *testing.T) {
	m, dir := setup(t)
	agreedPlan(t, m, "launch", "Launch", "")
	agreedPlan(t, m, "other", "Other", "")
	add := func(plan, title string) string {
		return must[ItemView](t)(m.ItemAdd(NewItem{Plan: plan, Title: title})).ID
	}
	a, b, c, d, e := add("launch", "A"), add("launch", "B"), add("other", "C"), add("launch", "D"), add("launch", "E")
	f, g := add("launch", "F stacked on A"), add("launch", "G nothing shared")

	// A, active: shared.txt and a.txt.
	_, wa := startIn(t, dir, a+"-a", a)
	gittest.Commit(t, wa, "shared.txt", "from A\n", "A")
	gittest.Commit(t, wa, "a.txt", "a\n", "A more")

	// B, in review and unmerged: shared.txt and b.txt.
	_, wb := startIn(t, dir, b+"-b", b)
	gittest.Commit(t, wb, "shared.txt", "from B\n", "B")
	gittest.Commit(t, wb, "b.txt", "b\n", "B more")
	markInReview(t, m, b)

	// C, another plan's, active: a.txt.
	_, wc := startIn(t, dir, c+"-c", c)
	gittest.Commit(t, wc, "a.txt", "from C\n", "C")

	// E, active, started before D lands and then takes main in.
	_, we := startIn(t, dir, e+"-e", e)
	gittest.Commit(t, we, "e.txt", "e\n", "E")

	// D, in review and merged into main: shared.txt. Not in flight.
	_, wd := startIn(t, dir, d+"-d", d)
	gittest.Commit(t, wd, "shared.txt", "from D\n", "D")
	gittest.Git(t, dir, "merge", "-q", "--no-ff", "--no-edit", d+"-d")
	markInReview(t, m, d)
	gittest.Git(t, we, "merge", "-q", "--no-edit", "main") // E now carries D's shared.txt, from main

	// F, active, started on A's branch: only f.txt is its own.
	wf := gittest.AddWorktree(t, wa, f+"-f")
	mf, err := Open(wf)
	if err != nil {
		t.Fatal(err)
	}
	must[StartResult](t)(mf.Start(f))
	gittest.Commit(t, wf, "f.txt", "f\n", "F")

	// G, active, changes nothing anyone else does.
	_, wg := startIn(t, dir, g+"-g", g)
	gittest.Commit(t, wg, "g.txt", "g\n", "G")

	ov := must[Overview](t)(m.Overview())
	got := overlapPairs(ov.Overlaps)
	want := map[string][]string{pair(a, b): {"shared.txt"}, pair(a, c): {"a.txt"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("overlaps = %v, want %v", got, want)
	}
	for _, o := range ov.Overlaps {
		if o.Items[0].ID == a && !reflect.DeepEqual(o.Items[0].Branches, []string{a + "-a"}) {
			t.Errorf("A's branches = %v", o.Items[0].Branches)
		}
	}

	// Each plan shows the pairs it has a side of.
	for _, ps := range ov.Plans {
		var n int
		switch ps.Slug {
		case "launch":
			n = 2
		case "other":
			n = 1
		}
		if len(ps.Overlaps) != n {
			t.Errorf("plan %s overlaps = %+v, want %d", ps.Slug, ps.Overlaps, n)
		}
	}
	pp := must[PlanPage](t)(m.PlanPage("other"))
	if len(pp.Status.Overlaps) != 1 {
		t.Errorf("plan page overlaps = %+v", pp.Status.Overlaps)
	}

	// The view: a problem on the plan's side of each pair.
	v := must[View](t)(m.Board(""))
	var lines []string
	for _, p := range v.Plans {
		for _, pr := range p.Problems {
			if pr.Kind == "overlap" {
				lines = append(lines, p.Slug+" "+pr.Item+" "+pr.With+" "+strings.Join(pr.Files, ","))
			}
		}
	}
	lo, hi := a, b // both in launch: the problem is on the first in ID order
	if hi < lo {
		lo, hi = hi, lo
	}
	wantLines := []string{
		"launch " + lo + " " + hi + " shared.txt",
		"launch " + a + " " + c + " a.txt",
		"other " + c + " " + a + " a.txt",
	}
	if !sameSet(lines, wantLines) {
		t.Errorf("overlap problems = %q, want %q", lines, wantLines)
	}
	text := v.Text(m.Now())
	other := map[string]string{a: b + " (in-review)", b: a + " (active)"}[lo]
	if !strings.Contains(text, lo+"  shares 1 changed file with "+other+": shared.txt") {
		t.Errorf("view text lacks the overlap:\n%s", text)
	}

	// Once B is merged into main, it's no longer in flight.
	gittest.Git(t, dir, "merge", "-q", "--no-ff", "--no-edit", "-X", "theirs", b+"-b") // B and D both changed shared.txt
	ov = must[Overview](t)(m.Overview())
	if got := overlapPairs(ov.Overlaps); !reflect.DeepEqual(got, map[string][]string{pair(a, c): {"a.txt"}}) {
		t.Errorf("after merging B, overlaps = %v", got)
	}
}

// pair is two IDs as overlapPairs keys them: in ID order.
func pair(x, y string) string {
	if y < x {
		x, y = y, x
	}
	return x + "+" + y
}

func TestOverlapProblemText(t *testing.T) {
	o := Overlap{Items: [2]OverlapItem{{ID: "aaaa", Plan: "p", State: model.StateActive}, {ID: "bbbb", Plan: "q", State: model.StateInReview}},
		Files: []string{"1", "2", "3", "4", "5", "6", "7"}}
	pr := overlapProblem(o, "q")
	if pr.Item != "bbbb" || pr.With != "aaaa" || len(pr.Files) != 7 {
		t.Errorf("problem = %+v", pr)
	}
	if want := "shares 7 changed files with aaaa (active): 1, 2, 3, 4, 5, and 2 more"; pr.Text != want {
		t.Errorf("text = %q, want %q", pr.Text, want)
	}
}
