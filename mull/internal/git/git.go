// Package git runs the git command line. mull talks to git only through
// this package, so every git invocation is visible in one place.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repo is a git working tree: the directory commands run in.
type Repo struct {
	// Dir is the top level of the working tree.
	Dir string
}

// Error is a failed git command, carrying its stderr.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		msg = e.Err.Error()
	}
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), msg)
}

func (e *Error) Unwrap() error { return e.Err }

// ErrNotRepo means the directory is not inside a git working tree.
var ErrNotRepo = errors.New("not inside a git working tree")

// Open finds the working tree containing dir.
func Open(dir string) (*Repo, error) {
	r := &Repo{Dir: dir}
	top, err := r.Output(nil, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, ErrNotRepo
	}
	return &Repo{Dir: top}, nil
}

// Run runs git with args and returns stdout as raw bytes. env entries
// (KEY=value) are added to the inherited environment.
func (r *Repo) Run(env []string, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, &Error{Args: args, Stderr: stderr.String(), Err: err}
	}
	return stdout.Bytes(), nil
}

// Output runs git and returns stdout with surrounding whitespace trimmed.
func (r *Repo) Output(env []string, stdin io.Reader, args ...string) (string, error) {
	out, err := r.Run(env, stdin, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RefCommit returns the commit a ref points at, or "" if the ref does not exist.
func (r *Repo) RefCommit(ref string) (string, error) {
	out, err := r.Output(nil, nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		var gerr *Error
		if errors.As(err, &gerr) {
			// --quiet exits 1 with no output when the ref is missing.
			if strings.TrimSpace(gerr.Stderr) == "" {
				return "", nil
			}
		}
		return "", err
	}
	return out, nil
}

// Branch returns the current branch name, or "HEAD" when detached.
func (r *Repo) Branch() string {
	out, err := r.Output(nil, nil, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "HEAD"
	}
	return out
}

// Worktree returns this working tree's absolute path.
func (r *Repo) Worktree() string {
	if abs, err := filepath.Abs(r.Dir); err == nil {
		return abs
	}
	return r.Dir
}
