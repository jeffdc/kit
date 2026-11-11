package serve

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"mull/internal/git"
	"mull/internal/store"
)

// Live updates. A Watcher watches every registered repository's mull branch
// and the registry itself, and tells the open pages (GET /api/events, in
// events.go) which project changed, so they refetch.
//
// Git updates a loose ref by writing <ref>.lock and renaming it over the
// ref, and packs refs into packed-refs the same way, so the watches are on
// directories, never on the files: the ref's directory (refs/heads) and the
// git common dir (packed-refs), both shared by every worktree. A burst of
// file events is debounced, then each touched repository's mull branch is
// read again, and only one whose tip actually moved is announced.

const (
	// debounce is how long a burst of file events is gathered before the
	// touched repositories are read again: one mull write is a few events
	// within milliseconds.
	debounce = 150 * time.Millisecond
	// keepalive is how often an idle event stream gets a comment, so a
	// client that went away is noticed and nothing in between times it out.
	keepalive = 25 * time.Second
)

// Watcher watches the registered repositories' mull branches and serves
// GET /api/events (see WithEvents). Start it with Watch; Close stops it.
type Watcher struct {
	repos     Repos
	registry  string // the registry file; its directory is watched
	fs        *fsnotify.Watcher
	debounce  time.Duration
	keepalive time.Duration

	// Owned by the run loop.
	ids     map[string]string     // registered path -> project ID, as last read
	watched map[string]*repoWatch // by registered path
	dirs    map[string]string     // watched directory -> registered path

	mu     sync.Mutex
	subs   map[*subscriber]bool
	closed bool

	done    chan struct{}
	stopped chan struct{} // closed when the run loop returns, if it started
	running bool
	once    sync.Once
}

type repoWatch struct {
	dirs   []string
	commit string // the mull branch's tip when last read; "" for none
}

// Watch watches every repository repos lists, and the registry file
// registry for repositories added or removed while it runs. It creates the
// registry's directory if there is none yet, so there is something to watch.
func Watch(repos Repos, registry string) (*Watcher, error) {
	w, err := newWatcher(repos, registry)
	if err != nil {
		return nil, err
	}
	w.running = true
	go w.run()
	return w, nil
}

// newWatcher sets up the watches without starting the run loop.
func newWatcher(repos Repos, registry string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(registry)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fsw.Close()
		return nil, err
	}
	if err := fsw.Add(dir); err != nil {
		fsw.Close()
		return nil, err
	}
	w := &Watcher{
		repos: repos, registry: registry, fs: fsw,
		debounce: debounce, keepalive: keepalive,
		ids: map[string]string{}, watched: map[string]*repoWatch{}, dirs: map[string]string{},
		subs: map[*subscriber]bool{},
		done: make(chan struct{}), stopped: make(chan struct{}),
	}
	w.sync(false)
	return w, nil
}

// Close stops watching and ends every open event stream. It is safe to call
// more than once.
func (w *Watcher) Close() {
	w.once.Do(func() {
		close(w.done)
		w.fs.Close()
		if w.running {
			<-w.stopped
		}
		w.mu.Lock()
		w.closed = true
		for s := range w.subs {
			close(s.wake)
			delete(w.subs, s)
		}
		w.mu.Unlock()
	})
}

// run gathers file events into bursts and flushes each one debounce after
// its first event, so a steady stream of writes still refreshes the page.
func (w *Watcher) run() {
	defer close(w.stopped)
	dirty, registry := map[string]bool{}, false
	var fire <-chan time.Time
	arm := func() {
		if fire == nil {
			fire = time.After(w.debounce)
		}
	}
	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.fs.Events:
			if !ok {
				return
			}
			if w.mark(ev, dirty, &registry) {
				arm()
			}
		case _, ok := <-w.fs.Errors:
			if !ok {
				return
			}
			// Events were lost (an overflow): read everything again.
			for p := range w.watched {
				dirty[p] = true
			}
			registry = true
			arm()
		case <-fire:
			fire = nil
			w.flush(dirty, registry)
			dirty, registry = map[string]bool{}, false
		}
	}
}

// mark notes what a file event touches: the registry, or a repository's
// mull branch. It reports whether the event matters at all.
func (w *Watcher) mark(ev fsnotify.Event, dirty map[string]bool, registry *bool) bool {
	dir, name := filepath.Dir(ev.Name), filepath.Base(ev.Name)
	if dir == filepath.Dir(w.registry) && name == filepath.Base(w.registry) {
		*registry = true
		return true
	}
	repo, ok := w.dirs[dir]
	if !ok {
		return false
	}
	switch strings.TrimSuffix(name, ".lock") {
	case path.Base(store.Ref), "packed-refs":
	default:
		if filepath.Base(dir) != "reftable" {
			return false
		}
	}
	dirty[repo] = true
	return true
}

// flush rereads the registry if it changed, and the mull branch of every
// repository touched, and announces each project whose branch moved.
func (w *Watcher) flush(dirty map[string]bool, registry bool) {
	if registry {
		w.sync(true)
	}
	for p := range dirty {
		rw, ok := w.watched[p]
		if !ok {
			continue
		}
		commit, err := mullTip(p)
		if err != nil || commit == rw.commit {
			continue
		}
		rw.commit = commit
		w.publish(w.ids[p])
	}
}

// sync makes the watches match the registry. With announce, every project
// added, removed or renamed (IDs shift when an earlier one goes) is
// announced, under its old ID and its new one. A repository that can't be
// watched yet (moved away, say) is tried again on the next registry change.
func (w *Watcher) sync(announce bool) {
	paths, err := w.repos()
	if err != nil {
		return
	}
	ids := map[string]string{}
	for i, id := range projectIDs(paths) {
		ids[paths[i]] = id
	}
	for p, old := range w.ids {
		if id, ok := ids[p]; !ok || id != old {
			if announce {
				w.publish(old)
			}
			if !ok {
				w.unwatch(p)
			}
		}
	}
	for _, p := range paths {
		if old, ok := w.ids[p]; announce && (!ok || old != ids[p]) {
			w.publish(ids[p])
		}
		if w.watched[p] == nil {
			w.watch(p)
		}
	}
	w.ids = ids
}

// watch adds the directories where repository p's mull branch is stored.
func (w *Watcher) watch(p string) {
	r, err := git.Open(p)
	if err != nil {
		return
	}
	common, err := r.Output(nil, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return
	}
	commit, err := r.RefCommit(store.Ref)
	if err != nil {
		return
	}
	dirs := []string{common, filepath.Join(common, filepath.FromSlash(path.Dir(store.Ref)))}
	if fi, err := os.Stat(filepath.Join(common, "reftable")); err == nil && fi.IsDir() {
		dirs = append(dirs, filepath.Join(common, "reftable"))
	}
	rw := &repoWatch{commit: commit}
	for _, d := range dirs {
		if err := w.fs.Add(d); err != nil {
			continue // refs/heads can be missing in a repo with no branches yet
		}
		rw.dirs = append(rw.dirs, d)
		w.dirs[d] = p
	}
	w.watched[p] = rw
}

func (w *Watcher) unwatch(p string) {
	rw := w.watched[p]
	if rw == nil {
		return
	}
	for _, d := range rw.dirs {
		_ = w.fs.Remove(d)
		delete(w.dirs, d)
	}
	delete(w.watched, p)
}

// mullTip is the commit at the tip of repository p's mull branch, "" if it
// has none.
func mullTip(p string) (string, error) {
	r, err := git.Open(p)
	if err != nil {
		return "", err
	}
	return r.RefCommit(store.Ref)
}
