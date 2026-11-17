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
