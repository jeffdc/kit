package git

import (
	"os/exec"
	"strings"
)

type RepoContext struct {
	Branch        string
	DirtyFiles    []string
	RecentCommits []string
}

// Context gathers the current git state for a repo.
func Context(dir string) (*RepoContext, error) {
	run := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}

	branch, err := run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}

	statusOut, err := run("status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var dirty []string
	if statusOut != "" {
		dirty = strings.Split(statusOut, "\n")
	}

	logOut, err := run("log", "--oneline", "-5")
	if err != nil {
		return nil, err
	}
	var commits []string
	if logOut != "" {
		commits = strings.Split(logOut, "\n")
	}

	return &RepoContext{
		Branch:        branch,
		DirtyFiles:    dirty,
		RecentCommits: commits,
	}, nil
}
