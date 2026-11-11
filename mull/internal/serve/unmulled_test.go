package serve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mull/internal/gittest"
)

// repoIn makes a git repository named name in root with one commit made at
// when (RFC 3339).
func repoIn(t *testing.T, root, name, when string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "init", "-q", "-b", "main")
	t.Setenv("GIT_COMMITTER_DATE", when)
	t.Setenv("GIT_AUTHOR_DATE", when)
	gittest.Commit(t, dir, "README", "x\n", "first")
	return dir
}

func TestUnmulled(t *testing.T) {
	gittest.Isolate(t)
	root := t.TempDir()
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	recent, old := "2026-11-01T10:00:00Z", "2026-03-01T10:00:00Z"

	registered := repoIn(t, root, "kit", recent)
	gittest.Git(t, registered, "branch", "mull") // on mull and registered
	unregistered := repoIn(t, root, "cloned", recent)
	gittest.Git(t, unregistered, "branch", "mull") // on mull, not registered yet
	fresh := repoIn(t, root, "Fresh", recent)
	repoIn(t, root, "dormant", old)
	oldMull := repoIn(t, root, "anxiety", old)
	if err := os.Mkdir(filepath.Join(oldMull, ".mull"), 0o755); err != nil {
		t.Fatal(err)
	}
	repoIn(t, root, ".hidden", recent)
	if err := os.Mkdir(filepath.Join(root, "notes"), 0o755); err != nil { // not a repository
		t.Fatal(err)
	}
	gittest.Git(t, fresh, "worktree", "add", "-q", "-b", "side", filepath.Join(root, "fresh-side"))

	s := &scanner{now: func() time.Time { return now }}
	got := s.get([]string{registered})
	want := []Unmulled{
		{Name: "anxiety", Path: oldMull, Kind: KindOldMull, LastCommit: old},
		{Name: "Fresh", Path: fresh, Kind: KindNotOnMull, LastCommit: recent},
	}
	if len(got.Roots) != 1 || got.Roots[0] != root {
		t.Errorf("roots = %v, want [%s]", got.Roots, root)
	}
	if b, _ := json.Marshal(got.Repos); string(b) != mustJSON(t, want) {
		t.Errorf("repos =\n%s\nwant\n%s", b, mustJSON(t, want))
	}

	// Kept for a while: a new repository shows up only once the scan is old.
	repoIn(t, root, "newer", recent)
	if again := s.get([]string{registered}); len(again.Repos) != 2 || again.ScannedAt != got.ScannedAt {
		t.Errorf("rescanned within %s: %+v", rescanAfter, again)
	}
	now = now.Add(rescanAfter)
	if later := s.get([]string{registered}); len(later.Repos) != 3 {
		t.Errorf("after %s: %d repos, want 3 (newer listed)", rescanAfter, len(later.Repos))
	}
}

func TestUnmulledEndpoint(t *testing.T) {
	gittest.Isolate(t)
	h := Handler(func() ([]string, error) { return []string{"/nowhere/kit"}, nil })
	rec := get(t, h, "/api/unmulled")
	var l UnmulledList
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &l) != nil || len(l.Repos) != 0 || len(l.Roots) != 1 || l.Roots[0] != "/nowhere" {
		t.Errorf("GET /api/unmulled with a missing root: %d %s", rec.Code, rec.Body)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
