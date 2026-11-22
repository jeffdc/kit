package mull

import (
	"strings"
	"testing"
	"time"

	"mull/internal/gittest"
	"mull/internal/model"
)

func TestPlanTargetSet(t *testing.T) {
	m, _ := setup(t)
	agreedPlan(t, m, "site", "The site", "")

	r := must[SetResult](t)(m.Set("site", "target", "refs/heads/site-shell"))
	if r.Plan.Target != "site-shell" {
		t.Fatalf("target = %q, want site-shell (refs/heads/ trimmed)", r.Plan.Target)
	}
	for _, bad := range []string{"a..b", "has space", "-x", "mull", "refs/heads/mull"} {
		_, err := m.Set("site", "target", bad)
		wantCode(t, err, CodeInvalid)
	}
	if p := must[PlanView](t)(m.PlanShow("site")); p.Target != "site-shell" {
		t.Errorf("a refused target changed it: %q", p.Target)
	}

	// The batch op is plan set, for plans only.
	must[[]BatchResult](t)(m.Batch(strings.NewReader(`{"op": "plan set", "slug": "site", "field": "target", "value": "next"}`), false))
	if p := must[PlanView](t)(m.PlanShow("site")); p.Target != "next" {
		t.Errorf("plan set: target %q", p.Target)
	}
	phase := must[*model.Phase](t)(m.PhaseAdd("site", "Pages", "", nil))
	_, err := m.Exec("plan set", &PlanSetIn{Slug: phase.ID, Field: "title", Value: "x"})
	wantCode(t, err, CodeNotFound)

	// "" clears it: the plan falls back to integration.
	must[SetResult](t)(m.Set("site", "target", ""))
	v := must[View](t)(m.Board("site"))
	if tg := v.Plans[0].Target; tg.Branch != "main" || tg.From != TargetFromIntegration || !tg.Found {
		t.Errorf("cleared target = %+v", tg)
	}
}

// at sets the committer date of the commits that follow.
func at(t *testing.T, when time.Time) string {
	t.Helper()
	d := when.UTC().Format(time.RFC3339)
	t.Setenv("GIT_COMMITTER_DATE", d)
	t.Setenv("GIT_AUTHOR_DATE", d)
	return d
}

func TestSeenInGitAgainstPlanTarget(t *testing.T) {
	m, dir := setup(t)
	must[model.Config](t)(m.ConfigSet("release", "main"))
	must[model.Config](t)(m.ConfigSet("integration", "dev"))
	agreedPlan(t, m, "site", "The site", "")
	agreedPlan(t, m, "tools", "Tools", "")
	must[SetResult](t)(m.Set("site", "target", "site-shell"))
	add := func(plan, title string) ItemView {
		return must[ItemView](t)(m.ItemAdd(NewItem{Plan: plan, Title: title}))
	}
	a, b, c := add("site", "A merged to the target"), add("site", "B only on integration"), add("tools", "C on integration")
	d := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "none", Title: "D unplanned"}))

	day := time.Now().Add(-10 * 24 * time.Hour).Truncate(time.Second)
	at(t, day)
	gittest.Git(t, dir, "checkout", "-q", "-b", "dev")
	gittest.Git(t, dir, "checkout", "-q", "-b", "site-shell")

	// A: worked on its branch on day 1, merged into site-shell on day 3
	// through a branch whose name carries no ID; the branch stays behind.
	gittest.Git(t, dir, "checkout", "-q", "-b", "a-work")
	at(t, day.Add(24*time.Hour))
	gittest.Commit(t, dir, "a", "a", "Do A", "Mull: "+a.ID)
	gittest.Git(t, dir, "checkout", "-q", "site-shell")
	merged := at(t, day.Add(3*24*time.Hour))
	gittest.Git(t, dir, "merge", "-q", "--no-ff", "--no-edit", "a-work")

	// B, C and D land on dev, integration; B's plan targets site-shell.
	gittest.Git(t, dir, "checkout", "-q", "dev")
	at(t, day.Add(4*24*time.Hour))
	gittest.Commit(t, dir, "b", "b", "Do B", "Mull: "+b.ID)
	gittest.Commit(t, dir, "c", "c", "Do C", "Mull: "+c.ID)
	gittest.Commit(t, dir, "d", "d", "Do D", "Mull: "+d.ID)
	for _, id := range []string{a.ID, b.ID, c.ID, d.ID} {
		markInReview(t, m, id)
	}

	m.Now = func() time.Time { return time.Now().Add(4 * 24 * time.Hour) } // past done-not-in-git's three days
	var g *gitState
	must[struct{}](t)(struct{}{}, m.view(func(s *state) error { var err error; g, err = m.readGit(s); return err }))
	ga, gb, gc, gd := g.Items[a.ID], g.Items[b.ID], g.Items[c.ID], g.Items[d.ID]
	same := func(a, b string) bool {
		x, okx := parseTime(a)
		y, oky := parseTime(b)
		return okx && oky && x.Equal(y)
	}
	if ga == nil || !ga.SeenInGit || ga.Target != "site-shell" || ga.Unmerged != 0 || len(ga.Branches) != 0 {
		t.Errorf("A = %+v: want seen on site-shell, nothing unmerged (a-work is merged)", ga)
	}
	if ga != nil && (!same(ga.MergedAt, merged) || same(ga.LastLanded, merged)) {
		t.Errorf("A merged_at %q, last_landed %q: want merged_at the merge's date %q, not the commit's", ga.MergedAt, ga.LastLanded, merged)
	}
	if gb != nil {
		t.Errorf("B = %+v: on integration only, so nothing on its target (and nothing unmerged: it's on a target)", gb)
	}
	if gc == nil || !gc.SeenInGit || gc.Target != "dev" || gd == nil || !gd.SeenInGit || gd.Target != "dev" {
		t.Errorf("C = %+v, D = %+v: want both seen on dev", gc, gd)
	}

	v := must[View](t)(m.Board(""))
	boards := map[string]PlanBoard{}
	for _, p := range v.Plans {
		boards[p.Slug] = p
	}
	if tg := boards["site"].Target; tg != (Target{Branch: "site-shell", From: TargetFromPlan, Found: true, AheadOfRelease: 2}) {
		t.Errorf("site target = %+v", tg)
	}
	if tg := boards["tools"].Target; tg != (Target{Branch: "dev", From: TargetFromIntegration, Found: true, AheadOfRelease: 3}) {
		t.Errorf("tools target = %+v", tg)
	}
	if v.Unplanned == nil || v.Unplanned.Target.Branch != "dev" {
		t.Errorf("unplanned target = %+v", v.Unplanned)
	}
	probs := problemsByItem(boards["site"])
	if probs[b.ID] != "done-not-in-git" || probs[a.ID] != "" {
		t.Errorf("site problems = %+v", boards["site"].Problems)
	}
	for _, p := range boards["site"].Problems {
		if p.Item == b.ID && !strings.HasSuffix(p.Text, "is on site-shell") {
			t.Errorf("B's problem names its target: %q", p.Text)
		}
	}
	if len(boards["tools"].Problems) != 0 {
		t.Errorf("tools problems = %+v", boards["tools"].Problems)
	}
	text := v.Text(m.Now())
	for _, want := range []string{"target site-shell · 2 ahead of main", "target dev (integration) · 3 ahead of main"} {
		if !strings.Contains(text, want) {
			t.Errorf("view text lacks %q:\n%s", want, text)
		}
	}

	// The same target shows in context, and on mull serve's pages.
	cx := must[Context](t)(m.Context(""))
	for _, p := range cx.Plans {
		if p.Slug == "site" && p.Target.Branch != "site-shell" {
			t.Errorf("context site target = %+v", p.Target)
		}
	}
	pp := must[PlanPage](t)(m.PlanPage("site"))
	if pp.Status.Target.Branch != "site-shell" || pp.Plan.Target != "site-shell" {
		t.Errorf("plan page target = %+v, plan %q", pp.Status.Target, pp.Plan.Target)
	}
	ip := must[ItemPage](t)(m.ItemPage(b.ID))
	if ip.Git.Target != "site-shell" || ip.Git.SeenInGit {
		t.Errorf("item page git = %+v", ip.Git)
	}
}

func TestMissingTarget(t *testing.T) {
	m, _ := setup(t)
	agreedPlan(t, m, "site", "The site", "")
	must[SetResult](t)(m.Set("site", "target", "not-yet"))
	i := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "site", Title: "Work"}))
	ip := must[ItemPage](t)(m.ItemPage(i.ID))
	if ip.Git.Target != "not-yet" {
		t.Errorf("an item with no commits is still measured against its target: %+v", ip.Git)
	}
	v := must[View](t)(m.Board("site"))
	if tg := v.Plans[0].Target; tg.Found || tg.Branch != "not-yet" {
		t.Errorf("target = %+v", tg)
	}
	if !strings.Contains(v.Text(m.Now()), "target not-yet · branch not found") {
		t.Errorf("text:\n%s", v.Text(m.Now()))
	}
}

func TestBroughtIn(t *testing.T) {
	// m1 - m2 - m3 (merge of x2) - m4 (merge of y1, which also reaches x1)
	//        \ x1 - x2 /          /
	//              \ y1 ---------
	c := func(sha, date string, parents ...string) commitInfo {
		return commitInfo{sha: sha, date: date, parents: parents}
	}
	commits := []commitInfo{
		c("m1", "1"), c("m2", "2", "m1"), c("x1", "3", "m2"), c("x2", "4", "x1"),
		c("m3", "5", "m2", "x2"), c("y1", "6", "x1"), c("m4", "7", "m3", "y1"),
	}
	bySha := map[string]commitInfo{}
	for _, x := range commits {
		bySha[x.sha] = x
	}
	got := broughtIn("m4", bySha)
	want := map[string]string{"m1": "1", "m2": "2", "x1": "5", "x2": "5", "m3": "5", "y1": "7", "m4": "7"}
	for sha, d := range want {
		if got[sha] != d {
			t.Errorf("%s brought in at %q, want %q", sha, got[sha], d)
		}
	}
}

// Accepted work with nothing on its plan's target is a problem at once; in
// review it waits three days, and merged work is never one.
func TestAcceptedNotInTarget(t *testing.T) {
	m, dir := setup(t)
	agreedPlan(t, m, "site", "The site", "")
	must[SetResult](t)(m.Set("site", "target", "site-shell"))
	gittest.Git(t, dir, "branch", "-q", "site-shell")
	add := func(title string) ItemView {
		return must[ItemView](t)(m.ItemAdd(NewItem{Plan: "site", Title: title}))
	}
	a, b, c, d := add("A accepted, on main only"), add("B in review, unmerged"), add("C accepted, merged"), add("D in review, merged")

	// A's work went to main, not the target; B's sits on its branch.
	gittest.Commit(t, dir, "a", "a", "Do A", "Mull: "+a.ID)
	gittest.Git(t, dir, "checkout", "-q", "-b", b.ID+"-work")
	gittest.Commit(t, dir, "b", "b", "Do B")
	// C and D are merged into the target.
	gittest.Git(t, dir, "checkout", "-q", "site-shell")
	gittest.Commit(t, dir, "c", "c", "Do C", "Mull: "+c.ID)
	gittest.Commit(t, dir, "d", "d", "Do D", "Mull: "+d.ID)
	gittest.Git(t, dir, "checkout", "-q", "main")
	for _, id := range []string{a.ID, b.ID, c.ID, d.ID} {
		markInReview(t, m, id)
	}
	must[AcceptResult](t)(m.Accept([]string{a.ID, c.ID}, "", SettleOpts{}))

	board := func() PlanBoard {
		t.Helper()
		return must[View](t)(m.Board("site")).Plans[0]
	}
	probs := problemsByItem(board())
	if probs[a.ID] != "done-not-in-git" || len(probs) != 1 {
		t.Errorf("just accepted: problems %v, want only A's done-not-in-git", probs)
	}
	for _, p := range board().Problems {
		if p.Item == a.ID && p.Text != "accepted, but none of its work is on site-shell" {
			t.Errorf("A's problem text: %q", p.Text)
		}
	}

	// Three days on, B (still in review, still unmerged) is one too.
	m.Now = func() time.Time { return time.Now().Add(4 * 24 * time.Hour) }
	probs = problemsByItem(board())
	if probs[a.ID] != "done-not-in-git" || probs[b.ID] != "done-not-in-git" || len(probs) != 2 {
		t.Errorf("three days on: problems %v, want A's and B's", probs)
	}
}
