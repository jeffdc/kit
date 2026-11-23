package mull

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
)

// finishItem takes a committed item in a phase through a session: start,
// one commit per file, done and accept (on request, from this worktree).
func finishItem(t *testing.T, m *Mull, dir, id string, files ...string) {
	t.Helper()
	must[StartResult](t)(m.Start(id))
	for _, f := range files {
		gittest.Commit(t, dir, f, f+" from "+id+"\n", "Change "+f, "Mull: "+id)
	}
	must[DoneResult](t)(m.Done(id, DoneInput{Summary: "Did " + id + ".", Tour: []string{"Open /"}, Design: "Plain."}))
	must[AcceptResult](t)(m.Accept([]string{id}, "", SettleOpts{OnRequest: true}))
}

func phaseDone(m *Mull, id string) (PhaseDoneResult, error) {
	return m.PhaseDone(id, DoneInput{Summary: "The pages fit together.", Tour: []string{"Open /pages"}, Design: "Read the combined diff."})
}

func TestPhaseDoneFilesTheIntegrationReview(t *testing.T) {
	m, dir := setup(t)
	check, _ := counterCheck(t, 0)
	must[model.Config](t)(m.ConfigChecks([]string{check}, true))
	must[model.Config](t)(m.ConfigLayer("ui", []string{"web/**"}))
	ph := must[*model.Phase](t)(m.PhaseAdd("general", "Pages", "", nil))
	x := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "Home page"}))
	y := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "About page"}))
	finishItem(t, m, dir, x.ID, "web/home.html", "lib/home.go")
	finishItem(t, m, dir, y.ID, "web/about.html")
	// Work from elsewhere on the target is no part of the phase's diff.
	gittest.Commit(t, dir, "other.txt", "other\n", "Unrelated")

	// Only a phase that needs an integration review files one.
	_, err := phaseDone(m, ph.ID)
	wantCode(t, err, CodeState)
	wantErr(t, err, "integration-review on")
	must[SetResult](t)(m.PhaseSet(ph.ID, "integration-review", "on", true))

	// Not with open work in it.
	z := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "Contact page"}))
	_, err = phaseDone(m, ph.ID)
	wantCode(t, err, CodeState)
	wantErr(t, err, z.ID+" (committed)")
	must[DropResult](t)(m.Drop([]string{z.ID}, "not needed", SettleOpts{}))

	// Only at the tip of the target.
	gittest.Git(t, dir, "checkout", "-q", "-b", "side")
	gittest.Commit(t, dir, "side.txt", "side\n", "Side")
	_, err = phaseDone(m, ph.ID)
	wantCode(t, err, CodeState)
	wantErr(t, err, "tip of its target, main")
	gittest.Git(t, dir, "checkout", "-q", "main")

	// Design notes and, with UI files in the diff, a tour.
	_, err = m.PhaseDone(ph.ID, DoneInput{Summary: "Fits.", Design: "none"})
	wantCode(t, err, CodeDesignRequired)
	_, err = m.PhaseDone(ph.ID, DoneInput{Summary: "Fits.", Design: "Read it."})
	wantCode(t, err, CodeTourRequired)

	res := must[PhaseDoneResult](t)(phaseDone(m, ph.ID))
	rep := res.Report
	if rep.Phase != ph.ID || rep.Item != "" || rep.N != 1 || !rep.Passed || res.ReportPath != model.ReportPath(ph.ID, 1) {
		t.Fatalf("report = %+v at %s", rep, res.ReportPath)
	}
	if rep.Commit != gittest.Git(t, dir, "rev-parse", "main") {
		t.Errorf("report commit %s isn't main's tip", rep.Commit)
	}
	if want := []model.ReportItem{{ID: x.ID, Title: "Home page"}, {ID: y.ID, Title: "About page"}}; !slices.Equal(rep.Items, want) {
		t.Errorf("report items = %+v, want %+v", rep.Items, want)
	}
	var files []string
	for _, lc := range rep.Changes {
		for _, f := range lc.Files {
			files = append(files, lc.Layer+":"+f.Status+" "+f.Path)
		}
	}
	if want := []string{"other:A lib/home.go", "ui:A web/about.html", "ui:A web/home.html"}; !slices.Equal(files, want) {
		t.Errorf("combined diff = %v, want %v", files, want)
	}
	// Filed, the phase still awaits its review.
	if !res.Phase.AwaitingIntegrationReview || res.Phase.Complete || res.Review == "" {
		t.Fatalf("phase after done = %+v, review %q", res.Phase, res.Review)
	}
	r := must[Shown](t)(m.Show(res.Review)).Entry
	if r.Kind != model.KindReview || r.Phase != ph.ID || r.Item != "" || r.Plan != "general" || r.Source != res.ReportPath ||
		!slices.Equal(r.Items, []string{x.ID, y.ID}) {
		t.Fatalf("review entry = %+v", r)
	}
	out := must[ReportOut](t)(m.ReportText(ph.ID, 0))
	for _, s := range []string{"# Integration review report 1: Pages (phase " + ph.ID + ")", "### Items", "- **" + x.ID + "** Home page",
		"combined diff", "A web/home.html"} {
		if !strings.Contains(out.Text, s) {
			t.Errorf("report text lacks %q:\n%s", s, out.Text)
		}
	}
	if strings.Contains(out.Text, "Decisions made") {
		t.Errorf("an integration review report lists decisions:\n%s", out.Text)
	}
	// Needs you shows it on its phase.
	page := must[PhasePage](t)(m.PhasePage(ph.ID))
	if page.NeedsYou.Open != 1 || page.NeedsYou.Tiers[0].Key != TierReview || page.NeedsYou.Tiers[0].Entries[0].Phase != ph.ID {
		t.Fatalf("phase page needs you = %+v", page.NeedsYou)
	}
	// And its page has the review packet, as an item's has.
	pk := page.Review
	if pk == nil || pk.Entry == nil || pk.Entry.ID != r.ID || pk.Entry.Settled != nil || pk.Report.N != 1 || pk.Report.Phase != ph.ID ||
		len(pk.Report.Items) != 2 || len(pk.Decisions) != 0 || pk.Target != "main" || len(pk.Tour) != 1 {
		t.Fatalf("phase page review = %+v", pk)
	}
	if len(page.Reports) != 1 || !page.Reports[0].Passed {
		t.Errorf("phase page reports = %+v", page.Reports)
	}

	// One open integration review at a time.
	_, err = phaseDone(m, ph.ID)
	wantCode(t, err, CodeState)
	wantErr(t, err, "integration review "+r.ID+" is open")

	// The worktree that filed it doesn't accept it on its own.
	_, err = m.Accept([]string{r.ID}, "", SettleOpts{})
	wantCode(t, err, CodeNotAllowed)
	_, err = m.Accept([]string{ph.ID}, "", SettleOpts{})
	wantCode(t, err, CodeNotAllowed)

	// Sent back, it adds a fix-up item to the phase, which reopens it.
	_, err = m.Do(DoInput{ID: r.ID})
	wantCode(t, err, CodeInvalid)
	_, err = m.Do(DoInput{ID: r.ID, Reason: "x", Plan: "other"})
	wantCode(t, err, CodeInvalid)
	_, err = m.Do(DoInput{ID: r.ID, Reason: "x", As: "phase"})
	wantCode(t, err, CodeInvalid)
	d := must[DoResult](t)(m.Do(DoInput{ID: r.ID, Reason: "The two pages use different widths.\n\nUse the layout's."}))
	fix := d.Item
	if fix == nil || fix.Phase != ph.ID || fix.State != model.StateCommitted || fix.From != r.ID || fix.Title != "The two pages use different widths." ||
		!strings.Contains(fix.Notes, "Use the layout's.") || d.Entry.Settled.Action != model.ActDo || d.Entry.Settled.Item != fix.ID {
		t.Fatalf("do = %+v, item %+v", d.Entry, fix)
	}
	if v := phaseViewOf(t, m, ph.ID); v.AwaitingIntegrationReview || v.Complete {
		t.Fatalf("phase after send back = %+v", v)
	}
	if pk := must[PhasePage](t)(m.PhasePage(ph.ID)).Review; pk == nil || pk.Entry == nil || pk.Entry.Settled == nil ||
		pk.Entry.Settled.Action != model.ActDo || pk.Entry.Settled.Item != fix.ID {
		t.Fatalf("phase page review after send back = %+v", pk)
	}
	_, err = m.Accept([]string{ph.ID}, "", SettleOpts{OnRequest: true})
	wantCode(t, err, CodeState)

	// Fixed, the next review covers the fix-up too; accepting it by the
	// phase completes the phase.
	finishItem(t, m, dir, fix.ID, "web/home.html")
	res = must[PhaseDoneResult](t)(phaseDone(m, ph.ID))
	if res.Report.N != 2 || len(res.Report.Items) != 3 {
		t.Fatalf("second report = %+v", res.Report)
	}
	page = must[PhasePage](t)(m.PhasePage(ph.ID))
	if pk := page.Review; pk == nil || pk.Entry == nil || pk.Entry.ID != res.Review || pk.Entry.Settled != nil || pk.Report.N != 2 || len(page.Reports) != 2 {
		t.Fatalf("phase page review after the second done = %+v, reports %+v", pk, page.Reports)
	}
	h := human(t, m, dir)
	acc := must[AcceptResult](t)(h.Accept([]string{ph.ID}, "", SettleOpts{}))
	if len(acc.Entries) != 1 || acc.Entries[0].ID != res.Review || acc.Entries[0].Settled.Action != model.ActAccept {
		t.Fatalf("accept = %+v", acc)
	}
	if v := phaseViewOf(t, m, ph.ID); !v.Complete || v.AwaitingIntegrationReview {
		t.Fatalf("phase after accept = %+v", v)
	}
	_, err = phaseDone(m, ph.ID)
	wantCode(t, err, CodeState)
	wantErr(t, err, "is complete")
}

func TestPhaseDoneFailedChecksFilesNothing(t *testing.T) {
	m, dir := setup(t)
	ph := must[*model.Phase](t)(m.PhaseAdd("general", "Pages", "", nil))
	x := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "Home page"}))
	check, _ := counterCheck(t, 0)
	must[model.Config](t)(m.ConfigChecks([]string{check}, true))
	finishItem(t, m, dir, x.ID, "lib/home.go")
	must[SetResult](t)(m.PhaseSet(ph.ID, "integration-review", "on", true))

	failing, _ := counterCheck(t, 2)
	must[model.Config](t)(m.ConfigChecks([]string{failing}, true))
	res, err := phaseDone(m, ph.ID)
	var failed *ErrChecksFailed
	if !errors.As(err, &failed) || !failed.Phase || failed.Report != model.ReportPath(ph.ID, 1) {
		t.Fatalf("got %v, want ErrChecksFailed for the phase's report 1", err)
	}
	wantCode(t, err, CodeChecksFailed)
	if res.Review != "" || len(must[[]*model.Entry](t)(m.Inbox(InboxFilter{Kind: model.KindReview}))) != 0 {
		t.Fatalf("a review was filed after failed checks: %+v", res)
	}
	out := must[ReportOut](t)(m.ReportText(ph.ID, 1))
	if out.Report.Passed || !strings.Contains(out.Text, "FAILED: no integration review is filed") {
		t.Fatalf("failed report:\n%s", out.Text)
	}

	// No checks at all: nothing can prove it, as for an item.
	must[model.Config](t)(m.ConfigChecks(nil, true))
	_, err = phaseDone(m, ph.ID)
	wantCode(t, err, CodeNoChecks)
}

func TestFinishTakesAnItemOrAPhase(t *testing.T) {
	m, _ := setup(t)
	ph := must[*model.Phase](t)(m.PhaseAdd("general", "Pages", "", nil))
	_, err := m.Finish(ph.ID, DoneInput{Summary: "Fits.", Design: "Read it."})
	wantErr(t, err, "doesn't need an integration review")
	it := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "Home page"}))
	_, err = m.Finish(it.ID, DoneInput{Summary: "Fits.", Design: "Read it."})
	wantErr(t, err, "not started")
	_, err = m.Finish("beef", DoneInput{Summary: "Fits.", Design: "Read it."})
	wantCode(t, err, CodeNotFound)
}

func TestFixUpTitle(t *testing.T) {
	long := strings.Repeat("word ", 30)
	for in, want := range map[string]string{
		"  Widths differ.\nMore detail.": "Widths differ.",
		long:                             strings.TrimSpace(strings.Repeat("word ", 15)) + "…",
	} {
		if got := fixUpTitle(in); got != want {
			t.Errorf("fixUpTitle(%q) = %q, want %q", in, got, want)
		}
	}
}
