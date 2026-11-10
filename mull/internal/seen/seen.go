// Package seen is mull serve's last-looked state: for each repository, the
// mull branch commit the human last marked seen, and when. What changed
// since then is clouded in the UI. It is per machine, kept in mull's config
// directory beside the registry, and never on the mull branch.
package seen

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"mull/internal/registry"
)

// Look is the last time the human looked at a repository: the mull branch
// commit they saw, and when they marked it seen (or when mull serve first
// read the repository, for the baseline).
type Look struct {
	Commit string `json:"commit"`
	At     string `json:"at"`
}

// file is the state file's content.
type file struct {
	Repos map[string]Look `json:"repos"` // by the repository's main worktree path, as registered
}

// mu serializes this process's read-modify-write of the file. Writes are
// atomic (a temporary file renamed over it), so a reader never sees half a
// file.
var mu sync.Mutex

// File is the state file's path.
func File() (string, error) {
	dir, err := registry.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "seen.json"), nil
}

// Get returns repo's last look; ok is false if it has none.
func Get(repo string) (l Look, ok bool, err error) {
	mu.Lock()
	defer mu.Unlock()
	f, err := read()
	if err != nil {
		return Look{}, false, err
	}
	l, ok = f.Repos[repo]
	return l, ok, nil
}

// Mark records l as repo's last look.
func Mark(repo string, l Look) error {
	mu.Lock()
	defer mu.Unlock()
	f, err := read()
	if err != nil {
		return err
	}
	f.Repos[repo] = l
	return write(f)
}

// Baseline records l as repo's last look if it has none, or if its look is
// still the stale one the caller found ("" for none). Otherwise someone
// marked it meanwhile, and that look is kept. It returns the look in effect.
func Baseline(repo string, l Look, stale string) (Look, error) {
	mu.Lock()
	defer mu.Unlock()
	f, err := read()
	if err != nil {
		return Look{}, err
	}
	if cur, ok := f.Repos[repo]; ok && cur.Commit != stale {
		return cur, nil
	}
	f.Repos[repo] = l
	return l, write(f)
}

func read() (*file, error) {
	path, err := File()
	if err != nil {
		return nil, err
	}
	f := &file{}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(b, f); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	if f.Repos == nil {
		f.Repos = map[string]Look{}
	}
	return f, nil
}

// write replaces the file atomically: a temporary file in the same
// directory, renamed over it.
func write(f *file) error {
	path, err := File()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".seen-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // gone already once renamed
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
