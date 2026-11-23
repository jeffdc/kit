package registry

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"mull/internal/core"
	"mull/internal/gittest"
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

// registerTemp lets the test's repositories, all under the system's
// temporary directory, register, as a repository anywhere else would.
func registerTemp(t *testing.T) {
	t.Cleanup(SetTempRoots(nil))
}

func TestRegisterOnlyReposOnMull(t *testing.T) {
	registerTemp(t)
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

	m, err := core.Open(repo)
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

func TestReposShareOneRegistryPerTest(t *testing.T) {
	// Register each repo as soon as it's made: a second repo must not move
	// the test to a new config dir and lose the first registration.
	registerTemp(t)
	a := onMull(t)
	if err := Register(a); err != nil {
		t.Fatal(err)
	}
	b := onMull(t)
	if err := Register(b); err != nil {
		t.Fatal(err)
	}
	if got, _ := Repos(); !reflect.DeepEqual(got, []string{a, b}) {
		t.Errorf("Repos = %v, want both repos", got)
	}
}

func TestRegisterSkipsTemporaryRepos(t *testing.T) {
	repo := onMull(t) // under the system's temporary directory
	wt := gittest.AddWorktree(t, repo, "abcd-work")
	for _, dir := range []string{repo, wt} {
		if err := Register(dir); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := Repos(); len(got) != 0 {
		t.Fatalf("registered %v from the temporary directory", got)
	}
	// Explicit registration isn't affected.
	if err := Add(repo); err != nil {
		t.Fatal(err)
	}
	if got, _ := Repos(); !reflect.DeepEqual(got, []string{repo}) {
		t.Errorf("Repos = %v, want %v added explicitly", got, repo)
	}
}

func TestSystemTempRoots(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("HOME", "/Users/someone") // not under any temporary directory
	resolved, _ := filepath.EvalSymlinks(tmp)
	for _, p := range []string{tmp, resolved, filepath.Join(tmp, "a", "b"), "/tmp", "/tmp/x", "/private/tmp/x", "/var/folders/ab/x"} {
		if !Temporary(p) {
			t.Errorf("Temporary(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"/Users/someone/dev/kit", "/tmpfoo/x", "/var/lib/x", "/"} {
		if Temporary(p) {
			t.Errorf("Temporary(%q) = true, want false", p)
		}
	}

	// A $TMPDIR holding the home directory doesn't make every repo temporary.
	home := filepath.Join(tmp, "home")
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	if Temporary(filepath.Join(home, "dev", "kit")) {
		t.Errorf("a $TMPDIR above the home directory counted as temporary")
	}
	t.Setenv("TMPDIR", "/")
	if Temporary("/opt/repo") {
		t.Errorf("a $TMPDIR of / counted everything as temporary")
	}
}

func TestRemove(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if removed, err := Remove("/a"); err != nil || removed {
		t.Fatalf("Remove from a missing registry = %v, %v; want false, nil", removed, err)
	}
	f, _ := File()
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte("/a\n# kept\n\n/b\n  /a  \n/c"), 0o644); err != nil {
		t.Fatal(err)
	}
	if removed, err := Remove("/x"); err != nil || removed {
		t.Fatalf("Remove(/x) = %v, %v; want false, nil", removed, err)
	}
	if removed, err := Remove("/a", "/c"); err != nil || !removed {
		t.Fatalf("Remove(/a, /c) = %v, %v; want true, nil", removed, err)
	}
	b, _ := os.ReadFile(f)
	if want := "# kept\n\n/b\n"; string(b) != want {
		t.Errorf("registry = %q, want %q", b, want)
	}
	if fi, _ := os.Stat(f); fi.Mode().Perm() != 0o644 {
		t.Errorf("registry mode = %v, want 0644", fi.Mode().Perm())
	}
	if err := Add("/d"); err != nil {
		t.Fatal(err)
	}
	if got, _ := Repos(); !reflect.DeepEqual(got, []string{"/b", "/d"}) {
		t.Errorf("Repos = %v, want [/b /d]", got)
	}
	entries, _ := os.ReadDir(filepath.Dir(f))
	if len(entries) != 1 {
		t.Errorf("config dir holds %d files, want only the registry (no temp file left)", len(entries))
	}
}

func onMull(t *testing.T) string {
	t.Helper()
	dir := gittest.NewRepo(t)
	m, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Init("", ""); err != nil {
		t.Fatal(err)
	}
	return dir
}
