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
//     the integration branch;
//   - a "Mull: <id>" trailer links its commit wherever it is;
//   - on the integration branch, a "Merge branch '<id>-...'" commit links
//     the work it merged.
// Bare 4-hex mentions in commit prose are never counted.

var (
	branchID = regexp.MustCompile(`^([0-9a-f]{4})-`)
	mergeID  = regexp.MustCompile(`^Merge (?:remote-tracking )?branch '(?:[^']*/)?([0-9a-f]{4})-[^']*'`)
)

// historyLimit bounds how many integration commits are read.
const historyLimit = 5000

// ItemGit is what git says about one item.
type ItemGit struct {
	Landed       int      `json:"landed_commits"`          // linked commits on the integration branch
	LastLanded   string   `json:"last_landed,omitempty"`   // commit date
	Unmerged     int      `json:"unmerged_commits"`        // linked commits only on other branches
	LastActivity string   `json:"last_activity,omitempty"` // latest linked commit anywhere
	Branches     []string `json:"branches,omitempty"`      // unmerged branches carrying its work
	SeenInGit    bool     `json:"seen_in_git"`             // landed > 0

	// For a done item: the commits carrying its trailer that were written
	// (author date) after it was accepted, anywhere, and the latest of them.
	afterAccept     int
	lastAfterAccept time.Time
}

// gitState is everything the view derives from git.
type gitState struct {
	Integration      string
	IntegrationFound bool
	AheadOfRelease   int
	Items            map[string]*ItemGit
	Unlinked         []string // recent first-parent commits on integration with no link (subjects)
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
// ID, and not from not (another commit ID, or ""). Both are IDs, so the
// answer never changes and is cached.
func (m *Mull) logFrom(tip, not string) ([]commitInfo, error) {
	args := []string{"-n", itoa(historyLimit), tip}
	if not != "" {
		args = append(args, "--not", not)
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

// aheadOfRelease counts the commits on the integration branch (at integ)
// that the release branch doesn't have; 0 when they are the same branch or
// the release branch is missing. heads are the branch tips.
func (m *Mull) aheadOfRelease(s *state, integration, integ string, heads map[string]string) int {
	rel := s.config.Release
	if rel == "" || rel == integration {
		return 0
	}
	relSha := heads[rel]
	if relSha == "" {
		return 0
	}
	n, _ := gitCounts.get(m.repo.Dir+"\x00"+integ+" --not "+relSha, func() (int, error) {
		n, err := m.repo.Output(nil, nil, "rev-list", "--count", integ, "--not", relSha)
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

// readGit derives git links for the items in s. A missing integration branch
// is reported, not fatal: the view still works from mull's own records.
func (m *Mull) readGit(s *state) (*gitState, error) {
	g := &gitState{Integration: s.config.Integration, Items: map[string]*ItemGit{}}
	if g.Integration == "" {
		g.Integration = "main"
	}
	branches, heads, err := m.branchHeads()
	if err != nil {
		return nil, err
	}
	integ := heads[g.Integration]
	g.IntegrationFound = integ != ""
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

	if g.IntegrationFound {
		commits, err := m.logFrom(integ, "")
		if err != nil {
			return nil, err
		}
		// Trailers anywhere in integration history.
		for _, c := range commits {
			afterAccept(c)
			for _, id := range c.trailers {
				if known(id) {
					ig := g.item(id)
					ig.Landed++
					ig.LastLanded = later(ig.LastLanded, c.date)
					ig.LastActivity = later(ig.LastActivity, c.date)
				}
			}
		}
		// Walk first parents: merges of id-branches link; other direct
		// commits without a trailer are unlinked work.
		bySha := make(map[string]commitInfo, len(commits))
		for _, c := range commits {
			bySha[c.sha] = c
		}
		for sha := integ; sha != ""; {
			c, ok := bySha[sha]
			if !ok {
				break
			}
			if mm := mergeID.FindStringSubmatch(c.subject); mm != nil && known(mm[1]) {
				ig := g.item(mm[1])
				ig.Landed++
				ig.LastLanded = later(ig.LastLanded, c.date)
				ig.LastActivity = later(ig.LastActivity, c.date)
			} else if len(c.parents) <= 1 && len(c.trailers) == 0 && c.date >= weekAgo {
				g.Unlinked = append(g.Unlinked, c.subject)
			}
			if len(c.parents) == 0 {
				break
			}
			sha = c.parents[0]
		}
		g.AheadOfRelease = m.aheadOfRelease(s, g.Integration, integ, heads)
	}

	// Unmerged work on other local branches.
	for _, b := range branches {
		if "refs/heads/"+b == store.Ref || b == g.Integration || b == s.config.Release {
			continue
		}
		commits, err := m.logFrom(heads[b], integ)
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
	for _, ig := range g.Items {
		ig.SeenInGit = ig.Landed > 0
		sort.Strings(ig.Branches)
	}
	return g, nil
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
