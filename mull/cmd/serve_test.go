package cmd

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"mull/internal/gittest"
	"mull/internal/registry"
)

// TestMain keeps every test off the user's own registry, even one that runs
// a command outside gittest (which isolates it per test too).
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "mull-cmd-config")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestCommandsRegisterTheirRepo(t *testing.T) {
	t.Cleanup(registry.SetTempRoots(nil)) // as if the test's repos weren't temporary
	dir := gittest.NewRepo(t)
	r := run{t, dir}
	r.fails("not_initialized", "view") // no mull branch: not registered
	if got, _ := registry.Repos(); len(got) != 0 {
		t.Fatalf("registered %v without a mull branch", got)
	}
	r.ok("init")
	wt := gittest.AddWorktree(t, dir, "abcd-work")
	run{t, wt}.ok("view")
	if got, _ := registry.Repos(); !reflect.DeepEqual(got, []string{dir}) {
		t.Errorf("registry = %v, want [%s] (once, by the main worktree)", got, dir)
	}
}

func TestCommandsNeverRegisterTemporaryRepos(t *testing.T) {
	dir := gittest.NewRepo(t) // under the system's temporary directory
	r := run{t, dir}
	r.ok("init")
	r.ok("view")
	run{t, gittest.AddWorktree(t, dir, "abcd-work")}.ok("view")
	if got, _ := registry.Repos(); len(got) != 0 {
		t.Errorf("registry = %v, want nothing from the temporary directory", got)
	}
}

func TestUnregister(t *testing.T) {
	t.Cleanup(registry.SetTempRoots(nil))
	a, b := gittest.NewRepo(t), gittest.NewRepo(t)
	run{t, a}.ok("init")
	run{t, b}.ok("init")
	if got, _ := registry.Repos(); !reflect.DeepEqual(got, []string{a, b}) {
		t.Fatalf("registry = %v, want [%s %s]", got, a, b)
	}
	file, _ := registry.File()

	// Default: the repository containing the current directory, from a
	// linked worktree, and it isn't registered again on the way out.
	wt := gittest.AddWorktree(t, a, "abcd-work")
	out := run{t, wt}.ok("unregister")
	if out["path"] != a || out["registry"] != file || out["removed"] != true {
		t.Errorf("unregister = %+v, want path %s, registry %s, removed", out, a, file)
	}
	if got, _ := registry.Repos(); !reflect.DeepEqual(got, []string{b}) {
		t.Errorf("registry = %v, want [%s]", got, b)
	}
	if again := (run{t, wt}).ok("unregister"); again["removed"] != false {
		t.Errorf("unregister again: removed = true, want false")
	}

	// A path, relative to the current directory; then a registered path
	// that's no longer a repository, removed as given.
	rel, err := filepath.Rel(a, b)
	if err != nil {
		t.Fatal(err)
	}
	out = run{t, a}.ok("unregister", rel)
	if out["path"] != b || out["removed"] != true {
		t.Errorf("unregister %s = %+v, want %s removed", rel, out, b)
	}
	gone := filepath.Join(b, "gone")
	if err := registry.Add(gone); err != nil {
		t.Fatal(err)
	}
	out = run{t, a}.ok("unregister", gone)
	if out["path"] != gone || out["removed"] != true {
		t.Errorf("unregister %s = %+v, want it removed as given", gone, out)
	}
	if got, _ := registry.Repos(); len(got) != 0 {
		t.Errorf("registry = %v, want empty", got)
	}

	// Any other command registers it again.
	run{t, a}.ok("view")
	if got, _ := registry.Repos(); !reflect.DeepEqual(got, []string{a}) {
		t.Errorf("registry = %v, want [%s] after a command in it", got, a)
	}
}

func TestServePortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	run{t, t.TempDir()}.fails("port_in_use", "serve", "--port", port)
}
