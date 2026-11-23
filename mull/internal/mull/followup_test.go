package mull

import (
	"strings"
	"testing"
	"time"

	"mull/internal/gittest"
	"mull/internal/model"
)

// accepted adds an item, puts it in review and accepts it, at m.Now.
func accepted(t *testing.T, m *Mull, in NewItem) ItemView {
	t.Helper()
	i := must[ItemView](t)(m.ItemAdd(in))
	markInReview(t, m, i.ID)
	must[AcceptResult](t)(m.Accept([]string{i.ID}, "", SettleOpts{}))
	return i
}

func TestAddFollowUp(t *testing.T) {
	m, _ := setup(t)
	phase := must[*model.Phase](t)(m.PhaseAdd("general", "Billing", "", nil))
	done := accepted(t, m, NewItem{Phase: phase.ID, Title: "Invoices"})
	other := must[ItemView](t)(m.ItemAdd(NewItem{Phase: phase.ID, Title: "Receipts"}))

	f := must[ItemView](t)(m.ItemAdd(NewItem{FollowUp: done.ID, Title: "Invoice totals round wrong"}))
	if f.Plan != "general" || f.Phase != phase.ID || f.From != done.ID || f.State != model.StateCommitted {
		t.Fatalf("follow-up = %+v", f)
	}
	if f.Rank <= other.Rank {
		t.Errorf("follow-up rank %d, want after %s's %d (last in its group)", f.Rank, other.ID, other.Rank)
	}
	if i := must[Shown](t)(m.Show(done.ID)).Item; i.State != model.StateDone {
		t.Errorf("the done item was reopened: %+v", i)
	}

	// A plan and phase that agree are fine; ones that don't are refused.
	must[ItemView](t)(m.ItemAdd(NewItem{FollowUp: done.ID, Plan: "general", Phase: phase.ID, Title: "Agrees"}))
	agreedPlan(t, m, "other", "Other", "")
	_, err := m.ItemAdd(NewItem{FollowUp: done.ID, Plan: "other", Title: "x"})
	wantCode(t, err, CodeInvalid)
	_, err = m.ItemAdd(NewItem{FollowUp: done.ID, Plan: model.NoPlan, Title: "x"})
	wantCode(t, err, CodeInvalid)
	p2 := must[*model.Phase](t)(m.PhaseAdd("general", "Tax", "", nil))
	_, err = m.ItemAdd(NewItem{FollowUp: done.ID, Phase: p2.ID, Title: "x"})
	wantCode(t, err, CodeInvalid)

	// Only done work is followed up: in review is sent back instead.
	_, err = m.ItemAdd(NewItem{FollowUp: other.ID, Title: "x"})
	wantCode(t, err, CodeState)
	markInReview(t, m, other.ID)
	_, err = m.ItemAdd(NewItem{FollowUp: other.ID, Title: "x"})
	wantErr(t, err, "send it back")
	_, err = m.ItemAdd(NewItem{FollowUp: "ffff", Title: "x"})
	wantCode(t, err, CodeNotFound)

	// Unplanned work follows up as unplanned work.
	loose := accepted(t, m, NewItem{Plan: model.NoPlan, Title: "Loose"})
	u := must[ItemView](t)(m.ItemAdd(NewItem{FollowUp: loose.ID, Title: "More loose"}))
	if u.Plan != "" || u.Phase != "" || u.From != loose.ID {
		t.Fatalf("unplanned follow-up = %+v", u)
	}

	// accept --follow-up records where it came from too.
	r := must[ItemView](t)(m.ItemAdd(NewItem{Phase: phase.ID, Title: "Refunds"}))
	markInReview(t, m, r.ID)
	res := must[AcceptResult](t)(m.Accept([]string{r.ID}, "Refund copy", SettleOpts{}))
	if res.FollowUp == nil || res.FollowUp.From != r.ID || res.FollowUp.Phase != phase.ID {
		t.Fatalf("accept follow-up = %+v", res.FollowUp)
	}
}

func TestAddFollowUpInBatch(t *testing.T) {
	m, _ := setup(t)
	done := accepted(t, m, NewItem{Plan: "general", Title: "Invoices"})
	res := must[[]BatchResult](t)(m.Batch(strings.NewReader(`{"op":"add","title":"Totals","follow-up":"`+done.ID+`"}`+"\n"), false))
	if v := res[0].Result.(ItemView); v.From != done.ID || v.Plan != "general" {
		t.Fatalf("batch follow-up = %+v", v)
	}
}

func TestWorkAfterAcceptProblem(t *testing.T) {
	m, dir := setup(t)
	now := time.Now()
	problem := func(id string) string {
		t.Helper()
		v := must[View](t)(m.Board("general"))
		for _, p := range v.Plans[0].Problems {
			if p.Item == id && p.Kind == "work-after-accept" {
				return p.Kind
			}
		}
		return ""
	}

	// Work committed before acceptance is fine.
	m.Now = func() time.Time { return now.Add(time.Hour) }
	early := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "Early"}))
	gittest.Commit(t, dir, "e", "e", "Do early", "Mull: "+early.ID)
	markInReview(t, m, early.ID)
	must[AcceptResult](t)(m.Accept([]string{early.ID}, "", SettleOpts{}))
	if k := problem(early.ID); k != "" {
		t.Errorf("work before acceptance: problem %q", k)
	}

	// A commit under a done item's trailer after it was accepted is a
	// problem, on main or on any other branch.
	m.Now = func() time.Time { return now.Add(-time.Hour) }
	a := accepted(t, m, NewItem{Plan: "general", Title: "A"})
	b := accepted(t, m, NewItem{Plan: "general", Title: "B"})
	m.Now = func() time.Time { return now.Add(time.Minute) }
	gittest.Commit(t, dir, "a", "a", "Tweak A", "Mull: "+a.ID)
	gittest.Git(t, dir, "checkout", "-q", "-b", "side")
	gittest.Commit(t, dir, "b", "b", "Tweak B", "Mull: "+b.ID)
	gittest.Git(t, dir, "checkout", "-q", "main")
	if problem(a.ID) != "work-after-accept" || problem(b.ID) != "work-after-accept" {
		t.Fatalf("problems: A %q, B %q", problem(a.ID), problem(b.ID))
	}
	if text := must[View](t)(m.Board("general")).Text(m.Now()); !strings.Contains(text, "1 commit carrying its trailer came after") ||
		!strings.Contains(text, "mull add --follow-up "+a.ID) {
		t.Errorf("view text:\n%s", text)
	}

	// A rebase or cherry-pick keeps the author date, so it isn't new work.
	m.Now = func() time.Time { return now.Add(-time.Hour) }
	c := accepted(t, m, NewItem{Plan: "general", Title: "C"})
	m.Now = func() time.Time { return now.Add(time.Minute) }
	t.Setenv("GIT_AUTHOR_DATE", now.Add(-2*time.Hour).Format(time.RFC3339))
	gittest.Commit(t, dir, "c", "c", "Do C, rebased", "Mull: "+c.ID)
	if k := problem(c.ID); k != "" {
		t.Errorf("rebased work: problem %q", k)
	}

	// A follow-up added since clears it.
	must[ItemView](t)(m.ItemAdd(NewItem{FollowUp: a.ID, Title: "A tweaks"}))
	if k := problem(a.ID); k != "" {
		t.Errorf("after a follow-up: problem %q", k)
	}
	// And it ages out after a week.
	m.Now = func() time.Time { return now.Add(8 * 24 * time.Hour) }
	if k := problem(b.ID); k != "" {
		t.Errorf("a week later: problem %q", k)
	}
}

// A merge of main into an accepted item's branch, made to settle conflicts
// before it lands, carries the item's trailer but isn't later work.
func TestWorkAfterAcceptSkipsMerges(t *testing.T) {
	m, dir := setup(t)
	now := time.Now()
	afterAccept := func(id string) *Problem {
		t.Helper()
		for _, p := range must[View](t)(m.Board("general")).Plans[0].Problems {
			if p.Item == id && p.Kind == "work-after-accept" {
				return &p
			}
		}
		return nil
	}
	m.Now = func() time.Time { return now.Add(-time.Hour) }
	i := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "Merged late"}))
	branch := i.ID + "-late"
	gittest.Git(t, dir, "checkout", "-q", "-b", branch)
	t.Setenv("GIT_AUTHOR_DATE", now.Add(-2*time.Hour).Format(time.RFC3339))
	gittest.Commit(t, dir, "late.txt", "late", "Do it", "Mull: "+i.ID)
	markInReview(t, m, i.ID)
	must[AcceptResult](t)(m.Accept([]string{i.ID}, "", SettleOpts{}))

	// After acceptance: main moves on, and main is merged into the branch.
	t.Setenv("GIT_AUTHOR_DATE", now.Format(time.RFC3339))
	m.Now = func() time.Time { return now.Add(time.Minute) }
	gittest.Git(t, dir, "checkout", "-q", "main")
	gittest.Commit(t, dir, "other.txt", "other", "Other work on main")
	gittest.Git(t, dir, "checkout", "-q", branch)
	gittest.Git(t, dir, "merge", "-q", "--no-ff", "-m", "Merge main into "+branch, "-m", "Mull: "+i.ID, "main")
	gittest.Git(t, dir, "checkout", "-q", "main")
	if p := afterAccept(i.ID); p != nil {
		t.Fatalf("a merge counted as work after acceptance: %+v", p)
	}

	// A plain commit after it still counts.
	gittest.Git(t, dir, "checkout", "-q", branch)
	gittest.Commit(t, dir, "more.txt", "more", "More", "Mull: "+i.ID)
	gittest.Git(t, dir, "checkout", "-q", "main")
	if p := afterAccept(i.ID); p == nil || !strings.Contains(p.Text, "1 commit carrying") {
		t.Errorf("a later commit: problem %+v", p)
	}
}
