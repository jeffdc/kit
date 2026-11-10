package mull

import (
	"strings"
	"testing"
	"time"

	"mull/internal/model"
)

func TestTriageStampsCapturedItems(t *testing.T) {
	m, _ := setup(t)
	clock := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	m.Now = func() time.Time { return clock }
	must[*model.Plan](t)(m.PlanNew("launch", "Public launch", ""))
	must[*model.Plan](t)(m.PlanAgree("launch", true))

	a := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Footer links", Plan: "launch"}))
	b := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemIdea, Title: "Dark mode", Plan: "launch"}))
	c := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Flaky test"}))
	work := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Title: "Pricing page"}))

	// Untriaged is counted at the top of the view and on each board.
	v := must[View](t)(m.Board(""))
	if v.Untriaged != 3 || v.LastTriage != "" {
		t.Fatalf("view untriaged = %d, last = %q", v.Untriaged, v.LastTriage)
	}
	if p := v.Plans[1]; p.Slug != "launch" || p.Progress.Captured != 2 || p.Progress.Untriaged != 2 {
		t.Fatalf("launch board = %+v", p)
	}
	if u := v.Unplanned; u == nil || u.Progress.Untriaged != 1 {
		t.Fatalf("unplanned board = %+v", u)
	}
	if text := v.Text(m.Now()); !strings.Contains(text, "BACKLOG  3 captured items untriaged") ||
		!strings.Contains(text, "2 captured (2 untriaged)") {
		t.Fatalf("view text:\n%s", text)
	}

	// Triage is run with the human, and stamps only captured items.
	_, err := m.Triage(TriageInput{IDs: []string{a.ID}})
	wantCode(t, err, CodeNotAllowed)
	_, err = m.Triage(TriageInput{IDs: []string{work.ID}, OnRequest: true})
	wantCode(t, err, CodeState)
	_, err = m.Triage(TriageInput{IDs: []string{"ffff"}, OnRequest: true})
	wantCode(t, err, CodeNotFound)
	_, err = m.Triage(TriageInput{OnRequest: true})
	wantCode(t, err, CodeInvalid)
	_, err = m.Triage(TriageInput{IDs: []string{a.ID}, All: true, OnRequest: true})
	wantCode(t, err, CodeInvalid)
	_, err = m.Triage(TriageInput{IDs: []string{a.ID}, Plan: "launch", OnRequest: true})
	wantCode(t, err, CodeInvalid)
	_, err = m.Triage(TriageInput{All: true, Plan: "nope", OnRequest: true})
	wantCode(t, err, CodeNotFound)

	res := must[TriageResult](t)(m.Triage(TriageInput{IDs: []string{a.ID, a.ID}, OnRequest: true}))
	if len(res.Items) != 1 || res.Items[0].Triaged != "2026-11-10T09:00:00Z" {
		t.Fatalf("triaged = %+v", res.Items)
	}
	un := must[[]ItemView](t)(m.ItemList(ItemFilter{Untriaged: true}))
	if len(un) != 2 || un[0].ID == a.ID || un[1].ID == a.ID {
		t.Fatalf("untriaged = %+v", un)
	}
	v = must[View](t)(m.Board(""))
	if v.Untriaged != 2 || v.LastTriage != "2026-11-10T09:00:00Z" || v.Plans[1].Progress.Untriaged != 1 {
		t.Fatalf("after triage: untriaged %d, last %q, launch %+v", v.Untriaged, v.LastTriage, v.Plans[1].Progress)
	}
	if text := v.Text(m.Now()); !strings.Contains(text, "last triage 2026-11-10") {
		t.Fatalf("view text:\n%s", text)
	}

	// --all with a plan stamps that plan's backlog only; none is the untagged backlog.
	clock = clock.Add(time.Hour)
	res = must[TriageResult](t)(m.Triage(TriageInput{All: true, Plan: "launch", OnRequest: true}))
	if len(res.Items) != 2 || item(t, m, c.ID).Triaged != "" || item(t, m, b.ID).Triaged != "2026-11-10T10:00:00Z" {
		t.Fatalf("triage --all --plan launch = %+v", res.Items)
	}
	res = must[TriageResult](t)(m.Triage(TriageInput{All: true, Plan: model.NoPlan, OnRequest: true}))
	if len(res.Items) != 1 || res.Items[0].ID != c.ID {
		t.Fatalf("triage --all --plan none = %+v", res.Items)
	}

	// Something captured since the last triage is untriaged; --all stamps every capture.
	d := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Typo"}))
	if un := must[[]ItemView](t)(m.ItemList(ItemFilter{Untriaged: true})); len(un) != 1 || un[0].ID != d.ID {
		t.Fatalf("untriaged = %+v", un)
	}
	res = must[TriageResult](t)(m.Triage(TriageInput{All: true, OnRequest: true}))
	if len(res.Items) != 4 {
		t.Fatalf("triage --all = %+v", res.Items)
	}
	if v := must[View](t)(m.Board("")); v.Untriaged != 0 || strings.Contains(v.Text(m.Now()), "BACKLOG") {
		t.Fatalf("view after triage --all = %+v", v)
	}
}

func TestRetagCapturedItems(t *testing.T) {
	m, _ := setup(t)
	must[*model.Plan](t)(m.PlanNew("launch", "Public launch", ""))
	must[*model.Plan](t)(m.PlanAgree("launch", true))
	billing := must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "", nil))
	ops := must[*model.Phase](t)(m.PhaseAdd("general", "Ops", "", nil))
	bug := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Footer links"}))

	set := func(field, value string) ItemView {
		t.Helper()
		return *must[SetResult](t)(m.Set(bug.ID, field, value)).Item
	}
	// A captured item is tagged to a plan, and untagged, without committing it.
	if v := set("plan", "launch"); v.Plan != "launch" || v.State != model.StateCaptured {
		t.Fatalf("tagged = %+v", v)
	}
	if v := set("phase", billing.ID); v.Plan != "launch" || v.Phase != billing.ID {
		t.Fatalf("phase = %+v", v)
	}
	// A phase in another plan retags its plan too.
	if v := set("phase", ops.ID); v.Plan != "general" || v.Phase != ops.ID {
		t.Fatalf("phase across plans = %+v", v)
	}
	// A new plan leaves the old phase behind; the same plan keeps it.
	if v := set("plan", "general"); v.Phase != ops.ID {
		t.Fatalf("same plan = %+v", v)
	}
	if v := set("plan", "launch"); v.Plan != "launch" || v.Phase != "" {
		t.Fatalf("new plan = %+v", v)
	}
	if v := set("plan", model.NoPlan); v.Plan != "" || v.Phase != "" {
		t.Fatalf("untagged = %+v", v)
	}
	if v := set("category", "copy"); v.Category != "copy" {
		t.Fatalf("category = %+v", v)
	}
	_, err := m.Set(bug.ID, "plan", "")
	wantCode(t, err, CodeInvalid)
	_, err = m.Set(bug.ID, "plan", "nope")
	wantCode(t, err, CodeNotFound)
	must[*model.Plan](t)(m.PlanNew("old", "Old", ""))
	must[*model.Plan](t)(m.PlanClose("old"))
	_, err = m.Set(bug.ID, "plan", "old")
	wantCode(t, err, CodeState)

	// Committed work doesn't move plans through set.
	work := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Title: "Pricing page"}))
	_, err = m.Set(work.ID, "plan", "general")
	wantCode(t, err, CodeState)
	_, err = m.Set(work.ID, "phase", ops.ID)
	wantCode(t, err, CodeInvalid)
}

func TestTriageInBatch(t *testing.T) {
	m, _ := setup(t)
	input := strings.Join([]string{
		`{"op": "bug", "ref": "b", "text": "Crash on empty list"}`,
		`{"op": "idea", "ref": "i", "text": "Dark mode"}`,
		`{"op": "set", "id": "@i", "field": "plan", "value": "general"}`,
		`{"op": "triage", "ids": "@b @i", "on-request": true}`,
	}, "\n")
	results, err := m.Batch(strings.NewReader(input), false)
	if err != nil {
		t.Fatal(err)
	}
	tr := results[3].Result.(TriageResult)
	if len(tr.Items) != 2 || tr.Items[1].Plan != "general" || tr.Items[0].Triaged == "" {
		t.Fatalf("triage = %+v", tr)
	}
	_, err = m.Batch(strings.NewReader(`{"op": "triage", "all": true}`), true)
	if e := batchErr(t, err); e.Lines[0].Code != CodeNotAllowed {
		t.Fatalf("lines = %+v", e.Lines)
	}
}
