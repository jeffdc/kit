package mull

import (
	"strings"
	"testing"

	"mull/internal/model"
)

func TestCaptureBugsAndIdeas(t *testing.T) {
	m, _ := setup(t)
	agreedPlan(t, m, "launch", "Public launch", "")
	ph := must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "", nil))

	bug := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Footer links old pricing",
		Category: "copy", Source: "human, review of 1a2b", Detail: "Seen on /pricing."}))
	if bug.State != model.StateCaptured || bug.Kind != model.ItemBug || bug.Plan != "" || bug.Ready ||
		bug.Category != "copy" || bug.Source != "human, review of 1a2b" || bug.Notes != "Seen on /pricing." {
		t.Fatalf("bug = %+v", bug)
	}
	// Captures may be tagged to a plan or a phase; the phase implies its plan.
	idea := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemIdea, Title: "Annual plans", Phase: ph.ID}))
	if idea.Plan != "launch" || idea.Phase != ph.ID || idea.State != model.StateCaptured {
		t.Fatalf("idea = %+v", idea)
	}
	_, err := m.CaptureItem(NewCapture{Kind: "chore", Title: "x"})
	wantCode(t, err, CodeInvalid)
	_, err = m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "x", Plan: "nope"})
	wantCode(t, err, CodeNotFound)

	bugs := must[[]ItemView](t)(m.ItemList(ItemFilter{Kind: model.ItemBug}))
	if len(bugs) != 1 || bugs[0].ID != bug.ID {
		t.Fatalf("bugs = %+v", bugs)
	}
	none := must[[]ItemView](t)(m.ItemList(ItemFilter{Plan: model.NoPlan}))
	if len(none) != 1 || none[0].ID != bug.ID {
		t.Fatalf("no plan = %+v", none)
	}
	// Captures aren't inbox entries.
	if n := len(must[[]*model.Entry](t)(m.Inbox(InboxFilter{All: true}))); n != 0 {
		t.Fatalf("%d inbox entries", n)
	}
}

func TestCommitIsTheHumans(t *testing.T) {
	m, _ := setup(t)
	agreedPlan(t, m, "launch", "Public launch", "")
	ph := must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "", nil))
	bug := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Two h1s"}))

	_, err := m.Commit(CommitInput{ID: bug.ID})
	wantCode(t, err, CodeNotAllowed)

	// Committed untagged, a bug is unplanned work, and ready.
	v := must[ItemView](t)(m.Commit(CommitInput{ID: bug.ID, OnRequest: true}))
	if v.State != model.StateCommitted || v.Plan != "" || !v.Ready || v.Kind != model.ItemBug {
		t.Fatalf("committed = %+v", v)
	}
	_, err = m.Commit(CommitInput{ID: bug.ID, OnRequest: true})
	wantCode(t, err, CodeState)

	// Committing can move it: to a phase, or from a phase to another plan.
	idea := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemIdea, Title: "Coupons", Plan: "launch"}))
	v = must[ItemView](t)(m.Commit(CommitInput{ID: idea.ID, Phase: ph.ID, OnRequest: true}))
	if v.Plan != "launch" || v.Phase != ph.ID {
		t.Fatalf("committed to phase = %+v", v)
	}
	other := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemIdea, Title: "Gift cards", Phase: ph.ID}))
	v = must[ItemView](t)(m.Commit(CommitInput{ID: other.ID, Plan: model.NoPlan, OnRequest: true}))
	if v.Plan != "" || v.Phase != "" {
		t.Fatalf("committed unplanned = %+v", v)
	}
	third := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemIdea, Title: "x"}))
	_, err = m.Commit(CommitInput{ID: third.ID, Plan: model.NoPlan, Phase: ph.ID, OnRequest: true})
	wantCode(t, err, CodeInvalid)
}

func TestUnplannedWork(t *testing.T) {
	m, _ := setup(t)
	_, err := m.ItemAdd(NewItem{Title: "x"})
	wantCode(t, err, CodeScope)
	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: model.NoPlan, Title: "Bump Go"}))
	if a.Plan != "" || !a.Ready {
		t.Fatalf("unplanned = %+v", a)
	}

	// Entries about unplanned work have no plan; they don't land in general.
	q := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "Go 1.24 or 1.25?", Item: a.ID}))
	if q.Plan != "" {
		t.Fatalf("question plan = %q", q.Plan)
	}

	c := must[Context](t)(m.Context(""))
	if len(c.Ready) != 1 || c.Ready[0].ID != a.ID {
		t.Fatalf("context ready = %+v", c.Ready)
	}

	// It goes through done and review like any item; a follow-up stays unplanned.
	must[StartResult](t)(m.Start(a.ID))
	markInReview(t, m, a.ID)
	res := must[AcceptResult](t)(m.Accept([]string{a.ID}, "Bump the CI image too", SettleOpts{OnRequest: true}))
	if res.FollowUp == nil || res.FollowUp.Plan != "" {
		t.Fatalf("follow-up = %+v", res.FollowUp)
	}

	must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Flaky test"}))
	v := must[View](t)(m.Board(""))
	u := v.Unplanned
	if u == nil || u.Progress.Done != 1 || u.Progress.Committed != 1 || u.Progress.Captured != 1 || u.NeedsYou.Questions != 1 {
		t.Fatalf("unplanned board = %+v", u)
	}
	if text := v.Text(m.Now()); !strings.Contains(text, "UNPLANNED") || !strings.Contains(text, "1 captured") {
		t.Fatalf("view text:\n%s", text)
	}
}

func TestCapturesInBatch(t *testing.T) {
	m, _ := setup(t)
	input := strings.Join([]string{
		`{"op": "plan new", "slug": "alpha", "title": "Alpha"}`,
		`{"op": "plan agree", "slug": "alpha", "on-request": true}`,
		`{"op": "bug", "ref": "b", "text": "Crash on empty list", "category": "ui", "source": "human"}`,
		`{"op": "idea", "ref": "i", "plan": "alpha", "text": "Dark mode", "detail": "Asked for twice."}`,
		`{"op": "commit", "id": "@b", "plan": "alpha", "on-request": true}`,
	}, "\n")
	results, err := m.Batch(strings.NewReader(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if b := results[4].Result.(ItemView); b.ID != results[2].ID || b.State != model.StateCommitted || b.Plan != "alpha" {
		t.Fatalf("committed bug = %+v", b)
	}
	if i := item(t, m, results[3].ID); i.Kind != model.ItemIdea || i.Notes != "Asked for twice." || i.State != model.StateCaptured {
		t.Fatalf("idea = %+v", i)
	}
	_, err = m.Batch(strings.NewReader(`{"op": "commit", "id": "`+results[3].ID+`"}`), true)
	if e := batchErr(t, err); e.Lines[0].Code != CodeNotAllowed {
		t.Fatalf("lines = %+v", e.Lines)
	}
}
