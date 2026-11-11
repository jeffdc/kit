// Package registry is this machine's list of repositories on mull, so mull
// serve can find them with no configuration. Every mull command run in a
// repository with a mull branch adds it. The list is a plain file, one
// absolute path per line, in mull's config directory; it is per machine and
// never on the mull branch.
package registry

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"mull/internal/git"
	"mull/internal/store"
)

// Dir is mull's per-machine config directory: $XDG_CONFIG_HOME/mull, or
// ~/.config/mull (on macOS too).
func Dir() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(x) {
		return filepath.Join(x, "mull"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "mull"), nil
}

// File is the registry's path.
func File() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "repos"), nil
}

// Register adds the repository containing dir, by its main worktree, if it
// has a mull branch. Outside a repository, or without a mull branch, it does
// nothing.
func Register(dir string) error {
	r, err := git.Open(dir)
	if err != nil {
		return nil
	}
	if c, err := r.RefCommit(store.Ref); err != nil || c == "" {
		return err
	}
	return Add(r.MainWorktree())
}

// Add adds path to the registry unless it is already there. Two processes
// adding the same path at once can write it twice; Repos reads one.
func Add(path string) error {
	f, err := File()
	if err != nil {
		return err
	}
	repos, err := Repos()
	if err != nil {
		return err
	}
	for _, r := range repos {
		if r == path {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(f, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := out.WriteString(path + "\n"); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Repos returns the registered paths in the order they were added, each
// once. Blank lines and lines starting with # are skipped, so the file can
// be edited by hand. A missing registry is empty.
func Repos() ([]string, error) {
	f, err := File()
	if err != nil {
		return nil, err
	}
	in, err := os.Open(f)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer in.Close()
	repos, seen := []string{}, map[string]bool{}
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || seen[line] {
			continue
		}
		seen[line] = true
		repos = append(repos, line)
	}
	return repos, sc.Err()
}
