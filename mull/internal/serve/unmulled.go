package serve

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"mull/internal/git"
	"mull/internal/mull"
)

// Not on mull yet: the repositories beside the registered ones that aren't
// on mull. They are looked for in the directories the registered
// repositories live in, one level down, so there's nothing to configure.
// A repository with old mull data (a .mull/ directory) is always listed, as
// needing converting; any other is listed only if it has had a commit
// recently. The scan reads the directories and runs git in them; it never
// writes or watches them, and it's kept for a few minutes.

const (
	// recently is how far back a commit makes a repository worth listing.
	recently = 60 * 24 * time.Hour
	// rescanAfter is how long a scan is kept before the next request runs
	// another.
	rescanAfter = 5 * time.Minute
)

// Unmulled kinds.
const (
	KindOldMull   = "old_mull"    // has old .mull/ data: needs converting
	KindNotOnMull = "not_on_mull" // never on mull, worked on recently
)

// UnmulledList is GET /api/unmulled.
type UnmulledList struct {
	Roots     []string   `json:"roots"` // the directories looked in
	Repos     []Unmulled `json:"repos"` // old mull first, then not on mull, each by name
	ScannedAt string     `json:"scanned_at"`
}

// Unmulled is one repository that isn't on mull.
type Unmulled struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	LastCommit string `json:"last_commit,omitempty"` // the newest commit on any ref
}

// scanner keeps the last scan, and scans again once it's old or the
// directories to look in have changed.
type scanner struct {
	now   func() time.Time
	mu    sync.Mutex
	roots []string
	at    time.Time
	list  UnmulledList
}

func (s *scanner) get(paths []string) UnmulledList {
	roots := rootsOf(paths)
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.at.IsZero() || now.Sub(s.at) >= rescanAfter || !slices.Equal(roots, s.roots) {
		s.list = scan(roots, now)
		s.roots, s.at = roots, now
	}
	return s.list
}

func registerUnmulled(mux *http.ServeMux, repos Repos, s *scanner) {
	mux.HandleFunc("GET /api/unmulled", func(w http.ResponseWriter, r *http.Request) {
		paths, err := repos()
		if err != nil {
			writeError(w, http.StatusInternalServerError, mull.AsError(err))
			return
		}
		writeJSON(w, http.StatusOK, s.get(paths))
	})
}

// rootsOf is the directories the registered repositories live in, sorted.
func rootsOf(paths []string) []string {
	seen := map[string]bool{}
	roots := []string{}
	for _, p := range paths {
		dir := filepath.Dir(filepath.Clean(p))
		if !seen[dir] {
			seen[dir] = true
			roots = append(roots, dir)
		}
	}
	sort.Strings(roots)
	return roots
}

// scan looks one level down in each root for git repositories that aren't
// on mull, concurrently.
func scan(roots []string, now time.Time) UnmulledList {
	var candidates []string
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue // a root that's gone or unreadable lists nothing
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			path := filepath.Join(root, e.Name())
			if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
				candidates = append(candidates, path)
			}
		}
	}
	found := make([]*Unmulled, len(candidates))
	var wg sync.WaitGroup
	for i, path := range candidates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			found[i] = check(path, now)
		}()
	}
	wg.Wait()
	list := UnmulledList{Roots: roots, Repos: []Unmulled{}, ScannedAt: now.UTC().Format(time.RFC3339)}
	for _, u := range found {
		if u != nil {
			list.Repos = append(list.Repos, *u)
		}
	}
	sort.Slice(list.Repos, func(a, b int) bool {
		x, y := list.Repos[a], list.Repos[b]
		if x.Kind != y.Kind {
			return x.Kind == KindOldMull
		}
		return strings.ToLower(x.Name) < strings.ToLower(y.Name)
	})
	return list
}

// check says whether the repository at path belongs in the list: not on
// mull, not a linked worktree of another, and either holding old mull data
// or committed to recently.
func check(path string, now time.Time) *Unmulled {
	r := &git.Repo{Dir: path}
	if r.MainWorktree() != path {
		return nil // a linked worktree: its main worktree speaks for it
	}
	if c, err := r.RefCommit("refs/heads/mull"); err != nil || c != "" {
		return nil // on mull (registered or not), or unreadable
	}
	u := &Unmulled{Name: filepath.Base(path), Path: path}
	if out, err := r.Output(nil, nil, "for-each-ref", "--sort=-committerdate", "--count=1", "--format=%(committerdate:iso-strict)"); err == nil && out != "" {
		if t, err := time.Parse(time.RFC3339, out); err == nil {
			u.LastCommit = t.UTC().Format(time.RFC3339)
			if now.Sub(t) <= recently {
				u.Kind = KindNotOnMull
			}
		}
	}
	if fi, err := os.Stat(filepath.Join(path, ".mull")); err == nil && fi.IsDir() {
		u.Kind = KindOldMull
	}
	if u.Kind == "" {
		return nil
	}
	return u
}
