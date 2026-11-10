package mull

import (
	"testing"

	"mull/internal/model"
)

func TestContextBriefing(t *testing.T) {
	m, _ := setup(t)
	must[*model.Plan](t)(m.PlanNew("launch", "Launch", ""))
	e := must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "Stripe first.", nil))
	a := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Phase: e.ID, Title: "Webhook"}))
	b := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Phase: e.ID, Title: "Invoices", Needs: []string{a.ID}}))
	c := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Title: "Pricing copy"}))
	must[StartResult](t)(m.Start(a.ID))

	q := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "Retry failed webhooks?", Item: a.ID}))
	d1 := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindDecision, Text: "Store raw payloads", Item: a.ID}))
	d2 := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "Annual plans?", Plan: "launch"}))
	must[AcceptResult](t)(m.Accept([]string{d1.ID}, "", SettleOpts{}))
	must[*model.Entry](t)(m.Decide(d2.ID, "Monthly only for launch", SettleOpts{}))

	ctx := must[Context](t)(m.Context(a.ID))
	if ctx.Item == nil || ctx.Item.ID != a.ID || ctx.Item.PhaseSummary != "Stripe first." {
		t.Fatalf("item = %+v", ctx.Item)
	}
	if len(ctx.Item.OpenQuestions) != 1 || ctx.Item.OpenQuestions[0].ID != q.ID {
		t.Errorf("open questions = %+v", ctx.Item.OpenQuestions)
	}
	if len(ctx.Item.ItemDecisions) != 1 || ctx.Item.ItemDecisions[0].Decision != "Store raw payloads" {
		t.Errorf("item decisions = %+v", ctx.Item.ItemDecisions)
	}
	if len(ctx.Decisions) != 1 || ctx.Decisions[0].Decision != "Monthly only for launch" {
		t.Errorf("other decisions = %+v", ctx.Decisions)
	}
	if len(ctx.MyClaims) != 1 || ctx.MyClaims[0].ID != a.ID || len(ctx.Active) != 1 {
		t.Errorf("claims = %+v active = %+v", ctx.MyClaims, ctx.Active)
	}
	ready := map[string]bool{}
	for _, r := range ctx.Ready {
		ready[r.ID] = true
	}
	if !ready[c.ID] || ready[b.ID] || ready[a.ID] {
		t.Errorf("ready = %+v", ctx.Ready)
	}
	if len(ctx.Plans) != 2 || len(ctx.Protocol) == 0 {
		t.Errorf("plans = %+v", ctx.Plans)
	}
	_, err := m.Context("ffff")
	wantErr(t, err, `no item "ffff"`)
}
