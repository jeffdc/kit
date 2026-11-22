package mull

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"mull/internal/git"
	"mull/internal/model"
	"mull/internal/store"
)

// Claim worktrees: the git worktrees items were claimed from. mull never
// creates or removes one; it lists them, read-only, and says which can go.
// A worktree is prunable when its item is done, the item's work is on its
// plan's target (with nothing in the worktree left off it), and it's clean,
// so git's own removal (git worktree remove, git branch -d) would go
// through without --force.

// WorktreeList is mull worktrees: the claim worktrees git knows, and the
// prunable ones' count and size.
type WorktreeList struct {
	Worktrees     []WorktreeRow `json:"worktrees"`
	Prunable      int           `json:"prunable"`
	PrunableBytes int64         `json:"prunable_bytes"`
}

// WorktreeRow is one claim worktree, with the item last claimed from it.
type WorktreeRow struct {
	Path     string   `json:"path"`
	Branch   string   `json:"branch,omitempty"` // checked out; none when detached
	Head     string   `json:"head"`
	Item     string   `json:"item"`
	Title    string   `json:"title"`
	State    string   `json:"state"`
	Plan     string   `json:"plan,omitempty"`
	Target   string   `json:"target"`
	Bytes    int64    `json:"bytes"`
	Clean    bool     `json:"clean"`
	Merged   bool     `json:"merged"` // its HEAD, and the item's branch, are on the target
	Locked   bool     `json:"locked,omitempty"`
	Prunable bool     `json:"prunable"`
	Why      string   `json:"why,omitempty"`      // why it isn't prunable
	Commands []string `json:"commands,omitempty"` // for a prunable one: what removes it and its merged branches
}

// Prunable is the count and size of the prunable claim worktrees, for the
// view's problems and mull serve's overview.
type Prunable struct {
	Count int   `json:"count"`
	Bytes int64 `json:"bytes"`
}

// Worktrees lists the claim worktrees, measuring each one now.
func (m *Mull) Worktrees() (WorktreeList, error) {
	out := WorktreeList{Worktrees: []WorktreeRow{}}
	err := m.view(func(s *state) error {
		g, err := m.readGit(s)
		if err != nil {
			return err
		}
		rows, err := m.worktreeRows(s, g, true)
		if err != nil {
			return err
		}
		out.Worktrees = rows
		for _, r := range rows {
			if r.Prunable {
				out.Prunable++
				out.PrunableBytes += r.Bytes
			}
		}
		return nil
	})
	return out, err
}

// prunable counts the prunable claim worktrees, of one plan's items or (plan
// "") of every item, closed plans' too; nil when there are none. Only those
// that could be prunable are measured, and their measurements are reused for
// a while (see worktreeStatsTTL), since the view and mull serve's overview
// are read often and a worktree's size takes a walk of its files.
func (m *Mull) prunable(s *state, g *gitState, plan string) (*Prunable, error) {
	rows, err := m.worktreeRows(s, g, false)
	if err != nil {
		return nil, err
	}
	var p Prunable
	for _, r := range rows {
		if r.Prunable && (plan == "" || r.Plan == plan) {
			p.Count++
			p.Bytes += r.Bytes
		}
	}
	if p.Count == 0 {
		return nil, nil
	}
	return &p, nil
}

// worktree is an entry of git worktree list.
type worktree struct {
	path, head, branch string
	bare, locked       bool
}

// listWorktrees reads git worktree list: one git call. The main worktree
// comes first.
func (m *Mull) listWorktrees() ([]worktree, error) {
	out, err := m.repo.Output(nil, nil, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var wts []worktree
	for _, block := range strings.Split(out, "\n\n") {
		var w worktree
		for _, line := range strings.Split(strings.TrimSpace(block), "\n") {
			key, val, _ := strings.Cut(line, " ")
			switch key {
			case "worktree":
				w.path = val
			case "HEAD":
				w.head = val
			case "branch":
				w.branch = strings.TrimPrefix(val, "refs/heads/")
			case "bare":
				w.bare = true
			case "locked":
				w.locked = true
			}
		}
		if w.path != "" {
			wts = append(wts, w)
		}
	}
	return wts, nil
}

// worktreeRows matches the linked worktrees (never the main one) to the
// items claimed from them, the latest claim winning. measureAll measures
// every one, fresh; otherwise only those that could be prunable are, through
// the cache.
func (m *Mull) worktreeRows(s *state, g *gitState, measureAll bool) ([]WorktreeRow, error) {
	if !measureAll && !s.doneInLinkedWorktree() {
		return []WorktreeRow{}, nil // nothing could be prunable: no git call
	}
	wts, err := m.listWorktrees()
	if err != nil {
		return nil, err
	}
	if len(wts) < 2 {
		return []WorktreeRow{}, nil
	}
	byPath := map[string]int{}
	for k, w := range wts[1:] {
		byPath[w.path] = k + 1
		if real, err := filepath.EvalSymlinks(w.path); err == nil {
			byPath[real] = k + 1
		}
	}
	claimed := map[int]*model.Item{}
	for _, id := range sortedKeys(s.items) {
		i := s.items[id]
		if i.Claim == nil {
			continue
		}
		k, ok := byPath[i.Claim.Worktree]
		if !ok {
			continue
		}
		if c := claimed[k]; c == nil || i.Claim.At >= c.Claim.At {
			claimed[k] = i
		}
	}
	protected := map[string]bool{strings.TrimPrefix(store.Ref, "refs/heads/"): true, s.config.Release: true}
	for b := range g.Targets.tips {
		protected[b] = true
	}
	for _, slug := range sortedKeys(s.plans) {
		b, _ := s.targetOf(slug)
		protected[b] = true
	}
	protected[s.integration()] = true
	delete(protected, "") // no release branch set
	checkedOut := map[string]bool{}
	for _, w := range wts {
		if w.branch != "" {
			checkedOut[w.branch] = true
		}
	}

	heads := g.Heads
	rows := []WorktreeRow{}
	for k := 1; k < len(wts); k++ {
		w, i := wts[k], claimed[k]
		if i == nil || w.bare {
			continue
		}
		ig := s.gitOf(g, i)
		r := WorktreeRow{Path: w.path, Branch: w.branch, Head: w.head, Item: i.ID, Title: i.Title,
			State: i.State, Plan: i.Plan, Target: ig.Target, Locked: w.locked}
		// The branches git branch -d would remove: the one checked out here
		// and the item's own, when it's another, unless a target's.
		var branches []string
		for _, b := range dedupe([]string{w.branch, i.Claim.Branch}) {
			if b != "" && b != "HEAD" && !protected[b] && heads[b] != "" && (b == w.branch || !checkedOut[b]) {
				branches = append(branches, b)
			}
		}
		r.Merged = w.head != "" && m.onTarget(g, ig.Target, w.head)
		for _, b := range branches {
			if !m.onTarget(g, ig.Target, heads[b]) {
				r.Merged = false
			}
		}
		switch {
		case w.branch != "" && protected[w.branch]:
			r.Why = fmt.Sprintf("it has %s checked out, a target or release branch", w.branch)
		case i.State != model.StateDone:
			r.Why = fmt.Sprintf("its item is %s, not done", i.State)
		case !ig.SeenInGit:
			r.Why = fmt.Sprintf("none of its item's work is on %s", ig.Target)
		case !r.Merged:
			r.Why = fmt.Sprintf("it has commits not on %s", ig.Target)
		case w.locked:
			r.Why = "it's locked (git worktree unlock)"
		}
		if r.Why == "" || measureAll {
			st, err := m.measureWorktree(w.path, w.head, !measureAll)
			if err != nil {
				return nil, err
			}
			r.Clean, r.Bytes = st.clean, st.bytes
		}
		if r.Why == "" && !r.Clean {
			r.Why = "it has uncommitted or untracked changes"
		}
		if r.Why == "" {
			r.Prunable = true
			r.Commands = append(r.Commands, "git worktree remove "+shellQuote(w.path))
			for _, b := range branches {
				r.Commands = append(r.Commands, "git branch -d "+shellQuote(b))
			}
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(a, b int) bool { return rows[a].Path < rows[b].Path })
	return rows, nil
}

// doneInLinkedWorktree reports whether a done item was claimed from a
// linked worktree still on this machine: one whose .git is a file (a main
// worktree's is a directory). Without one, nothing can be prunable, and the
// view and mull serve skip git worktree list.
func (s *state) doneInLinkedWorktree() bool {
	seen := map[string]bool{}
	for _, i := range s.items {
		if i.State != model.StateDone || i.Claim == nil || seen[i.Claim.Worktree] {
			continue
		}
		seen[i.Claim.Worktree] = true
		if fi, err := os.Lstat(filepath.Join(i.Claim.Worktree, ".git")); err == nil && fi.Mode().IsRegular() {
			return true
		}
	}
	return false
}

// onTarget reports whether commit sha is on the target branch: in its
// history as read for the view, or, past what that reads, by asking git.
func (m *Mull) onTarget(g *gitState, target, sha string) bool {
	tip := g.Targets.tips[target]
	if tip == "" || sha == "" {
		return false
	}
	if sha == tip {
		return true
	}
	h, err := m.history(tip)
	if err != nil {
		return false
	}
	if _, ok := h.bySha[sha]; ok {
		return true
	}
	if len(h.commits) < historyLimit {
		return false
	}
	_, code, err := m.repo.RunStatus(nil, nil, "merge-base", "--is-ancestor", sha, tip)
	return err == nil && code == 0
}

// worktreeStats is what measuring a worktree finds.
type worktreeStats struct {
	clean bool
	bytes int64
	at    time.Time
}

// worktreeStatsTTL is how long a measurement is reused by the view and
// mull serve. mull worktrees always measures afresh.
var worktreeStatsTTL = 5 * time.Minute

var (
	statsMu    sync.Mutex
	statsCache = map[string]worktreeStats{}
)

// measureWorktree says whether the worktree at path is clean (git status
// shows nothing, untracked files included, as git worktree remove requires)
// and how many bytes its files take. cached reuses a recent measurement of
// the same path at the same HEAD.
func (m *Mull) measureWorktree(path, head string, cached bool) (worktreeStats, error) {
	key := path + "\x00" + head
	if cached {
		statsMu.Lock()
		st, ok := statsCache[key]
		statsMu.Unlock()
		if ok && time.Since(st.at) < worktreeStatsTTL {
			return st, nil
		}
	}
	out, err := (&git.Repo{Dir: path}).Output(nil, nil, "status", "--porcelain")
	if err != nil {
		return worktreeStats{}, err
	}
	st := worktreeStats{clean: out == "", bytes: diskSize(path), at: time.Now()}
	statsMu.Lock()
	for k, old := range statsCache {
		if time.Since(old.at) >= worktreeStatsTTL {
			delete(statsCache, k)
		}
	}
	statsCache[key] = st
	statsMu.Unlock()
	return st, nil
}

// diskSize is the total size of the regular files under dir, not following
// symlinks. Unreadable parts are skipped.
func diskSize(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

// shellQuote quotes s for a POSIX shell when it needs it.
func shellQuote(s string) string {
	if s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./=:@+%,", r))
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// bytesText renders a size compactly, in decimal units: 48 KB, 312 MB, 1.4 GB.
func bytesText(n int64) string {
	units := []string{"bytes", "KB", "MB", "GB", "TB"}
	f, u := float64(n), 0
	for f >= 1000 && u < len(units)-1 {
		f /= 1000
		u++
	}
	switch {
	case u == 0:
		return fmt.Sprintf("%d bytes", n)
	case f < 10:
		return fmt.Sprintf("%.1f %s", f, units[u])
	default:
		return fmt.Sprintf("%.0f %s", f, units[u])
	}
}

// prunableText is the view's line for the prunable worktrees.
func prunableText(p Prunable) string {
	verb := "are"
	if p.Count == 1 {
		verb = "is"
	}
	return fmt.Sprintf("%s (%s) %s left behind by done work, merged and clean: mull worktrees gives the commands that remove them",
		plural(p.Count, "claim worktree"), bytesText(p.Bytes), verb)
}
