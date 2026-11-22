package mull

import (
	"strings"
	"testing"

	"mull/internal/model"
)

// integrationReview files an integration review of phase covering items,
// settled with action, standing in for what mull done on a phase files.
func integrationReview(t *testing.T, m *Mull, phase, action string, items ...string) *model.Entry {
	t.Helper()
	var out *model.Entry
	err := m.update("test integration review "+phase, func(s *state) error {
		e, err := m.newEntry(s, model.KindReview, "Integration review", "", s.phases[phase].Plan, "", "")
		if err != nil {
			return err
		}
		e.Phase, e.Items = phase, items
		e.Settled = &model.Settlement{Action: action, By: "Jeff", Human: true, At: m.now()}
		out = e
		return s.putEntry(e)
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func phaseViewOf(t *testing.T, m *Mull, id string) PhaseView {
	t.Helper()
	for _, v := range must[[]PhaseView](t)(m.PhaseList("")) {
		if v.ID == id {
			return v
		}
	}
	t.Fatalf("no phase %s", id)
	return PhaseView{}
}

func TestPhaseSetIntegrationReview(t *testing.T) {
	m, dir := setup(t)
	ph := must[*model.Phase](t)(m.PhaseAdd("general", "Pages", "", nil))

	// It's the human's call: an agent must say they asked, through either set.
	_, err := m.PhaseSet(ph.ID, "integration-review", "on", false)
	wantCode(t, err, CodeNotAllowed)
	wantErr(t, err, "--on-request")
	_, err = m.Set(ph.ID, "integration-review", "on")
	wantCode(t, err, CodeNotAllowed)
	wantErr(t, err, "mull phase set "+ph.ID+" integration-review on --on-request")
	_, err = m.PhaseSet(ph.ID, "integration-review", "yes", true)
	wantCode(t, err, CodeInvalid)
	_, err = m.PhaseSet("beef", "integration-review", "on", true)
	wantCode(t, err, CodeNotFound)
	_, err = m.PhaseSet(ph.ID, "target", "main", true)
	wantCode(t, err, CodeInvalid)

	r := must[SetResult](t)(m.PhaseSet(ph.ID, "integration-review", "on", true))
	if r.Type != "phase" || !r.Phase.IntegrationReview {
		t.Fatalf("phase set on = %+v", r.Phase)
	}
	// Unchanged writes nothing.
	before := mullHead(t, dir)
	must[SetResult](t)(m.PhaseSet(ph.ID, "integration-review", "on", true))
	if mullHead(t, dir) != before {
		t.Fatal("setting integration-review to what it was wrote a commit")
	}
	// phase set changes the other fields too, with no gate.
	if r := must[SetResult](t)(m.PhaseSet(ph.ID, "title", "Site pages", false)); r.Phase.Title != "Site pages" || !r.Phase.IntegrationReview {
		t.Fatalf("phase set title = %+v", r.Phase)
	}
	// The human, from mull serve, needs no --on-request.
	h := human(t, m, dir)
	if r := must[SetResult](t)(h.Set(ph.ID, "integration-review", "off")); r.Phase.IntegrationReview {
		t.Fatalf("human set off = %+v", r.Phase)
	}
	// Batch takes it as phase set.
	res, err := m.Batch(strings.NewReader(jsonl(t, map[string]any{"op": "phase set", "id": ph.ID, "field": "integration-review", "value": "on", "on-request": true})), false)
	if err != nil || !phaseViewOf(t, m, ph.ID).IntegrationReview {
		t.Fatalf("batch phase set: %v, %+v", err, res)
	}
}

func TestIntegrationReviewHoldsThePhaseOpen(t *testing.T) {
	m, _ := setup(t)
	p1 := must[*model.Phase](t)(m.PhaseAdd("general", "Pages", "", nil))
	p2 := must[*model.Phase](t)(m.PhaseAdd("general", "Launch", "", []string{p1.ID}))
	must[SetResult](t)(m.PhaseSet(p1.ID, "integration-review", "on", true))
	x := accepted(t, m, NewItem{Phase: p1.ID, Title: "Home page"})
	gone := must[ItemView](t)(m.ItemAdd(NewItem{Phase: p1.ID, Title: "Old page"}))
	must[DropResult](t)(m.Drop([]string{gone.ID}, "not needed", SettleOpts{}))
	y := must[ItemView](t)(m.ItemAdd(NewItem{Phase: p2.ID, Title: "Go live"}))

	awaiting := func(want bool) {
		t.Helper()
		v := phaseViewOf(t, m, p1.ID)
		if v.AwaitingIntegrationReview != want || v.Complete == want {
			t.Fatalf("awaiting %v: phase view = %+v", want, v)
		}
		if ready := item(t, m, y.ID).Ready; ready == want {
			t.Fatalf("awaiting %v: the phase that needs it has ready %v", want, ready)
		}
	}

	// Built, but not complete: it awaits its review, and Launch waits on it.
	awaiting(true)
	page := must[PlanPage](t)(m.PlanPage("general"))
	if len(page.Status.Now) != 1 || page.Status.Now[0].ID != p1.ID || page.Status.Now[0].State != PhaseAwaiting || page.Status.Built != 0 {
		t.Fatalf("plan status = now %+v, built %d", page.Status.Now, page.Status.Built)
	}
	if gl := page.Status.GoLive; gl == nil || gl.ID != p2.ID || len(gl.WaitsFor) != 1 || gl.WaitsFor[0].State != PhaseAwaiting {
		t.Fatalf("go live = %+v", gl)
	}
	if g := page.Schedule[0]; g.Key != "review" || len(g.Phases) != 1 || g.Phases[0].ID != p1.ID {
		t.Fatalf("schedule = %+v", page.Schedule)
	}
	board := must[View](t)(m.Board("general"))
	if text := board.Text(m.Now()); !strings.Contains(text, "awaiting integration review") {
		t.Fatalf("view text lacks awaiting integration review:\n%s", text)
	}
	ctx := must[Context](t)(m.Context(""))
	if ph := ctx.Plans[0].Phases; !strings.Contains(ph[0].Progress+ph[1].Progress, "awaiting integration review") {
		t.Fatalf("context phases = %+v", ph)
	}

	// A review sent back (do) doesn't count, nor one that's still open.
	integrationReview(t, m, p1.ID, model.ActDo, x.ID)
	awaiting(true)
	// An accepted one covering the done items completes it; dropped items
	// need no review.
	integrationReview(t, m, p1.ID, model.ActAccept, x.ID)
	awaiting(false)
	if st := must[PlanPage](t)(m.PlanPage("general")).Status; st.Built != 1 {
		t.Fatalf("built = %d after the review", st.Built)
	}

	// Work added after it (a follow-up) reopens the phase, and once done
	// needs a review that covers it too.
	f := accepted(t, m, NewItem{FollowUp: x.ID, Title: "Home page spacing"})
	awaiting(true)
	integrationReview(t, m, p1.ID, model.ActAccept, x.ID, f.ID)
	awaiting(false)

	// Turning it off completes a built phase at once; on again, the earlier
	// review still covers it.
	must[SetResult](t)(m.PhaseSet(p1.ID, "integration-review", "off", true))
	awaiting(false)
	must[SetResult](t)(m.PhaseSet(p1.ID, "integration-review", "on", true))
	awaiting(false)
}

func TestIntegrationReviewNotBuilt(t *testing.T) {
	m, _ := setup(t)
	ph := must[*model.Phase](t)(m.PhaseAdd("general", "Pages", "", nil))
	must[SetResult](t)(m.PhaseSet(ph.ID, "integration-review", "on", true))
	// An empty phase, or one with open work, isn't awaiting review: it's
	// not built.
	if v := phaseViewOf(t, m, ph.ID); v.AwaitingIntegrationReview || v.Complete {
		t.Fatalf("empty phase = %+v", v)
	}
	must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "Home page"}))
	if v := phaseViewOf(t, m, ph.ID); v.AwaitingIntegrationReview || v.Complete {
		t.Fatalf("phase with open work = %+v", v)
	}
	if st := must[PlanPage](t)(m.PlanPage("general")).Status.Now; len(st) != 1 || st[0].State != PhaseNow {
		t.Fatalf("now = %+v", st)
	}
}
