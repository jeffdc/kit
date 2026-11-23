package mull

import (
	"fmt"
	"sort"
	"strings"

	"mull/internal/model"
)

// The queue is the captured items: discovered work and ideas, the agents'
// own. Nobody gates it. Each item may say where in the code it is (its
// area, a path or glob), and work in an area pulls in the queue there:
// context --item lists it, done lists what's near the change, and the view
// shows where queued work clusters, a sign the design there is wrong.

// clusterSize is how many queued items in one area make a cluster.
const clusterSize = 3

// areaMatches reports whether a file path lies in area, a path or glob: a
// glob matches as globs do, and a plain path matches itself and everything
// under it.
func areaMatches(area, path string) bool {
	area = strings.TrimSpace(area)
	if area == "" {
		return false
	}
	if strings.ContainsAny(area, "*?") {
		re, err := globRegexp(area)
		return err == nil && re.MatchString(path)
	}
	area = strings.TrimSuffix(area, "/")
	return path == area || strings.HasPrefix(path, area+"/")
}

// areasOverlap reports whether two areas share ground: either lies in the
// other, compared by their fixed prefixes.
func areasOverlap(a, b string) bool {
	pa, pb := areaPrefix(a), areaPrefix(b)
	if pa == "" || pb == "" {
		return false
	}
	return pa == pb || strings.HasPrefix(pa, pb+"/") || strings.HasPrefix(pb, pa+"/")
}

// areaPrefix is an area's part before any wildcard, as a directory path.
func areaPrefix(area string) string {
	area = strings.TrimSpace(area)
	if k := strings.IndexAny(area, "*?"); k >= 0 {
		area = area[:k]
	}
	return strings.TrimSuffix(area, "/")
}

// queued reports whether an item is in the queue: captured, and not covered.
func queued(i *model.Item) bool { return i.State == model.StateCaptured && i.CoveredBy == "" }

// queueNear lists the queued items whose area holds one of paths, in ID
// order, leaving out skip.
func (s *state) queueNear(paths []string, skip map[string]bool) []BriefItem {
	var out []BriefItem
	for _, id := range sortedKeys(s.items) {
		i := s.items[id]
		if !queued(i) || skip[id] || i.Area == "" {
			continue
		}
		for _, p := range paths {
			if areaMatches(i.Area, p) {
				out = append(out, BriefItem{ID: i.ID, Title: i.Title, Plan: i.Plan, Phase: i.Phase})
				break
			}
		}
	}
	return out
}

// queueFor lists the queued items that bear on item i: in an area that
// overlaps its own, or tagged to its phase.
func (s *state) queueFor(i *model.Item) []BriefItem {
	var out []BriefItem
	for _, id := range sortedKeys(s.items) {
		q := s.items[id]
		if !queued(q) || id == i.ID {
			continue
		}
		if (i.Area != "" && areasOverlap(i.Area, q.Area)) || (i.Phase != "" && q.Phase == i.Phase) {
			out = append(out, BriefItem{ID: q.ID, Title: q.Title, Plan: q.Plan, Phase: q.Phase})
		}
	}
	return out
}

// Cluster is an area where queued work piles up.
type Cluster struct {
	Area  string   `json:"area"`
	Count int      `json:"count"`
	Items []string `json:"items"`
}

// clusters lists the areas holding clusterSize or more queued items, by
// their fixed prefix, biggest first.
func (s *state) clusters() []Cluster {
	by := map[string][]string{}
	for _, id := range sortedKeys(s.items) {
		if i := s.items[id]; queued(i) && areaPrefix(i.Area) != "" {
			by[areaPrefix(i.Area)] = append(by[areaPrefix(i.Area)], id)
		}
	}
	var out []Cluster
	for area, ids := range by {
		if len(ids) >= clusterSize {
			out = append(out, Cluster{Area: area, Count: len(ids), Items: ids})
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Count != out[b].Count {
			return out[a].Count > out[b].Count
		}
		return out[a].Area < out[b].Area
	})
	return out
}

// cover marks queued or committed items as done by item by: its work
// covered them. One can't cover itself, work someone is on, or done work.
func (m *Mull) cover(s *state, by string, ids []string) ([]string, error) {
	var out []string
	for _, id := range dedupe(ids) {
		i, err := s.item(id)
		if err != nil {
			return nil, err
		}
		switch {
		case id == by:
			return nil, errf(CodeInvalid, "item %s can't cover itself", id)
		case i.State != model.StateCaptured && i.State != model.StateCommitted:
			return nil, errf(CodeState, "item %s is %s; only queued or committed work can be covered", id, i.State)
		}
		i.State, i.CoveredBy, i.Updated = model.StateDone, by, m.now()
		i.Notes = joinNotes(i.Notes, fmt.Sprintf("Covered by %s (%s).", by, i.Updated))
		if err := s.putItem(i); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// changedPaths lists every changed file's path.
func changedPaths(changes []model.LayerChanges) []string {
	var out []string
	for _, lc := range changes {
		for _, f := range lc.Files {
			out = append(out, f.Path)
		}
	}
	return out
}
