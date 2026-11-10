package mull

import (
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
	"mull/internal/store"
)

// format1 is a mull branch as the mull before Model v2 wrote it: phases in
// epics/, items with state: (open, done, accepted) and epic:, a closed plan,
// the standing general plan, item needs and a phase need on items, loose
// ends and inbox ideas, and a settlement with epic:.
var format1 = map[string]string{
	"config.yml": "integration: main\nrelease: main\nremote: origin\ndone_checks: []\nlayers: {}\n",
	"plans/general.md": "---\ntitle: General\nstate: active\ncreated: \"2026-11-01T00:00:00Z\"\nupdated: \"2026-11-01T00:00:00Z\"\n---\n\n" +
		"Work outside any big effort.\n",
	"plans/launch.md": "---\ntitle: Launch\nstate: active\ncreated: \"2026-11-01T00:00:00Z\"\nupdated: \"2026-11-01T00:00:00Z\"\n---\n\nShip it.\n",
	"plans/old.md":    "---\ntitle: Old\nstate: closed\ncreated: \"2026-11-01T00:00:00Z\"\nupdated: \"2026-11-02T00:00:00Z\"\n---\n",
	"epics/1a2b.md": "---\nplan: launch\ntitle: Billing\ncreated: \"2026-11-01T00:00:00Z\"\nupdated: \"2026-11-01T00:00:00Z\"\n---\n\n" +
		"Stripe first.\n",
	"docs/epics/1a2b.md": "Design: webhooks.\n",
	"epics/3c4d.md": "---\nplan: launch\ntitle: Go live\nneeds:\n  - aa01\n  - aa02\ncreated: \"2026-11-01T00:00:00Z\"\nupdated: \"2026-11-01T00:00:00Z\"\n---\n\n" +
		"Flip the switch.\n",
	"items/aa01.md": "---\nplan: launch\nepic: 1a2b\ntitle: Webhook\nstate: open\ncreated: \"2026-11-01T00:00:00Z\"\nupdated: \"2026-11-01T00:00:00Z\"\n---\n",
	"items/aa02.md": "---\nplan: launch\nepic: 1a2b\ntitle: Invoices\nstate: accepted\ncreated: \"2026-11-01T00:00:00Z\"\nupdated: \"2026-11-01T00:00:00Z\"\n---\n",
	"items/aa03.md": "---\nplan: launch\nepic: 1a2b\ntitle: Refunds\nstate: done\ncreated: \"2026-11-01T00:00:00Z\"\nupdated: \"2026-11-01T00:00:00Z\"\n---\n",
	"items/aa04.md": "---\nplan: general\ntitle: Tidy the README\nstate: open\nneeds:\n  - aa01\ncreated: \"2026-11-01T00:00:00Z\"\nupdated: \"2026-11-01T00:00:00Z\"\n---\n\nShort.\n",
	"items/aa05.md": "---\nplan: general\ntitle: Flaky test\nstate: captured\nkind: bug\ncreated: \"2026-11-01T00:00:00Z\"\nupdated: \"2026-11-01T00:00:00Z\"\n---\n",
	"items/aa06.md": "---\nplan: launch\ntitle: Copy\nstate: accepted\nneeds:\n  - aa02\ncreated: \"2026-11-01T00:00:00Z\"\nupdated: \"2026-11-01T00:00:00Z\"\n---\n",
	"inbox/bb01.md": "---\nkind: loose-end\ntext: Footer links to old pricing\nplan: launch\nitem: aa01\nsource: session 3\nby: /w/a\ncreated: \"2026-11-03T00:00:00Z\"\n---\n\n" +
		"Three links.\n",
	"inbox/bb02.md": "---\nkind: idea\ntext: Mobile ID tool\nplan: \"\"\nby: /w/a\ncreated: \"2026-11-03T00:00:00Z\"\n---\n",
	"inbox/bb03.md": "---\nkind: loose-end\ntext: Stale branch\nplan: general\nby: /w/a\ncreated: \"2026-11-03T00:00:00Z\"\n" +
		"settled:\n  action: park\n  note: after launch\n  by: /w/h\n  at: \"2026-11-04T00:00:00Z\"\n---\n",
	"inbox/bb04.md": "---\nkind: loose-end\ntext: Decided already\nplan: launch\nby: /w/a\ncreated: \"2026-11-03T00:00:00Z\"\n" +
		"settled:\n  action: decide\n  note: leave it\n  by: /w/h\n  at: \"2026-11-04T00:00:00Z\"\n---\n",
	"inbox/bb05.md": "---\nkind: question\ntext: Which licence?\nplan: general\nby: /w/a\ncreated: \"2026-11-03T00:00:00Z\"\n---\n",
	"inbox/bb06.md": "---\nkind: idea\ntext: Billing\nplan: launch\nby: /w/a\ncreated: \"2026-11-01T00:00:00Z\"\n" +
		"settled:\n  action: do\n  epic: 1a2b\n  by: /w/h\n  on_request: true\n  at: \"2026-11-01T00:00:00Z\"\n---\n",
}

// setupFormat1 makes a repository whose mull branch holds format1, written
// directly through the store.
func setupFormat1(t *testing.T) (*Mull, string) {
	t.Helper()
	dir := gittest.NewRepo(t)
	m, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for p, c := range format1 {
		files[p] = []byte(c)
	}
	if err := m.store.Init("mull: format 1 fixture", files); err != nil {
		t.Fatal(err)
	}
	return m, dir
}

func head(t *testing.T, dir string) string {
	t.Helper()
	return gittest.Git(t, dir, "rev-parse", store.Ref)
}

func file(t *testing.T, m *Mull, path string) (string, bool) {
	t.Helper()
	var out string
	var ok bool
	err := m.store.View(func(tx *store.Tx) error {
		b, found := tx.Get(path)
		out, ok = string(b), found
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out, ok
}

func TestFormat1IsReadButNotWritten(t *testing.T) {
	m, dir := setupFormat1(t)
	before := head(t, dir)

	// Reads work, with the old names read correctly: stored done is in review.
	for id, want := range map[string]string{"aa01": model.StateCommitted, "aa02": model.StateDone, "aa03": model.StateInReview} {
		if v := item(t, m, id); v.State != want || v.Phase != "1a2b" {
			t.Errorf("%s = %s in %q, want %s in 1a2b", id, v.State, v.Phase, want)
		}
	}
	if p := must[PlanView](t)(m.PlanShow("old")); p.State != model.PlanDone {
		t.Errorf("old plan = %s", p.State)
	}
	if d := must[DocResult](t)(m.PhaseDoc("1a2b")); d.Doc != "Design: webhooks." {
		t.Errorf("format 1 phase doc = %q", d.Doc)
	}
	v := must[View](t)(m.Board(""))
	if v.Format != 1 || !v.MigrationRequired || !strings.Contains(v.Text(m.Now()), "MIGRATE") {
		t.Errorf("view format %d, migration_required %v", v.Format, v.MigrationRequired)
	}
	if c := must[Context](t)(m.Context("")); !strings.Contains(c.Protocol[0], "migration_required") {
		t.Errorf("context protocol = %v", c.Protocol)
	}
	if cfg := must[model.Config](t)(m.Config()); cfg.Format != 1 {
		t.Errorf("config format = %d", cfg.Format)
	}

	// Every write is refused, so nothing writes the old names under the new
	// format or the other way round.
	_, err := m.ItemAdd(NewItem{Plan: "launch", Title: "New"})
	wantCode(t, err, CodeMigrationRequired)
	_, err = m.Start("aa01")
	wantCode(t, err, CodeMigrationRequired)
	_, err = m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "x"})
	wantCode(t, err, CodeMigrationRequired)
	_, err = m.Batch(strings.NewReader(`{"op": "ask", "plan": "launch", "text": "x"}`+"\n"), true)
	wantCode(t, err, CodeMigrationRequired)
	if head(t, dir) != before {
		t.Fatal("a refused write moved the mull branch")
	}
}

func TestMigrateDryRunOnlyReads(t *testing.T) {
	m, dir := setupFormat1(t)
	before := head(t, dir)
	r := must[MigrateResult](t)(m.Migrate(true, false))
	if head(t, dir) != before {
		t.Fatal("a dry run moved the mull branch")
	}
	if !r.DryRun || !r.Changed || r.FormatBefore != 1 || r.Format != 2 || r.Commit != "" {
		t.Fatalf("dry run = %+v", r)
	}
	want := MigrateCounts{Plans: 3, Phases: 2, Items: 6, Entries: 5, Docs: 3, Captured: 3, Needs: 4}
	if r.Counts != want {
		t.Errorf("counts = %+v, want %+v", r.Counts, want)
	}
	// It lists the converted loose ends and ideas, for the human to re-kind.
	from := map[string]MigrateCapture{}
	for _, c := range r.Captured {
		from[c.Entry] = c
	}
	if c := from["bb01"]; c.Kind != model.ItemIdea || c.EntryKind != model.KindLoose || c.Plan != "launch" || c.Was != "open" {
		t.Errorf("bb01 captured as %+v", c)
	}
	if c := from["bb03"]; c.Was != "parked" || c.Plan != "" {
		t.Errorf("bb03 captured as %+v", c)
	}
	if c := from["bb02"]; c.EntryKind != model.KindIdea {
		t.Errorf("bb02 captured as %+v", c)
	}
	if _, ok := from["bb04"]; ok {
		t.Error("a settled loose end was captured")
	}
	text := r.Text()
	for _, s := range []string{"Nothing was written", "epics/1a2b.md → phases/1a2b.md", "state: done → stage: in-review",
		"mull set <id> kind bug", "retired", "converted to phase 1a2b"} {
		if !strings.Contains(text, s) {
			t.Errorf("dry run text lacks %q:\n%s", s, text)
		}
	}
}

func TestMigrate(t *testing.T) {
	m, dir := setupFormat1(t)
	before := head(t, dir)

	_, err := m.Migrate(false, false)
	wantCode(t, err, CodeNotAllowed)

	r := must[MigrateResult](t)(m.Migrate(false, true))
	after := head(t, dir)
	if !r.Changed || r.DryRun || r.Commit != after {
		t.Fatalf("migrate = %+v (head %s)", r, after)
	}
	// One commit, straight on top of the old data.
	if parent := gittest.Git(t, dir, "rev-parse", after+"^"); parent != before {
		t.Fatalf("migrate wrote more than one commit: parent %s, before %s", parent, before)
	}

	// The files are in format 2.
	for _, p := range []string{"epics/1a2b.md", "epics/3c4d.md", "docs/epics/1a2b.md"} {
		if _, ok := file(t, m, p); ok {
			t.Errorf("%s is still there", p)
		}
	}
	if doc, _ := file(t, m, "docs/phases/1a2b.md"); doc != "Design: webhooks.\n" {
		t.Errorf("moved phase doc = %q", doc)
	}
	if cfg, _ := file(t, m, "config.yml"); !strings.Contains(cfg, "format: 2\n") || !strings.Contains(cfg, "integration: main\n") {
		t.Errorf("config.yml = %q", cfg)
	}
	if f, _ := file(t, m, "items/aa03.md"); !strings.Contains(f, "\nstage: in-review\n") || !strings.Contains(f, "\nphase: 1a2b\n") ||
		strings.Contains(f, "state:") || strings.Contains(f, "epic:") {
		t.Errorf("items/aa03.md = %q", f)
	}
	if f, _ := file(t, m, "plans/old.md"); !strings.Contains(f, "\nstate: done\n") || !strings.Contains(f, "updated: \"2026-11-02T00:00:00Z\"") {
		t.Errorf("plans/old.md = %q", f)
	}
	if f, _ := file(t, m, "inbox/bb06.md"); !strings.Contains(f, "  phase: 1a2b\n") || strings.Contains(f, "epic:") {
		t.Errorf("inbox/bb06.md = %q", f)
	}

	// Stages and phases read the same as before.
	for id, want := range map[string]string{"aa01": model.StateCommitted, "aa02": model.StateDone, "aa03": model.StateInReview} {
		if v := item(t, m, id); v.State != want || v.Phase != "1a2b" {
			t.Errorf("%s = %s in %q, want %s in 1a2b", id, v.State, v.Phase, want)
		}
	}
	// The general plan is retired: its items are unplanned, its entries project-wide.
	if p := must[PlanView](t)(m.PlanShow(GeneralPlan)); p.State != model.PlanDone || p.Doc != "" {
		t.Errorf("general = %s, doc %q", p.State, p.Doc)
	}
	if v := item(t, m, "aa04"); v.Plan != "" || len(v.Needs) != 0 || !strings.Contains(v.Notes, "needed aa01") {
		t.Errorf("aa04 = %+v", v)
	}
	if v := item(t, m, "aa05"); v.Plan != "" || v.State != model.StateCaptured || v.Kind != model.ItemBug {
		t.Errorf("aa05 = %+v", v)
	}
	if e := must[Shown](t)(m.Show("bb05")); e.Entry.Plan != "" {
		t.Errorf("bb05 plan = %q", e.Entry.Plan)
	}
	// Item needs are gone; a finished item's are not kept in its notes.
	if v := item(t, m, "aa06"); len(v.Needs) != 0 || v.Notes != "" {
		t.Errorf("aa06 = %+v", v)
	}
	// The phase's need on an unfinished item became a need on its phase;
	// the one on a done item was met, so dropped.
	ph := must[Shown](t)(m.Show("3c4d")).Phase
	if len(ph.Needs) != 1 || ph.Needs[0] != "1a2b" {
		t.Errorf("3c4d needs = %v", ph.Needs)
	}
	// Summaries became doc stubs where there was no doc, and stay.
	if p := must[PlanView](t)(m.PlanShow("launch")); p.Doc != "Ship it." || p.Summary != "Ship it." {
		t.Errorf("launch doc %q, summary %q", p.Doc, p.Summary)
	}
	if d := must[DocResult](t)(m.PhaseDoc("3c4d")); d.Doc != "Flip the switch." {
		t.Errorf("3c4d doc = %q", d.Doc)
	}
	if d := must[DocResult](t)(m.PhaseDoc("1a2b")); d.Doc != "Design: webhooks." {
		t.Errorf("1a2b doc = %q (it had one, so no stub)", d.Doc)
	}
	// A moved doc keeps its history.
	if l := must[DocLog](t)(m.PhaseDocLog("1a2b")); len(l.Versions) != 2 || l.Versions[0].Commit != after || l.Versions[1].Commit != before {
		t.Errorf("1a2b doc log = %+v", l)
	}

	// Loose ends and inbox ideas are captured ideas now, linked both ways.
	captured := map[string]ItemView{}
	for _, v := range must[[]ItemView](t)(m.ItemList(ItemFilter{Kind: model.ItemIdea})) {
		captured[v.From] = v
	}
	c := captured["bb01"]
	if c.State != model.StateCaptured || c.Plan != "launch" || c.Title != "Footer links to old pricing" ||
		c.Source != "session 3" || !strings.Contains(c.Notes, "Three links.") || !strings.Contains(c.Notes, "aa01") ||
		c.Created != "2026-11-03T00:00:00Z" || c.Triaged != "" {
		t.Errorf("bb01's item = %+v", c)
	}
	if e := must[Shown](t)(m.Show("bb01")).Entry; e.Settled == nil || e.Settled.Action != model.ActDo || e.Settled.Item != c.ID {
		t.Errorf("bb01 = %+v", e.Settled)
	}
	if c := captured["bb03"]; c.Plan != "" || !strings.Contains(c.Notes, "after launch") {
		t.Errorf("bb03's item = %+v", c)
	}
	if _, ok := captured["bb02"]; !ok || len(captured) != 3 {
		t.Errorf("captured = %v", captured)
	}
	if e := must[Shown](t)(m.Show("bb04")).Entry; e.Settled.Action != model.ActDecide {
		t.Errorf("a settled loose end changed: %+v", e.Settled)
	}
	if v := must[View](t)(m.Board("")); v.Legacy != nil || v.MigrationRequired || v.Format != 2 {
		t.Errorf("view after migrate: legacy %+v, migration_required %v, format %d", v.Legacy, v.MigrationRequired, v.Format)
	}

	// Writes work, and the human can re-kind a captured loose end.
	must[SetResult](t)(m.Set(c.ID, "kind", model.ItemBug))
	if v := item(t, m, c.ID); v.Kind != model.ItemBug {
		t.Errorf("kind = %s", v.Kind)
	}
	must[StartResult](t)(m.Start("aa01"))

	// Idempotent: on migrated data it changes nothing.
	at := head(t, dir)
	again := must[MigrateResult](t)(m.Migrate(false, true))
	if again.Changed || len(again.Records) != 0 || again.Commit != "" || head(t, dir) != at {
		t.Fatalf("second migrate = %+v", again)
	}
	if !strings.Contains(again.Text(), "Nothing to migrate") {
		t.Errorf("text = %q", again.Text())
	}
}

// Records in format 1 that reach migrated data (sync merging work from a
// machine still on the older mull) block writes until migrate converts them.
// The first-migration steps don't run again.
func TestMigrateStragglers(t *testing.T) {
	m, _ := setupFormat1(t)
	must[MigrateResult](t)(m.Migrate(false, true))
	must[DocResult](t)(m.SetPlanDoc("launch", "")) // the human removes the stub
	err := m.store.Update("an older mull", func(tx *store.Tx) error {
		tx.Put("items/cc01.md", []byte("---\nplan: launch\nepic: 1a2b\ntitle: From the laptop\nstate: done\ncreated: \"2026-11-05T00:00:00Z\"\nupdated: \"2026-11-05T00:00:00Z\"\n---\n"))
		tx.Put("epics/dd01.md", []byte("---\nplan: launch\ntitle: Laptop phase\ncreated: \"2026-11-05T00:00:00Z\"\nupdated: \"2026-11-05T00:00:00Z\"\n---\n\nA summary.\n"))
		tx.Put("inbox/ee01.md", []byte("---\nkind: loose-end\ntext: From the laptop too\nplan: launch\nby: /w/b\ncreated: \"2026-11-05T00:00:00Z\"\n---\n"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if v := item(t, m, "cc01"); v.State != model.StateInReview || v.Phase != "1a2b" {
		t.Errorf("straggler read as %+v", v)
	}
	_, err = m.ItemAdd(NewItem{Plan: "launch", Title: "New"})
	wantCode(t, err, CodeMigrationRequired)
	if v := must[View](t)(m.Board("")); !v.MigrationRequired || v.Format != 2 {
		t.Errorf("view: migration_required %v, format %d", v.MigrationRequired, v.Format)
	}

	r := must[MigrateResult](t)(m.Migrate(false, true))
	if r.FormatBefore != 2 || r.Counts != (MigrateCounts{Phases: 1, Items: 1, Entries: 1, Captured: 1}) {
		t.Errorf("straggler migrate = %+v", r)
	}
	if p := must[PlanView](t)(m.PlanShow("launch")); p.Doc != "" {
		t.Errorf("the stub came back: %q", p.Doc)
	}
	if d := must[DocResult](t)(m.PhaseDoc("dd01")); d.Doc != "" {
		t.Errorf("a straggler phase got a stub: %q", d.Doc)
	}
	must[ItemView](t)(m.ItemAdd(NewItem{Plan: "launch", Title: "New"}))
}

func TestMigrateKeepsGeneralWithPhases(t *testing.T) {
	m, _ := setupFormat1(t)
	err := m.store.Update("general phase", func(tx *store.Tx) error {
		tx.Put("epics/ff01.md", []byte("---\nplan: general\ntitle: Real work\ncreated: \"2026-11-01T00:00:00Z\"\nupdated: \"2026-11-01T00:00:00Z\"\n---\n"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	r := must[MigrateResult](t)(m.Migrate(false, true))
	if len(r.Warnings) == 0 || !strings.Contains(r.Warnings[0], "general has phases") {
		t.Errorf("warnings = %v", r.Warnings)
	}
	if p := must[PlanView](t)(m.PlanShow(GeneralPlan)); p.State != model.PlanActive || p.Doc != "Work outside any big effort." {
		t.Errorf("general = %s, doc %q", p.State, p.Doc)
	}
	if v := item(t, m, "aa04"); v.Plan != GeneralPlan {
		t.Errorf("aa04 plan = %q", v.Plan)
	}
}

func TestNewerFormatIsRefused(t *testing.T) {
	m, _ := setup(t)
	err := m.store.Update("from the future", func(tx *store.Tx) error {
		tx.Put("config.yml", []byte("format: 3\nintegration: main\nrelease: main\nremote: origin\ndone_checks: []\nlayers: {}\n"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.ItemList(ItemFilter{})
	wantCode(t, err, CodeUnsupportedFormat)
	_, err = m.Migrate(true, false)
	wantCode(t, err, CodeUnsupportedFormat)
}

func TestSetKind(t *testing.T) {
	m, _ := setup(t)
	idea := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemIdea, Title: "Was a loose end"}))
	must[SetResult](t)(m.Set(idea.ID, "kind", model.ItemBug))
	if v := item(t, m, idea.ID); v.Kind != model.ItemBug {
		t.Errorf("kind = %s", v.Kind)
	}
	_, err := m.Set(idea.ID, "kind", "chore")
	wantCode(t, err, CodeInvalid)
	work := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "Work"}))
	_, err = m.Set(work.ID, "kind", model.ItemBug)
	wantCode(t, err, CodeState)
}
