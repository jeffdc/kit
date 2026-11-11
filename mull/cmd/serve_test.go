package cmd

import (
	"net"
	"os"
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

func TestServePortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	run{t, t.TempDir()}.fails("port_in_use", "serve", "--port", port)
}
