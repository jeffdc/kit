package mull

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"mull/internal/model"
	"mull/internal/store"
)

// How git links to items:
//   - a branch named "<id>-..." links every commit on it that is not yet on
//     a target branch;
//   - a "Mull: <id>" trailer links its commit wherever it is;
//   - on a target branch, a "Merge branch '<id>-...'" commit links the work
//     it merged.
// Bare 4-hex mentions in commit prose are never counted.
//
// Each item's work is measured against its plan's target branch (config's
// integration branch when the plan sets none, and for unplanned work): it
// has landed, and is seen in git, once linked commits are on that branch.
// When it landed is derived, never stored: the date of the target's
// first-parent commit that brought the work in.

var (
	branchID = regexp.MustCompile(`^([0-9a-f]{4})-`)
	mergeID  = regexp.MustCompile(`^Merge (?:remote-tracking )?branch '(?:[^']*/)?([0-9a-f]{4})-[^']*'`)
)

// historyLimit bounds how many commits of each branch are read.
const historyLimit = 5000

// ItemGit is what git says about one item.
type ItemGit struct {
	Target       string   `json:"target"`                  // the branch it's measured against: its plan's target
	Landed       int      `json:"landed_commits"`          // linked commits on the target
	LastLanded   string   `json:"last_landed,omitempty"`   // commit date
	MergedAt     string   `json:"merged_at,omitempty"`     // when the target's first-parent commit that brought in its latest work was made
	Unmerged     int      `json:"unmerged_commits"`        // linked commits on no target branch
	LastActivity string   `json:"last_activity,omitempty"` // latest linked commit anywhere
	Branches     []string `json:"branches,omitempty"`      // unmerged branches carrying its work
	SeenInGit    bool     `json:"seen_in_git"`             // landed > 0

	// For a done item: the commits carrying its trailer that were written
	// (author date) after it was accepted, anywhere, and the latest of them.
	afterAccept     int
	lastAfterAccept time.Time
}

// Target is where a plan's work lands: the plan's own target branch, or
// config's integration branch when it sets none.
type Target struct {
	Branch         string `json:"branch"`
	From           string `json:"from"`             // plan (the plan sets it) or integration (config's)
	Found          bool   `json:"found"`            // the branch exists
	AheadOfRelease int    `json:"ahead_of_release"` // its commits the release branch doesn't have
}

// Where a target comes from.
const (
	TargetFromPlan        = "plan"
	TargetFromIntegration = "integration"
)

// integration is config's integration branch, main when unset.
func (s *state) integration() string {
	if s.config.Integration != "" {
		return s.config.Integration
	}
	return "main"
}

// targetOf is the branch a plan's work lands on, and where that comes from.
// "" (unplanned work) and a plan with no target use integration.
func (s *state) targetOf(slug string) (string, string) {
	if p := s.plans[slug]; p != nil && p.Target != "" {
		return p.Target, TargetFromPlan
	}
	return s.integration(), TargetFromIntegration
}

// targets are the target branches (each plan's, and integration) that
// exist, with their tips and how far each is ahead of release.
type targets struct {
	tips  map[string]string // branch: tip commit
	ahead map[string]int
}

// readTargets reads every target branch; heads are the branch tips.
func (m *Mull) readTargets(s *state, heads map[string]string) targets {
	t := targets{tips: map[string]string{}, ahead: map[string]int{}}
	names := []string{s.integration()}
	for _, slug := range sortedKeys(s.plans) {
		b, _ := s.targetOf(slug)
		names = append(names, b)
	}
	for _, b := range dedupe(names) {
		if tip := heads[b]; tip != "" {
			t.tips[b] = tip
			t.ahead[b] = m.aheadOfRelease(s, b, tip, heads)
		}
	}
	return t
}

// target is a plan's target ("" for unplanned work), with what git says.
func (s *state) target(slug string, t targets) Target {
	b, from := s.targetOf(slug)
	return Target{Branch: b, From: from, Found: t.tips[b] != "", AheadOfRelease: t.ahead[b]}
}

// gitState is everything the view derives from git.
type gitState struct {
	Integration      string
	IntegrationFound bool
	AheadOfRelease   int
	Targets          targets
	Items            map[string]*ItemGit
	Unlinked         []string // recent first-parent commits on integration with no link (subjects)

	heads    map[string]string // every local branch's tip
	overlaps []Overlap         // pairs of items in flight changing the same files; read by readOverlaps
}

func (g *gitState) item(id string) *ItemGit {
	ig := g.Items[id]
	if ig == nil {
		ig = &ItemGit{}
		g.Items[id] = ig
	}
	return ig
}

func later(a, b string) string {
	if b > a {
		return b
	}
	return a
}

type commitInfo struct {
	sha, date, subject string
	authored           time.Time // author date: kept by a rebase or cherry-pick, unlike date
	parents            []string
	trailers           []string
}

const (
	fieldSep  = "\x1f"
	recordSep = "\x1e"
	logFormat = "%H%x1f%P%x1f%cI%x1f%s%x1f%(trailers:key=Mull,valueonly,separator=%x2C)%x1f%aI%x1e"
)

// logFrom returns up to historyLimit commits reachable from tip, a commit
// ID, and from none of not (other commit IDs). All are IDs, so the answer
// never changes and is cached.
func (m *Mull) logFrom(tip string, not ...string) ([]commitInfo, error) {
	args := []string{"-n", itoa(historyLimit), tip}
	if len(not) > 0 {
		args = append(append(args, "--not"), not...)
	}
	return gitLogs.get(m.repo.Dir+"\x00"+strings.Join(args, " "), func() ([]commitInfo, error) {
		return m.logCommits(args...)
	})
}

func (m *Mull) logCommits(args ...string) ([]commitInfo, error) {
	out, err := m.repo.Output(nil, nil, append([]string{"log", "--format=" + logFormat}, args...)...)
	if err != nil {
		return nil, err
	}
	var commits []commitInfo
	for _, rec := range strings.Split(out, recordSep) {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		f := strings.Split(rec, fieldSep)
		if len(f) < 6 {
			continue
		}
		c := commitInfo{sha: f[0], parents: strings.Fields(f[1]), date: f[2], subject: f[3]}
		c.authored, _ = time.Parse(time.RFC3339, f[5])
		for _, t := range strings.Split(f[4], ",") {
			if t = strings.TrimSpace(t); model.IsID(t) {
				c.trailers = append(c.trailers, t)
			}
		}
		commits = append(commits, c)
	}
	return commits, nil
}

// aheadOfRelease counts the commits on a branch (at tip) that the release
// branch doesn't have; 0 when they are the same branch or the release
// branch is missing. heads are the branch tips.
func (m *Mull) aheadOfRelease(s *state, branch, tip string, heads map[string]string) int {
	rel := s.config.Release
	if rel == "" || rel == branch {
		return 0
	}
	relSha := heads[rel]
	if relSha == "" {
		return 0
	}
	n, _ := gitCounts.get(m.repo.Dir+"\x00"+tip+" --not "+relSha, func() (int, error) {
		n, err := m.repo.Output(nil, nil, "rev-list", "--count", tip, "--not", relSha)
		return atoi(n), err
	})
	return n
}

// branchHeads returns the local branches, in refname order, and the commit
// each points at: one git call.
func (m *Mull) branchHeads() ([]string, map[string]string, error) {
	out, err := m.repo.Output(nil, nil, "for-each-ref", "--format=%(objectname) %(refname:short)", "refs/heads")
	if err != nil {
		return nil, nil, err
	}
	var names []string
	heads := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		sha, name, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		names = append(names, name)
		heads[name] = sha
	}
	return names, heads, nil
}

// readGit derives git links for the items in s, each measured against its
// plan's target. A missing target branch is reported, not fatal: the view
// still works from mull's own records.
func (m *Mull) readGit(s *state) (*gitState, error) {
	g := &gitState{Integration: s.integration(), Items: map[string]*ItemGit{}}
	branches, heads, err := m.branchHeads()
	if err != nil {
		return nil, err
	}
	g.heads = heads
	g.Targets = m.readTargets(s, heads)
	g.IntegrationFound = g.Targets.tips[g.Integration] != ""
	g.AheadOfRelease = g.Targets.ahead[g.Integration]
	targetOf := make(map[string]string, len(s.items)) // each item is measured against its plan's target
	for id, i := range s.items {
		targetOf[id], _ = s.targetOf(i.Plan)
	}
	known := func(id string) bool { return s.items[id] != nil }
	weekAgo := m.Now().Add(-7 * 24 * time.Hour).UTC().Format(time.RFC3339)
	accepted := s.acceptedAt()
	counted := map[string]bool{} // sha and item, so a commit on several branches counts once
	afterAccept := func(c commitInfo) {
		for _, id := range c.trailers {
			at, ok := accepted[id]
			if !ok || !c.authored.After(at) || counted[c.sha+id] {
				continue
			}
			counted[c.sha+id] = true
			ig := g.item(id)
			ig.afterAccept++
			if c.authored.After(ig.lastAfterAccept) {
				ig.lastAfterAccept = c.authored
			}
		}
	}

	var tips []string // every target's tip: work on none of them is unmerged
	for _, b := range sortedKeys(g.Targets.tips) {
		tip := g.Targets.tips[b]
		tips = append(tips, tip)
		h, err := m.history(tip)
		if err != nil {
			return nil, err
		}
		commits, bySha, in := h.commits, h.bySha, h.in
		landed := func(id string, c commitInfo) {
			ig := g.item(id)
			ig.Landed++
			ig.LastLanded = later(ig.LastLanded, c.date)
			ig.LastActivity = later(ig.LastActivity, c.date)
			at := in[c.sha]
			if at == "" {
				at = c.date // beyond the history read
			}
			ig.MergedAt = later(ig.MergedAt, at)
		}
		// Trailers anywhere in the target's history.
		for _, c := range commits {
			afterAccept(c)
			for _, id := range c.trailers {
				if known(id) && targetOf[id] == b {
					landed(id, c)
				}
			}
		}
		// Walk first parents: merges of id-branches link; on integration,
		// other direct commits without a trailer are unlinked work.
		for sha := tip; sha != ""; {
			c, ok := bySha[sha]
			if !ok {
				break
			}
			if mm := mergeID.FindStringSubmatch(c.subject); mm != nil && known(mm[1]) {
				if targetOf[mm[1]] == b {
					landed(mm[1], c)
				}
			} else if b == g.Integration && len(c.parents) <= 1 && len(c.trailers) == 0 && c.date >= weekAgo {
				g.Unlinked = append(g.Unlinked, c.subject)
			}
			if len(c.parents) == 0 {
				break
			}
			sha = c.parents[0]
		}
	}

	// Unmerged work on other local branches: commits on no target.
	for _, b := range branches {
		if "refs/heads/"+b == store.Ref || g.Targets.tips[b] != "" || b == s.config.Release {
			continue
		}
		commits, err := m.logFrom(heads[b], tips...)
		if err != nil {
			return nil, err
		}
		branchItem := ""
		if mm := branchID.FindStringSubmatch(b); mm != nil && known(mm[1]) {
			branchItem = mm[1]
		}
		touched := map[string]bool{}
		for _, c := range commits {
			afterAccept(c)
			ids := append([]string{}, c.trailers...)
			if branchItem != "" {
				ids = append(ids, branchItem)
			}
			for _, id := range dedupe(ids) {
				if !known(id) {
					continue
				}
				ig := g.item(id)
				ig.Unmerged++
				ig.LastActivity = later(ig.LastActivity, c.date)
				touched[id] = true
			}
		}
		if branchItem != "" {
			touched[branchItem] = true // an id-branch with no commits yet still shows
		}
		for id := range touched {
			ig := g.item(id)
			ig.Branches = append(ig.Branches, b)
		}
	}
	for id, ig := range g.Items {
		ig.Target = targetOf[id]
		ig.SeenInGit = ig.Landed > 0
		sort.Strings(ig.Branches)
	}
	return g, nil
}

// gitOf is what git says about an item: its entry in g, or, with no linked
// commits anywhere, an empty one measured against its plan's target.
func (s *state) gitOf(g *gitState, i *model.Item) ItemGit {
	if ig := g.Items[i.ID]; ig != nil {
		return *ig
	}
	t, _ := s.targetOf(i.Plan)
	return ItemGit{Target: t}
}

// history is a target branch's history read from its tip: the commits, by
// ID, and when each was brought in (see broughtIn).
type history struct {
	commits []commitInfo
	bySha   map[string]commitInfo
	in      map[string]string
}

// history reads and indexes the history from tip, a commit ID; cached, as
// the answer never changes.
func (m *Mull) history(tip string) (*history, error) {
	return targetHistories.get(m.repo.Dir+"\x00"+tip, func() (*history, error) {
		commits, err := m.logFrom(tip)
		if err != nil {
			return nil, err
		}
		h := &history{commits: commits, bySha: make(map[string]commitInfo, len(commits))}
		for _, c := range commits {
			h.bySha[c.sha] = c
		}
		h.in = broughtIn(tip, h.bySha)
		return h, nil
	})
}

// broughtIn maps each commit read from tip's history to the date of the
// first-parent commit of tip that brought it in: a commit on the first-
// parent line brings in itself, and a merge brings in everything it reaches
// that older first-parent commits don't. Commits beyond the history read
// are absent.
func broughtIn(tip string, bySha map[string]commitInfo) map[string]string {
	var line []commitInfo
	for sha := tip; sha != ""; {
		c, ok := bySha[sha]
		if !ok {
			break
		}
		line = append(line, c)
		if len(c.parents) == 0 {
			break
		}
		sha = c.parents[0]
	}
	out := make(map[string]string, len(bySha))
	// Oldest first, so what an older commit reaches is already claimed: the
	// claimed set stays closed under parents, and a walk stops at it.
	for k := len(line) - 1; k >= 0; k-- {
		f := line[k]
		out[f.sha] = f.date
		var stack []string
		if len(f.parents) > 1 {
			stack = append(stack, f.parents[1:]...)
		}
		for len(stack) > 0 {
			sha := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if _, done := out[sha]; done {
				continue
			}
			c, ok := bySha[sha]
			if !ok {
				continue
			}
			out[sha] = f.date
			stack = append(stack, c.parents...)
		}
	}
	return out
}

// acceptedAt is when each done item was accepted: its latest accepted
// review, or else (done some other way, as from before Model v2) when it was
// last updated, which is never before it was done.
func (s *state) acceptedAt() map[string]time.Time {
	out := map[string]time.Time{}
	for _, e := range s.entries {
		if e.Kind != model.KindReview || e.Settled == nil || e.Settled.Action != model.ActAccept {
			continue
		}
		if i := s.items[e.Item]; i == nil || i.State != model.StateDone {
			continue
		}
		if at, ok := parseTime(e.Settled.At); ok && at.After(out[e.Item]) {
			out[e.Item] = at
		}
	}
	for id, i := range s.items {
		if _, ok := out[id]; ok || i.State != model.StateDone {
			continue
		}
		if at, ok := parseTime(i.Updated); ok {
			out[id] = at
		}
	}
	return out
}

func itoa(n int) string { return strconv.Itoa(n) }

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
