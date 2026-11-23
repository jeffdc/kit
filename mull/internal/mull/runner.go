package mull

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"mull/internal/git"
	"mull/internal/model"
	"mull/internal/store"
)

// The runner owns branches and worktrees, so neither the human nor agents
// manage them. start makes an item's branch from its target (making the
// target from the release branch if it's missing) and a worktree for it, and
// runs the project's setup there; done fast-forwards the target to work that
// landed; and worktrees whose work is all on a target are pruned. The runner
// writes local code branches only: it never pushes, and never writes the
// release branch.

// parallel is how many items may be active at once.
func (s *state) parallel() int {
	if s.config.Parallel > 0 {
		return s.config.Parallel
	}
	return model.DefaultParallel
}

// activeCount counts active items other than skip.
func (s *state) activeCount(skip string) int {
	n := 0
	for id, i := range s.items {
		if id != skip && i.State == model.StateActive {
			n++
		}
	}
	return n
}

// StartWorkResult is what start reports: the claim, and where to work.
type StartWorkResult struct {
	StartResult
	Worktree string   `json:"worktree"`
	Branch   string   `json:"branch"`
	Target   string   `json:"target"`
	Pruned   []string `json:"pruned,omitempty"`
}

// StartWork claims an item. With here, it claims it for this worktree on
// the branch checked out (small work done in place, such as in the
// integration worktree). Otherwise it makes the item's branch from its
// target and a worktree for it, runs the project's setup there, and claims
// the item for that worktree: the agent works there.
func (m *Mull) StartWork(id string, here bool) (StartWorkResult, error) {
	var out StartWorkResult
	var title, target, release, dir string
	var setup []string
	err := m.view(func(s *state) error {
		i, err := s.item(id)
		if err != nil {
			return err
		}
		switch i.State {
		case model.StateCaptured, model.StateCommitted:
		case model.StateActive:
			if here && i.Claim != nil && i.Claim.Worktree == m.Worktree() {
				return nil
			}
			return errf(CodeClaimed, "item %s is already claimed by %s", id, i.Claim.Worktree)
		default:
			return errf(CodeState, "item %s is %s and cannot be started", id, i.State)
		}
		if n, max := s.activeCount(id), s.parallel(); n >= max {
			return errf(CodeAtCapacity, "%d items are active, the most at once (%d): finish or release one first. %s", n, max,
				m.personal("Only the human raises the cap (mull config set parallel <n> --on-request)."))
		}
		title, release, dir, setup = i.Title, s.release(), s.config.Worktrees, s.config.Setup
		target, _ = s.targetOf(i.Plan)
		return nil
	})
	if err != nil {
		return out, err
	}
	if here {
		res, err := m.Start(id)
		out.StartResult, out.Worktree, out.Branch, out.Target = res, m.Worktree(), m.repo.Branch(), target
		return out, err
	}
	if target == release {
		return out, errf(CodeState, "item %s would land on %s, the release branch, where mull never lands work. %s", id, release,
			m.personal("The human gives its plan a target (mull plan set <slug> target plan/<slug> --on-request)."))
	}
	out.Pruned, _ = m.Prune()
	tip, err := m.ensureBranch(target, release)
	if err != nil {
		return out, err
	}
	branch := id + "-" + branchSlug(title)
	exists, err := m.repo.RefCommit("refs/heads/" + branch)
	if err != nil {
		return out, err
	}
	main := m.repo.MainWorktree()
	if dir == "" {
		dir = filepath.Join(filepath.Dir(main), filepath.Base(main)+".work")
	} else if !filepath.IsAbs(dir) {
		dir = filepath.Join(main, dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return out, err
	}
	path := filepath.Join(dir, branch)
	args := []string{"worktree", "add", "--quiet", "-b", branch, path, target}
	if exists != "" {
		args = []string{"worktree", "add", "--quiet", path, branch} // its own, from before: pick it up
	}
	if _, err := m.repo.Run(nil, nil, args...); err != nil {
		return out, errf(CodeState, "making the worktree at %s: %v", path, err)
	}
	wt, err := git.Open(path)
	if err != nil {
		return out, err
	}
	undo := func() {
		_, _ = m.repo.Run(nil, nil, "worktree", "remove", "--force", path)
		if exists == "" {
			_, _ = m.repo.Run(nil, nil, "branch", "-D", branch)
		}
	}
	for _, c := range setup {
		if res := runIn(wt.Dir, c, []string{"MULL_MAIN=" + main, "MULL_ITEM=" + id}); res.Exit != 0 {
			undo()
			return out, errf(CodeSetupFailed, "setup `%s` exited %d in the new worktree (removed again); last output:\n%s", c, res.Exit, res.Tail)
		}
	}
	base := tip
	if exists != "" {
		base, _ = m.repo.RefCommit("refs/heads/" + branch)
	}
	err = m.update("start "+id, func(s *state) error {
		var err error
		out.StartResult, err = m.startAt(s, id, wt.Worktree(), branch, base)
		return err
	})
	if err != nil {
		undo()
		return out, err
	}
	out.Worktree, out.Branch, out.Target = wt.Worktree(), branch, target
	return out, nil
}

// ensureBranch returns the tip of branch, making it from release's tip if
// it doesn't exist.
func (m *Mull) ensureBranch(branch, release string) (string, error) {
	tip, err := m.repo.RefCommit("refs/heads/" + branch)
	if err != nil || tip != "" {
		return tip, err
	}
	rel, err := m.repo.RefCommit("refs/heads/" + release)
	if err != nil {
		return "", err
	}
	if rel == "" {
		return "", errf(CodeState, "neither the target %s nor the release branch %s exists", branch, release)
	}
	if _, err := m.repo.Run(nil, nil, "branch", branch, rel); err != nil {
		return "", err
	}
	return rel, nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// branchSlug is a short branch-name form of an item's title.
func branchSlug(title string) string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if len(s) > 30 {
		s = strings.TrimRight(s[:30], "-")
		if k := strings.LastIndex(s, "-"); k > 10 {
			s = s[:k]
		}
	}
	if s == "" {
		s = "work"
	}
	return s
}

// runIn runs a shell command in dir with extra env, as a check result.
func runIn(dir, command string, env []string) model.CheckResult {
	start := time.Now()
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	res := model.CheckResult{Command: command, DurationMS: time.Since(start).Milliseconds()}
	if exit, ok := err.(*exec.ExitError); ok {
		res.Exit = exit.ExitCode()
	} else if err != nil {
		res.Exit = -1
		buf.WriteString("\n" + err.Error())
	}
	if res.Exit != 0 {
		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		if len(lines) > tailLines {
			lines = lines[len(lines)-tailLines:]
		}
		res.Tail = strings.Join(lines, "\n")
	}
	return res
}

// behindTarget reports whether commit lacks target's tip: work that can't
// land by fast-forward until the target is merged into it. A missing target
// is never behind.
func (m *Mull) behindTarget(target, commit string) (bool, error) {
	tip, err := m.repo.RefCommit("refs/heads/" + target)
	if err != nil || tip == "" || tip == commit {
		return false, err
	}
	return !m.isAncestor(tip, commit), nil
}

// land fast-forwards target to commit: the work that passed the checks, so
// the target only ever holds trees that did. Where the target is checked out
// (the human's preview, say), it moves that worktree too, which must be
// clean. It never writes the release branch.
func (m *Mull) land(target, release, commit string) error {
	if target == release {
		return errf(CodeState, "the target is %s, the release branch, where mull never lands work", release)
	}
	tip, err := m.ensureBranch(target, release)
	if err != nil {
		return err
	}
	if tip == commit {
		return nil
	}
	if !m.isAncestor(tip, commit) {
		return errf(CodeBehindTarget, "%s has moved on: merge it into the item's branch (git merge %s) and finish again", target, target)
	}
	wts, err := m.listWorktrees()
	if err != nil {
		return err
	}
	for _, w := range wts {
		if w.branch != target {
			continue
		}
		r := &git.Repo{Dir: w.path}
		if st, err := r.Output(nil, nil, "status", "--porcelain", "--untracked-files=no"); err != nil || st != "" {
			return errf(CodeDirty, "%s is checked out at %s with uncommitted changes, so it can't move to the new work: commit or stash them there, and the work lands on the next finish", target, w.path)
		}
		if _, err := r.Run(nil, nil, "merge", "--ff-only", "--quiet", commit); err != nil {
			return errf(CodeState, "moving %s at %s: %v", target, w.path, err)
		}
		return nil
	}
	_, err = m.repo.Run(nil, nil, "update-ref", "-m", "mull: land", "refs/heads/"+target, commit, tip)
	return err
}

// Prune removes the worktrees whose work is all on a target or the release
// branch, and the branches checked out in them, never the one this runs in.
// A worktree goes when it's clean (nothing uncommitted, nothing untracked),
// isn't the main one, doesn't have a target or release branch checked out,
// and has its HEAD on one of those; and either its item is done or dropped,
// or it's no item's (a harness's) and hasn't changed in a while. Removal
// forces past a lock, since nothing in it would be lost.
func (m *Mull) Prune() ([]string, error) {
	var protected map[string]bool
	var claimed map[string]*model.Item
	err := m.view(func(s *state) error {
		protected = map[string]bool{strings.TrimPrefix(store.Ref, "refs/heads/"): true, s.release(): true, s.integration(): true}
		for slug := range s.plans {
			b, _ := s.targetOf(slug)
			protected[b] = true
		}
		claimed = map[string]*model.Item{}
		for _, i := range s.items {
			if i.Claim == nil {
				continue
			}
			if c := claimed[i.Claim.Worktree]; c == nil || i.Claim.At >= c.Claim.At {
				claimed[i.Claim.Worktree] = i
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	wts, err := m.listWorktrees()
	if err != nil || len(wts) < 2 {
		return nil, err
	}
	var onto []string
	for b := range protected {
		if tip, _ := m.repo.RefCommit("refs/heads/" + b); tip != "" {
			onto = append(onto, tip)
		}
	}
	here := m.Worktree()
	var removed []string
	for _, w := range wts[1:] {
		real := w.path
		if r, err := filepath.EvalSymlinks(w.path); err == nil {
			real = r
		}
		if w.bare || w.path == here || real == here || protected[w.branch] || w.head == "" {
			continue
		}
		i := claimed[w.path]
		if i == nil {
			i = claimed[real]
		}
		switch {
		case i != nil && i.State != model.StateDone && i.State != model.StateDropped:
			continue // its item is still being worked
		case i == nil && !idle(w.path, 2*time.Hour):
			continue // no item's, and maybe someone's working in it
		}
		on := false
		for _, tip := range onto {
			if tip == w.head || m.isAncestor(w.head, tip) {
				on = true
				break
			}
		}
		if !on {
			continue
		}
		r := &git.Repo{Dir: w.path}
		if st, err := r.Output(nil, nil, "status", "--porcelain"); err != nil || st != "" {
			continue
		}
		if _, err := m.repo.Run(nil, nil, "worktree", "remove", "--force", "--force", w.path); err != nil {
			continue
		}
		removed = append(removed, w.path)
		if w.branch != "" {
			_, _ = m.repo.Run(nil, nil, "branch", "-D", w.branch) // its commits are all on a target
		}
	}
	return removed, nil
}

// idle reports whether the worktree at path hasn't changed for at least d:
// its index (or, with none, its directory) is older than that.
func idle(path string, d time.Duration) bool {
	r := &git.Repo{Dir: path}
	gd, err := r.Output(nil, nil, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return false
	}
	fi, err := os.Stat(filepath.Join(gd, "index"))
	if err != nil {
		if fi, err = os.Stat(path); err != nil {
			return false
		}
	}
	return time.Since(fi.ModTime()) >= d
}

// prunedText is a line for what Prune removed.
func prunedText(paths []string) string {
	return fmt.Sprintf("removed %d merged worktrees: %s", len(paths), strings.Join(paths, ", "))
}

// landItem lands commit on target, saying whether it did and, if not, why.
func (m *Mull) landItem(target, release, commit string) (bool, string) {
	if err := m.land(target, release, commit); err != nil {
		return false, err.Error()
	}
	return true, ""
}

// HookResult is what mull hooks did.
type HookResult struct {
	Path      string `json:"path"`
	Installed bool   `json:"installed"`
	Note      string `json:"note,omitempty"`
}

const hookMark = "# mull: release guard"

// InstallHook installs a pre-push hook that refuses any push to the release
// branch, so no agent ever pushes there; the human ships with
// git push --no-verify, or through their forge. A pre-push hook mull didn't
// write is left alone.
func (m *Mull) InstallHook() (HookResult, error) {
	var release string
	if err := m.view(func(s *state) error { release = s.release(); return nil }); err != nil {
		return HookResult{}, err
	}
	dir, _ := m.repo.Output(nil, nil, "config", "core.hooksPath")
	if dir == "" {
		common, err := m.repo.Output(nil, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil {
			return HookResult{}, err
		}
		dir = filepath.Join(common, "hooks")
	} else if !filepath.IsAbs(dir) {
		dir = filepath.Join(m.repo.MainWorktree(), dir)
	}
	path := filepath.Join(dir, "pre-push")
	out := HookResult{Path: path}
	if b, err := os.ReadFile(path); err == nil && !bytes.Contains(b, []byte(hookMark)) {
		out.Note = fmt.Sprintf("a pre-push hook mull didn't write is there, so mull leaves it: make sure it refuses pushes to %s", release)
		return out, nil
	}
	script := fmt.Sprintf(`#!/bin/sh
%s
# Installed by mull hooks: refuses any push to %[2]s, the release branch.
# Ship with git push --no-verify, or through your forge.
while read local_ref local_sha remote_ref remote_sha; do
  if [ "$remote_ref" = "refs/heads/%[2]s" ]; then
    echo "mull: refusing to push to %[2]s, the release branch" >&2
    exit 1
  fi
done
exit 0
`, hookMark, release)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return out, err
	}
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		return out, err
	}
	out.Installed = true
	return out, nil
}
