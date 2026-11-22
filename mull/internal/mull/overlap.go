package mull

import (
	"sort"
	"strings"

	"mull/internal/model"
	"mull/internal/store"
)

// Overlap is derived from git, never declared or stored: two items in
// flight whose work branches changed the same files. Each is likely to
// conflict with the other, or to make the same thing twice, when both
// land on their target.
//
// In flight is active, or in review with none of its work on its target
// yet. An item's work branches are its claim's branch, when that's a work
// branch (a local branch that isn't a target, the release branch or mull),
// and any other unmerged branch named for it ("<id>-…"). What a branch
// changed is measured from where it left its target, the merge base with
// the target's tip, so work it took in from the target since (a rebase, or
// a merge of the target) isn't counted; or from the claim's base, when that
// is later or isn't on the target (work started on another branch).

// overlapFilesShown is how many shared files a problem's text names.
const overlapFilesShown = 5

// Overlap is a pair of items in flight whose branches changed the same
// files.
type Overlap struct {
	Items [2]OverlapItem `json:"items"` // in ID order
	Files []string       `json:"files"` // changed by both, sorted
}

// OverlapItem is one side of an overlap.
type OverlapItem struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	State    string   `json:"state"`
	Plan     string   `json:"plan,omitempty"`
	Branches []string `json:"branches"` // the work branches read
}

// gitFiles caches the files a work branch changed, keyed by the commits it
// was measured from: the claim's base, the target's tip and the branch tip.
var gitFiles = newLRU[[]string](64)

// readOverlaps finds every pair of items in flight, across the project,
// whose work branches changed the same files, and keeps them in g.
func (m *Mull) readOverlaps(s *state, g *gitState) error {
	type side struct {
		item  OverlapItem
		files []string // sorted
	}
	var sides []side
	for _, id := range sortedKeys(s.items) {
		i := s.items[id]
		ig := s.gitOf(g, i)
		if i.State != model.StateActive && (i.State != model.StateInReview || ig.SeenInGit) {
			continue
		}
		branches := s.workBranches(g, i, ig)
		if len(branches) == 0 {
			continue
		}
		base := ""
		if i.Claim != nil && m.knownCommit(i.Claim.Base) {
			base = i.Claim.Base
		}
		var files []string
		for _, b := range branches {
			fs, err := m.branchFiles(base, g.Targets.tips[ig.Target], g.heads[b])
			if err != nil {
				return err
			}
			files = append(files, fs...)
		}
		if files = dedupe(files); len(files) == 0 {
			continue
		}
		sort.Strings(files)
		sides = append(sides, side{OverlapItem{ID: i.ID, Title: i.Title, State: i.State, Plan: i.Plan, Branches: branches}, files})
	}
	g.overlaps = []Overlap{}
	for a := range sides {
		for b := a + 1; b < len(sides); b++ {
			if shared := intersect(sides[a].files, sides[b].files); len(shared) > 0 {
				g.overlaps = append(g.overlaps, Overlap{Items: [2]OverlapItem{sides[a].item, sides[b].item}, Files: shared})
			}
		}
	}
	return nil
}

// workBranches are the branches item i's work is read from: its claim's
// branch, when that's a work branch, and its other unmerged branches named
// "<id>-…".
func (s *state) workBranches(g *gitState, i *model.Item, ig ItemGit) []string {
	work := func(b string) bool {
		return b != "" && g.heads[b] != "" && g.Targets.tips[b] == "" && b != s.config.Release && "refs/heads/"+b != store.Ref
	}
	var out []string
	if i.Claim != nil && work(i.Claim.Branch) {
		out = append(out, i.Claim.Branch)
	}
	for _, b := range ig.Branches {
		if mm := branchID.FindStringSubmatch(b); mm != nil && mm[1] == i.ID && work(b) {
			out = append(out, b)
		}
	}
	return dedupe(out)
}

// branchFiles lists the files changed on a work branch at tip, a commit
// ID: from its merge base with targetTip (the target's tip, "" when the
// target is missing), or from base (the claim's, "" when unknown) when that
// is later or isn't on the target. Renames list both paths. Named by commit
// IDs alone, so cached.
func (m *Mull) branchFiles(base, targetTip, tip string) ([]string, error) {
	if tip == "" {
		return nil, nil
	}
	return gitFiles.get(m.repo.Dir+"\x00"+base+" "+targetTip+" "+tip, func() ([]string, error) {
		from := base
		if targetTip != "" {
			// No common history exits non-zero: measure from base alone.
			if mb, err := m.repo.Output(nil, nil, "merge-base", targetTip, tip); err == nil && mb != "" {
				if from == "" || m.isAncestor(from, mb) {
					from = mb
				}
			}
		}
		if from == "" || from == tip {
			return nil, nil
		}
		out, err := m.repo.Output(nil, nil, "diff", "--name-only", "--no-renames", from, tip)
		if err != nil {
			return nil, err
		}
		var files []string
		for _, f := range strings.Split(out, "\n") {
			if f = strings.TrimSpace(f); f != "" {
				files = append(files, f)
			}
		}
		return files, nil
	})
}

// intersect returns the strings in both sorted lists, sorted.
func intersect(a, b []string) []string {
	var out []string
	for x, y := 0, 0; x < len(a) && y < len(b); {
		switch {
		case a[x] < b[y]:
			x++
		case a[x] > b[y]:
			y++
		default:
			out = append(out, a[x])
			x++
			y++
		}
	}
	return out
}

// overlapsOf are g's overlaps with an item in plan slug ("" for unplanned
// work), never nil.
func (g *gitState) overlapsOf(slug string) []Overlap {
	out := []Overlap{}
	for _, o := range g.overlaps {
		if o.Items[0].Plan == slug || o.Items[1].Plan == slug {
			out = append(out, o)
		}
	}
	return out
}

// overlapProblem is o as a problem on the side of it in plan slug: on the
// first item when both are in it.
func overlapProblem(o Overlap, slug string) Problem {
	me, other := o.Items[0], o.Items[1]
	if me.Plan != slug {
		me, other = other, me
	}
	shown := o.Files[:min(len(o.Files), overlapFilesShown)]
	list := strings.Join(shown, ", ")
	if more := len(o.Files) - len(shown); more > 0 {
		list += ", and " + itoa(more) + " more"
	}
	return Problem{Kind: "overlap", Item: me.ID, With: other.ID, Files: o.Files,
		Text: "shares " + plural(len(o.Files), "changed file") + " with " + other.ID + " (" + other.State + "): " + list}
}
