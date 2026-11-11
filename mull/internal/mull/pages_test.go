package mull

import (
	"reflect"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
)

// entryIn files an entry with any scope, plan "" included (a project-wide
// entry, as mull migrate leaves the old general plan's).
func entryIn(t *testing.T, m *Mull, kind, text, item, plan, source string) *model.Entry {
	t.Helper()
	var out *model.Entry
	err := m.update("test entry", func(s *state) error {
		var err error
		out, err = m.newEntry(s, kind, text, item, plan, source, "")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func tierIDs(n NeedsYouTiers) map[string][]string {
	out := map[string][]string{}
	for _, t := range n.Tiers {
		for _, e := range t.Entries {
			out[t.Key] = append(out[t.Key], e.ID)
		}
		for _, g := range t.Groups {
			for _, e := range g.Entries {
				out[t.Key] = append(out[t.Key], e.ID)
			}
		}
	}
	return out
}

// rankFixture is a plan with three phases in a row (first ← second ←
// third) and work in each stage, and one of each kind of question.
type rankFixture struct {
	first, second, third         *model.Phase
	ready, active, captured      ItemView // in the first phase
	waiting                      ItemView // committed, in the second
	unplannedReady, unplannedBug ItemView
	q                            map[string]*model.Entry
}

func newRankFixture(t *testing.T, m *Mull) rankFixture {
	t.Helper()
	agreedPlan(t, m, "launch", "Public launch", "")
	var f rankFixture
	f.first = must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "", nil))
	f.second = must[*model.Phase](t)(m.PhaseAdd("launch", "Invoices", "", []string{f.first.ID}))
	f.third = must[*model.Phase](t)(m.PhaseAdd("launch", "Go live", "", []string{f.second.ID}))
	add := func(phase, title string) ItemView {
		return must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Phase: phase, Title: title}))
	}
	f.ready = add(f.first.ID, "Ready work")
	f.active = add(f.first.ID, "Active work")
	must[StartResult](t)(m.Start(f.active.ID))
	f.captured = must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemIdea, Title: "Later idea", Plan: "launch", Phase: f.first.ID}))
	f.waiting = add(f.second.ID, "Waiting work")
	f.unplannedReady = must[ItemView](t)(m.ItemAdd(NewItem{Plan: model.NoPlan, Title: "Unplanned fix"}))
	f.unplannedBug = must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Footer bug"}))

	q := func(text, item, plan string) *model.Entry {
		return entryIn(t, m, model.KindQuestion, text, item, plan, "")
	}
	f.q = map[string]*model.Entry{
		"ready":     q("On ready work?", f.ready.ID, ""),
		"active":    q("On active work?", f.active.ID, ""),
		"captured":  q("On a captured item in a phase that can start?", f.captured.ID, ""),
		"waiting":   q("On work in a later phase?", f.waiting.ID, ""),
		"plan":      q("About the whole plan?", "", "launch"),
		"project":   q("About the whole project?", "", ""),
		"unplanned": q("On unplanned ready work?", f.unplannedReady.ID, ""),
		"bug":       q("On a captured unplanned bug?", f.unplannedBug.ID, ""),
	}
	return f
}

func TestNeedsYouRanking(t *testing.T) {
	m, _ := setup(t)
	f := newRankFixture(t, m)
	reviewed := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Phase: f.first.ID, Title: "Finished work"}))
	markInReview(t, m, reviewed.ID)

	// Decisions: two from one item (one carried by a review), one with only
	// a source, one about the plan.
	d1 := entryIn(t, m, model.KindDecision, "Chose A", f.active.ID, "", "")
	d2 := entryIn(t, m, model.KindDecision, "Chose B", f.active.ID, "", "")
	d3 := entryIn(t, m, model.KindDecision, "Chose C", "", "launch", "intake")
	d4 := entryIn(t, m, model.KindDecision, "Chose D", "", "launch", "")
	carrier := entryIn(t, m, model.KindReview, "Review: Active work", f.active.ID, "", "")
	err := m.update("carry", func(s *state) error {
		e := s.entries[carrier.ID]
		e.Decisions = []string{d1.ID, d2.ID}
		return s.putEntry(e)
	})
	if err != nil {
		t.Fatal(err)
	}

	// Not listed: parked, settled, and a retired kind.
	parked := entryIn(t, m, model.KindQuestion, "Parked?", "", "launch", "")
	must[*model.Entry](t)(m.Park(parked.ID, "later", SettleOpts{}))
	settled := entryIn(t, m, model.KindQuestion, "Settled?", "", "launch", "")
	must[*model.Entry](t)(m.Decide(settled.ID, "yes", SettleOpts{OnRequest: true}))
	must[*model.Entry](t)(m.captureLegacy(CaptureInput{Kind: model.KindLoose, Text: "Old", Plan: "launch"}))

	var n NeedsYouTiers
	must[struct{}](t)(struct{}{}, m.view(func(s *state) error { n = s.needsYou(nil); return nil }))

	var keys []string
	var ranks []int
	for _, tr := range n.Tiers {
		keys = append(keys, tr.Key)
		ranks = append(ranks, tr.Rank)
	}
	if want := []string{TierBlocking, TierReview, TierNow, TierLater, TierPlan, TierDecide}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("tiers = %v, want %v", keys, want)
	}
	if !reflect.DeepEqual(ranks, []int{1, 2, 3, 4, 5, 6}) {
		t.Errorf("ranks = %v", ranks)
	}
	got := tierIDs(n)
	want := map[string][]string{
		TierBlocking: {f.q["ready"].ID, f.q["active"].ID, f.q["unplanned"].ID},
		TierReview:   nil, // checked below: the two reviews, oldest first
		TierNow:      {f.q["captured"].ID},
		TierLater:    {f.q["waiting"].ID},
		TierPlan:     {f.q["plan"].ID, f.q["project"].ID, f.q["bug"].ID},
		TierDecide:   {d1.ID, d2.ID, d3.ID, d4.ID},
	}
	for k, ids := range want {
		if k == TierReview {
			continue
		}
		if !sameSet(got[k], ids) {
			t.Errorf("tier %s = %v, want %v", k, got[k], ids)
		}
	}
	if len(got[TierReview]) != 2 {
		t.Errorf("reviews = %v", got[TierReview])
	}
	if n.Open != 3+2+1+1+3+4 || n.Parked != 1 {
		t.Errorf("open = %d, parked = %d", n.Open, n.Parked)
	}

	// Rows say where they are.
	for _, tr := range n.Tiers {
		for _, e := range tr.Entries {
			if e.ID == f.q["ready"].ID && (e.Phase != f.first.ID || e.PhaseTitle != "Billing" || e.ItemTitle != "Ready work" ||
				e.ItemState != model.StateCommitted || !e.ItemReady || e.PlanTitle != "Public launch") {
				t.Errorf("ready question row = %+v", e)
			}
		}
	}

	// Decisions group by source. (Entries made in the same second order by
	// ID, so look the groups up by key.)
	groups := map[string]DecisionGroup{}
	for _, g := range n.Tiers[5].Groups {
		groups[g.Key] = g
	}
	if len(groups) != 3 {
		t.Fatalf("decision groups = %+v", n.Tiers[5].Groups)
	}
	if g := groups["item:"+f.active.ID]; g.Title != "Active work" || g.Item != f.active.ID || g.Review != carrier.ID || len(g.Entries) != 2 {
		t.Errorf("item group = %+v", g)
	}
	if g := groups["source:intake"]; g.Title != "intake" || g.Review != "" || len(g.Entries) != 1 {
		t.Errorf("source group = %+v", g)
	}
	if g := groups["plan:launch"]; g.Title != "Public launch" || len(g.Entries) != 1 {
		t.Errorf("plan group = %+v", g)
	}
	if n.Tiers[5].Count != 4 {
		t.Errorf("decide count = %d", n.Tiers[5].Count)
	}

	// Blocking counts the first tier, in the summary and the tree.
	sum := must[Summary](t)(m.Summary())
	if sum.Blocking != 3 {
		t.Errorf("summary blocking = %d, want 3", sum.Blocking)
	}
	tree := must[Tree](t)(m.TreePage())
	if tree.Blocking != 3 {
		t.Errorf("tree blocking = %d", tree.Blocking)
	}

	// Answering moves work up: once the first phase is done, the second can
	// start and its committed work is ready, so its question blocks.
	err = m.update("finish first", func(s *state) error {
		for _, i := range s.phaseItems(f.first.ID) {
			if i.State != model.StateCaptured {
				i.State = model.StateDone
				if err := s.putItem(i); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	must[struct{}](t)(struct{}{}, m.view(func(s *state) error { n = s.needsYou(nil); return nil }))
	got = tierIDs(n)
	if !contains(got[TierBlocking], f.q["waiting"].ID) {
		t.Errorf("after the first phase: blocking = %v, want the second phase's question in it", got[TierBlocking])
	}
	if !contains(got[TierNow], f.q["captured"].ID) || !contains(got[TierNow], f.q["active"].ID) {
		t.Errorf("after the first phase: now = %v (questions on done work in a done phase wait for nothing)", got[TierNow])
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, v := range seen {
		if v != 0 {
			return false
		}
	}
	return true
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestPlanStatusNowNextGoLive(t *testing.T) {
	m, dir := setup(t)
	agreedPlan(t, m, "launch", "Public launch", "")
	phase := func(title string, needs ...string) *model.Phase {
		return must[*model.Phase](t)(m.PhaseAdd("launch", title, "", needs))
	}
	add := func(p *model.Phase, title string) ItemView {
		return must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Phase: p.ID, Title: title}))
	}
	built := phase("Groundwork")
	a := phase("Billing", built.ID)
	side := phase("Docs")
	b := phase("Invoices", a.ID)
	far := phase("Reports", b.ID) // two steps away: neither now nor next
	golive := phase("Go live", far.ID, side.ID)

	g1 := add(built, "Ground")
	must[StartResult](t)(m.Start(g1.ID))
	markInReview(t, m, g1.ID)
	must[AcceptResult](t)(m.Accept([]string{g1.ID}, "", SettleOpts{OnRequest: true}))
	working := add(a, "Charge cards")
	add(a, "Refunds")
	add(b, "Invoice PDF")
	must[StartResult](t)(m.Start(working.ID))
	gittest.Git(t, dir, "checkout", "-q", "-b", working.ID+"-cards")
	gittest.Commit(t, dir, "cards", "1", "Cards")
	gittest.Git(t, dir, "checkout", "-q", "main")
	entryIn(t, m, model.KindQuestion, "Which processor?", working.ID, "", "")

	page := must[PlanPage](t)(m.PlanPage("launch"))
	st := page.Status
	ids := func(rows []PhaseRow) []string {
		var out []string
		for _, r := range rows {
			out = append(out, r.ID)
		}
		return out
	}
	if st.Phases != 6 || st.Built != 1 {
		t.Errorf("phases = %d, built = %d", st.Phases, st.Built)
	}
	if !sameSet(ids(st.Now), []string{a.ID, side.ID}) {
		t.Errorf("now = %v, want billing and docs", ids(st.Now))
	}
	if !sameSet(ids(st.Next), []string{b.ID}) {
		t.Errorf("next = %v, want invoices (one step away; reports is two, go live is its own line)", ids(st.Next))
	}
	if st.GoLive == nil || st.GoLive.ID != golive.ID || st.GoLive.State != PhaseWaits {
		t.Fatalf("go live = %+v", st.GoLive)
	}
	if !sameSet(refIDs(st.GoLive.WaitsFor), []string{far.ID, side.ID}) {
		t.Errorf("go live waits for %v", st.GoLive.WaitsFor)
	}
	if st.Questions != 1 || st.Blocking != 1 {
		t.Errorf("questions = %d, blocking = %d", st.Questions, st.Blocking)
	}
	for _, r := range st.Now {
		if r.ID == a.ID && (r.Blocking != 1 || r.Questions != 1 || r.Progress.Total != 2 || r.Progress.Active != 1) {
			t.Errorf("billing row = %+v", r)
		}
	}
	if len(st.Agents) != 1 || st.Agents[0].ID != working.ID || st.Agents[0].PhaseTitle != "Billing" ||
		st.Agents[0].Commits != 1 || st.Agents[0].Claim == nil {
		t.Errorf("agents = %+v", st.Agents)
	}

	// The schedule: done, then a step per depth of unfinished needs.
	var steps []string
	for _, g := range page.Schedule {
		steps = append(steps, g.Key)
	}
	if want := []string{"done", "now", "then", "then", "go_live"}; !reflect.DeepEqual(steps, want) {
		t.Fatalf("schedule = %v, want %v", steps, want)
	}
	if !sameSet(ids(page.Schedule[1].Phases), []string{a.ID, side.ID}) || ids(page.Schedule[2].Phases)[0] != b.ID ||
		ids(page.Schedule[3].Phases)[0] != far.ID || ids(page.Schedule[4].Phases)[0] != golive.ID {
		t.Errorf("schedule = %+v", page.Schedule)
	}
	if len(page.Next) != 2 {
		t.Fatalf("what's next = %+v", page.Next)
	}
	for _, n := range page.Next {
		if n.ID == a.ID && len(n.Items) != 2 {
			t.Errorf("billing's open items = %+v", n.Items)
		}
	}
	if page.NeedsYou.Open != 1 || page.NeedsYou.Tiers[0].Key != TierBlocking {
		t.Errorf("plan needs you = %+v", page.NeedsYou)
	}

	// A plan whose phases need nothing, or that ends in two at once, has no go live.
	agreedPlan(t, m, "flat", "Flat", "")
	must[*model.Phase](t)(m.PhaseAdd("flat", "One", "", nil))
	must[*model.Phase](t)(m.PhaseAdd("flat", "Two", "", nil))
	if p := must[PlanPage](t)(m.PlanPage("flat")); p.Status.GoLive != nil || len(p.Status.Now) != 2 {
		t.Errorf("flat plan: go live %+v, now %d", p.Status.GoLive, len(p.Status.Now))
	}
	agreedPlan(t, m, "fork", "Fork", "")
	root := must[*model.Phase](t)(m.PhaseAdd("fork", "Root", "", nil))
	must[*model.Phase](t)(m.PhaseAdd("fork", "Left", "", []string{root.ID}))
	must[*model.Phase](t)(m.PhaseAdd("fork", "Right", "", []string{root.ID}))
	if p := must[PlanPage](t)(m.PlanPage("fork")); p.Status.GoLive != nil || len(p.Status.Next) != 2 {
		t.Errorf("forked plan: go live %+v, next %d", p.Status.GoLive, len(p.Status.Next))
	}

	wantCode(t, func() error { _, err := m.PlanPage("nope"); return err }(), CodeNotFound)
}

func refIDs(refs []PhaseRef) []string {
	var out []string
	for _, r := range refs {
		out = append(out, r.ID)
	}
	return out
}

func TestOverviewTreeAndLists(t *testing.T) {
	m, _ := setup(t)
	f := newRankFixture(t, m)
	must[*model.Plan](t)(m.PlanNew("ideas", "Ideas", "", true)) // shaping: listed after the active plans
	must[StartResult](t)(m.Start(f.unplannedReady.ID))
	must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Typo", Category: "docs", Plan: "launch"}))
	must[TriageResult](t)(m.Triage(TriageInput{IDs: []string{f.unplannedBug.ID}, OnRequest: true}))

	ov := must[Overview](t)(m.Overview())
	var slugs []string
	for _, p := range ov.Plans {
		slugs = append(slugs, p.Slug)
	}
	if !reflect.DeepEqual(slugs, []string{"general", "launch", "ideas"}) {
		t.Errorf("plans = %v", slugs)
	}
	if ov.Counts.Unplanned != 1 || ov.Counts.Backlog != 3 || ov.Counts.Untriaged != 2 || ov.Counts.LastTriage == "" {
		t.Errorf("counts = %+v", ov.Counts)
	}
	if len(ov.Agents) != 2 || ov.Summary.Agents != 2 || ov.Summary.Blocking != 3 {
		t.Errorf("agents = %+v, summary = %+v", ov.Agents, ov.Summary)
	}
	if ov.NeedsYou.Open != 8 {
		t.Errorf("needs you open = %d", ov.NeedsYou.Open)
	}

	tree := must[Tree](t)(m.TreePage())
	if len(tree.Plans) != 3 || tree.Plans[1].Slug != "launch" || len(tree.Plans[1].Phases) != 3 {
		t.Fatalf("tree = %+v", tree.Plans)
	}
	first := tree.Plans[1].Phases[0]
	if first.ID != f.first.ID || first.State != PhaseNow || first.Blocking != 2 || len(first.Items) != 2 {
		t.Errorf("tree's first phase = %+v (captured items are backlog, not listed)", first)
	}
	if first.Items[0].ID != f.active.ID || !first.Items[1].Ready {
		t.Errorf("tree items = %+v (active first)", first.Items)
	}
	if tree.Plans[1].Blocking != 2 || tree.Counts != ov.Counts {
		t.Errorf("tree plan blocking = %d, counts = %+v", tree.Plans[1].Blocking, tree.Counts)
	}

	un := must[Unplanned](t)(m.UnplannedPage())
	if len(un.Items) != 1 || un.Items[0].ID != f.unplannedReady.ID || un.Items[0].Questions != 1 || un.NeedsYou.Open != 1 {
		t.Errorf("unplanned = %+v", un)
	}

	bl := must[Backlog](t)(m.BacklogPage(""))
	if len(bl.Items) != 3 || bl.Bugs != 2 || bl.Ideas != 1 || bl.Untriaged != 2 || bl.LastTriage == "" {
		t.Fatalf("backlog = %+v", bl)
	}
	if bl.Items[2].ID != f.unplannedBug.ID {
		t.Errorf("backlog order = %+v (untriaged first)", bl.Items)
	}
	if !reflect.DeepEqual(bl.Categories, []string{"", "docs"}) || len(bl.Plans) != 1 || bl.Plans[0].Slug != "launch" {
		t.Errorf("facets: categories %q, plans %+v", bl.Categories, bl.Plans)
	}
	if b := must[Backlog](t)(m.BacklogPage("launch")); len(b.Items) != 2 {
		t.Errorf("launch backlog = %+v", b.Items)
	}
	if b := must[Backlog](t)(m.BacklogPage(model.NoPlan)); len(b.Items) != 1 || b.Items[0].ID != f.unplannedBug.ID {
		t.Errorf("untagged backlog = %+v", b.Items)
	}
	wantCode(t, func() error { _, err := m.BacklogPage("nope"); return err }(), CodeNotFound)

	ph := must[PhasePage](t)(m.PhasePage(f.second.ID))
	if ph.PlanTitle != "Public launch" || ph.Status.State != PhaseWaits || len(ph.Items) != 1 || len(ph.Captured) != 0 ||
		len(ph.NeededBy) != 1 || ph.NeededBy[0].ID != f.third.ID || ph.NeedsYou.Open != 1 {
		t.Errorf("phase page = %+v", ph)
	}
	if ph := must[PhasePage](t)(m.PhasePage(f.first.ID)); len(ph.Captured) != 1 || len(ph.Items) != 2 {
		t.Errorf("first phase page: items %+v, captured %+v", ph.Items, ph.Captured)
	}
	if ph := must[PhasePage](t)(m.PhasePage(f.third.ID)); !ph.Status.GoLive {
		t.Errorf("third phase is the plan's go live: %+v", ph.Status)
	}
	wantCode(t, func() error { _, err := m.PhasePage("ffff"); return err }(), CodeNotFound)
}

func TestDocRefs(t *testing.T) {
	m, _ := setup(t)
	f := newRankFixture(t, m)
	doc := "Billing first {{" + f.first.ID + "}}, then " + f.second.ID + " and " + f.first.ID + " again; item " + f.ready.ID + "; not a record: beef."
	must[DocResult](t)(m.SetPlanDoc("launch", doc))
	page := must[PlanPage](t)(m.PlanPage("launch"))
	if page.Plan.Doc != doc {
		t.Errorf("doc = %q", page.Plan.Doc)
	}
	var got []string
	for _, r := range page.DocRefs {
		got = append(got, r.Type+":"+r.ID+":"+r.State)
	}
	want := []string{"phase:" + f.first.ID + ":now", "phase:" + f.second.ID + ":waits", "item:" + f.ready.ID + ":committed"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("doc refs = %v, want %v", got, want)
	}
}

func TestItemPageReviewPacket(t *testing.T) {
	m, dir := setup(t)
	check, _ := counterCheck(t, 0)
	must[model.Config](t)(m.ConfigChecks([]string{check}))
	it := startWork(t, m, dir, "Invoices page")
	gittest.Commit(t, dir, "lib/invoice.go", "package x\n", "Model", "Mull: "+it.ID)
	keep := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindDecision, Text: "Plain structs", Item: it.ID}))
	other := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindDecision, Text: "No cache", Item: it.ID}))
	q := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "Which currency?", Item: it.ID}))

	page := must[ItemPage](t)(m.ItemPage(it.ID))
	if page.Review != nil || len(page.Reports) != 0 || page.Git.Landed != 1 || page.PlanTitle != "General" {
		t.Errorf("before done: %+v", page)
	}

	res := must[DoneResult](t)(m.Done(it.ID, DoneInput{Summary: "Adds invoices.", Design: "Plain structs."}))
	must[*model.Entry](t)(m.Decide(other.ID, "Cache for a minute", SettleOpts{OnRequest: true}))

	page = must[ItemPage](t)(m.ItemPage(it.ID))
	r := page.Review
	if r == nil || r.Entry == nil || r.Entry.ID != res.Review || r.Report.N != 1 || r.Report.Summary != "Adds invoices." || r.Report.Design != "Plain structs." {
		t.Fatalf("review = %+v", r)
	}
	if len(r.Decisions) != 2 {
		t.Fatalf("decisions = %+v", r.Decisions)
	}
	for _, d := range r.Decisions {
		switch d.ID {
		case keep.ID:
			if d.Settled != nil {
				t.Errorf("open decision shows settled: %+v", d)
			}
		case other.ID:
			if d.Settled == nil || d.Settled.Action != model.ActDecide || d.Settled.Note != "Cache for a minute" {
				t.Errorf("decided-differently decision = %+v", d)
			}
		}
	}
	if len(page.Reports) != 1 || !page.Reports[0].Passed {
		t.Errorf("reports = %+v", page.Reports)
	}
	ids := tierIDs(page.NeedsYou)
	if !reflect.DeepEqual(ids[TierReview], []string{res.Review}) || !reflect.DeepEqual(ids[TierDecide], []string{keep.ID}) ||
		len(ids[TierPlan]) != 1 || ids[TierPlan][0] != q.ID {
		t.Errorf("item needs you = %v (a question on in-review work in no phase is about the plan)", ids)
	}

	// Sent back, then a failed attempt: the packet is the latest attempt,
	// with no review entry.
	must[DoResult](t)(m.Do(DoInput{ID: res.Review, Reason: "Needs a test", SettleOpts: SettleOpts{OnRequest: true}}))
	must[StartResult](t)(m.Start(it.ID))
	must[model.Config](t)(m.ConfigChecks([]string{"exit 1"}))
	gittest.Commit(t, dir, "lib/invoice_test.go", "package x\n", "Test", "Mull: "+it.ID)
	_, err := m.Done(it.ID, DoneInput{Summary: "Adds invoices, tested.", Design: "Plain structs."})
	wantCode(t, err, CodeChecksFailed)
	page = must[ItemPage](t)(m.ItemPage(it.ID))
	if page.Review == nil || page.Review.Entry != nil || page.Review.Report.N != 2 || page.Review.Report.Passed || len(page.Reports) != 2 {
		t.Errorf("after a failed attempt: review %+v, reports %+v", page.Review, page.Reports)
	}
	wantCode(t, func() error { _, err := m.ItemPage("ffff"); return err }(), CodeNotFound)
}
