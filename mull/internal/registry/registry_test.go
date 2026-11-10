package registry

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"mull/internal/gittest"
	"mull/internal/mull"
)

func TestDirFollowsXDGConfigHome(t *testing.T) {
	x := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", x)
	if d, _ := Dir(); d != filepath.Join(x, "mull") {
		t.Errorf("Dir = %q, want under %q", d, x)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "relative/ignored")
	if d, _ := Dir(); d != filepath.Join(home, ".config", "mull") {
		t.Errorf("Dir with a relative XDG_CONFIG_HOME = %q, want ~/.config/mull", d)
	}
}

func TestRegisterOnlyReposOnMull(t *testing.T) {
	repo := gittest.NewRepo(t)
	if err := Register(repo); err != nil {
		t.Fatal(err)
	}
	if err := Register(t.TempDir()); err != nil { // not a repository
		t.Fatal(err)
	}
	if got, _ := Repos(); len(got) != 0 {
		t.Fatalf("registered %v before the repo had a mull branch", got)
	}

	m, err := mull.Open(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Init("", ""); err != nil {
		t.Fatal(err)
	}
	wt := gittest.AddWorktree(t, repo, "abcd-work")
	sub := filepath.Join(wt, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{repo, sub, repo} {
		if err := Register(dir); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Repos()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{repo}; !reflect.DeepEqual(got, want) {
		t.Errorf("Repos = %v, want %v (one entry, by the main worktree)", got, want)
	}
}

func TestReposSkipsDuplicatesBlanksAndComments(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got, err := Repos(); err != nil || len(got) != 0 {
		t.Fatalf("missing registry: %v, %v; want empty", got, err)
	}
	f, _ := File()
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte("/a\n\n# old\n/b\n/a\n  /c  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Add("/b"); err != nil {
		t.Fatal(err)
	}
	if err := Add("/d"); err != nil {
		t.Fatal(err)
	}
	got, _ := Repos()
	if want := []string{"/a", "/b", "/c", "/d"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Repos = %v, want %v", got, want)
	}
}
