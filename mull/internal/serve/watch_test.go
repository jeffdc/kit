package serve

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"mull/internal/gittest"
	"mull/internal/mull"
	"mull/internal/registry"
)

// wait bounds every wait for something the watcher must do; tests never
// sleep to prove something doesn't happen.
const wait = 10 * time.Second

// watching starts a Watcher on the test's registry. gittest isolates the
// registry per test, afresh each time: make the repositories first.
func watching(t *testing.T) *Watcher {
	t.Helper()
	file, err := registry.File()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(file, os.TempDir()) {
		t.Fatalf("registry %s isn't the test's own: call gittest first", file)
	}
	w, err := Watch(registry.Repos, file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)
	return w
}

// capture writes to the mull branch of the repository dir is in, as any
// mull command would.
func capture(t *testing.T, dir, title string) {
	t.Helper()
	m, err := mull.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.CaptureItem(mull.NewCapture{Kind: "idea", Title: title}); err != nil {
		t.Fatal(err)
	}
}

// announced waits until s has been told about every one of ids.
func announced(t *testing.T, w *Watcher, s *subscriber, ids ...string) {
	t.Helper()
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	timeout := time.After(wait)
	for len(want) > 0 {
		select {
		case <-s.wake:
			for _, id := range w.take(s) {
				delete(want, id)
			}
		case <-timeout:
			t.Fatalf("not announced: %v", want)
		}
	}
}

func register(t *testing.T, dir string) {
	t.Helper()
	if err := registry.Add(dir); err != nil {
		t.Fatal(err)
	}
}

func TestWatchMullWrites(t *testing.T) {
	dir := onMull(t)
	register(t, dir)
	w := watching(t)
	s := w.subscribe()
	id := filepath.Base(dir)

	capture(t, dir, "from the main worktree")
	announced(t, w, s, id)

	// Linked worktrees share the git common dir, and so the ref.
	wt := gittest.AddWorktree(t, dir, "abcd-work")
	capture(t, wt, "from a linked worktree")
	announced(t, w, s, id)

	// packed-refs: once the ref is packed, deleting it touches only that file.
	gittest.Git(t, dir, "pack-refs", "--all")
	if _, err := os.Stat(filepath.Join(dir, ".git", "refs", "heads", "mull")); !os.IsNotExist(err) {
		t.Fatalf("mull ref still loose after pack-refs: %v", err)
	}
	gittest.Git(t, dir, "update-ref", "-d", "refs/heads/mull")
	announced(t, w, s, id)
}

func TestWatchRegistry(t *testing.T) {
	dir := onMull(t)
	id := filepath.Base(dir)
	w := watching(t) // before the registry exists
	s := w.subscribe()
	register(t, dir)
	announced(t, w, s, id) // added while running
	capture(t, dir, "now watched")
	announced(t, w, s, id)

	file, _ := registry.File()
	if err := os.WriteFile(file, []byte("# by hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	announced(t, w, s, id) // removed
}

// TestFlushOnlyWhenTheBranchMoves drives the watcher by hand, with no run
// loop: a file event in a watched directory announces a project only if its
// mull branch moved.
func TestFlushOnlyWhenTheBranchMoves(t *testing.T) {
	dir := onMull(t)
	register(t, dir)
	file, _ := registry.File()
	w, err := newWatcher(registry.Repos, file)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s := w.subscribe()
	id := filepath.Base(dir)

	common := filepath.Join(dir, ".git")
	heads := filepath.Join(common, "refs", "heads")
	for name, want := range map[string]bool{
		filepath.Join(heads, "mull"): true, filepath.Join(heads, "mull.lock"): true,
		filepath.Join(common, "packed-refs"): true, filepath.Join(common, "packed-refs.lock"): true,
		filepath.Join(heads, "main"): false, filepath.Join(common, "index"): false, filepath.Join(common, "HEAD"): false,
	} {
		dirty, reg := map[string]bool{}, false
		got := w.mark(fsnotify.Event{Name: name, Op: fsnotify.Create}, dirty, &reg)
		if got != want || dirty[dir] != want || reg {
			t.Errorf("mark(%s) = %v, dirty %v, registry %v; want %v", name, got, dirty, reg, want)
		}
	}
	dirty, reg := map[string]bool{}, false
	if !w.mark(fsnotify.Event{Name: file, Op: fsnotify.Write}, dirty, &reg) || !reg || len(dirty) != 0 {
		t.Errorf("mark(registry): registry %v, dirty %v", reg, dirty)
	}

	gittest.Commit(t, dir, "x", "x", "another branch moves")
	w.flush(map[string]bool{dir: true}, false)
	capture(t, dir, "the mull branch moves")
	w.flush(map[string]bool{dir: true}, false)
	w.flush(map[string]bool{dir: true}, false) // no further change
	if got := w.take(s); !reflect.DeepEqual(got, []string{id}) {
		t.Errorf("announced %v, want [%s] once", got, id)
	}
	if got := w.take(s); len(got) != 0 {
		t.Errorf("announced again: %v", got)
	}
}

// stream opens GET /api/events on srv and returns its lines as they come;
// the channel closes when the stream ends.
func stream(t *testing.T, srv *httptest.Server) <-chan string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/events", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("GET /api/events: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	lines := make(chan string, 100)
	go func() {
		defer close(lines)
		defer res.Body.Close()
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	return lines
}

// expect reads lines until it has seen want, in order, skipping others.
func expect(t *testing.T, lines <-chan string, want ...string) {
	t.Helper()
	timeout := time.After(wait)
	for len(want) > 0 {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatalf("stream ended; still expecting %q", want)
			}
			if line == want[0] {
				want = want[1:]
			}
		case <-timeout:
			t.Fatalf("still expecting %q", want)
		}
	}
}

func TestEventsEndToEnd(t *testing.T) {
	dir := onMull(t)
	register(t, dir)
	w := watching(t)
	srv := httptest.NewServer(Handler(registry.Repos, WithEvents(w)))
	t.Cleanup(srv.Close) // after the stream's client goes (cleanups run last first)

	lines := stream(t, srv)
	expect(t, lines, ": mull serve", "") // subscribed: the opening comment is sent after
	capture(t, dir, "seen live")
	expect(t, lines, "event: project", `data: {"id":"`+filepath.Base(dir)+`"}`, "")
}

func TestEventsStream(t *testing.T) {
	gittest.Isolate(t) // first: the registry file is under the test's config dir
	file, _ := registry.File()
	w, err := newWatcher(func() ([]string, error) { return []string{}, nil }, file)
	if err != nil {
		t.Fatal(err)
	}
	w.keepalive = 10 * time.Millisecond
	srv := httptest.NewServer(Handler(func() ([]string, error) { return []string{}, nil }, WithEvents(w)))
	t.Cleanup(srv.Close)

	lines := stream(t, srv)
	expect(t, lines, ": mull serve", "", ": keepalive", "")
	w.publish("kit")
	w.publish("forage")
	w.publish("kit") // collapses into the pending one, or comes after
	expect(t, lines, "event: project", `data: {"id":"forage"}`, "")

	// Closing the watcher (as shutdown does) ends the stream, and refuses new ones.
	w.Close()
	timeout := time.After(wait)
	for open := true; open; {
		select {
		case _, open = <-lines:
		case <-timeout:
			t.Fatal("stream still open after Close")
		}
	}
	rec := get(t, Handler(func() ([]string, error) { return []string{}, nil }, WithEvents(w)), "/api/events")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /api/events after Close: %d", rec.Code)
	}
}

func TestEventsClientGone(t *testing.T) {
	gittest.Isolate(t) // first: the registry file is under the test's config dir
	file, _ := registry.File()
	w, err := newWatcher(func() ([]string, error) { return []string{}, nil }, file)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the client is already gone
	req := httptest.NewRequest("GET", "/api/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	w.ServeHTTP(rec, req) // returns, rather than streaming forever
	if !strings.HasPrefix(rec.Body.String(), ": mull serve\n\n") || len(w.subs) != 0 {
		t.Errorf("after the client went: body %q, %d subscribers", rec.Body, len(w.subs))
	}
}

func TestNoEventsWithoutWatcher(t *testing.T) {
	h := Handler(func() ([]string, error) { return []string{}, nil })
	if rec := get(t, h, "/api/events"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /api/events with no watcher: %d", rec.Code)
	}
}
