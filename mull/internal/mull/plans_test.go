package mull

import (
	"strings"
	"testing"

	"mull/internal/model"
)

func TestPlanStates(t *testing.T) {
	m, _ := setup(t) // setup's general plan is agreed
	// Only the human creates a plan.
	_, err := m.PlanNew("launch", "Public launch", "", false)
	wantCode(t, err, CodeNotAllowed)
	p := must[*model.Plan](t)(m.PlanNew("launch", "Public launch", "Billing, then the switch.", true))
	if p.State != model.PlanShaping {
		t.Fatalf("a new plan is %q, want shaping", p.State)
	}

	// A shaping plan holds a doc, questions and captures; it refuses phases
	// and committed work until the human agrees it.
	_, err = m.PhaseAdd("launch", "Billing", "", nil)
	wantCode(t, err, CodeState)
	wantErr(t, err, "plan agree launch")
	_, err = m.ItemAdd(NewItem{Plan: "launch", Title: "Webhook"})
	wantCode(t, err, CodeState)
	idea := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemIdea, Title: "Annual billing", Plan: "launch"}))
	must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "Stripe or Paddle?", Plan: "launch"}))
	// A capture tagged to it can't be committed, or started, into it yet.
	_, err = m.Commit(CommitInput{ID: idea.ID, OnRequest: true})
	wantCode(t, err, CodeState)
	_, err = m.Start(idea.ID)
	wantCode(t, err, CodeState)
	if pv := must[PlanView](t)(m.PlanShow("launch")); pv.State != model.PlanShaping || pv.Progress.Captured != 1 || len(pv.Phases) != 0 {
		t.Fatalf("plan show = %+v", pv)
	}

	// Context lists it with its state and none of its work as ready.
	ctx := must[Context](t)(m.Context(""))
	states := map[string]string{}
	for _, bp := range ctx.Plans {
		states[bp.Slug] = bp.State
	}
	if states["launch"] != model.PlanShaping || states["general"] != model.PlanActive || len(ctx.Ready) != 0 {
		t.Fatalf("context plans = %+v ready = %+v", ctx.Plans, ctx.Ready)
	}

	// The view shows it after the active plans, marked as shaping, with its
	// questions in needs you.
	v := must[View](t)(m.Board(""))
	if len(v.Plans) != 2 || v.Plans[0].Slug != "general" || v.Plans[1].Slug != "launch" ||
		v.Plans[1].State != model.PlanShaping || v.Plans[1].NeedsYou.Questions != 1 {
		t.Fatalf("view plans = %+v", v.Plans)
	}
	if text := v.Text(m.Now()); !strings.Contains(text, "LAUNCH  Public launch  (shaping: not agreed yet)") {
		t.Fatalf("view text:\n%s", text)
	}

	// Only the human agrees a plan.
	_, err = m.PlanAgree("launch", false)
	wantCode(t, err, CodeNotAllowed)
	p = must[*model.Plan](t)(m.PlanAgree("launch", true))
	if p.State != model.PlanActive {
		t.Fatalf("agreed plan = %+v", p)
	}
	// Agreed, it takes phases and committed work, and the work is ready.
	phase := must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "", nil))
	if it := must[ItemView](t)(m.ItemAdd(NewItem{Phase: phase.ID, Title: "Webhook"})); !it.Ready {
		t.Fatalf("item after agree = %+v", it)
	}
	if c := must[ItemView](t)(m.Commit(CommitInput{ID: idea.ID, OnRequest: true})); c.State != model.StateCommitted || !c.Ready {
		t.Fatalf("committed idea = %+v", c)
	}
	_, err = m.PlanAgree("launch", true)
	wantCode(t, err, CodeState)

	// Closing makes a plan done, from shaping or active. A done plan takes
	// no new work and leaves the view unless asked for.
	p = must[*model.Plan](t)(m.PlanClose("launch"))
	if p.State != model.PlanDone {
		t.Fatalf("closed plan = %+v", p)
	}
	_, err = m.PlanClose("launch")
	wantCode(t, err, CodeState)
	_, err = m.PlanAgree("launch", true)
	wantCode(t, err, CodeState)
	_, err = m.ItemAdd(NewItem{Plan: "launch", Title: "late"})
	wantCode(t, err, CodeState)
	if v := must[View](t)(m.Board("")); len(v.Plans) != 1 || v.Plans[0].Slug != "general" {
		t.Fatalf("view plans = %+v", v.Plans)
	}
	if v := must[View](t)(m.Board("launch")); len(v.Plans) != 1 || v.Plans[0].State != model.PlanDone {
		t.Fatalf("view --plan launch = %+v", v.Plans)
	}
	must[*model.Plan](t)(m.PlanNew("someday", "Someday", "", true))
	if p := must[*model.Plan](t)(m.PlanClose("someday")); p.State != model.PlanDone {
		t.Fatalf("closed shaping plan = %+v", p)
	}
	list := must[[]PlanSummary](t)(m.PlanList())
	for _, ps := range list {
		if want := map[string]string{"general": model.PlanActive, "launch": model.PlanDone, "someday": model.PlanDone}[ps.Slug]; ps.State != want {
			t.Errorf("plan list: %s is %q, want %q", ps.Slug, ps.State, want)
		}
	}
}

func TestPlanAgreeInBatch(t *testing.T) {
	m, _ := setup(t)
	input := strings.Join([]string{
		`{"op": "plan new", "ref": "p", "slug": "launch", "title": "Launch", "on-request": true}`,
		`{"op": "plan agree", "slug": "@p", "on-request": true}`,
		`{"op": "add", "ref": "a", "plan": "@p", "title": "Webhook"}`,
	}, "\n")
	results, err := m.Batch(strings.NewReader(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if p := results[1].Result.(*model.Plan); p.State != model.PlanActive || results[1].ID != "launch" {
		t.Fatalf("plan agree result = %+v", results[1])
	}
	if !item(t, m, results[2].ID).Ready {
		t.Fatal("work in an agreed plan should be ready")
	}
	must[*model.Plan](t)(m.PlanNew("later", "Later", "", true))
	_, err = m.Batch(strings.NewReader(`{"op": "plan agree", "slug": "later"}`), false)
	if e := batchErr(t, err); e.Lines[0].Code != CodeNotAllowed {
		t.Fatalf("lines = %+v", e.Lines)
	}
}
