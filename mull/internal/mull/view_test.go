package mull

import (
	"strings"
	"testing"
	"time"

	"mull/internal/gittest"
	"mull/internal/model"
)

func problemsByItem(b PlanBoard) map[string]string {
	out := map[string]string{}
	for _, p := range b.Problems {
		out[p.Item] = p.Kind
	}
	return out
}

func flightByItem(b PlanBoard) map[string]FlightLine {
	out := map[string]FlightLine{}
	for _, f := range b.InFlight {
		out[f.ID] = f
	}
	return out
}

func TestBoardFromGit(t *testing.T) {
	m, dir := setup(t)
	must[*model.Plan](t)(m.PlanNew("launch", "Public launch", ""))
	phase := must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "", nil))
	add := func(title string) ItemView {
		return must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Phase: phase.ID, Title: title}))
	}
	a, b, c, d, e, f := add("A unmerged"), add("B trailer"), add("C merged"), add("D done no git"), add("E idle"), add("F unclaimed")

	// A: claimed, work on an unmerged id-branch.
	must[StartResult](t)(m.Start(a.ID))
	gittest.Git(t, dir, "checkout", "-q", "-b", a.ID+"-work")
	gittest.Commit(t, dir, "a1", "1", "A step one")
	gittest.Commit(t, dir, "a2", "2", "A step two")
	gittest.Git(t, dir, "checkout", "-q", "main")

	// B: landed on main through a trailer.
	gittest.Commit(t, dir, "b", "b", "Do B", "Mull: "+b.ID)

	// C: landed through a merge of its id-branch.
	gittest.Git(t, dir, "checkout", "-q", "-b", c.ID+"-x")
	gittest.Commit(t, dir, "c", "c", "Do C")
	gittest.Git(t, dir, "checkout", "-q", "main")
	gittest.Git(t, dir, "merge", "-q", "--no-ff", "--no-edit", c.ID+"-x")
	gittest.Git(t, dir, "branch", "-q", "-d", c.ID+"-x")

	// Unlinked direct work on main.
	gittest.Commit(t, dir, "u", "u", "Tweak something, no link")

	// F: someone works on F's branch without claiming it.
	gittest.Git(t, dir, "checkout", "-q", "-b", f.ID+"-y")
	gittest.Commit(t, dir, "f", "f", "Start F")
	gittest.Git(t, dir, "checkout", "-q", "main")

	// Done states and a claim nobody works on.
	markInReview(t, m, b.ID)
	markInReview(t, m, d.ID)
	must[StartResult](t)(m.Start(e.ID))

	// Look at the board four days later.
	later := time.Now().Add(4 * 24 * time.Hour)
	m.Now = func() time.Time { return later }
	v := must[View](t)(m.Board(""))
	if len(v.Plans) != 2 || v.Plans[1].Slug != "launch" { // general, empty, and launch
		t.Fatalf("plans on board: %+v", v.Plans)
	}
	board := v.Plans[1]

	probs := problemsByItem(board)
	want := map[string]string{d.ID: "done-not-in-git", e.ID: "idle-claim", f.ID: "unclaimed-work", a.ID: "idle-claim"}
	for id, kind := range want {
		if probs[id] != kind {
			t.Errorf("item %s: problem %q, want %q (all: %v)", id, probs[id], kind, probs)
		}
	}
	if _, ok := probs[b.ID]; ok {
		t.Errorf("B is done and seen in git; no problem expected")
	}

	flight := flightByItem(board)
	if fa := flight[a.ID]; fa.Commits != 2 || len(fa.Branches) != 1 || fa.Branches[0] != a.ID+"-work" {
		t.Errorf("A in flight = %+v", fa)
	}
	if _, ok := flight[f.ID]; !ok {
		t.Error("F (unclaimed work) should show in flight")
	}

	g, err := func() (*gitState, error) {
		var g *gitState
		err := m.view(func(s *state) error { var err error; g, err = m.readGit(s); return err })
		return g, err
	}()
	if err != nil {
		t.Fatal(err)
	}
	if !g.Items[b.ID].SeenInGit || !g.Items[c.ID].SeenInGit || g.Items[a.ID].SeenInGit {
		t.Errorf("seen in git: B %v, C %v, A %v", g.Items[b.ID].SeenInGit, g.Items[c.ID].SeenInGit, g.Items[a.ID].SeenInGit)
	}
	// "first" (the repo's initial commit) and the unlinked tweak are within the week.
	if v.UnlinkedCount != 2 || !strings.Contains(strings.Join(v.UnlinkedRecent, ";"), "Tweak something") {
		t.Errorf("unlinked = %d %v", v.UnlinkedCount, v.UnlinkedRecent)
	}

	if board.NeedsYou.Reviews != 2 {
		t.Errorf("needs you = %+v", board.NeedsYou)
	}
	text := v.Text(later)
	for _, s := range []string{"LAUNCH", "PHASES", "PROBLEMS (4)", "IN FLIGHT", "NEEDS YOU  2 reviews", "UNLINKED  2 commits"} {
		if !strings.Contains(text, s) {
			t.Errorf("view text lacks %q:\n%s", s, text)
		}
	}
}

func TestBoardWithNoPlansAndMissingIntegration(t *testing.T) {
	dir := gittest.NewRepo(t)
	m, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Init("trunk", "main"); err != nil { // no trunk branch exists
		t.Fatal(err)
	}
	v := must[View](t)(m.Board(""))
	if len(v.Plans) != 0 || v.IntegrationFound {
		t.Fatalf("view = %+v", v)
	}
	if !strings.Contains(v.Text(time.Now()), "branch not found") {
		t.Error("missing integration branch not reported")
	}
	must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "x"}))
	v = must[View](t)(m.Board(""))
	if len(v.Plans) != 0 || v.Unplanned == nil || v.Unplanned.NeedsYou.Questions != 1 {
		t.Fatalf("a project-wide question should show under unplanned: %+v %+v", v.Plans, v.Unplanned)
	}
	_, err = m.Board("nope")
	wantErr(t, err, `no plan "nope"`)
}
