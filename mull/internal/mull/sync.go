package mull

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"mull/internal/git"
	"mull/internal/model"
	"mull/internal/store"
)

// SyncResult reports what sync did (or, with dry run, would do).
type SyncResult struct {
	Remote       string     `json:"remote"`
	Action       string     `json:"action"` // up-to-date, pushed, pulled, merged, cloned
	Local        string     `json:"local"`  // the mull branch after sync
	RemoteBefore string     `json:"remote_before,omitempty"`
	Pushed       int        `json:"pushed_commits"`
	DryRun       bool       `json:"dry_run,omitempty"`
	Commands     [][]string `json:"commands,omitempty"` // with dry run: the git commands that would run
}

// ErrSyncConflict means the same records changed locally and on the remote.
type ErrSyncConflict struct{ Paths []string }

func (e *ErrSyncConflict) Error() string {
	return "the same records changed here and on the remote; nothing was changed. Conflicting: " + strings.Join(e.Paths, ", ")
}

const syncAttempts = 3

// Sync exchanges the mull branch with the remote. It is the only operation
// that pushes, and it can push only the mull branch: the destination is the
// constant store.Ref, the source is a commit sync has checked contains only
// mull records, and the push is never forced and never skips hooks.
func (m *Mull) Sync(dryRun bool) (SyncResult, error) {
	cfg, err := m.Config()
	if err != nil && !errors.Is(err, store.ErrNotInitialized) {
		return SyncResult{}, err
	}
	remote := cfg.Remote
	if remote == "" {
		remote = "origin"
	}
	var res SyncResult
	for attempt := 0; attempt < syncAttempts; attempt++ {
		res, err = m.syncOnce(remote, dryRun)
		if !errors.Is(err, errRetry) {
			return res, err
		}
	}
	return res, fmt.Errorf("sync: the mull branch kept moving; try again")
}

var errRetry = errors.New("retry")

func (m *Mull) syncOnce(remote string, dryRun bool) (SyncResult, error) {
	res := SyncResult{Remote: remote, DryRun: dryRun}
	plan := func(args ...string) { res.Commands = append(res.Commands, args) }

	local, err := m.repo.RefCommit(store.Ref)
	if err != nil {
		return res, err
	}
	remoteSha, err := m.remoteHead(remote)
	if err != nil {
		return res, err
	}
	res.RemoteBefore = remoteSha
	if local == "" && remoteSha == "" {
		return res, store.ErrNotInitialized
	}

	if remoteSha != "" && !m.hasCommit(remoteSha) {
		args := []string{"fetch", "--quiet", "--no-tags", remote, store.Ref}
		if _, err := m.repo.Run(nil, nil, args...); err != nil {
			return res, err
		}
		if !m.hasCommit(remoteSha) {
			return res, errRetry // the remote moved between ls-remote and fetch
		}
	}

	switch {
	case local == remoteSha:
		res.Action, res.Local = "up-to-date", local
		return res, nil

	case local == "":
		// A new machine or clone: adopt the remote's branch.
		res.Action, res.Local = "cloned", remoteSha
		plan("update-ref", store.Ref, remoteSha, "")
		if dryRun {
			return res, nil
		}
		return res, m.setRef(remoteSha, "")

	case remoteSha != "" && m.isAncestor(local, remoteSha):
		// Only the remote has new records: fast-forward.
		res.Action, res.Local = "pulled", remoteSha
		plan("update-ref", store.Ref, remoteSha, local)
		if dryRun {
			return res, nil
		}
		return res, m.setRef(remoteSha, local)
	}

	target := local
	res.Action = "pushed"
	if remoteSha != "" && !m.isAncestor(remoteSha, local) {
		// Both sides have new records: merge them.
		merged, err := m.mergeCommit(local, remoteSha)
		if err != nil {
			return res, err
		}
		target, res.Action = merged, "merged"
		plan("update-ref", store.Ref, merged, local)
	}

	count, err := m.checkOutgoing(target, remoteSha)
	if err != nil {
		return res, err
	}
	res.Pushed, res.Local = count, target
	pushArgs := []string{"push", "--quiet", remote, target + ":" + store.Ref}
	plan(pushArgs...)
	if dryRun {
		return res, nil
	}
	if target != local {
		if err := m.setRef(target, local); err != nil {
			return res, err
		}
	}
	if m.pushHook != nil {
		m.pushHook(pushArgs)
	}
	if _, err := m.repo.Run(nil, nil, pushArgs...); err != nil {
		var gerr *git.Error
		if errors.As(err, &gerr) && rejectedAsStale(gerr.Stderr) {
			return res, errRetry
		}
		return res, err
	}
	return res, nil
}

func (m *Mull) setRef(newCommit, old string) error {
	err := m.store.SetRef(newCommit, old)
	if errors.Is(err, store.ErrMoved) {
		return errRetry
	}
	return err
}

func rejectedAsStale(stderr string) bool {
	for _, s := range []string{"non-fast-forward", "fetch first", "stale info", "[rejected]"} {
		if strings.Contains(stderr, s) {
			return true
		}
	}
	return false
}

// remoteHead returns the remote's mull commit, or "" if it has none.
func (m *Mull) remoteHead(remote string) (string, error) {
	out, code, err := m.repo.RunStatus(nil, nil, "ls-remote", "--exit-code", remote, store.Ref)
	if err != nil {
		return "", err
	}
	switch code {
	case 0:
		fields := strings.Fields(string(out))
		if len(fields) < 1 {
			return "", fmt.Errorf("unexpected ls-remote output %q", out)
		}
		return fields[0], nil
	case 2: // the remote has no such ref
		return "", nil
	default:
		return "", fmt.Errorf("cannot reach remote %q (git ls-remote exited %d)", remote, code)
	}
}

func (m *Mull) hasCommit(sha string) bool {
	_, code, err := m.repo.RunStatus(nil, nil, "cat-file", "-e", sha+"^{commit}")
	return err == nil && code == 0
}

func (m *Mull) isAncestor(a, b string) bool {
	_, code, err := m.repo.RunStatus(nil, nil, "merge-base", "--is-ancestor", a, b)
	return err == nil && code == 0
}

// mergeCommit merges two mull histories without touching any working tree.
func (m *Mull) mergeCommit(local, remote string) (string, error) {
	out, code, err := m.repo.RunStatus(nil, nil, "merge-tree", "--write-tree", "--name-only", "--no-messages",
		"--allow-unrelated-histories", local, remote)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	switch code {
	case 0:
		tree := strings.TrimSpace(lines[0])
		return m.repo.Output(nil, nil, "commit-tree", tree, "-p", local, "-p", remote, "-m", "mull: sync merge")
	case 1:
		var paths []string
		for _, l := range lines[1:] {
			if l = strings.TrimSpace(l); l != "" {
				paths = append(paths, l)
			}
		}
		return "", &ErrSyncConflict{Paths: paths}
	default:
		return "", fmt.Errorf("git merge-tree failed (exit %d): %s", code, bytes.TrimSpace(out))
	}
}

// checkOutgoing verifies that every commit the push would send touches only
// mull's record paths, and returns how many commits that is.
func (m *Mull) checkOutgoing(target, remoteSha string) (int, error) {
	rangeArgs := []string{target}
	if remoteSha != "" {
		rangeArgs = append(rangeArgs, "--not", remoteSha)
	}
	countOut, err := m.repo.Output(nil, nil, append([]string{"rev-list", "--count"}, rangeArgs...)...)
	if err != nil {
		return 0, err
	}
	var count int
	fmt.Sscanf(countOut, "%d", &count)
	paths, err := m.repo.Output(nil, nil, append([]string{"log", "--format=", "--name-only", "--no-renames"}, rangeArgs...)...)
	if err != nil {
		return 0, err
	}
	var bad []string
	for _, p := range strings.Split(paths, "\n") {
		if p = strings.TrimSpace(p); p != "" && !model.ValidDataPath(p) {
			bad = append(bad, p)
		}
	}
	if len(bad) > 0 {
		return 0, fmt.Errorf("refusing to push: the mull branch has commits touching paths that are not mull records: %s", strings.Join(dedupe(bad), ", "))
	}
	return count, nil
}
