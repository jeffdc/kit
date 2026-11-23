package mull

import (
	"encoding/json"
	"testing"

	"mull/internal/gittest"
	humanname "mull/internal/human"
	"mull/internal/model"
)

func opLines(t *testing.T, lines ...map[string]any) []json.RawMessage {
	t.Helper()
	out := make([]json.RawMessage, 0, len(lines))
	for _, l := range lines {
		data, err := json.Marshal(l)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, data)
	}
	return out
}

// human is m acting for the human, named Jeff.
func human(t *testing.T, m *Mull, dir string) *Mull {
	t.Helper()
	gittest.Git(t, dir, "config", "user.name", "Jeff")
	return must[*Mull](t)(m.AsHuman())
}

func TestAsHumanNeedsAName(t *testing.T) {
	m, dir := setup(t)
	_, err := m.AsHuman()
	wantCode(t, err, CodeNotAllowed)
	wantErr(t, err, "mull name")
	gittest.Git(t, dir, "config", "user.name", "  ")
	_, err = m.AsHuman()
	wantCode(t, err, CodeNotAllowed)
}

// The human's name is the one this machine goes by (mull name) when it's
// set, ahead of the repository's git user.name.
func TestAsHumanPrefersMullName(t *testing.T) {
	m, dir := setup(t)
	gittest.Git(t, dir, "config", "user.name", "jeffdc")
	if h := must[*Mull](t)(m.AsHuman()); h.human != "jeffdc" {
		t.Errorf("with only git user.name, human = %q", h.human)
	}
	if err := humanname.SetName("Jeff"); err != nil {
		t.Fatal(err)
	}
	if h := must[*Mull](t)(m.AsHuman()); h.human != "Jeff" {
		t.Errorf("with mull name set, human = %q", h.human)
	}
	gittest.Git(t, dir, "config", "--unset", "user.name")
	if h := must[*Mull](t)(m.AsHuman()); h.human != "Jeff" {
		t.Errorf("with mull name and no git user.name, human = %q", h.human)
	}
}

// What the human settles is recorded by their name, as theirs, and not as
// an agent's on request.
func TestHumanSettlesByName(t *testing.T) {
	m, dir := setup(t)
	d := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindDecision, Text: "use sqlite", Plan: "general"}))
	h := human(t, m, dir)
	results, commit, err := h.Apply(opLines(t, map[string]any{"op": "accept", "ids": []string{d.ID}}), mullHead(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if commit != mullHead(t, dir) {
		t.Errorf("Apply returned commit %s, the tip is %s", commit, mullHead(t, dir))
	}
	got := results[0].Result.(AcceptResult).Entries[0].Settled
	if got.By != "Jeff" || !got.Human || got.OnRequest || got.Action != model.ActAccept {
		t.Errorf("settled = %+v, want by Jeff, human, not on request", got)
	}
	log := must[[]Decision](t)(m.Decisions(""))
	if len(log) != 1 || log[0].By != "Jeff" || !log[0].Human || log[0].OnRequest {
		t.Errorf("decision log = %+v", log)
	}

	// An agent's settling is unchanged: by its worktree, on request if asked.
	q := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "q", Plan: "general"}))
	e := must[*model.Entry](t)(m.Decide(q.ID, "yes", SettleOpts{OnRequest: true}))
	if e.Settled.By != m.Worktree() || e.Settled.Human || !e.Settled.OnRequest {
		t.Errorf("agent's settled = %+v", e.Settled)
	}
}

// The steps only the human takes need no on-request when they take them.
func TestHumanNeedsNoOnRequest(t *testing.T) {
	m, dir := setup(t)
	h := human(t, m, dir)
	must[*model.Plan](t)(h.PlanNew("next", "Next", "", false))
	must[*model.Plan](t)(h.PlanAgree("next", false))
	c := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "b"}))
	must[ItemView](t)(h.Commit(CommitInput{ID: c.ID, Plan: "next"}))

	// The human accepts work done in the worktree mull serve opens.
	i := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "w"}))
	must[StartResult](t)(m.Start(i.ID))
	markInReview(t, m, i.ID)
	_, err := m.Accept([]string{i.ID}, "", SettleOpts{})
	wantCode(t, err, CodeNotAllowed)
	must[AcceptResult](t)(h.Accept([]string{i.ID}, "", SettleOpts{}))
}

// A page acts on what it showed: if a record the ops name or write changed
// since, nothing is written.
func TestApplyRefusesStalePages(t *testing.T) {
	m, dir := setup(t)
	d1 := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindDecision, Text: "d1", Plan: "general"}))
	d2 := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindDecision, Text: "d2", Plan: "general"}))
	i := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "w"}))
	markInReview(t, m, i.ID)
	review := must[[]*model.Entry](t)(m.Inbox(InboxFilter{Kind: model.KindReview}))[0]
	h := human(t, m, dir)
	shown := mullHead(t, dir)

	// d1 is settled elsewhere after the page showed it.
	must[AcceptResult](t)(m.Accept([]string{d1.ID}, "", SettleOpts{OnRequest: true}))
	tip := mullHead(t, dir)
	_, _, err := h.Apply(opLines(t, map[string]any{"op": "accept", "ids": []string{d1.ID, d2.ID}}), shown)
	wantCode(t, err, CodeConflict)
	wantErr(t, err, "entry "+d1.ID)
	if mullHead(t, dir) != tip {
		t.Fatal("a refused Apply wrote")
	}

	// A change elsewhere to a record the ops don't touch is no conflict.
	must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "unrelated", Plan: "general"}))
	if _, _, err := h.Apply(opLines(t, map[string]any{"op": "accept", "ids": []string{d2.ID}}), shown); err != nil {
		t.Fatalf("an unrelated change refused the write: %v", err)
	}

	// A record the ops write but don't name counts too: accepting a review
	// writes its item.
	shown = mullHead(t, dir)
	must[SetResult](t)(m.Set(i.ID, "notes", "changed while the page was open"))
	_, _, err = h.Apply(opLines(t, map[string]any{"op": "accept", "ids": []string{review.ID}}), shown)
	wantCode(t, err, CodeConflict)
	wantErr(t, err, "item "+i.ID)
}

func TestApplyInput(t *testing.T) {
	m, dir := setup(t)
	d := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindDecision, Text: "d", Plan: "general"}))
	h := human(t, m, dir)
	accept := opLines(t, map[string]any{"op": "accept", "ids": []string{d.ID}})

	_, _, err := h.Apply(accept, "")
	wantCode(t, err, CodeInvalid)
	_, _, err = h.Apply(nil, mullHead(t, dir))
	wantCode(t, err, CodeInvalid)
	_, _, err = h.Apply(accept, "0123456789abcdef0123456789abcdef01234567")
	wantCode(t, err, CodeInvalid)
	// A commit off the mull branch isn't a revision a page showed.
	_, _, err = h.Apply(accept, gittest.Git(t, dir, "rev-parse", "main"))
	wantCode(t, err, CodeInvalid)

	// Bad ops fail as a batch does, numbered from 1, and write nothing.
	tip := mullHead(t, dir)
	_, _, err = h.Apply(append(accept, json.RawMessage(`{"op": "accept", "ids": ["nope"]}`), json.RawMessage(`[]`)), tip)
	e := batchErr(t, err)
	if len(e.Lines) != 2 || e.Lines[0].Line != 2 || e.Lines[0].Code != CodeNotFound || e.Lines[1].Line != 3 || e.Lines[1].Code != CodeInvalidJSON {
		t.Errorf("lines = %+v", e.Lines)
	}
	if mullHead(t, dir) != tip {
		t.Fatal("a failed Apply wrote")
	}
}
