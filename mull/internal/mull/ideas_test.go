package mull

import (
	"strings"
	"testing"

	"mull/internal/model"
)

func TestIdeasFiledListedAndCounted(t *testing.T) {
	m, _ := setup(t)
	must[*model.Plan](t)(m.PlanNew("alpha", "Alpha", ""))
	must[*model.Plan](t)(m.PlanNew("beta", "Beta", ""))

	// An idea needs no plan, even with several plans: unlike other entries.
	loose := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindIdea, Text: "Mobile ID tool",
		Detail: "Phones in the field.", Source: "conversation 2026-11-09"}))
	if loose.Plan != "" || loose.Kind != model.KindIdea || loose.Detail != "Phones in the field." {
		t.Fatalf("idea = %+v", loose)
	}
	planned := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindIdea, Text: "Phenology explorer", Plan: "alpha"}))
	_, err := m.Capture(CaptureInput{Kind: model.KindIdea, Text: "x", Plan: "nope"})
	wantCode(t, err, CodeNotFound)
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "alpha", Title: "work"}))
	_, err = m.Capture(CaptureInput{Kind: model.KindIdea, Text: "x", Item: it.ID})
	wantCode(t, err, CodeInvalid)
	must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "a real question", Plan: "alpha"}))

	// Ideas stay out of the inbox...
	inbox := must[[]*model.Entry](t)(m.Inbox(InboxFilter{All: true}))
	if len(inbox) != 1 || inbox[0].Kind != model.KindQuestion {
		t.Fatalf("inbox = %+v", inbox)
	}
	// ...and are listed by Ideas.
	all := must[[]*model.Entry](t)(m.Ideas(IdeaFilter{}))
	if len(all) != 2 {
		t.Fatalf("ideas = %+v", all)
	}
	if none := must[[]*model.Entry](t)(m.Ideas(IdeaFilter{Plan: "none"})); len(none) != 1 || none[0].ID != loose.ID {
		t.Fatalf("ideas with no plan = %+v", none)
	}
	if a := must[[]*model.Entry](t)(m.Ideas(IdeaFilter{Plan: "alpha"})); len(a) != 1 || a[0].ID != planned.ID {
		t.Fatalf("alpha ideas = %+v", a)
	}

	// The board shows counts, not lists, and needs_you leaves ideas out.
	v := must[View](t)(m.Board(""))
	var alpha PlanBoard
	for _, p := range v.Plans {
		if p.Slug == "alpha" {
			alpha = p
		}
	}
	if alpha.Ideas != 1 || alpha.NeedsYou.total() != 1 || v.IdeasUnplanned != 1 {
		t.Fatalf("board alpha = ideas %d needs %+v; unplanned %d", alpha.Ideas, alpha.NeedsYou, v.IdeasUnplanned)
	}
	text := v.Text(m.Now())
	if !strings.Contains(text, "IDEAS  1 idea (mull ideas --plan alpha)") || !strings.Contains(text, "IDEAS  1 idea with no plan") {
		t.Fatalf("board text:\n%s", text)
	}
}

func TestPromotingIdeas(t *testing.T) {
	m, _ := setup(t)
	must[*model.Plan](t)(m.PlanNew("alpha", "Alpha", ""))
	asItem := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindIdea, Text: "Host-family filter", Plan: "alpha", Detail: "Browse by family.", Source: "matter 67c9"}))
	asPhase := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindIdea, Text: "iNaturalist integration", Plan: "alpha"}))
	noPlan := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindIdea, Text: "DNA barcodes"}))

	// Only the human promotes ideas: an agent needs the human's say-so.
	_, err := m.Do(DoInput{ID: asItem.ID})
	wantCode(t, err, CodeNotAllowed)
	// Ideas are promoted, dropped or parked; never accepted or decided.
	_, err = m.Accept([]string{asItem.ID}, "", SettleOpts{OnRequest: true})
	wantCode(t, err, CodeNotAllowed)
	_, err = m.Decide(asItem.ID, "yes", SettleOpts{OnRequest: true})
	wantCode(t, err, CodeNotAllowed)

	res := must[DoResult](t)(m.Do(DoInput{ID: asItem.ID, SettleOpts: SettleOpts{OnRequest: true}}))
	if res.Item == nil || res.Item.From != asItem.ID || res.Item.Plan != "alpha" || res.Phase != nil {
		t.Fatalf("promoted item = %+v", res)
	}
	if !strings.Contains(res.Item.Notes, "Browse by family.") || !strings.Contains(res.Item.Notes, "matter 67c9") {
		t.Errorf("promoted item notes = %q", res.Item.Notes)
	}
	if res.Entry.Settled.Item != res.Item.ID || !res.Entry.Settled.OnRequest {
		t.Errorf("settlement = %+v", res.Entry.Settled)
	}

	res = must[DoResult](t)(m.Do(DoInput{ID: asPhase.ID, As: "phase", SettleOpts: SettleOpts{OnRequest: true}}))
	if res.Phase == nil || res.Phase.From != asPhase.ID || res.Phase.Plan != "alpha" || res.Item != nil || res.Entry.Settled.Phase != res.Phase.ID {
		t.Fatalf("promoted phase = %+v", res)
	}
	// Items can then go in the phase.
	must[ItemView](t)(m.ItemAdd(NewItem{Phase: res.Phase.ID, Title: "Pull observations"}))

	// An idea with no plan needs one to become work.
	_, err = m.Do(DoInput{ID: noPlan.ID, SettleOpts: SettleOpts{OnRequest: true}})
	wantCode(t, err, CodeScope)
	res = must[DoResult](t)(m.Do(DoInput{ID: noPlan.ID, Plan: "alpha", SettleOpts: SettleOpts{OnRequest: true}}))
	if res.Item.Plan != "alpha" {
		t.Fatalf("promoted unplanned idea = %+v", res.Item)
	}

	// Promoted ideas leave the open list but stay on record with --all.
	if n := len(must[[]*model.Entry](t)(m.Ideas(IdeaFilter{}))); n != 0 {
		t.Fatalf("%d ideas still open", n)
	}
	if n := len(must[[]*model.Entry](t)(m.Ideas(IdeaFilter{All: true}))); n != 3 {
		t.Fatalf("--all lists %d ideas, want 3", n)
	}
	_, err = m.Do(DoInput{ID: asItem.ID, SettleOpts: SettleOpts{OnRequest: true}})
	wantCode(t, err, CodeSettled)
	_, err = m.Do(DoInput{ID: asPhase.ID, As: "story", SettleOpts: SettleOpts{OnRequest: true}})
	wantCode(t, err, CodeInvalid)
}

func TestIdeasParkAndDrop(t *testing.T) {
	m, _ := setup(t)
	a := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindIdea, Text: "a"}))
	b := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindIdea, Text: "b"}))
	must[*model.Entry](t)(m.Park(a.ID, "after launch", SettleOpts{OnRequest: true}))
	must[DropResult](t)(m.Drop([]string{b.ID}, "not worth it", SettleOpts{OnRequest: true}))
	if n := len(must[[]*model.Entry](t)(m.Ideas(IdeaFilter{}))); n != 0 {
		t.Fatalf("%d open ideas", n)
	}
	if p := must[[]*model.Entry](t)(m.Ideas(IdeaFilter{Parked: true})); len(p) != 1 || p[0].ID != a.ID {
		t.Fatalf("parked = %+v", p)
	}
	// A parked idea can still be promoted later.
	must[*model.Plan](t)(m.PlanNew("later", "Later", ""))
	res := must[DoResult](t)(m.Do(DoInput{ID: a.ID, Plan: "later", SettleOpts: SettleOpts{OnRequest: true}}))
	if res.Item.From != a.ID {
		t.Fatalf("promoted parked idea = %+v", res.Item)
	}
}

func TestIdeasInBatch(t *testing.T) {
	m, dir := setup(t)
	input := strings.Join([]string{
		`{"op": "plan new", "ref": "p", "slug": "alpha", "title": "Alpha"}`,
		`{"op": "idea", "ref": "i1", "text": "Mobile ID tool", "detail": "Phones in the field.", "source": "conversation"}`,
		`{"op": "idea", "ref": "i2", "plan": "@p", "text": "iNaturalist integration"}`,
		`{"op": "do", "ref": "e", "id": "@i2", "as": "phase", "on-request": true}`,
		`{"op": "add", "phase": "@e", "title": "Pull observations"}`,
	}, "\n")
	results, err := m.Batch(strings.NewReader(input), false)
	if err != nil {
		t.Fatal(err)
	}
	phase := results[3].Result.(DoResult).Phase
	if phase == nil || results[3].ID != phase.ID || results[4].Result.(ItemView).Phase != phase.ID {
		t.Fatalf("results = %+v", results)
	}
	if ideas := must[[]*model.Entry](t)(m.Ideas(IdeaFilter{})); len(ideas) != 1 || ideas[0].Text != "Mobile ID tool" {
		t.Fatalf("open ideas = %+v", ideas)
	}

	// Promoting without the human's say-so fails the whole batch.
	before := mullHead(t, dir)
	_, err = m.Batch(strings.NewReader(`{"op": "idea", "ref": "x", "plan": "alpha", "text": "x"}
{"op": "do", "id": "@x"}`), false)
	if e := batchErr(t, err); len(e.Lines) != 1 || e.Lines[0].Line != 2 || e.Lines[0].Code != CodeNotAllowed {
		t.Fatalf("lines = %+v", e.Lines)
	}
	if mullHead(t, dir) != before {
		t.Fatal("failed batch wrote")
	}
	// Ideas take no item, in batch as on the command line.
	_, err = m.Batch(strings.NewReader(`{"op": "idea", "text": "x", "item": "abcd"}`), true)
	if e := batchErr(t, err); e.Lines[0].Code != CodeInvalid {
		t.Fatalf("lines = %+v", e.Lines)
	}
}
