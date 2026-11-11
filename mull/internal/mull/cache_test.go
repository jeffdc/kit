package mull

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"mull/internal/model"
)

// fingerprint is every record of s as it would be stored, docs included.
func fingerprint(t *testing.T, s *state) string {
	t.Helper()
	var out []string
	add := func(path string, v any, body string) {
		b, err := model.Encode(v, body)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, path+"\n"+string(b))
	}
	for _, p := range s.plans {
		add(model.PlanPath(p.Slug), p, p.Summary+"\n"+p.Doc)
	}
	for _, p := range s.phases {
		add(model.PhasePath(p.ID), p, p.Summary+"\n"+p.Doc)
	}
	for _, i := range s.items {
		add(model.ItemPath(i.ID), i, i.Notes)
	}
	for _, e := range s.entries {
		add(model.EntryPath(e.ID), e, e.Detail)
	}
	sort.Strings(out)
	return strings.Join(out, "\n")
}

// Every read leaves the cached state as it was parsed, even when the caller
// changes what it was given.
func TestReadsDontChangeTheCachedState(t *testing.T) {
	m, _ := setup(t)
	f := newRankFixture(t, m)
	entryIn(t, m, model.KindDecision, "Bill monthly", f.active.ID, "", "")
	markInReview(t, m, f.active.ID)
	first, err := m.store.Head()
	if err != nil {
		t.Fatal(err)
	}
	must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Phase: f.second.ID, Title: "More work"}))

	head, err := m.headState()
	if err != nil {
		t.Fatal(err)
	}
	before := fingerprint(t, head)

	reads := map[string]func() error{
		"Overview":      func() error { _, err := m.Overview(); return err },
		"TreePage":      func() error { _, err := m.TreePage(); return err },
		"BacklogPage":   func() error { _, err := m.BacklogPage(""); return err },
		"UnplannedPage": func() error { _, err := m.UnplannedPage(false); return err },
		"Board":         func() error { _, err := m.Board(""); return err },
		"Summary":       func() error { _, err := m.Summary(); return err },
		"Decisions":     func() error { _, err := m.Decisions(""); return err },
		"ItemList":      func() error { _, err := m.ItemList(ItemFilter{}); return err },
		"PlanList":      func() error { _, err := m.PlanList(); return err },
		"PhaseList":     func() error { _, err := m.PhaseList(""); return err },
		"Context":       func() error { _, err := m.Context(f.active.ID); return err },
		"Revision":      func() error { _, err := m.Revision(first); return err },
		"Inbox": func() error {
			es, err := m.Inbox(InboxFilter{All: true})
			for _, e := range es { // a caller changing what it got
				e.Text, e.Decisions = "changed", append(e.Decisions, "ffff")
				if e.Settled != nil {
					e.Settled.Note = "changed"
				}
			}
			return err
		},
	}
	for slug := range head.plans {
		reads["PlanPage "+slug] = func() error { _, err := m.PlanPage(slug); return err }
		reads["PlanShow "+slug] = func() error { _, err := m.PlanShow(slug); return err }
	}
	for id := range head.phases {
		reads["PhasePage "+id] = func() error { _, err := m.PhasePage(id); return err }
	}
	for id := range head.items {
		reads["ItemPage "+id] = func() error { _, err := m.ItemPage(id); return err }
		reads["Show "+id] = func() error { _, err := m.Show(id); return err }
	}
	for id := range head.entries {
		reads["Show "+id] = func() error { _, err := m.Show(id); return err }
	}
	for name, read := range reads {
		if err := read(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := fingerprint(t, head); got != before {
			t.Fatalf("%s changed the cached state", name)
		}
	}
	// And the cache holds what a fresh parse gives.
	tx, err := m.store.ReadOnly(head.tx.Commit(), nil)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := load(tx)
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint(t, fresh) != before {
		t.Fatal("the cached state differs from a fresh parse")
	}
}

// A cached state's transaction refuses writes.
func TestCachedStateRefusesWrites(t *testing.T) {
	m, _ := setup(t)
	s, err := m.headState()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("writing to a cached state didn't panic")
		}
	}()
	s.tx.Put("plans/x.md", nil)
}

// A write is seen by the next read.
func TestReadSeesTheLastWrite(t *testing.T) {
	m, _ := setup(t)
	if _, err := m.ItemList(ItemFilter{}); err != nil { // cache the state before the write
		t.Fatal(err)
	}
	v := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "New"}))
	items := must[[]ItemView](t)(m.ItemList(ItemFilter{}))
	if len(items) != 1 || items[0].ID != v.ID {
		t.Fatalf("items after the write = %+v", items)
	}
}

func TestStateCache(t *testing.T) {
	loads := map[string]*atomic.Int32{"a": {}, "b": {}, "c": {}}
	c := newLRU[*state](2)
	get := func(commit string) {
		t.Helper()
		s, err := c.get(commit, func() (*state, error) {
			loads[commit].Add(1)
			return &state{}, nil
		})
		if err != nil || s == nil {
			t.Fatalf("get %s: %v, %v", commit, s, err)
		}
	}

	// Concurrent readers of one commit share a load.
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() { defer wg.Done(); get("a") }()
	}
	wg.Wait()
	if n := loads["a"].Load(); n != 1 {
		t.Fatalf("20 readers loaded a %d times", n)
	}

	// Least recently used goes first: a is used after b, so c pushes out b.
	get("b")
	get("a")
	get("c")
	get("a")
	if n := loads["a"].Load(); n != 1 {
		t.Fatalf("a reloaded (%d loads)", n)
	}
	get("b")
	if n := loads["b"].Load(); n != 2 {
		t.Fatalf("b loaded %d times, want 2 (evicted)", n)
	}

	// A failed load isn't kept.
	fail := errors.New("boom")
	calls := 0
	for range 2 {
		_, err := c.get("d", func() (*state, error) { calls++; return nil, fail })
		if !errors.Is(err, fail) {
			t.Fatalf("err = %v", err)
		}
	}
	if calls != 2 {
		t.Fatalf("failed load tried %d times, want 2", calls)
	}
}

// Pages read one cached state from many goroutines at once (run with -race).
func TestConcurrentReadsShareAState(t *testing.T) {
	m, _ := setup(t)
	f := newRankFixture(t, m)
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for range 8 {
		for _, read := range []func() error{
			func() error { _, err := m.TreePage(); return err },
			func() error { _, err := m.Overview(); return err },
			func() error { _, err := m.PlanPage("launch"); return err },
			func() error { _, err := m.PhasePage(f.first.ID); return err },
			func() error { _, err := m.ItemPage(f.active.ID); return err },
			func() error { _, err := m.Board(""); return err },
		} {
			wg.Add(1)
			go func() { defer wg.Done(); errs <- read() }()
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

// A state read as the difference from the last one is the state a fresh
// read gives, after writes of every kind: new records, changed ones, docs
// written and removed.
func TestIncrementalLoadMatchesFresh(t *testing.T) {
	m, _ := setup(t)
	check := func(step string) {
		t.Helper()
		s, err := m.headState()
		if err != nil {
			t.Fatal(err)
		}
		tx, err := m.store.ReadOnly(s.tx.Commit(), nil)
		if err != nil {
			t.Fatal(err)
		}
		fresh, err := load(tx)
		if err != nil {
			t.Fatal(err)
		}
		if fingerprint(t, s) != fingerprint(t, fresh) {
			t.Fatalf("after %s, the incremental state differs from a fresh read", step)
		}
	}
	check("setup")
	f := newRankFixture(t, m)
	check("the fixture")
	must[DocResult](t)(m.SetPlanDoc("launch", "The launch plan."))
	check("a plan doc")
	must[DocResult](t)(m.SetPhaseDoc(f.first.ID, "Billing first."))
	check("a phase doc")
	must[SetResult](t)(m.Set(f.ready.ID, "title", "Ready work, renamed"))
	check("a changed item")
	must[DocResult](t)(m.SetPlanDoc("launch", ""))
	check("a removed doc")
	entryIn(t, m, model.KindDecision, "Bill monthly", f.active.ID, "", "")
	markInReview(t, m, f.active.ID)
	check("a review")
	must[DropResult](t)(m.Drop([]string{f.q["plan"].ID}, "not needed", SettleOpts{OnRequest: true}))
	check("a settled entry")
}
