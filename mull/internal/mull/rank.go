package mull

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"mull/internal/model"
)

// Rank orders open work within its group: a phase, a plan's items with no
// phase, or Unplanned. It sorts and warns; it never holds work back. Agents
// give an order (mull rank, add --before/--after) and mull writes the
// numbers, a step apart so an item can usually be slotted between two
// without renumbering. Rank 0 is unranked: items from before ranks, which
// sort after ranked ones, oldest first, until their group is ranked.

const rankStep = 10

// group is where an item is ordered: its phase when it has one, else its
// plan's items with no phase, else Unplanned (both empty).
type group struct{ Plan, Phase string }

func groupOf(i *model.Item) group {
	if i.Phase != "" {
		return group{Plan: i.Plan, Phase: i.Phase}
	}
	return group{Plan: i.Plan}
}

func (g group) String() string {
	switch {
	case g.Phase != "":
		return "phase " + g.Phase
	case g.Plan != "":
		return fmt.Sprintf("plan %s (its items with no phase)", g.Plan)
	}
	return "Unplanned"
}

// openWork is an item a rank orders: committed, active or in review.
func openWork(i *model.Item) bool {
	switch i.State {
	case model.StateCommitted, model.StateActive, model.StateInReview:
		return true
	}
	return false
}

// rankLess orders items by rank: ranked before unranked, then by rank, then
// oldest first.
func rankLess(a, b *model.Item) bool {
	if (a.Rank == 0) != (b.Rank == 0) {
		return a.Rank != 0
	}
	if a.Rank != b.Rank {
		return a.Rank < b.Rank
	}
	return a.Created < b.Created
}

// ranked is the group's open work in order, leaving out skip.
func (s *state) ranked(g group, skip string) []*model.Item {
	var out []*model.Item
	for _, id := range sortedKeys(s.items) {
		if i := s.items[id]; id != skip && openWork(i) && groupOf(i) == g {
			out = append(out, i)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return rankLess(out[a], out[b]) })
	return out
}

// place ranks i, open work entering its group, at the group's end, or just
// before or after anchor (open work in the same group). It sets i's rank
// and leaves writing i to the caller. When there's no gap at that spot, or
// the group still holds unranked items, it renumbers the group's open work
// in order, writing the others it changes.
func (s *state) place(i *model.Item, before, after, now string) error {
	if before != "" && after != "" {
		return errf(CodeInvalid, "give --before or --after, not both")
	}
	g := groupOf(i)
	order := s.ranked(g, i.ID)
	pos := len(order)
	if anchor := before + after; anchor != "" {
		k := slices.IndexFunc(order, func(o *model.Item) bool { return o.ID == anchor })
		if k < 0 {
			return errf(CodeInvalid, "%s isn't open work in %s, so the new item can't be placed by it", anchor, g)
		}
		pos = k
		if after != "" {
			pos++
		}
	}
	lo, hi := 0, 0 // the ranks either side of the spot; 0 past an end
	if pos > 0 {
		lo = order[pos-1].Rank
	}
	if pos < len(order) {
		hi = order[pos].Rank
	}
	if !slices.ContainsFunc(order, func(o *model.Item) bool { return o.Rank == 0 }) {
		switch {
		case pos == len(order):
			i.Rank = lo + rankStep
			return nil
		case hi-lo > 1:
			i.Rank = (lo + hi) / 2
			return nil
		}
	}
	return s.renumber(slices.Insert(order, pos, i), i, now)
}

// renumber ranks order one step apart, writing each item whose rank changes
// except self, which the caller writes.
func (s *state) renumber(order []*model.Item, self *model.Item, now string) error {
	for k, o := range order {
		r := (k + 1) * rankStep
		if o.Rank == r {
			continue
		}
		o.Rank = r
		if o == self {
			continue
		}
		o.Updated = now
		if err := s.putItem(o); err != nil {
			return err
		}
	}
	return nil
}

// RankRow is an item in a ranked group.
type RankRow struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	State string `json:"state"`
	Rank  int    `json:"rank"`
}

// RankResult is a group's open work in its new order.
type RankResult struct {
	Plan  string    `json:"plan,omitempty"`  // the group's plan; neither for Unplanned
	Phase string    `json:"phase,omitempty"` // the group's phase, if it's a phase
	Items []RankRow `json:"items"`
}

// Rank sets the order of a group's open work: ref is a phase ID, a plan slug
// (its items with no phase) or none (Unplanned), and ids must list every
// open item in it, in the new order.
func (m *Mull) Rank(ref string, ids []string) (RankResult, error) {
	var out RankResult
	err := m.update(fmt.Sprintf("rank %s %v", ref, ids), func(s *state) error {
		var err error
		out, err = m.rank(s, ref, ids)
		return err
	})
	return out, err
}

// rankGroup resolves the group mull rank names.
func (s *state) rankGroup(ref string) (group, error) {
	if ref == model.NoPlan {
		return group{}, nil
	}
	e, p := s.phases[ref], s.plans[ref]
	switch {
	case e != nil && p != nil:
		return group{}, errf(CodeInvalid, "%q is both a phase ID and a plan slug, so it doesn't say which group to rank", ref)
	case e != nil:
		return group{Plan: e.Plan, Phase: e.ID}, nil
	case p != nil:
		return group{Plan: p.Slug}, nil
	}
	return group{}, errf(CodeNotFound, "no phase or plan %q (none: Unplanned)", ref)
}

func (m *Mull) rank(s *state, ref string, ids []string) (RankResult, error) {
	out := RankResult{Items: []RankRow{}}
	g, err := s.rankGroup(ref)
	if err != nil {
		return out, err
	}
	out.Plan, out.Phase = g.Plan, g.Phase
	ids = dedupe(ids)
	if len(ids) == 0 {
		return out, errf(CodeInvalid, "give the open items in %s in their new order", g)
	}
	var order []*model.Item
	for _, id := range ids {
		i, err := s.item(id)
		if err != nil {
			return out, err
		}
		if !openWork(i) || groupOf(i) != g {
			return out, errf(CodeInvalid, "%s isn't open work in %s (it's %s in %s); rank orders only committed, active and in-review items in one group", id, g, i.State, groupOf(i))
		}
		order = append(order, i)
	}
	var missing []string
	for _, i := range s.ranked(g, "") {
		if !slices.Contains(ids, i.ID) {
			missing = append(missing, i.ID)
		}
	}
	if len(missing) > 0 {
		return out, errf(CodeInvalid, "list every open item in %s: %s missing", g, strings.Join(missing, ", "))
	}
	if err := s.renumber(order, nil, m.now()); err != nil {
		return out, err
	}
	for _, i := range order {
		out.Items = append(out.Items, RankRow{ID: i.ID, Title: i.Title, State: i.State, Rank: i.Rank})
	}
	return out, nil
}

// workOrder sorts items the way the CLI lists work: by group, then by rank
// within it. Groups go plan by plan (by slug, Unplanned last); within a
// plan, its phases in schedule order (by depth of needs not yet done, then
// age, done phases last), then its items with no phase. Within a group,
// open work comes first in rank order, then the rest (captured, done and
// dropped) by rank, unranked last.
func (s *state) workOrder(items []*model.Item) {
	pos := s.phaseOrder()
	key := func(i *model.Item) (bool, string, int) {
		p, ok := pos[i.Phase]
		switch {
		case i.Phase == "":
			p = len(pos) + 1
		case !ok:
			p = len(pos) // a phase that's gone: after the plan's others
		}
		return i.Plan == "", i.Plan, p
	}
	sort.SliceStable(items, func(a, b int) bool {
		x, y := items[a], items[b]
		xu, xp, xe := key(x)
		yu, yp, ye := key(y)
		switch {
		case xu != yu:
			return yu
		case xp != yp:
			return xp < yp
		case xe != ye:
			return xe < ye
		case x.Phase != y.Phase:
			return x.Phase < y.Phase
		case openWork(x) != openWork(y):
			return openWork(x)
		}
		return rankLess(x, y)
	})
}

// phaseOrder numbers every phase by its place in its plan's schedule:
// unfinished phases by depth of needs not yet done, then age; done phases
// after them.
func (s *state) phaseOrder() map[string]int {
	type row struct {
		id      string
		done    bool
		depth   int
		created string
	}
	memo := map[string]int{}
	var rows []row
	for _, id := range sortedKeys(s.phases) {
		e := s.phases[id]
		r := row{id: id, created: e.Created, done: s.phaseComplete(id)}
		if !r.done {
			r.depth = s.depth(id, true, memo)
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(a, b int) bool {
		x, y := rows[a], rows[b]
		switch {
		case x.done != y.done:
			return y.done
		case x.depth != y.depth:
			return x.depth < y.depth
		}
		return x.created < y.created
	})
	out := make(map[string]int, len(rows))
	for k, r := range rows {
		out[r.id] = k
	}
	return out
}

// rankedAhead is the open work ranked before i in its group, for start's
// warning. Only ranks someone gave count: an unranked item, or one with an
// equal rank, has no order against i.
func (s *state) rankedAhead(i *model.Item) []*model.Item {
	if i.Rank == 0 {
		return nil
	}
	var out []*model.Item
	for _, o := range s.ranked(groupOf(i), i.ID) {
		if o.Rank != 0 && o.Rank < i.Rank {
			out = append(out, o)
		}
	}
	return out
}
