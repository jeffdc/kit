package store

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"mull/internal/git"
	"mull/internal/gittest"
)

func newStore(t *testing.T, dir string) *Store {
	t.Helper()
	r, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return New(r)
}

func TestLoadBeforeInit(t *testing.T) {
	s := newStore(t, gittest.NewRepo(t))
	if _, err := s.Load(); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Load before init: got %v, want ErrNotInitialized", err)
	}
	ok, err := s.Initialized()
	if err != nil || ok {
		t.Fatalf("Initialized = %v, %v; want false, nil", ok, err)
	}
}

func TestInitAndLoad(t *testing.T) {
	s := newStore(t, gittest.NewRepo(t))
	err := s.Init("init", map[string][]byte{"config.yml": []byte("a: 1\n"), "plans/general.md": []byte("x")})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if string(snap.Files["config.yml"]) != "a: 1\n" || string(snap.Files["plans/general.md"]) != "x" {
		t.Fatalf("unexpected files: %q", snap.Files)
	}
	if err := s.Init("again", nil); !errors.Is(err, ErrExists) {
		t.Fatalf("second Init: got %v, want ErrExists", err)
	}
}

func TestUpdatePutDeleteAndNoop(t *testing.T) {
	dir := gittest.NewRepo(t)
	s := newStore(t, dir)
	if err := s.Init("init", map[string][]byte{"a": []byte("1"), "b": []byte("2")}); err != nil {
		t.Fatal(err)
	}
	err := s.Update("change", func(tx *Tx) error {
		tx.Put("c/d.md", []byte("3"))
		tx.Delete("a")
		if _, ok := tx.Get("a"); ok {
			t.Error("deleted file still visible in tx")
		}
		if got := tx.List("c/"); len(got) != 1 || got[0] != "c/d.md" {
			t.Errorf("List(c/) = %v", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := s.Load()
	if _, ok := snap.Files["a"]; ok {
		t.Error("a not deleted")
	}
	if string(snap.Files["c/d.md"]) != "3" || string(snap.Files["b"]) != "2" {
		t.Errorf("unexpected files: %q", snap.Files)
	}
	before := snap.Commit
	if err := s.Update("noop", func(tx *Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	after, _ := s.Load()
	if after.Commit != before {
		t.Error("a no-op update made a commit")
	}
	// The mull branch has its own history: one root commit plus one update.
	if n := gittest.Git(t, dir, "rev-list", "--count", Ref); n != "2" {
		t.Errorf("mull branch has %s commits, want 2", n)
	}
}

func TestWritesDoNotTouchWorkingTreeOrIndex(t *testing.T) {
	dir := gittest.NewRepo(t)
	s := newStore(t, dir)
	if err := s.Init("init", map[string][]byte{"x": []byte("1")}); err != nil {
		t.Fatal(err)
	}
	if err := s.Update("u", func(tx *Tx) error { tx.Put("y", []byte("2")); return nil }); err != nil {
		t.Fatal(err)
	}
	if st := gittest.Git(t, dir, "status", "--porcelain"); st != "" {
		t.Errorf("working tree changed: %q", st)
	}
	if b := gittest.Git(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); b != "main" {
		t.Errorf("HEAD moved to %s", b)
	}
}

func TestSharedAcrossWorktrees(t *testing.T) {
	dir := gittest.NewRepo(t)
	wt := gittest.AddWorktree(t, dir, "feature")
	if err := newStore(t, dir).Init("init", map[string][]byte{"x": []byte("1")}); err != nil {
		t.Fatal(err)
	}
	other := newStore(t, wt)
	if err := other.Update("from worktree", func(tx *Tx) error { tx.Put("y", []byte("2")); return nil }); err != nil {
		t.Fatal(err)
	}
	snap, err := newStore(t, dir).Load()
	if err != nil {
		t.Fatal(err)
	}
	if string(snap.Files["y"]) != "2" {
		t.Error("write from a worktree not visible in the main checkout")
	}
}

func TestConcurrentWritersAllLand(t *testing.T) {
	dir := gittest.NewRepo(t)
	if err := newStore(t, dir).Init("init", map[string][]byte{"counter": []byte("0")}); err != nil {
		t.Fatal(err)
	}
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := newStore(t, dir)
			errs <- s.Update(fmt.Sprintf("w%d", i), func(tx *Tx) error {
				tx.Put(fmt.Sprintf("w/%d", i), []byte("x"))
				return nil
			})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	snap, _ := newStore(t, dir).Load()
	for i := 0; i < n; i++ {
		if _, ok := snap.Files[fmt.Sprintf("w/%d", i)]; !ok {
			t.Errorf("write %d lost", i)
		}
	}
}

func TestUpdateRerunsOnRace(t *testing.T) {
	dir := gittest.NewRepo(t)
	s := newStore(t, dir)
	if err := s.Init("init", map[string][]byte{"owner": []byte("")}); err != nil {
		t.Fatal(err)
	}
	// The first run of fn sees "owner" empty; before it commits, another writer
	// claims it. The retry must see the claim and refuse.
	runs := 0
	err := s.Update("claim", func(tx *Tx) error {
		runs++
		if b, _ := tx.Get("owner"); len(b) > 0 {
			return errors.New("taken by " + string(b))
		}
		if runs == 1 {
			if err := newStore(t, dir).Update("rival", func(tx *Tx) error {
				tx.Put("owner", []byte("rival"))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
		tx.Put("owner", []byte("me"))
		return nil
	})
	if err == nil || err.Error() != "taken by rival" {
		t.Fatalf("got %v, want the retry to see the rival's claim", err)
	}
	if runs != 2 {
		t.Errorf("fn ran %d times, want 2", runs)
	}
}
