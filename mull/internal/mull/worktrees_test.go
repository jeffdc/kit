package mull

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
)

func TestWorktreesPrunable(t *testing.T) {
	m, dir := setup(t)
	add := func(title string) ItemView {
		return must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: title}))
	}
	// work claims id in a worktree of its own, commits there, and merges
	// the branch into main.
	work := func(id string) string {
		t.Helper()
		branch := id + "-work"
		wt := gittest.AddWorktree(t, dir, branch)
		w, err := Open(wt)
		if err != nil {
			t.Fatal(err)
		}
		must[StartResult](t)(w.Start(id))
		gittest.Commit(t, wt, id+".txt", strings.Repeat("x", 2000), "Work on "+id, "Mull: "+id)
		gittest.Git(t, dir, "merge", "-q", "--no-ff", "--no-edit", branch)
		return wt
	}
	accept := func(id string) {
		t.Helper()
		markInReview(t, m, id)
		must[AcceptResult](t)(m.Accept([]string{id}, "", SettleOpts{OnRequest: true}))
	}

	pruned, dirty, ahead, active, here := add("Prunable"), add("Dirty"), add("Ahead"), add("Active"), add("Main worktree")
	wtPruned, wtDirty, wtAhead, wtActive := work(pruned.ID), work(dirty.ID), work(ahead.ID), work(active.ID)
	accept(pruned.ID)
	accept(dirty.ID)
	accept(ahead.ID)
	if err := os.WriteFile(filepath.Join(wtDirty, "scratch.txt"), []byte("left behind"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Commit(t, wtAhead, "more.txt", "more", "More after the merge", "Mull: "+ahead.ID)
	// An item claimed from the main worktree: never listed.
	must[StartResult](t)(m.Start(here.ID))
	accept(here.ID)

	l := must[WorktreeList](t)(m.Worktrees())
	if len(l.Worktrees) != 4 {
		t.Fatalf("worktrees = %+v, want the four claim worktrees (not the main one)", l.Worktrees)
	}
	rows := map[string]WorktreeRow{}
	for _, r := range l.Worktrees {
		rows[r.Item] = r
	}
	p := rows[pruned.ID]
	if !p.Prunable || !p.Clean || !p.Merged || p.Why != "" || p.Path != wtPruned || p.Branch != pruned.ID+"-work" || p.Target != "main" || p.State != model.StateDone {
		t.Errorf("prunable row = %+v", p)
	}
	if want := []string{"git worktree remove " + wtPruned, "git branch -d " + pruned.ID + "-work"}; strings.Join(p.Commands, "\n") != strings.Join(want, "\n") {
		t.Errorf("commands = %q, want %q", p.Commands, want)
	}
	if p.Bytes < 2000 {
		t.Errorf("bytes = %d, want at least the file committed there", p.Bytes)
	}
	if l.Prunable != 1 || l.PrunableBytes != p.Bytes {
		t.Errorf("prunable %d (%d bytes), want 1 (%d)", l.Prunable, l.PrunableBytes, p.Bytes)
	}
	for id, want := range map[string]string{dirty.ID: "uncommitted", ahead.ID: "commits not on main", active.ID: "is active"} {
		r := rows[id]
		if r.Prunable || len(r.Commands) > 0 || !strings.Contains(r.Why, want) {
			t.Errorf("%s: %+v, want not prunable because %q", id, r, want)
		}
	}
	if rows[dirty.ID].Clean || !rows[dirty.ID].Merged || rows[ahead.ID].Merged || rows[active.ID].Path != wtActive || rows[ahead.ID].Path != wtAhead {
		t.Errorf("dirty %+v, ahead %+v, active %+v", rows[dirty.ID], rows[ahead.ID], rows[active.ID])
	}

	// The view and the overview count them.
	v := must[View](t)(m.Board(""))
	if v.PrunableWorktrees == nil || *v.PrunableWorktrees != (Prunable{Count: 1, Bytes: p.Bytes}) {
		t.Fatalf("view prunable = %+v", v.PrunableWorktrees)
	}
	if txt := v.Text(m.Now()); !strings.Contains(txt, "WORKTREES  1 claim worktree (") || !strings.Contains(txt, "mull worktrees") {
		t.Errorf("view text:\n%s", txt)
	}
	ov := must[Overview](t)(m.Overview())
	if ov.Worktrees == nil || ov.Worktrees.Count != 1 {
		t.Errorf("overview prunable = %+v", ov.Worktrees)
	}

	// mull worktrees only lists: nothing was removed.
	if _, err := os.Stat(wtPruned); err != nil {
		t.Errorf("worktree gone: %v", err)
	}
	if gittest.Git(t, dir, "branch", "--list", pruned.ID+"-work") == "" {
		t.Error("branch gone")
	}
}

func TestWorktreesNoneLinked(t *testing.T) {
	m, _ := setup(t)
	l := must[WorktreeList](t)(m.Worktrees())
	if len(l.Worktrees) != 0 || l.Prunable != 0 {
		t.Fatalf("worktrees = %+v", l)
	}
	if v := must[View](t)(m.Board("")); v.PrunableWorktrees != nil || strings.Contains(v.Text(m.Now()), "WORKTREES") {
		t.Errorf("view = %+v", v.PrunableWorktrees)
	}
}

func TestShellQuoteAndBytes(t *testing.T) {
	for in, want := range map[string]string{
		"/tmp/wt/abcd-x": "/tmp/wt/abcd-x",
		"/tmp/my wt":     "'/tmp/my wt'",
		"it's":           `'it'\''s'`,
	} {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
	for n, want := range map[int64]string{0: "0 bytes", 999: "999 bytes", 48_000: "48 KB", 327_925_041: "328 MB", 5_500_000_000: "5.5 GB"} {
		if got := bytesText(n); got != want {
			t.Errorf("bytesText(%d) = %q, want %q", n, got, want)
		}
	}
}
