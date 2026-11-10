package mull

import (
	"strings"
	"testing"

	"mull/internal/model"
)

func TestPlanStates(t *testing.T) {
	m, _ := setup(t) // setup's general plan is agreed
	p := must[*model.Plan](t)(m.PlanNew("launch", "Public launch", "Billing, then the switch."))
	if p.State != model.PlanShaping {
		t.Fatalf("a new plan is %q, want shaping", p.State)
	}

	// A shaping plan takes phases, items, captures and questions, but none
	// of its work is ready until the human agrees it.
	phase := must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "", nil))
	it := must[ItemView](t)(m.ItemAdd(NewItem{Phase: phase.ID, Title: "Webhook"}))
	if it.Ready || len(it.BlockedBy) > 0 {
		t.Fatalf("item in a shaping plan = %+v", it)
	}
	must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemIdea, Title: "Annual billing", Plan: "launch"}))
	must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "Stripe or Paddle?", Plan: "launch"}))
	if ready := must[[]ItemView](t)(m.ItemList(ItemFilter{ReadyOnly: true})); len(ready) != 0 {
		t.Fatalf("ready = %+v", ready)
	}
	if pv := must[PlanView](t)(m.PlanShow("launch")); pv.State != model.PlanShaping || pv.Progress.Ready != 0 || pv.Phases[0].Progress.Ready != 0 {
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

	// Starting its work is allowed but warned about.
	res := must[StartResult](t)(m.Start(it.ID))
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "shaping") {
		t.Fatalf("warnings = %v", res.Warnings)
	}
	must[ItemView](t)(m.Release(it.ID))

	// Only the human agrees a plan.
	_, err := m.PlanAgree("launch", false)
	wantCode(t, err, CodeNotAllowed)
	p = must[*model.Plan](t)(m.PlanAgree("launch", true))
	if p.State != model.PlanActive {
		t.Fatalf("agreed plan = %+v", p)
	}
	if got := item(t, m, it.ID); !got.Ready {
		t.Fatalf("item after agree = %+v", got)
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
	must[*model.Plan](t)(m.PlanNew("someday", "Someday", ""))
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
		`{"op": "plan new", "ref": "p", "slug": "launch", "title": "Launch"}`,
		`{"op": "add", "ref": "a", "plan": "@p", "title": "Webhook"}`,
		`{"op": "plan agree", "slug": "@p", "on-request": true}`,
	}, "\n")
	results, err := m.Batch(strings.NewReader(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if p := results[2].Result.(*model.Plan); p.State != model.PlanActive || results[2].ID != "launch" {
		t.Fatalf("plan agree result = %+v", results[2])
	}
	if !item(t, m, results[1].ID).Ready {
		t.Fatal("work in an agreed plan should be ready")
	}
	must[*model.Plan](t)(m.PlanNew("later", "Later", ""))
	_, err = m.Batch(strings.NewReader(`{"op": "plan agree", "slug": "later"}`), false)
	if e := batchErr(t, err); e.Lines[0].Code != CodeNotAllowed {
		t.Fatalf("lines = %+v", e.Lines)
	}
}
