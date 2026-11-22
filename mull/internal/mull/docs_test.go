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

func TestPlanAndPhaseRules(t *testing.T) {
	m, dir := setup(t)
	must[*model.Plan](t)(m.PlanNew("launch", "Public launch", "", true))
	if r := must[RulesResult](t)(m.PlanRules("launch")); r.Type != "plan" || r.ID != "launch" || r.Rules != "" {
		t.Fatalf("rules before any = %+v", r)
	}
	rules := "- Use the shared components in lib/ui.\n- Don't edit priv/static."
	if r := must[RulesResult](t)(m.SetPlanRules("launch", rules+"\n\n")); r.Rules != rules {
		t.Fatalf("set plan rules = %+v", r)
	}

	// Rules are their own file, under rules/ plus the record's path, and
	// writing them touches nothing else: not the record, not the doc.
	if file := gittest.Git(t, dir, "show", store.Ref+":rules/plans/launch.md"); file != rules {
		t.Fatalf("rules file = %q", file)
	}
	if changed := gittest.Git(t, dir, "show", "--format=", "--name-only", store.Ref); changed != "rules/plans/launch.md" {
		t.Fatalf("plan rules commit touched %q", changed)
	}
	if pv := must[PlanView](t)(m.PlanShow("launch")); pv.Rules != rules || pv.Doc != "" {
		t.Fatalf("plan show = %+v", pv.Plan)
	}
	before := gittest.Git(t, dir, "rev-parse", store.Ref)
	must[RulesResult](t)(m.SetPlanRules("launch", rules))
	if after := gittest.Git(t, dir, "rev-parse", store.Ref); after != before {
		t.Fatalf("unchanged rules made a commit")
	}

	// context --item gives the item's plan's rules, then its phase's, in
	// full, with a protocol line saying to follow them.
	must[*model.Plan](t)(m.PlanAgree("launch", true))
	ph := must[*model.Phase](t)(m.PhaseAdd("launch", "Billing", "", nil))
	it := must[ItemView](t)(m.ItemAdd(NewItem{Phase: ph.ID, Title: "Webhook"}))
	ctx := must[Context](t)(m.Context(it.ID))
	if ctx.Item.PlanRules != rules || ctx.Item.PhaseRules != "" || !strings.Contains(strings.Join(ctx.Protocol, "\n"), "plan_rules, then its phase_rules") {
		t.Fatalf("context = %+v, protocol %v", ctx.Item, ctx.Protocol)
	}
	must[RulesResult](t)(m.SetPhaseRules(ph.ID, "Test against Stripe's test mode."))
	if file := gittest.Git(t, dir, "show", store.Ref+":rules/phases/"+ph.ID+".md"); file != "Test against Stripe's test mode." {
		t.Fatalf("phase rules file = %q", file)
	}
	ctx = must[Context](t)(m.Context(it.ID))
	if ctx.Item.PlanRules != rules || ctx.Item.PhaseRules != "Test against Stripe's test mode." {
		t.Fatalf("context item = %+v", ctx.Item)
	}
	if sh := must[Shown](t)(m.Show(ph.ID)); sh.Phase.Rules != "Test against Stripe's test mode." {
		t.Fatalf("phase show = %+v", sh.Phase)
	}
	// Context without an item has no rules line.
	if c := must[Context](t)(m.Context("")); strings.Contains(strings.Join(c.Protocol, "\n"), "plan_rules") {
		t.Fatalf("protocol without an item = %v", c.Protocol)
	}

	// Empty rules remove them, and with none there's no rules line.
	must[RulesResult](t)(m.SetPlanRules("launch", ""))
	must[RulesResult](t)(m.SetPhaseRules(ph.ID, " \n"))
	ctx = must[Context](t)(m.Context(it.ID))
	if ctx.Item.PlanRules != "" || ctx.Item.PhaseRules != "" || strings.Contains(strings.Join(ctx.Protocol, "\n"), "plan_rules") {
		t.Fatalf("context without rules = %+v, protocol %v", ctx.Item, ctx.Protocol)
	}

	// History: a version per change, the removal included.
	log := must[RulesLog](t)(m.PlanRulesLog("launch"))
	if log.Type != "plan" || log.ID != "launch" || len(log.Versions) != 2 || log.Versions[0].Rules != "" || log.Versions[1].Rules != rules {
		t.Fatalf("plan rules log = %+v", log)
	}
	if pl := must[RulesLog](t)(m.PhaseRulesLog(ph.ID)); len(pl.Versions) != 2 {
		t.Fatalf("phase rules log = %+v", pl)
	}

	_, err := m.PlanRules("nope")
	wantCode(t, err, CodeNotFound)
	_, err = m.SetPhaseRules("ffff", "x")
	wantCode(t, err, CodeNotFound)
	_, err = m.PhaseRulesLog("ffff")
	wantCode(t, err, CodeNotFound)
}

func TestRulesInBatchAndSync(t *testing.T) {
	m, dir := setup(t)
	remote := bareRemote(t, dir)
	in := `{"op": "plan new", "ref": "p", "slug": "launch", "title": "Launch", "on-request": true}
{"op": "plan rules", "slug": "@p", "rules": "Branch from main."}
{"op": "plan agree", "slug": "@p", "on-request": true}
{"op": "phase add", "ref": "b", "plan": "@p", "title": "Billing"}
{"op": "phase rules", "id": "@b", "rules": "Stripe test mode only."}
`
	res := must[[]BatchResult](t)(m.Batch(strings.NewReader(in), false))
	if r := res[4].Result.(RulesResult); r.Type != "phase" || r.ID != res[3].ID || r.Rules != "Stripe test mode only." {
		t.Fatalf("phase rules result = %+v", r)
	}
	// rules is required: leaving it out can't clear them.
	_, err := m.Batch(strings.NewReader(`{"op": "plan rules", "slug": "launch"}`), false)
	wantCode(t, err, CodeBatchInvalid)

	// Rules are mull data, so sync pushes them.
	if r := must[SyncResult](t)(m.Sync(false)); r.Action != "pushed" {
		t.Fatalf("sync = %+v", r)
	}
	if got := gittest.Git(t, remote, "show", store.Ref+":rules/plans/launch.md"); got != "Branch from main." {
		t.Fatalf("remote rules = %q", got)
	}
}
