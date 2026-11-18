package mull

import (
	"fmt"
	"strings"
	"testing"

	"mull/internal/model"
)

// order is the group's open work in rank order, as "title:rank" words.
func order(t *testing.T, m *Mull, ref string) string {
	t.Helper()
	var out []string
	err := m.view(func(s *state) error {
		g, err := s.rankGroup(ref)
		if err != nil {
			return err
		}
		for _, i := range s.ranked(g, "") {
			out = append(out, fmt.Sprintf("%s:%d", i.Title, i.Rank))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(out, " ")
}

func wantOrder(t *testing.T, m *Mull, ref, want string) {
	t.Helper()
	if got := order(t, m, ref); got != want {
		t.Fatalf("order of %s = %q, want %q", ref, got, want)
	}
}

func TestAddRanksLast(t *testing.T) {
	m, _ := setup(t)
	ph := must[*model.Phase](t)(m.PhaseAdd("general", "P", "", nil))
	a := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "a"}))
	b := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "b"}))
	// Groups rank apart: the plan's items with no phase, and Unplanned.
	x := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "x"}))
	must[ItemView](t)(m.ItemAdd(NewItem{Plan: model.NoPlan, Title: "u"}))
	wantOrder(t, m, ph.ID, "a:10 b:20")
	wantOrder(t, m, "general", "x:10")
	wantOrder(t, m, model.NoPlan, "u:10")

	// --after slots it into the gap; --before at the front halves the first.
	c := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "c", After: a.ID}))
	d := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "d", Before: a.ID}))
	wantOrder(t, m, ph.ID, "d:5 a:10 c:15 b:20")

	// With no gap left, the group is renumbered in order.
	must[RankResult](t)(m.Rank(ph.ID, []string{d.ID, a.ID, c.ID, b.ID}))
	e := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "e", After: d.ID}))
	f := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "f", Before: e.ID}))
	g := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "g", After: f.ID}))
	wantOrder(t, m, ph.ID, "d:10 f:12 g:13 e:15 a:20 c:30 b:40")
	must[any](t)(m.Exec("add", &AddIn{Title: "h", Phase: ph.ID, Before: g.ID}))
	wantOrder(t, m, ph.ID, "d:10 f:20 h:30 g:40 e:50 a:60 c:70 b:80")

	// An anchor must be open work in the same group; captures take none.
	_, err := m.ItemAdd(NewItem{Phase: ph.ID, Title: "i", After: x.ID})
	wantErr(t, err, "isn't open work in phase "+ph.ID)
	_, err = m.ItemAdd(NewItem{Phase: ph.ID, Title: "i", After: a.ID, Before: b.ID})
	wantCode(t, err, CodeInvalid)
	_, err = m.Exec("add", &AddIn{Title: "i", Phase: ph.ID, After: "ffff"})
	wantCode(t, err, CodeInvalid)
	idea := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemIdea, Title: "maybe", Phase: ph.ID}))
	if idea.Rank != 0 {
		t.Fatalf("a captured item has rank %d", idea.Rank)
	}
}

func TestRank(t *testing.T) {
	m, _ := setup(t)
	ph := must[*model.Phase](t)(m.PhaseAdd("general", "P", "", nil))
	other := must[*model.Phase](t)(m.PhaseAdd("general", "Other", "", nil))
	a := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "a"}))
	b := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "b"}))
	c := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "c"}))
	done := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "dropped"}))
	must[DropResult](t)(m.Drop([]string{done.ID}, "", SettleOpts{}))
	elsewhere := must[ItemView](t)(m.ItemAdd(NewItem{Phase: other.ID, Title: "o"}))

	res := must[RankResult](t)(m.Rank(ph.ID, []string{c.ID, a.ID, b.ID}))
	if res.Phase != ph.ID || res.Plan != "general" || len(res.Items) != 3 || res.Items[0].ID != c.ID || res.Items[0].Rank != 10 {
		t.Fatalf("rank = %+v", res)
	}
	wantOrder(t, m, ph.ID, "c:10 a:20 b:30")
	if v := item(t, m, done.ID); v.Rank != 40 {
		t.Fatalf("a dropped item keeps its rank: got %d", v.Rank)
	}

	// Strict: every open item in the group, and only those (ed34).
	_, err := m.Rank(ph.ID, []string{a.ID, b.ID})
	wantErr(t, err, c.ID+" missing")
	_, err = m.Rank(ph.ID, []string{a.ID, b.ID, c.ID, done.ID})
	wantErr(t, err, done.ID+" isn't open work")
	_, err = m.Rank(ph.ID, []string{a.ID, b.ID, c.ID, elsewhere.ID})
	wantErr(t, err, elsewhere.ID+" isn't open work in phase "+ph.ID)
	_, err = m.Rank(ph.ID, nil)
	wantCode(t, err, CodeInvalid)
	_, err = m.Rank("nope", []string{a.ID})
	wantCode(t, err, CodeNotFound)

	// The batch op does the same, and active work is ranked too.
	must[StartResult](t)(m.Start(b.ID))
	must[any](t)(m.Exec("rank", &RankIn{Group: ph.ID, IDs: IDList{b.ID, c.ID, a.ID}}))
	wantOrder(t, m, ph.ID, "b:10 c:20 a:30")
}

func TestUnrankedAreRankedOnFirstPlacement(t *testing.T) {
	m, _ := setup(t)
	ph := must[*model.Phase](t)(m.PhaseAdd("general", "P", "", nil))
	a := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "a"}))
	b := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "b"}))
	// Data from before ranks: no rank on either.
	err := m.update("unrank", func(s *state) error {
		for k, id := range []string{a.ID, b.ID} {
			i := s.items[id]
			i.Rank, i.Created = 0, fmt.Sprintf("2026-01-0%dT00:00:00Z", k+1)
			if err := s.putItem(i); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	wantOrder(t, m, ph.ID, "a:0 b:0")
	// A new item goes last, after them, so they're ranked in their order.
	must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "c"}))
	wantOrder(t, m, ph.ID, "a:10 b:20 c:30")
}

func TestJoiningAGroupRanksLast(t *testing.T) {
	m, _ := setup(t)
	ph := must[*model.Phase](t)(m.PhaseAdd("general", "P", "", nil))
	other := must[*model.Phase](t)(m.PhaseAdd("general", "Other", "", nil))
	must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "a"}))
	must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "b"}))

	// Committing a capture.
	idea := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemIdea, Title: "idea", Phase: ph.ID}))
	must[ItemView](t)(m.Commit(CommitInput{ID: idea.ID, OnRequest: true}))
	wantOrder(t, m, ph.ID, "a:10 b:20 idea:30")

	// Starting a capture commits it.
	bug := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "bug", Phase: ph.ID}))
	must[StartResult](t)(m.Start(bug.ID))
	wantOrder(t, m, ph.ID, "a:10 b:20 idea:30 bug:40")

	// Moving to another phase, or out of any phase.
	must[ItemView](t)(m.ItemAdd(NewItem{Phase: other.ID, Title: "o"}))
	must[SetResult](t)(m.Set(idea.ID, "phase", other.ID))
	wantOrder(t, m, other.ID, "o:10 idea:20")
	must[SetResult](t)(m.Set(idea.ID, "phase", ""))
	wantOrder(t, m, "general", "idea:10")
	// Setting the phase it's already in leaves its rank alone.
	must[SetResult](t)(m.Set(bug.ID, "phase", ph.ID))
	wantOrder(t, m, ph.ID, "a:10 b:20 bug:40")
}

func titles[T any](rows []T, title func(T) string) string {
	var out []string
	for _, r := range rows {
		out = append(out, title(r))
	}
	return strings.Join(out, " ")
}

func viewTitle(v ItemView) string { return v.Title }

func TestListsFollowRank(t *testing.T) {
	m, _ := setup(t)
	// later is older but needs first, so the schedule puts first ahead.
	later := must[*model.Phase](t)(m.PhaseAdd("general", "Later", "", nil))
	first := must[*model.Phase](t)(m.PhaseAdd("general", "First", "", nil))
	must[NeedResult](t)(m.Need(later.ID, []string{first.ID}))
	add := func(in NewItem) string { t.Helper(); return must[ItemView](t)(m.ItemAdd(in)).ID }
	add(NewItem{Phase: later.ID, Title: "z1"})
	add(NewItem{Phase: later.ID, Title: "z2"})
	a := add(NewItem{Phase: first.ID, Title: "a"})
	b := add(NewItem{Phase: first.ID, Title: "b"})
	c := add(NewItem{Phase: first.ID, Title: "c"})
	gone := add(NewItem{Phase: first.ID, Title: "gone"})
	must[DropResult](t)(m.Drop([]string{gone}, "not needed", SettleOpts{OnRequest: true}))
	must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemIdea, Title: "cap", Phase: first.ID}))
	add(NewItem{Plan: "general", Title: "u"})
	add(NewItem{Plan: model.NoPlan, Title: "n"})
	agreedPlan(t, m, "alpha", "Alpha", "")
	add(NewItem{Plan: "alpha", Title: "al"})
	must[RankResult](t)(m.Rank(first.ID, []string{c, a, b}))

	// Plans by slug, Unplanned last; phases by schedule, then the plan's
	// items with no phase; open work by rank, then the rest.
	all := must[[]ItemView](t)(m.ItemList(ItemFilter{}))
	if got, want := titles(all, viewTitle), "al c a b gone cap z1 z2 u n"; got != want {
		t.Fatalf("list = %q, want %q", got, want)
	}
	ready := must[[]ItemView](t)(m.ItemList(ItemFilter{ReadyOnly: true}))
	if got, want := titles(ready, viewTitle), "al c a b u n"; got != want {
		t.Fatalf("ready = %q, want %q", got, want)
	}
	ctx := must[Context](t)(m.Context(""))
	if got, want := titles(ctx.Ready, func(b BriefItem) string { return b.Title }), "al c a b u n"; got != want {
		t.Fatalf("context ready = %q, want %q", got, want)
	}
	shown := must[Shown](t)(m.Show(first.ID))
	if got, want := titles(shown.Phase.Items, viewTitle), "c a b gone cap"; got != want {
		t.Fatalf("show phase = %q, want %q", got, want)
	}
	shown = must[Shown](t)(m.Show("general"))
	if got, want := titles(shown.Plan.Items, viewTitle), "c a b gone cap z1 z2 u"; got != want {
		t.Fatalf("show plan = %q, want %q", got, want)
	}

	// Once first is done, later can start, and its work joins the ready list.
	for _, id := range []string{c, a, b} {
		finish(t, m, id)
	}
	ready = must[[]ItemView](t)(m.ItemList(ItemFilter{ReadyOnly: true}))
	if got, want := titles(ready, viewTitle), "al z1 z2 u n"; got != want {
		t.Fatalf("ready = %q, want %q", got, want)
	}
}

func TestBoardInFlightFollowsRank(t *testing.T) {
	m, _ := setup(t)
	ph := must[*model.Phase](t)(m.PhaseAdd("general", "P", "", nil))
	var ids []string
	for _, title := range []string{"a", "b", "c"} {
		ids = append(ids, must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: title})).ID)
	}
	must[RankResult](t)(m.Rank(ph.ID, []string{ids[2], ids[0], ids[1]}))
	for _, id := range ids {
		must[StartResult](t)(m.Start(id))
	}
	v := must[View](t)(m.Board("general"))
	if got, want := titles(v.Plans[0].InFlight, func(f FlightLine) string { return f.Title }), "c a b"; got != want {
		t.Fatalf("in flight = %q, want %q", got, want)
	}
}

func TestStartWarnsAheadOfRank(t *testing.T) {
	m, _ := setup(t)
	ph := must[*model.Phase](t)(m.PhaseAdd("general", "P", "", nil))
	add := func(title string) string {
		t.Helper()
		return must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: title})).ID
	}
	a, b, c, d := add("a"), add("b"), add("c"), add("d")
	must[RankResult](t)(m.Rank(ph.ID, []string{c, a, b, d}))

	// The first in rank starts quietly.
	if r := must[StartResult](t)(m.Start(c)); len(r.Warnings) != 0 {
		t.Fatalf("warnings = %v", r.Warnings)
	}
	// Starting later work warns, naming every open item ranked before it.
	r := must[StartResult](t)(m.Start(b))
	if len(r.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one", r.Warnings)
	}
	for _, want := range []string{c, a, `"c" (active)`, `"a" (committed)`, "are ranked before it", "phase " + ph.ID} {
		if !strings.Contains(r.Warnings[0], want) {
			t.Fatalf("warning %q doesn't say %q", r.Warnings[0], want)
		}
	}
	// Finished work no longer comes first.
	finish(t, m, c)
	r = must[StartResult](t)(m.Start(d))
	if len(r.Warnings) != 1 || strings.Contains(r.Warnings[0], c) || !strings.Contains(r.Warnings[0], a+` "a" (committed), `+b) {
		t.Fatalf("warnings = %v, want a and b only", r.Warnings)
	}

	// A capture started now joins last, but nobody ranked it: no warning.
	bug := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "bug", Phase: ph.ID}))
	if r := must[StartResult](t)(m.Start(bug.ID)); len(r.Warnings) != 0 {
		t.Fatalf("warnings = %v", r.Warnings)
	}

	// Equal ranks, and unranked items, have no order between them.
	other := must[*model.Phase](t)(m.PhaseAdd("general", "Other", "", nil))
	x := must[ItemView](t)(m.ItemAdd(NewItem{Phase: other.ID, Title: "x"})).ID
	y := must[ItemView](t)(m.ItemAdd(NewItem{Phase: other.ID, Title: "y"})).ID
	z := must[ItemView](t)(m.ItemAdd(NewItem{Phase: other.ID, Title: "z"})).ID
	err := m.update("rerank", func(s *state) error {
		for id, rank := range map[string]int{x: 10, y: 10, z: 0} {
			i := s.items[id]
			i.Rank = rank
			if err := s.putItem(i); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{y, z} {
		if r := must[StartResult](t)(m.Start(id)); len(r.Warnings) != 0 {
			t.Fatalf("starting %s: warnings = %v", id, r.Warnings)
		}
	}
}

func rowTitle(r ItemRow) string { return fmt.Sprintf("%s:%d", r.Title, r.Position) }

func TestServePagesFollowRank(t *testing.T) {
	m, _ := setup(t)
	ph := must[*model.Phase](t)(m.PhaseAdd("general", "P", "", nil))
	add := func(in NewItem) string { t.Helper(); return must[ItemView](t)(m.ItemAdd(in)).ID }
	a := add(NewItem{Phase: ph.ID, Title: "a"})
	b := add(NewItem{Phase: ph.ID, Title: "b"})
	c := add(NewItem{Phase: ph.ID, Title: "c"})
	d := add(NewItem{Phase: ph.ID, Title: "d"})
	gone := add(NewItem{Phase: ph.ID, Title: "gone"})
	must[DropResult](t)(m.Drop([]string{gone}, "not needed", SettleOpts{OnRequest: true}))
	must[RankResult](t)(m.Rank(ph.ID, []string{c, a, b, d}))
	must[StartResult](t)(m.Start(b))
	must[StartResult](t)(m.Start(d))
	finish(t, m, a)

	// Open work in rank order, numbered among itself; finished work after,
	// with no number.
	page := must[PhasePage](t)(m.PhasePage(ph.ID))
	if got, want := titles(page.Items, rowTitle), "c:1 b:2 d:3 a:0 gone:0"; got != want {
		t.Fatalf("phase items = %q, want %q", got, want)
	}
	plan := must[PlanPage](t)(m.PlanPage("general"))
	if got, want := titles(plan.Next[0].Items, rowTitle), "c:1 b:2 d:3"; got != want {
		t.Fatalf("what's next = %q, want %q", got, want)
	}
	if got, want := titles(plan.Status.Agents, func(r AgentRow) string { return r.Title }), "b d"; got != want {
		t.Fatalf("agents = %q, want %q", got, want)
	}
	tree := must[Tree](t)(m.TreePage())
	if got, want := titles(tree.Plans[0].Phases[0].Items, func(r TreeItem) string { return r.Title }), "c b d a gone"; got != want {
		t.Fatalf("tree = %q, want %q", got, want)
	}

	// Equal ranks share a position; unranked items (from before ranks) have
	// none and come last, as do the plan's unfiled and Unplanned lists.
	u1 := add(NewItem{Plan: model.NoPlan, Title: "u1"})
	u2 := add(NewItem{Plan: model.NoPlan, Title: "u2"})
	u3 := add(NewItem{Plan: model.NoPlan, Title: "u3"})
	u4 := add(NewItem{Plan: model.NoPlan, Title: "u4"})
	err := m.update("rerank", func(s *state) error {
		for k, id := range []string{u1, u2, u3, u4} {
			i := s.items[id]
			i.Rank, i.Created = []int{20, 10, 10, 0}[k], fmt.Sprintf("2026-01-0%dT00:00:00Z", k+1)
			if err := s.putItem(i); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	un := must[Unplanned](t)(m.UnplannedPage(false))
	if got, want := titles(un.Items, rowTitle), "u2:1 u3:1 u1:3 u4:0"; got != want {
		t.Fatalf("unplanned = %q, want %q", got, want)
	}
}
