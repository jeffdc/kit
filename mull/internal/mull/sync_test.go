package mull

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
	"mull/internal/store"
)

// bareRemote creates a bare repository and adds it as origin of each repo.
func bareRemote(t *testing.T, repos ...string) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "remote.git")
	gittest.Git(t, filepath.Dir(remote), "init", "-q", "--bare", remote)
	for _, r := range repos {
		gittest.Git(t, r, "remote", "add", "origin", remote)
	}
	return remote
}

func openAt(t *testing.T, dir string) *Mull {
	t.Helper()
	m, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSyncFirstPushSendsOnlyTheMullBranch(t *testing.T) {
	m, dir := setup(t)
	remote := bareRemote(t, dir)
	must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A"}))

	var pushed []string
	m.pushHook = func(args []string) { pushed = args }
	res := must[SyncResult](t)(m.Sync(false))
	if res.Action != "pushed" || res.Pushed != 2 {
		t.Fatalf("sync = %+v", res)
	}
	local := gittest.Git(t, dir, "rev-parse", store.Ref)
	want := []string{"push", "--quiet", "origin", local + ":refs/heads/mull"}
	if !reflect.DeepEqual(pushed, want) {
		t.Fatalf("push args = %v, want %v", pushed, want)
	}
	// The remote holds the mull branch and nothing else: not main, not tags.
	if refs := gittest.Git(t, remote, "for-each-ref", "--format=%(refname)"); refs != "refs/heads/mull" {
		t.Fatalf("remote refs = %q", refs)
	}
	again := must[SyncResult](t)(m.Sync(false))
	if again.Action != "up-to-date" {
		t.Fatalf("second sync = %+v", again)
	}
}

func TestSyncCloneAndMerge(t *testing.T) {
	a, dirA := setup(t)
	gittest.Isolate(t)
	dirB := gittest.NewRepo(t)
	bareRemote(t, dirA, dirB)
	must[ItemView](t)(a.ItemAdd(NewItem{Plan: "general", Title: "from A"}))
	must[SyncResult](t)(a.Sync(false))

	// A second machine: no init, just sync.
	b := openAt(t, dirB)
	if res := must[SyncResult](t)(b.Sync(false)); res.Action != "cloned" {
		t.Fatalf("clone = %+v", res)
	}
	if n := len(must[[]ItemView](t)(b.ItemList(ItemFilter{}))); n != 1 {
		t.Fatalf("clone has %d items", n)
	}

	// Both write; B pushes first, A merges.
	must[ItemView](t)(a.ItemAdd(NewItem{Plan: "general", Title: "A again"}))
	must[*model.Entry](t)(b.Capture(CaptureInput{Kind: model.KindLoose, Text: "from B"}))
	if res := must[SyncResult](t)(b.Sync(false)); res.Action != "pushed" {
		t.Fatalf("B sync = %+v", res)
	}
	if res := must[SyncResult](t)(a.Sync(false)); res.Action != "merged" {
		t.Fatalf("A sync = %+v", res)
	}
	if res := must[SyncResult](t)(b.Sync(false)); res.Action != "pulled" {
		t.Fatalf("B second sync = %+v", res)
	}
	for _, m := range []*Mull{a, b} {
		items := must[[]ItemView](t)(m.ItemList(ItemFilter{}))
		inbox := must[[]*model.Entry](t)(m.Inbox(InboxFilter{}))
		if len(items) != 2 || len(inbox) != 1 {
			t.Fatalf("after merge: %d items, %d entries", len(items), len(inbox))
		}
	}
}

func TestSyncConflictChangesNothing(t *testing.T) {
	a, dirA := setup(t)
	dirB := gittest.NewRepo(t)
	bareRemote(t, dirA, dirB)
	it := must[ItemView](t)(a.ItemAdd(NewItem{Plan: "general", Title: "shared"}))
	must[SyncResult](t)(a.Sync(false))
	b := openAt(t, dirB)
	must[SyncResult](t)(b.Sync(false))

	must[SetResult](t)(a.Set(it.ID, "title", "A's title"))
	must[SetResult](t)(b.Set(it.ID, "title", "B's title"))
	must[SyncResult](t)(b.Sync(false))

	before := gittest.Git(t, dirA, "rev-parse", store.Ref)
	_, err := a.Sync(false)
	var conflict *ErrSyncConflict
	if !errors.As(err, &conflict) || len(conflict.Paths) != 1 || conflict.Paths[0] != model.ItemPath(it.ID) {
		t.Fatalf("got %v, want a conflict on %s", err, model.ItemPath(it.ID))
	}
	if after := gittest.Git(t, dirA, "rev-parse", store.Ref); after != before {
		t.Fatal("a conflicted sync moved the local mull branch")
	}
}

func TestSyncRefusesNonMullPaths(t *testing.T) {
	m, dir := setup(t)
	remote := bareRemote(t, dir)
	// Smuggle a code file onto the mull branch with plumbing.
	err := m.store.Update("smuggle", func(tx *store.Tx) error {
		tx.Put("src/secret.go", []byte("package secret"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	pushedAny := false
	m.pushHook = func([]string) { pushedAny = true }
	_, err = m.Sync(false)
	wantErr(t, err, "not mull records: src/secret.go")
	if pushedAny {
		t.Fatal("push attempted")
	}
	if refs := gittest.Git(t, remote, "for-each-ref"); refs != "" {
		t.Fatalf("remote got refs: %q", refs)
	}
}

func TestSyncDryRunChangesNothing(t *testing.T) {
	m, dir := setup(t)
	remote := bareRemote(t, dir)
	res := must[SyncResult](t)(m.Sync(true))
	if !res.DryRun || res.Action != "pushed" || len(res.Commands) != 1 || res.Commands[0][0] != "push" {
		t.Fatalf("dry run = %+v", res)
	}
	if refs := gittest.Git(t, remote, "for-each-ref"); refs != "" {
		t.Fatalf("dry run pushed: %q", refs)
	}
}

func TestSyncRunsPrePushHook(t *testing.T) {
	m, dir := setup(t)
	bareRemote(t, dir)
	hook := filepath.Join(dir, ".git", "hooks", "pre-push")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho blocked-by-hook >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := m.Sync(false)
	if err == nil || !strings.Contains(err.Error(), "blocked-by-hook") {
		t.Fatalf("got %v, want the pre-push hook to block", err)
	}
}

func TestSyncWithoutRemoteBranchOrInit(t *testing.T) {
	dir := gittest.NewRepo(t)
	bareRemote(t, dir)
	_, err := openAt(t, dir).Sync(false)
	if !errors.Is(err, store.ErrNotInitialized) {
		t.Fatalf("got %v", err)
	}
}
