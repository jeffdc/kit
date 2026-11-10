package mull

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
	"mull/internal/store"
)

func setup(t *testing.T) (*Mull, string) {
	t.Helper()
	dir := gittest.NewRepo(t)
	m, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Init("main", "main"); err != nil {
		t.Fatal(err)
	}
	return m, dir
}

func must[T any](t *testing.T) func(T, error) T {
	return func(v T, err error) T {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("got no error, want code %s", code)
	}
	if got := AsError(err).Code; got != code {
		t.Fatalf("got code %s (%v), want %s", got, err, code)
	}
}

func wantErr(t *testing.T, err error, contains string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), contains) {
		t.Fatalf("got error %v, want one containing %q", err, contains)
	}
}

// markInReview stands in for `mull done`: it puts the item in review and files its review entry.
func markInReview(t *testing.T, m *Mull, id string) {
	t.Helper()
	err := m.update("test done "+id, func(s *state) error {
		i, err := s.item(id)
		if err != nil {
			return err
		}
		i.State = model.StateInReview
		if err := s.putItem(i); err != nil {
			return err
		}
		_, err = m.newEntry(s, model.KindReview, "Review "+i.Title, id, "", "done", "")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func item(t *testing.T, m *Mull, id string) ItemView {
	t.Helper()
	v, err := m.Show(id)
	if err != nil {
		t.Fatal(err)
	}
	if v.Item == nil {
		t.Fatalf("%s is a %s, not an item", id, v.Type)
	}
	return *v.Item
}

func TestInitCreatesGeneralPlan(t *testing.T) {
	m, _ := setup(t)
	plans := must[[]PlanSummary](t)(m.PlanList())
	if len(plans) != 1 || plans[0].Slug != model.GeneralPlan {
		t.Fatalf("plans = %+v", plans)
	}
	_, err := m.Init("main", "main")
	if !errors.Is(err, store.ErrExists) {
		t.Fatalf("second init: %v", err)
	}
	_, err = m.PlanClose(model.GeneralPlan)
	wantErr(t, err, "cannot be closed")
}

func TestNotInitialized(t *testing.T) {
	m, err := Open(gittest.NewRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.PlanList(); !errors.Is(err, store.ErrNotInitialized) {
		t.Fatalf("got %v", err)
	}
}

func TestPlansPhasesItems(t *testing.T) {
	m, _ := setup(t)
	_, err := m.PlanNew("Bad Slug", "x", "")
	wantErr(t, err, "invalid plan slug")
	_, err = m.PlanNew("beef", "x", "")
	wantErr(t, err, "invalid plan slug")
	must[*model.Plan](t)(m.PlanNew("taxogall", "Taxogall", "Galls as catalog entries."))
	_, err = m.PlanNew("taxogall", "again", "")
	wantErr(t, err, "already exists")

	phase := must[*model.Phase](t)(m.PhaseAdd("taxogall", "Phase 2", "", nil))
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "taxogall", Phase: phase.ID, Title: "Tree edits become Facts"}))
	if it.State != model.StateCommitted || !it.Ready {
		t.Fatalf("new item = %+v", it)
	}
	_, err = m.ItemAdd(NewItem{Plan: "general", Phase: phase.ID, Title: "wrong plan"})
	wantErr(t, err, "belongs to plan")
	_, err = m.ItemAdd(NewItem{Plan: "nope", Title: "x"})
	wantErr(t, err, `no plan "nope"`)

	pv := must[PlanView](t)(m.PlanShow("taxogall"))
	if len(pv.Phases) != 1 || len(pv.Items) != 1 || pv.Progress.Total != 1 || pv.Phases[0].Progress.Ready != 1 {
		t.Fatalf("plan view = %+v", pv)
	}

	set := must[SetResult](t)(m.Set(it.ID, "notes", "Read 8567 first."))
	if set.Item.Notes != "Read 8567 first." {
		t.Errorf("notes not set")
	}
	_, err = m.Set(it.ID, "state", "done")
	wantErr(t, err, "settable fields")

	must[*model.Plan](t)(m.PlanClose("taxogall"))
	_, err = m.ItemAdd(NewItem{Plan: "taxogall", Title: "x"})
	wantErr(t, err, "closed")
}

func TestReadinessAndUnreviewedIsIncomplete(t *testing.T) {
	m, _ := setup(t)
	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A"}))
	b := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "B", Needs: []string{a.ID}}))
	if b.Ready || len(b.BlockedBy) != 1 || b.BlockedBy[0] != a.ID {
		t.Fatalf("B should be blocked by A: %+v", b)
	}
	ready := must[[]ItemView](t)(m.ItemList(ItemFilter{ReadyOnly: true}))
	if len(ready) != 1 || ready[0].ID != a.ID {
		t.Fatalf("ready = %+v", ready)
	}
	must[StartResult](t)(m.Start(a.ID))
	markInReview(t, m, a.ID)
	if item(t, m, b.ID).Ready {
		t.Fatal("B ready while A is in review")
	}
	// Accept from another worktree identity: simulate by on-request.
	must[AcceptResult](t)(m.Accept([]string{a.ID}, "", SettleOpts{OnRequest: true}))
	if !item(t, m, b.ID).Ready {
		t.Fatal("B not ready after A is done")
	}
}

func TestPhaseNeeds(t *testing.T) {
	m, _ := setup(t)
	p1 := must[*model.Phase](t)(m.PhaseAdd("general", "Phase 1", "", nil))
	p2 := must[*model.Phase](t)(m.PhaseAdd("general", "Phase 2", "", []string{p1.ID}))
	x := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Phase: p1.ID, Title: "x"}))
	y := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Phase: p2.ID, Title: "y"}))
	if y.Ready {
		t.Fatal("an item in a blocked phase should not be ready")
	}
	must[DropResult](t)(m.Drop([]string{x.ID}, "not needed", SettleOpts{}))
	if !item(t, m, y.ID).Ready {
		t.Fatal("phase 1 complete (all dropped) should unblock phase 2's items")
	}
	// An empty phase is never complete.
	empty := must[*model.Phase](t)(m.PhaseAdd("general", "Empty", "", nil))
	z := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "z", Needs: []string{empty.ID}}))
	if z.Ready {
		t.Fatal("needing an empty phase should block")
	}
}

// markCaptured stands in for capture, which arrives in a later step: it
// moves the item back to the backlog.
func markCaptured(t *testing.T, m *Mull, id string) {
	t.Helper()
	err := m.update("test capture "+id, func(s *state) error {
		i, err := s.item(id)
		if err != nil {
			return err
		}
		i.State = model.StateCaptured
		return s.putItem(i)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestStartFromCaptured(t *testing.T) {
	m, _ := setup(t)
	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A small bug"}))
	if a.State != model.StateCommitted || !a.Ready {
		t.Fatalf("a new item should be committed and ready: %+v", a.Item)
	}
	markCaptured(t, m, a.ID)
	if v := item(t, m, a.ID); v.Ready {
		t.Fatal("a captured item should never be ready")
	}
	st := must[StartResult](t)(m.Start(a.ID))
	if st.Item.State != model.StateActive {
		t.Fatalf("start from captured: %+v", st.Item.Item)
	}
	// Released, it stays committed: starting it committed to it.
	if v := must[ItemView](t)(m.Release(a.ID)); v.State != model.StateCommitted {
		t.Fatalf("released item = %s, want committed", v.State)
	}
}

func TestCapturedItemsAreBacklog(t *testing.T) {
	m, _ := setup(t)
	p1 := must[*model.Phase](t)(m.PhaseAdd("general", "Phase 1", "", nil))
	p2 := must[*model.Phase](t)(m.PhaseAdd("general", "Phase 2", "", []string{p1.ID}))
	x := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Phase: p1.ID, Title: "x"}))
	bug := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Phase: p1.ID, Title: "a bug for later"}))
	y := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Phase: p2.ID, Title: "y"}))
	markCaptured(t, m, bug.ID)
	must[StartResult](t)(m.Start(x.ID))
	markInReview(t, m, x.ID)
	must[AcceptResult](t)(m.Accept([]string{x.ID}, "", SettleOpts{OnRequest: true}))

	err := m.view(func(s *state) error {
		ev := s.phaseView(s.phases[p1.ID])
		want := Progress{Total: 1, Captured: 1, Done: 1}
		if ev.Progress != want || !ev.Complete {
			t.Errorf("phase 1 = %+v complete=%v, want %+v complete", ev.Progress, ev.Complete, want)
		}
		if got := progressText(ev.Progress); got != "1/1 done · 1 captured" {
			t.Errorf("progress text = %q", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !item(t, m, y.ID).Ready {
		t.Fatal("a captured item tagged to phase 1 should not hold phase 2 back")
	}
	// A phase with only captured items has no committed work, so it isn't complete.
	markCaptured(t, m, y.ID)
	err = m.view(func(s *state) error {
		if s.phaseComplete(p2.ID) {
			t.Error("a phase with only captured items should not be complete")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListByStage(t *testing.T) {
	m, _ := setup(t)
	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A"}))
	must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "B"}))
	markCaptured(t, m, a.ID)
	got := must[[]ItemView](t)(m.ItemList(ItemFilter{State: model.StateCaptured}))
	if len(got) != 1 || got[0].ID != a.ID {
		t.Fatalf("captured = %+v", got)
	}
	_, err := m.ItemList(ItemFilter{State: "open"})
	wantCode(t, err, CodeInvalid)
}

func TestCyclesRefused(t *testing.T) {
	m, _ := setup(t)
	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A"}))
	b := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "B", Needs: []string{a.ID}}))
	_, err := m.Need(a.ID, []string{b.ID})
	wantErr(t, err, "dependency cycle")
	_, err = m.Need(a.ID, []string{a.ID})
	wantErr(t, err, "cannot need itself")
	// An item that needs its own phase is a cycle too.
	e := must[*model.Phase](t)(m.PhaseAdd("general", "E", "", nil))
	c := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Phase: e.ID, Title: "C"}))
	_, err = m.Need(c.ID, []string{e.ID})
	wantErr(t, err, "dependency cycle")
	// A refused change writes nothing.
	if len(item(t, m, a.ID).Needs) != 0 {
		t.Fatal("refused need was saved")
	}
	must[NeedResult](t)(m.Unneed(b.ID, []string{a.ID}))
	if !item(t, m, b.ID).Ready {
		t.Fatal("unneed did not unblock")
	}
}

func TestClaimsAcrossWorktrees(t *testing.T) {
	m, dir := setup(t)
	wt := gittest.AddWorktree(t, dir, "f923-tree")
	other, err := Open(wt)
	if err != nil {
		t.Fatal(err)
	}
	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A"}))
	res := must[StartResult](t)(other.Start(a.ID))
	if res.Item.Claim.Worktree != wt || res.Item.Claim.Branch != "f923-tree" || res.Item.State != model.StateActive {
		t.Fatalf("claim = %+v", res.Item.Claim)
	}
	must[StartResult](t)(other.Start(a.ID)) // idempotent for the claimant
	_, err = m.Start(a.ID)
	wantErr(t, err, "already claimed by "+wt)
	must[ItemView](t)(m.Release(a.ID)) // a supervisor may release
	must[StartResult](t)(m.Start(a.ID))

	_, err = m.ItemAdd(NewItem{Plan: "general", Title: "B", Needs: []string{"ffff"}})
	wantErr(t, err, `no item or phase "ffff"`)
}

func TestStartRaceOneWinner(t *testing.T) {
	m, dir := setup(t)
	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A"}))
	var wts []string
	for _, b := range []string{"w1", "w2", "w3", "w4"} {
		wts = append(wts, gittest.AddWorktree(t, dir, b))
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(wts))
	for _, wt := range wts {
		wg.Add(1)
		go func(wt string) {
			defer wg.Done()
			w, err := Open(wt)
			if err != nil {
				errs <- err
				return
			}
			_, err = w.Start(a.ID)
			errs <- err
		}(wt)
	}
	wg.Wait()
	close(errs)
	wins := 0
	for err := range errs {
		if err == nil {
			wins++
		} else if !strings.Contains(err.Error(), "already claimed") {
			t.Fatal(err)
		}
	}
	if wins != 1 {
		t.Fatalf("%d worktrees claimed the same item, want 1", wins)
	}
}

func TestStartBlockedWarns(t *testing.T) {
	m, _ := setup(t)
	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A"}))
	b := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "B", Needs: []string{a.ID}}))
	res := must[StartResult](t)(m.Start(b.ID))
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], a.ID) {
		t.Fatalf("warnings = %v", res.Warnings)
	}
}

func TestCaptureAndSettle(t *testing.T) {
	m, _ := setup(t)
	must[*model.Plan](t)(m.PlanNew("taxogall", "Taxogall", ""))
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "taxogall", Title: "Phase 2"}))

	dec := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindDecision, Text: "Any admin can undo anyone's decision", Item: it.ID, Detail: "Simplest rule."}))
	if dec.Plan != "taxogall" || dec.Item != it.ID {
		t.Fatalf("entry plan/item not derived from item: %+v", dec)
	}
	q := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "Do split undos run on preview?", Item: it.ID}))
	// With two plans, an entry must say which plan or item it is about.
	_, err := m.Capture(CaptureInput{Kind: model.KindLoose, Text: "Canonical URL emitted as meta"})
	wantCode(t, err, CodeScope)
	loose := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindLoose, Text: "Canonical URL emitted as meta", Plan: "general", Source: "2c8c"}))
	if loose.Plan != model.GeneralPlan {
		t.Fatalf("loose end without item or plan should land in general: %+v", loose)
	}
	_, err = m.Capture(CaptureInput{Kind: model.KindReview, Text: "x"})
	wantErr(t, err, "cannot capture")

	inbox := must[[]*model.Entry](t)(m.Inbox(InboxFilter{}))
	if len(inbox) != 3 {
		t.Fatalf("inbox has %d entries", len(inbox))
	}

	// Kind rules.
	_, err = m.Accept([]string{q.ID}, "", SettleOpts{})
	wantErr(t, err, "cannot be settled")
	_, err = m.Do(DoInput{ID: dec.ID})
	wantErr(t, err, "cannot be settled")

	must[AcceptResult](t)(m.Accept([]string{dec.ID}, "", SettleOpts{}))
	_, err = m.Accept([]string{dec.ID}, "", SettleOpts{})
	wantErr(t, err, "already settled")

	must[*model.Entry](t)(m.Decide(q.ID, "No; no compatibility path needed.", SettleOpts{OnRequest: true}))
	do := must[DoResult](t)(m.Do(DoInput{ID: loose.ID}))
	if do.Item.Title != "Canonical URL emitted as meta" || do.Item.Plan != model.GeneralPlan || do.Entry.Settled.Item != do.Item.ID {
		t.Fatalf("do = %+v", do)
	}

	if n := len(must[[]*model.Entry](t)(m.Inbox(InboxFilter{}))); n != 0 {
		t.Fatalf("inbox should be empty, has %d", n)
	}
	if n := len(must[[]*model.Entry](t)(m.Inbox(InboxFilter{All: true}))); n != 3 {
		t.Fatalf("--all should list 3, got %d", n)
	}

	log := must[[]Decision](t)(m.Decisions(""))
	if len(log) != 2 {
		t.Fatalf("decision log = %+v", log)
	}
	got := map[string]Decision{}
	for _, d := range log {
		got[d.ID] = d
	}
	if got[dec.ID].Decision != "Any admin can undo anyone's decision" {
		t.Errorf("ratified decision = %+v", got[dec.ID])
	}
	if d := got[q.ID]; d.Decision != "No; no compatibility path needed." || !d.OnRequest {
		t.Errorf("human decision = %+v", d)
	}
}

func TestParkKeepsEntryListedAndSettleable(t *testing.T) {
	m, _ := setup(t)
	e := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindLoose, Text: "Stale branch quercus-leafminer-key"}))
	_, err := m.Park(e.ID, "", SettleOpts{})
	wantErr(t, err, "needs a reason")
	must[*model.Entry](t)(m.Park(e.ID, "after release", SettleOpts{}))
	if n := len(must[[]*model.Entry](t)(m.Inbox(InboxFilter{}))); n != 0 {
		t.Fatal("parked entry still in the open inbox")
	}
	if n := len(must[[]*model.Entry](t)(m.Inbox(InboxFilter{Parked: true}))); n != 1 {
		t.Fatal("parked entry not listed with --parked")
	}
	must[DropResult](t)(m.Drop([]string{e.ID}, "", SettleOpts{}))
}

func TestReviewAcceptGuardAndFollowUp(t *testing.T) {
	m, dir := setup(t)
	wt := gittest.AddWorktree(t, dir, "worker")
	worker, err := Open(wt)
	if err != nil {
		t.Fatal(err)
	}
	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A"}))
	must[StartResult](t)(worker.Start(a.ID))
	markInReview(t, worker, a.ID)

	_, err = worker.Accept([]string{a.ID}, "", SettleOpts{})
	wantErr(t, err, "cannot accept its own work")

	// A supervisor in another worktree may accept, with a follow-up.
	res := must[AcceptResult](t)(m.Accept([]string{a.ID}, "Tighten the empty-state copy", SettleOpts{}))
	if res.Items[0].State != model.StateDone || len(res.Entries) != 1 || res.FollowUp == nil {
		t.Fatalf("accept = %+v", res)
	}
	if n := len(must[[]*model.Entry](t)(m.Inbox(InboxFilter{Kind: model.KindReview}))); n != 0 {
		t.Fatal("review entry not settled by accepting the item")
	}
	if res.FollowUp.Plan != "general" || !strings.Contains(res.FollowUp.Notes, a.ID) {
		t.Fatalf("follow-up = %+v", res.FollowUp)
	}
	_, err = m.Drop([]string{a.ID}, "", SettleOpts{})
	wantErr(t, err, "done and cannot be dropped")
}

func TestWorkerMayAcceptOnRequest(t *testing.T) {
	m, _ := setup(t)
	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A"}))
	must[StartResult](t)(m.Start(a.ID))
	markInReview(t, m, a.ID)
	res := must[AcceptResult](t)(m.Accept([]string{a.ID}, "", SettleOpts{OnRequest: true}))
	if !res.Entries[0].Settled.OnRequest {
		t.Fatal("on-request not recorded")
	}
}

func TestReviewSendBack(t *testing.T) {
	m, _ := setup(t)
	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "A"}))
	must[StartResult](t)(m.Start(a.ID))
	markInReview(t, m, a.ID)
	review := must[[]*model.Entry](t)(m.Inbox(InboxFilter{Kind: model.KindReview}))[0]
	_, err := m.Do(DoInput{ID: review.ID})
	wantErr(t, err, "say why")
	res := must[DoResult](t)(m.Do(DoInput{ID: review.ID, Reason: "The tour's step 2 shows the old page"}))
	if res.Item.State != model.StateCommitted || res.Item.Claim != nil || !strings.Contains(res.Item.Notes, "step 2") {
		t.Fatalf("sent-back item = %+v", res.Item)
	}
	_, err = m.Decide(review.ID, "x", SettleOpts{})
	wantErr(t, err, "already settled")
}
