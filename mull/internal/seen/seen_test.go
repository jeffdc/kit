package seen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func isolate(t *testing.T) string {
	t.Helper()
	x := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", x)
	return filepath.Join(x, "mull", "seen.json")
}

func TestMarkAndGet(t *testing.T) {
	path := isolate(t)
	if f, _ := File(); f != path {
		t.Errorf("File = %q, want %q", f, path)
	}
	if _, ok, err := Get("/r/kit"); ok || err != nil {
		t.Fatalf("Get with no file = %v, %v; want nothing", ok, err)
	}
	a := Look{Commit: "aaaa", At: "2026-11-10T08:15:00Z"}
	if err := Mark("/r/kit", a); err != nil {
		t.Fatal(err)
	}
	if err := Mark("/r/forage", Look{Commit: "bbbb", At: "2026-11-10T09:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if l, ok, err := Get("/r/kit"); !ok || err != nil || l != a {
		t.Errorf("Get = %+v, %v, %v; want %+v", l, ok, err, a)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil || len(f.Repos) != 2 {
		t.Errorf("file = %s (%v)", b, err)
	}
	// Only the state file is left: the temporary files were renamed.
	if ents, _ := os.ReadDir(filepath.Dir(path)); len(ents) != 1 {
		t.Errorf("config dir holds %d files, want only seen.json", len(ents))
	}
}

func TestBaseline(t *testing.T) {
	isolate(t)
	first := Look{Commit: "aaaa", At: "t1"}
	if l, err := Baseline("/r/kit", first, ""); err != nil || l != first {
		t.Fatalf("first Baseline = %+v, %v", l, err)
	}
	// Someone marked it meanwhile: a baseline from a stale read keeps theirs.
	if l, err := Baseline("/r/kit", Look{Commit: "bbbb", At: "t2"}, ""); err != nil || l != first {
		t.Errorf("Baseline over a look = %+v, %v; want the existing %+v", l, err, first)
	}
	// Replacing a stale commit (one the repository doesn't have).
	if l, err := Baseline("/r/kit", Look{Commit: "cccc", At: "t3"}, "aaaa"); err != nil || l.Commit != "cccc" {
		t.Errorf("Baseline replacing the stale commit = %+v, %v", l, err)
	}
}

func TestCorruptFile(t *testing.T) {
	path := isolate(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Get("/r/kit"); err == nil {
		t.Error("Get on a corrupt file: no error")
	}
	if err := Mark("/r/kit", Look{Commit: "aaaa"}); err == nil {
		t.Error("Mark on a corrupt file: no error (it would lose every other repo's look)")
	}
	if b, _ := os.ReadFile(path); string(b) != "{nope" {
		t.Errorf("corrupt file was overwritten: %q", b)
	}
}

func TestConcurrentMarks(t *testing.T) {
	isolate(t)
	var wg sync.WaitGroup
	repos := []string{"/a", "/b", "/c", "/d", "/e", "/f", "/g", "/h"}
	for _, r := range repos {
		wg.Add(1)
		go func(r string) {
			defer wg.Done()
			if err := Mark(r, Look{Commit: r}); err != nil {
				t.Error(err)
			}
		}(r)
	}
	wg.Wait()
	for _, r := range repos {
		if l, ok, _ := Get(r); !ok || l.Commit != r {
			t.Errorf("lost the mark for %s", r)
		}
	}
}
