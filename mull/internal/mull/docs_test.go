package mull

import (
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
	"mull/internal/store"
)

func TestPlanAndPhaseDocs(t *testing.T) {
	m, dir := setup(t)
	// A shaping plan takes a doc: a shaping plan is its doc and questions.
	must[*model.Plan](t)(m.PlanNew("launch", "Public launch", "Billing, then the switch.", true))
	if d := must[DocResult](t)(m.PlanDoc("launch")); d.Type != "plan" || d.ID != "launch" || d.Doc != "" {
		t.Fatalf("doc before any = %+v", d)
	}
	doc := "# Launch\n\nDone when: paying customers.\n\n---\nWhy the order: billing first."
	if d := must[DocResult](t)(m.SetPlanDoc("launch", doc+"\n\n")); d.Doc != doc {
		t.Fatalf("set plan doc = %+v", d)
	}

	// The doc is its own file: plain markdown, no frontmatter, beside the
	// record, which is untouched (its summary included).
	file := gittest.Git(t, dir, "show", store.Ref+":docs/plans/launch.md")
	if file != strings.TrimSpace(doc) {
		t.Fatalf("doc file = %q", file)
	}
	changed := gittest.Git(t, dir, "show", "--format=", "--name-only", store.Ref)
	if changed != "docs/plans/launch.md" {
		t.Fatalf("plan doc commit touched %q", changed)
	}
	pv := must[PlanView](t)(m.PlanShow("launch"))
	if pv.Doc != doc || pv.Summary != "Billing, then the switch." || pv.State != model.PlanShaping {
		t.Fatalf("plan show = %+v", pv.Plan)
	}

	// Writing the same doc again writes nothing.
	before := gittest.Git(t, dir, "rev-parse", store.Ref)
	must[DocResult](t)(m.SetPlanDoc("launch", doc))
	if after := gittest.Git(t, dir, "rev-parse", store.Ref); after != before {
		t.Fatalf("an unchanged doc made a commit")
	}

	// Phases: the doc shows on the phase and in the context of its items.
	must[*model.Plan](t)(m.PlanAgree("launch", true))
	ph := must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "Stripe first.", nil))
	must[DocResult](t)(m.SetPhaseDoc(ph.ID, "Design: webhooks.\nDone when: invoices go out."))
	if file := gittest.Git(t, dir, "show", store.Ref+":docs/phases/"+ph.ID+".md"); file != "Design: webhooks.\nDone when: invoices go out." {
		t.Fatalf("phase doc file = %q", file)
	}
	it := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "Webhook"}))
	if ps := must[[]PhaseView](t)(m.PhaseList("launch")); len(ps) != 1 || ps[0].Doc != "Design: webhooks.\nDone when: invoices go out." || ps[0].Summary != "Stripe first." {
		t.Fatalf("phase list = %+v", ps)
	}
	ctx := must[Context](t)(m.Context(it.ID))
	if ctx.Item.PhaseDoc != "Design: webhooks.\nDone when: invoices go out." || ctx.Item.PhaseSummary != "Stripe first." {
		t.Fatalf("context item = %+v", ctx.Item)
	}

	// History: one version per change, newest first; removing it is a version.
	must[DocResult](t)(m.SetPhaseDoc(ph.ID, "Design: webhooks, retried.\nDone when: invoices go out."))
	if d := must[DocResult](t)(m.SetPhaseDoc(ph.ID, "\n")); d.Doc != "" {
		t.Fatalf("emptied doc = %+v", d)
	}
	if sh := must[Shown](t)(m.Show(ph.ID)); sh.Phase.Doc != "" {
		t.Fatalf("removed doc still shown: %+v", sh.Phase)
	}
	log := must[DocLog](t)(m.PhaseDocLog(ph.ID))
	if log.Type != "phase" || log.ID != ph.ID || len(log.Versions) != 3 {
		t.Fatalf("phase doc log = %+v", log)
	}
	if log.Versions[0].Doc != "" || !strings.Contains(log.Versions[1].Doc, "retried") || strings.Contains(log.Versions[2].Doc, "retried") {
		t.Fatalf("versions = %+v", log.Versions)
	}
	for _, v := range log.Versions {
		if len(v.Commit) < 40 || !strings.HasSuffix(v.At, "Z") {
			t.Fatalf("version = %+v", v)
		}
	}
	// A record change isn't a doc version.
	must[SetResult](t)(m.Set("launch", "title", "The launch"))
	if pl := must[DocLog](t)(m.PlanDocLog("launch")); len(pl.Versions) != 1 || pl.Versions[0].Doc != doc {
		t.Fatalf("plan doc log = %+v", pl)
	}

	// Unknown plans and phases.
	_, err := m.PlanDoc("nope")
	wantCode(t, err, CodeNotFound)
	_, err = m.SetPhaseDoc("ffff", "x")
	wantCode(t, err, CodeNotFound)
	_, err = m.PhaseDocLog("ffff")
	wantCode(t, err, CodeNotFound)
}

func TestSyncPushesDocs(t *testing.T) {
	m, dir := setup(t)
	remote := bareRemote(t, dir)
	must[DocResult](t)(m.SetPlanDoc("general", "Work outside any big effort."))
	if res := must[SyncResult](t)(m.Sync(false)); res.Action != "pushed" {
		t.Fatalf("sync = %+v", res)
	}
	if got := gittest.Git(t, remote, "show", store.Ref+":docs/plans/general.md"); got != "Work outside any big effort." {
		t.Fatalf("remote doc = %q", got)
	}
}

func TestDocsInBatch(t *testing.T) {
	m, _ := setup(t)
	in := `{"op": "plan new", "ref": "p", "slug": "launch", "title": "Launch", "on-request": true}
{"op": "plan doc", "slug": "@p", "doc": "The picture."}
{"op": "plan agree", "slug": "@p", "on-request": true}
{"op": "phase add", "ref": "b", "plan": "@p", "title": "Billing"}
{"op": "phase doc", "id": "@b", "doc": "Stripe first."}
`
	res := must[[]BatchResult](t)(m.Batch(strings.NewReader(in), false))
	if len(res) != 5 || res[1].ID != "launch" || res[4].ID != res[3].ID {
		t.Fatalf("batch = %+v", res)
	}
	if d := res[4].Result.(DocResult); d.Type != "phase" || d.Doc != "Stripe first." {
		t.Fatalf("phase doc result = %+v", d)
	}
	if d := must[DocResult](t)(m.PlanDoc("launch")); d.Doc != "The picture." {
		t.Fatalf("plan doc = %+v", d)
	}
	// doc is required: leaving it out can't clear a doc.
	_, err := m.Batch(strings.NewReader(`{"op": "plan doc", "slug": "launch"}`), false)
	wantCode(t, err, CodeBatchInvalid)
	if e := AsError(err); len(e.Lines) != 1 || e.Lines[0].Code != CodeInvalid {
		t.Fatalf("lines = %+v", e.Lines)
	}
	// "" removes it.
	must[[]BatchResult](t)(m.Batch(strings.NewReader(`{"op": "plan doc", "slug": "launch", "doc": ""}`), false))
	if d := must[DocResult](t)(m.PlanDoc("launch")); d.Doc != "" {
		t.Fatalf("plan doc after removal = %+v", d)
	}
}
