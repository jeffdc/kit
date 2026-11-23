package core

import (
	"strings"
	"testing"
)

func TestBatchIsAllOrNothing(t *testing.T) {
	m, _ := setup(t)
	good := `{"op": "plan new", "slug": "launch", "title": "Launch", "on-request": true}
{"op": "plan agree", "slug": "launch", "on-request": true}
{"op": "phase add", "ref": "p", "plan": "launch", "title": "Billing"}
{"op": "add", "ref": "i", "phase": "@p", "title": "Charge"}
{"op": "bug", "title": "Footer 404s", "area": "web/footer"}
{"op": "ask", "item": "@i", "text": "Stripe?", "default": "Stripe"}`
	res := must[[]BatchResult](t)(m.Batch(strings.NewReader(good), true))
	if len(res) != 6 {
		t.Fatalf("dry run = %+v", res)
	}
	if _, err := m.Show("launch"); err == nil {
		t.Fatal("a dry run wrote")
	}
	_, err := m.Batch(strings.NewReader(good+"\n"+`{"op": "commit", "id": "nope"}`), false)
	wantCode(t, err, CodeBatchInvalid)
	if _, err := m.Show("launch"); err == nil {
		t.Fatal("a failed batch wrote")
	}
	res = must[[]BatchResult](t)(m.Batch(strings.NewReader(good), false))
	i := item(t, m, res[3].ID)
	if i.Plan != "launch" || i.Phase != res[2].ID {
		t.Fatalf("item = %+v", i)
	}
	if e := entry(t, m, res[5].ID); e.Item != i.ID || e.Default != "Stripe" {
		t.Fatalf("question = %+v", e)
	}
}

func TestSendBackReopensWork(t *testing.T) {
	m, dir := riskSetup(t)
	it := startWork(t, m, "Risky")
	commitFile(t, dir, "go.mod", it.ID)
	res := must[DoneOut](t)(m.Done(it.ID, DoneInput{Summary: "Module.", Rules: "none"}))
	back := must[*Item](t)(m.SendBack(it.ID, "wrong module path", true))
	if back.ID != it.ID || back.State != Committed || back.Claim != nil || !strings.Contains(back.Notes, "wrong module path") {
		t.Fatalf("sent back = %+v", back)
	}
	if entry(t, m, res.Review).Settled == nil {
		t.Fatal("the review is open")
	}
}
