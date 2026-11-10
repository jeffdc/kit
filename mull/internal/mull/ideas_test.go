package mull

import (
	"strings"
	"testing"

	"mull/internal/model"
)

func TestLooseEndsAndInboxIdeasAreRetired(t *testing.T) {
	m, dir := setup(t)
	_, err := m.Capture(CaptureInput{Kind: model.KindLoose, Text: "x", Plan: "general"})
	wantCode(t, err, CodeRetired)
	wantErr(t, err, "mull bug")
	_, err = m.Capture(CaptureInput{Kind: model.KindIdea, Text: "x"})
	wantCode(t, err, CodeRetired)

	before := mullHead(t, dir)
	_, err = m.Batch(strings.NewReader(`{"op": "loose", "plan": "general", "text": "x"}`), false)
	if e := batchErr(t, err); e.Lines[0].Code != CodeRetired {
		t.Fatalf("lines = %+v", e.Lines)
	}
	if mullHead(t, dir) != before {
		t.Fatal("failed batch wrote")
	}
}

func TestLegacyEntriesStayOutOfNeedsYou(t *testing.T) {
	m, _ := setup(t)
	must[*model.Plan](t)(m.PlanNew("alpha", "Alpha", ""))
	loose := must[*model.Entry](t)(m.captureLegacy(CaptureInput{Kind: model.KindLoose, Text: "Stale branch", Plan: "alpha"}))
	idea := must[*model.Entry](t)(m.captureLegacy(CaptureInput{Kind: model.KindIdea, Text: "Mobile ID tool"}))
	must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "a real question", Plan: "alpha"}))

	// Needs you is questions, decisions and reviews; the retired kinds are
	// listed only by kind.
	inbox := must[[]*model.Entry](t)(m.Inbox(InboxFilter{All: true}))
	if len(inbox) != 1 || inbox[0].Kind != model.KindQuestion {
		t.Fatalf("inbox = %+v", inbox)
	}
	if l := must[[]*model.Entry](t)(m.Inbox(InboxFilter{Kind: model.KindLoose})); len(l) != 1 || l[0].ID != loose.ID {
		t.Fatalf("loose ends = %+v", l)
	}
	if l := must[[]*model.Entry](t)(m.Inbox(InboxFilter{Kind: model.KindIdea, Plan: model.NoPlan})); len(l) != 1 || l[0].ID != idea.ID {
		t.Fatalf("ideas with no plan = %+v", l)
	}

	v := must[View](t)(m.Board(""))
	var alpha PlanBoard
	for _, p := range v.Plans {
		if p.Slug == "alpha" {
			alpha = p
		}
	}
	if alpha.NeedsYou.total() != 1 || v.Legacy == nil || *v.Legacy != (Legacy{LooseEnds: 1, Ideas: 1}) || v.Unplanned != nil {
		t.Fatalf("board alpha needs %+v; legacy %+v; unplanned %+v", alpha.NeedsYou, v.Legacy, v.Unplanned)
	}
	if text := v.Text(m.Now()); !strings.Contains(text, "LEGACY  1 loose end (mull inbox --kind loose-end) · 1 inbox idea") {
		t.Fatalf("board text:\n%s", text)
	}

	// Settled, they leave the legacy count.
	must[DropResult](t)(m.Drop([]string{loose.ID, idea.ID}, "", SettleOpts{OnRequest: true}))
	if v := must[View](t)(m.Board("")); v.Legacy != nil {
		t.Fatalf("legacy = %+v", v.Legacy)
	}
}

func TestPromotingLegacyIdeas(t *testing.T) {
	m, _ := setup(t)
	must[*model.Plan](t)(m.PlanNew("alpha", "Alpha", ""))
	asItem := must[*model.Entry](t)(m.captureLegacy(CaptureInput{Kind: model.KindIdea, Text: "Host-family filter", Plan: "alpha", Detail: "Browse by family.", Source: "matter 67c9"}))
	asPhase := must[*model.Entry](t)(m.captureLegacy(CaptureInput{Kind: model.KindIdea, Text: "iNaturalist integration", Plan: "alpha"}))
	noPlan := must[*model.Entry](t)(m.captureLegacy(CaptureInput{Kind: model.KindIdea, Text: "DNA barcodes"}))

	// Only the human promotes ideas.
	_, err := m.Do(DoInput{ID: asItem.ID})
	wantCode(t, err, CodeNotAllowed)
	// Ideas are promoted, dropped or parked; never accepted or decided.
	_, err = m.Accept([]string{asItem.ID}, "", SettleOpts{})
	wantCode(t, err, CodeNotAllowed)

	res := must[DoResult](t)(m.Do(DoInput{ID: asItem.ID, SettleOpts: SettleOpts{OnRequest: true}}))
	if res.Item == nil || res.Item.Plan != "alpha" || res.Item.From != asItem.ID ||
		!strings.Contains(res.Item.Notes, "Browse by family.") || !strings.Contains(res.Item.Notes, "matter 67c9") {
		t.Fatalf("promoted item = %+v", res.Item)
	}
	ph := must[DoResult](t)(m.Do(DoInput{ID: asPhase.ID, As: "phase", SettleOpts: SettleOpts{OnRequest: true}}))
	if ph.Phase == nil || ph.Phase.From != asPhase.ID || ph.Entry.Settled.Phase != ph.Phase.ID {
		t.Fatalf("promoted phase = %+v", ph)
	}
	// An idea with no plan needs one to become work, or --plan none.
	_, err = m.Do(DoInput{ID: noPlan.ID, SettleOpts: SettleOpts{OnRequest: true}})
	wantCode(t, err, CodeScope)
	np := must[DoResult](t)(m.Do(DoInput{ID: noPlan.ID, Plan: model.NoPlan, SettleOpts: SettleOpts{OnRequest: true}}))
	if np.Item.Plan != "" {
		t.Fatalf("unplanned promotion = %+v", np.Item)
	}
}

func TestPromotingLegacyIdeasInBatch(t *testing.T) {
	m, dir := setup(t)
	must[*model.Plan](t)(m.PlanNew("alpha", "Alpha", ""))
	i1 := must[*model.Entry](t)(m.captureLegacy(CaptureInput{Kind: model.KindIdea, Text: "Mobile ID tool"}))
	i2 := must[*model.Entry](t)(m.captureLegacy(CaptureInput{Kind: model.KindIdea, Plan: "alpha", Text: "iNaturalist integration"}))
	input := strings.Join([]string{
		`{"op": "do", "ref": "e", "id": "` + i2.ID + `", "as": "phase", "on-request": true}`,
		`{"op": "add", "phase": "@e", "title": "Pull observations"}`,
	}, "\n")
	results, err := m.Batch(strings.NewReader(input), false)
	if err != nil {
		t.Fatal(err)
	}
	phase := results[0].Result.(DoResult).Phase
	if phase == nil || results[0].ID != phase.ID || results[1].Result.(ItemView).Phase != phase.ID {
		t.Fatalf("results = %+v", results)
	}

	// Promoting without the human's say-so fails the whole batch.
	before := mullHead(t, dir)
	_, err = m.Batch(strings.NewReader(`{"op": "add", "plan": "alpha", "title": "x"}
{"op": "do", "id": "`+i1.ID+`", "plan": "alpha"}`), false)
	if e := batchErr(t, err); len(e.Lines) != 1 || e.Lines[0].Line != 2 || e.Lines[0].Code != CodeNotAllowed {
		t.Fatalf("lines = %+v", e.Lines)
	}
	if mullHead(t, dir) != before {
		t.Fatal("failed batch wrote")
	}
}
