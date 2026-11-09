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
	parents            []string
	trailers           []string
}

const (
	fieldSep  = "\x1f"
	recordSep = "\x1e"
	logFormat = "%H%x1f%P%x1f%cI%x1f%s%x1f%(trailers:key=Mull,valueonly,separator=%x2C)%x1e"
)

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
		if len(f) < 5 {
			continue
		}
		c := commitInfo{sha: f[0], parents: strings.Fields(f[1]), date: f[2], subject: f[3]}
		for _, t := range strings.Split(f[4], ",") {
			if t = strings.TrimSpace(t); model.IsID(t) {
				c.trailers = append(c.trailers, t)
			}
		}
		commits = append(commits, c)
	}
	return commits, nil
}

// readGit derives git links for the items in s. A missing integration branch
// is reported, not fatal: the view still works from mull's own records.
func (m *Mull) readGit(s *state) (*gitState, error) {
	g := &gitState{Integration: s.config.Integration, Items: map[string]*ItemGit{}}
	if g.Integration == "" {
		g.Integration = "main"
	}
	integ, err := m.repo.RefCommit("refs/heads/" + g.Integration)
	if err != nil {
		return nil, err
	}
	g.IntegrationFound = integ != ""
	known := func(id string) bool { return s.items[id] != nil }
	weekAgo := m.Now().Add(-7 * 24 * time.Hour).UTC().Format(time.RFC3339)

	if g.IntegrationFound {
		commits, err := m.logCommits("-n", itoa(historyLimit), integ)
		if err != nil {
			return nil, err
		}
		// Trailers anywhere in integration history.
		for _, c := range commits {
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
		if rel := s.config.Release; rel != "" && rel != g.Integration {
			if relSha, _ := m.repo.RefCommit("refs/heads/" + rel); relSha != "" {
				n, err := m.repo.Output(nil, nil, "rev-list", "--count", integ, "--not", relSha)
				if err == nil {
					g.AheadOfRelease = atoi(n)
				}
			}
		}
	}

	// Unmerged work on other local branches.
	branches, err := m.repo.Output(nil, nil, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	for _, b := range strings.Split(branches, "\n") {
		b = strings.TrimSpace(b)
		if b == "" || "refs/heads/"+b == store.Ref || b == g.Integration || b == s.config.Release {
			continue
		}
		args := []string{b}
		if g.IntegrationFound {
			args = append(args, "--not", integ)
		}
		commits, err := m.logCommits(append([]string{"-n", itoa(historyLimit)}, args...)...)
		if err != nil {
			return nil, err
		}
		branchItem := ""
		if mm := branchID.FindStringSubmatch(b); mm != nil && known(mm[1]) {
			branchItem = mm[1]
		}
		touched := map[string]bool{}
		for _, c := range commits {
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

func itoa(n int) string { return strconv.Itoa(n) }

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
