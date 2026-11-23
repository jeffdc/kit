package core

import (
	"strings"
	"testing"
	"time"
)

func TestPlansPhasesItems(t *testing.T) {
	m, _ := setup(t)
	_, err := m.PlanNew("launch", "Launch", "", "", false)
	wantCode(t, err, CodeNotAllowed)
	p := must[*Plan](t)(m.PlanNew("launch", "Launch", "", "", true))
	if p.Target != "plan/launch" || p.State != Shaping {
		t.Fatalf("plan = %+v", p)
	}
	_, err = m.Add(NewItem{Plan: "launch", Title: "Work"})
	wantCode(t, err, CodeState) // shaping: queued items only
	must[*Item](t)(m.Add(NewItem{Plan: "launch", Title: "An idea", Kind: Idea}))
	must[*Plan](t)(m.PlanState("launch", PlanActive, true))

	a := must[*Phase](t)(m.PhaseAdd("launch", "Billing", "Done when: a card is charged.", nil))
	b := must[*Phase](t)(m.PhaseAdd("launch", "Receipts", "", []string{a.ID}))
	_, err = m.PhaseSet(a.ID, "needs", b.ID)
	wantCode(t, err, CodeCycle)

	x := must[*Item](t)(m.Add(NewItem{Phase: a.ID, Title: "Charge"}))
	y := must[*Item](t)(m.Add(NewItem{Phase: b.ID, Title: "Email"}))
	ready := func(id string) bool {
		for _, i := range must[[]*Item](t)(m.Ready()) {
			if i.ID == id {
				return true
			}
		}
		return false
	}
	if !ready(x.ID) || ready(y.ID) {
		t.Fatal("ready")
	}
	// Work all dropped is an outline, not a done phase: it still blocks.
	must[[]*Item](t)(m.DropItems([]string{x.ID}, "back to an outline"))
	if ready(y.ID) {
		t.Fatal("a phase whose work was all dropped unblocked another")
	}
	z := must[*Item](t)(m.Add(NewItem{Phase: a.ID, Title: "Charge again"}))
	must[StartResult](t)(m.Start(z.ID))
	markDone(t, m, z.ID)
	if !ready(y.ID) {
		t.Fatal("a done phase didn't unblock")
	}
	plan := must[Shown](t)(m.Show("launch")).Plan
	if len(plan.Phases) != 2 || plan.Phases[0].ID != a.ID || plan.Phases[0].Hill != "done" || plan.Phases[1].Hill != "uphill" {
		t.Fatalf("hill = %+v", plan.Phases)
	}

	// Moving is a move, for work nobody's on.
	moved := must[*Item](t)(m.Set(y.ID, "plan", "general"))
	if moved.Plan != "general" || moved.Phase != "" {
		t.Fatalf("moved = %+v", moved)
	}
	must[StartResult](t)(m.Start(y.ID))
	_, err = m.Set(y.ID, "plan", "launch")
	wantCode(t, err, CodeState)
}

func TestQueuedWorkStartedInACompletePhase(t *testing.T) {
	m, _ := setup(t)
	ph := must[*Phase](t)(m.PhaseAdd("general", "Shell", "", nil))
	done := must[*Item](t)(m.Add(NewItem{Phase: ph.ID, Title: "Masthead"}))
	must[StartResult](t)(m.Start(done.ID))
	markDone(t, m, done.ID)
	bug := must[*Item](t)(m.Add(NewItem{Phase: ph.ID, Title: "Masthead wraps", Kind: Bug}))
	res := must[StartResult](t)(m.Start(bug.ID))
	if res.Item.Phase != "" || !strings.Contains(res.Item.Notes, "About phase "+ph.ID) {
		t.Fatalf("started = %+v", res.Item)
	}
}

func TestQuestionsCarryADefault(t *testing.T) {
	m, _ := setup(t)
	_, err := m.Ask("Centre prose?", "", "general", "", "", "")
	wantErr(t, err, "--default")
	_, err = m.Ask("Centre prose?", "", "general", "Centred", "soon", "")
	wantErr(t, err, "--by")
	q := must[*Entry](t)(m.Ask("Centre prose?", "", "general", "Centred", "1h", ""))
	if q.Default != "Centred" || q.Due == "" {
		t.Fatalf("question = %+v", q)
	}
	v := must[Surface](t)(m.Surface(""))
	if len(v.Calls) != 1 || v.Calls[0].Lapsed {
		t.Fatalf("calls = %+v", v.Calls)
	}
	m.Now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if v = must[Surface](t)(m.Surface("")); !v.Calls[0].Lapsed || !strings.Contains(v.Text(), "default stands: Centred") {
		t.Fatalf("calls = %+v", v.Calls)
	}
	// Taking the default settles it, as the human's decision.
	must[AcceptResult](t)(m.Accept([]string{q.ID}, true))
	log := must[[]Decision](t)(m.Decisions())
	if len(log) != 1 || log[0].Decision != "Centred" || !log[0].Human {
		t.Fatalf("log = %+v", log)
	}
}

// markDone takes an item through done and review by hand.
func markDone(t *testing.T, m *Mull, id string) {
	t.Helper()
	err := m.update("test done", func(s *state) error {
		i := s.items[id]
		i.State = Done
		return s.putItem(i)
	})
	if err != nil {
		t.Fatal(err)
	}
}
