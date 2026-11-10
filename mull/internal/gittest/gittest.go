// Package gittest makes throwaway git repositories for tests, isolated from
// the user's global and system git configuration.
package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Isolate points git at no global or system config and sets a fixed identity.
// It also gives mull a fresh config directory, so the repo registry
// (internal/registry) never sees the user's own.
func Isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
}

// NewRepo creates a repository on branch main with one commit and returns its path.
func NewRepo(t *testing.T) string {
	t.Helper()
	Isolate(t)
	dir := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var) so paths compare equal to git's.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	Git(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	Git(t, dir, "add", "README")
	Git(t, dir, "commit", "-q", "-m", "first")
	return dir
}

// AddWorktree adds a worktree of repo on a new branch and returns its path.
func AddWorktree(t *testing.T, repo, branch string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), branch)
	Git(t, repo, "worktree", "add", "-q", "-b", branch, dir)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	return dir
}

// Commit writes file with content in dir, stages it and commits with the
// given message paragraphs (later paragraphs can carry trailers).
func Commit(t *testing.T, dir, file, content string, message ...string) {
	t.Helper()
	path := filepath.Join(dir, file)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	Git(t, dir, "add", file)
	args := []string{"commit", "-q"}
	for _, m := range message {
		args = append(args, "-m", m)
	}
	Git(t, dir, args...)
}

// Git runs git in dir and returns trimmed stdout, failing the test on error.
func Git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
