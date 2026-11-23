package mull

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
)

// runnerSetup is riskSetup with work landing on a plan branch, plan/web,
// made from main (the release branch here).
func runnerSetup(t *testing.T) (*Mull, string) {
	t.Helper()
	m, dir := riskSetup(t)
	must[model.Config](t)(m.ConfigSet("release", "main"))
	must[*model.Plan](t)(m.PlanNew("web", "Web", "", true))
	must[*model.Plan](t)(m.PlanAgree("web", true))
	must[SetResult](t)(m.Set("web", "target", "plan/web"))
	return m, dir
}

// worker opens mull in a worktree start made.
func worker(t *testing.T, path string) *Mull {
	t.Helper()
	w, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestStartMakesBranchAndWorktree(t *testing.T) {
	m, dir := runnerSetup(t)
	must[model.Config](t)(m.ConfigSetup([]string{`test -n "$MULL_MAIN" && echo "$MULL_ITEM" > setup.txt`}, true))
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "web", Title: "Footer: say Oak Compendium!"}))

	res := must[StartWorkResult](t)(m.StartWork(it.ID, false))
	if res.Branch != it.ID+"-footer-say-oak-compendium" || res.Target != "plan/web" {
		t.Fatalf("start = %+v", res)
	}
	if want := filepath.Base(dir) + ".work"; filepath.Base(filepath.Dir(res.Worktree)) != want {
		t.Errorf("worktree %s is not under %s", res.Worktree, want)
	}
	if b, err := os.ReadFile(filepath.Join(res.Worktree, "setup.txt")); err != nil || strings.TrimSpace(string(b)) != it.ID {
		t.Errorf("setup didn't run in the worktree: %q %v", b, err)
	}
	if tip := gittest.Git(t, dir, "rev-parse", "plan/web"); tip != gittest.Git(t, dir, "rev-parse", "main") {
		t.Errorf("plan/web wasn't made from main")
	}
	got := item(t, m, it.ID)
	if got.State != model.StateActive || got.Claim.Worktree != res.Worktree || got.Claim.Branch != res.Branch {
		t.Fatalf("claim = %+v", got.Claim)
	}

	// The agent works there and finishes: the work lands on plan/web.
	w := worker(t, res.Worktree)
	gittest.Commit(t, res.Worktree, "web/footer.html", "Oak Compendium\n", "Footer", "Mull: "+it.ID)
	os.Remove(filepath.Join(res.Worktree, "setup.txt")) // setup's output, not work
	done := must[DoneResult](t)(w.Done(it.ID, DoneInput{Summary: "Footer.", Rules: "none"}))
	if !done.Landed || !done.Merged || done.MergeError != "" {
		t.Fatalf("done = %+v", done)
	}
	if tip := gittest.Git(t, dir, "rev-parse", "plan/web"); tip != done.Report.Commit {
		t.Fatalf("plan/web is at %s, want %s", tip, done.Report.Commit)
	}
	if gittest.Git(t, dir, "rev-parse", "main") == done.Report.Commit {
		t.Fatal("work landed on main, the release branch")
	}

	// Its worktree goes once someone else runs the runner, and its branch.
	pruned := must[[]string](t)(m.Prune())
	if len(pruned) != 1 || pruned[0] != res.Worktree {
		t.Fatalf("pruned = %v", pruned)
	}
	if _, err := os.Stat(res.Worktree); !os.IsNotExist(err) {
		t.Fatal("the worktree is still there")
	}
	if out := gittest.Git(t, dir, "branch", "--list", res.Branch); out != "" {
		t.Fatalf("the item's branch is still there: %q", out)
	}
}

func TestSetupFailureLeavesNothing(t *testing.T) {
	m, dir := runnerSetup(t)
	must[model.Config](t)(m.ConfigSetup([]string{"echo broken; exit 3"}, true))
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "web", Title: "Broken"}))
	_, err := m.StartWork(it.ID, false)
	wantCode(t, err, CodeSetupFailed)
	wantErr(t, err, "broken")
	if item(t, m, it.ID).State != model.StateCommitted {
		t.Fatal("the item was claimed")
	}
	if out := gittest.Git(t, dir, "worktree", "list"); strings.Count(out, "\n") != 0 {
		t.Fatalf("worktrees left: %s", out)
	}
}

func TestBehindTargetMergesFirst(t *testing.T) {
	m, dir := runnerSetup(t)
	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "web", Title: "A"}))
	b := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "web", Title: "B"}))
	ra := must[StartWorkResult](t)(m.StartWork(a.ID, false))
	rb := must[StartWorkResult](t)(m.StartWork(b.ID, false))

	gittest.Commit(t, ra.Worktree, "a.txt", "a\n", "A", "Mull: "+a.ID)
	must[DoneResult](t)(worker(t, ra.Worktree).Done(a.ID, DoneInput{Summary: "A.", Rules: "none"}))

	gittest.Commit(t, rb.Worktree, "b.txt", "b\n", "B", "Mull: "+b.ID)
	wb := worker(t, rb.Worktree)
	_, err := wb.Done(b.ID, DoneInput{Summary: "B.", Rules: "none"})
	wantCode(t, err, CodeBehindTarget)
	gittest.Git(t, rb.Worktree, "merge", "-q", "--no-edit", "plan/web")
	done := must[DoneResult](t)(wb.Done(b.ID, DoneInput{Summary: "B.", Rules: "none"}))
	if !done.Merged {
		t.Fatalf("done = %+v", done)
	}
	for _, f := range []string{"a.txt", "b.txt"} {
		gittest.Git(t, dir, "cat-file", "-e", "plan/web:"+f)
	}
}

func TestLandMovesACheckedOutTarget(t *testing.T) {
	m, dir := runnerSetup(t)
	preview := gittest.AddWorktree(t, dir, "plan/web") // the human's preview of the plan
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "web", Title: "Page"}))
	res := must[StartWorkResult](t)(m.StartWork(it.ID, false))
	gittest.Commit(t, res.Worktree, "page.html", "p\n", "Page", "Mull: "+it.ID)
	w := worker(t, res.Worktree)

	// A dirty preview can't move: the item is done, not yet on the target.
	gittest.Commit(t, preview, "notes.txt", "n\n", "Notes")
	gittest.Git(t, res.Worktree, "merge", "-q", "--no-edit", "plan/web")
	if err := os.WriteFile(filepath.Join(preview, "notes.txt"), []byte("editing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	done := must[DoneResult](t)(w.Done(it.ID, DoneInput{Summary: "Page.", Rules: "none"}))
	if !done.Landed || done.Merged || !strings.Contains(done.MergeError, "uncommitted") {
		t.Fatalf("done = %+v", done)
	}
	gittest.Git(t, preview, "checkout", "-q", "--", "notes.txt")

	// Clean, it moves, files and all.
	if ok, why := m.landItem("plan/web", "main", done.Report.Commit); !ok {
		t.Fatal(why)
	}
	if _, err := os.Stat(filepath.Join(preview, "page.html")); err != nil {
		t.Fatal("the preview's files didn't move with the branch")
	}
}

func TestNeverLandsOnRelease(t *testing.T) {
	m, dir := runnerSetup(t)
	must[SetResult](t)(m.Set("web", "target", "main"))
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "web", Title: "X"}))
	_, err := m.StartWork(it.ID, false)
	wantErr(t, err, "release branch")
	if ok, _ := m.landItem("main", "main", gittest.Git(t, dir, "rev-parse", "HEAD")); ok {
		t.Fatal("landed on main")
	}
}

func TestParallelCap(t *testing.T) {
	m, _ := runnerSetup(t)
	must[model.Config](t)(m.configTx("parallel", func(s *state) (model.Config, error) { return m.configSet(s, "parallel", "2", true) }))
	var ids []string
	for _, title := range []string{"A", "B", "C"} {
		ids = append(ids, must[ItemView](t)(m.ItemAdd(NewItem{Plan: "web", Title: title})).ID)
	}
	must[StartWorkResult](t)(m.StartWork(ids[0], false))
	must[StartWorkResult](t)(m.StartWork(ids[1], false))
	_, err := m.StartWork(ids[2], false)
	wantCode(t, err, CodeAtCapacity)
	_, err = m.ConfigSet("parallel", "5")
	wantCode(t, err, CodeNotAllowed)
}

func TestStandingBranchForUnplannedWork(t *testing.T) {
	m, _ := runnerSetup(t)
	must[model.Config](t)(m.ConfigSet("integration", "main"))
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "none", Title: "Small fix"}))
	res := must[StartWorkResult](t)(m.StartWork(it.ID, false))
	if res.Target != model.DefaultStanding {
		t.Fatalf("unplanned work lands on %s, want next", res.Target)
	}
}
