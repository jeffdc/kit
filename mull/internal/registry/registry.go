// Package registry is this machine's list of repositories on mull, so mull
// serve can find them with no configuration. Every mull command run in a
// repository with a mull branch adds it, unless the repository lives in the
// system's temporary directory (an agent's scratch repo, a test's). The list
// is a plain file, one absolute path per line, in mull's config directory; it
// is per machine and never on the mull branch.
package registry

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"mull/internal/confdir"
	"mull/internal/git"
	"mull/internal/store"
)

// Dir is mull's per-machine config directory: $XDG_CONFIG_HOME/mull, or
// ~/.config/mull (on macOS too).
func Dir() (string, error) { return confdir.Dir() }

// File is the registry's path.
func File() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "repos"), nil
}

// Register adds the repository containing dir, by its main worktree, if it
// has a mull branch. Outside a repository, without a mull branch, or when the
// main worktree is under a temporary directory (Temporary), it does nothing.
func Register(dir string) error {
	r, err := git.Open(dir)
	if err != nil {
		return nil
	}
	if c, err := r.RefCommit(store.Ref); err != nil || c == "" {
		return err
	}
	main := r.MainWorktree()
	if Temporary(main) {
		return nil
	}
	return Add(main)
}

// tempRoots lists the directories whose repositories are never registered on
// their own. Tests replace it with SetTempRoots.
var tempRoots = systemTempRoots

// systemTempRoots is the system's temporary directory (os.TempDir, so
// $TMPDIR), /tmp, /private/tmp and /var/folders (macOS's per-user temporary
// directories). A root that is / or holds the home directory (a $TMPDIR set
// oddly) is left out, so it can't stop every repository registering.
func systemTempRoots() []string {
	home, _ := os.UserHomeDir()
	var roots []string
	for _, d := range []string{os.TempDir(), "/tmp", "/private/tmp", "/var/folders"} {
		if !filepath.IsAbs(d) {
			continue
		}
		if isRoot(d) || (home != "" && under(home, d)) {
			continue
		}
		roots = append(roots, filepath.Clean(d))
	}
	return roots
}

// isRoot reports whether d is the filesystem root, as given or resolved.
func isRoot(d string) bool {
	for _, f := range forms(d) {
		if f == string(filepath.Separator) {
			return true
		}
	}
	return false
}

// SetTempRoots makes Register treat roots as the temporary directories, and
// returns a function that restores the system's. Tests use it to register
// their repositories, which live under the system's temporary directory; nil
// means no directory is temporary.
func SetTempRoots(roots []string) (restore func()) {
	old := tempRoots
	tempRoots = func() []string { return roots }
	return func() { tempRoots = old }
}

// Temporary reports whether path is inside one of the temporary directories
// (or is one).
func Temporary(path string) bool {
	for _, root := range tempRoots() {
		if under(path, root) {
			return true
		}
	}
	return false
}

// under reports whether path is root or inside it, comparing each as given
// (cleaned) and with symlinks resolved, so /tmp/x and /private/tmp/x both
// match /tmp and /private/tmp on macOS, and a path that doesn't exist yet
// still matches a root as written.
func under(path, root string) bool {
	for _, p := range forms(path) {
		for _, r := range forms(root) {
			if within(p, r) {
				return true
			}
		}
	}
	return false
}

// forms returns path cleaned and, when it differs, with symlinks resolved.
func forms(path string) []string {
	path = filepath.Clean(path)
	if r, err := filepath.EvalSymlinks(path); err == nil && r != path {
		return []string{path, r}
	}
	return []string{path}
}

// within reports whether path is root or under it.
func within(path, root string) bool {
	if path == root {
		return true
	}
	if !strings.HasSuffix(root, string(filepath.Separator)) {
		root += string(filepath.Separator)
	}
	return strings.HasPrefix(path, root)
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

// Remove takes paths out of the registry, every line naming one of them,
// and reports whether any was there. Other lines (comments, blank lines) are
// kept as they are. The file is replaced atomically, so mull serve, which
// watches its directory, rereads it once. An Add racing with Remove can be
// lost; running the command again adds it.
func Remove(paths ...string) (bool, error) {
	f, err := File()
	if err != nil {
		return false, err
	}
	b, err := os.ReadFile(f)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	drop := map[string]bool{}
	for _, p := range paths {
		drop[p] = true
	}
	var kept []string
	removed := false
	for _, line := range strings.SplitAfter(string(b), "\n") {
		if line == "" {
			continue
		}
		if drop[strings.TrimSpace(line)] {
			removed = true
			continue
		}
		kept = append(kept, line)
	}
	if !removed {
		return false, nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(f), ".repos-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp.Name()) // gone already once renamed
	if _, err := tmp.WriteString(strings.Join(kept, "")); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return false, err
	}
	return true, os.Rename(tmp.Name(), f)
}
