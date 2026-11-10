package mull

import (
	"strconv"
	"testing"
	"time"

	"mull/internal/gittest"
	"mull/internal/model"
)

// byRecord indexes changes by type and ID.
func byRecord(cs []Change) map[string]Change {
	out := map[string]Change{}
	for _, c := range cs {
		out[c.Type+":"+c.ID] = c
	}
	return out
}

func fieldOf(c Change, name string) (FieldChange, bool) {
	for _, f := range c.Fields {
		if f.Field == name {
			return f, true
		}
	}
	return FieldChange{}, false
}

func TestRevisionBaseline(t *testing.T) {
	m, dir := setup(t)
	tip := head(t, dir)
	count, _ := strconv.Atoi(gittest.Git(t, dir, "rev-list", "--count", tip))
	for _, since := range []string{"", "0123456789abcdef0123456789abcdef01234567", "not a commit", "--all"} {
		r := must[Revision](t)(m.Revision(since))
		if !r.Baseline || r.Tip != r.Since || r.Tip.Commit != tip || r.Tip.Number != count || len(r.Changes) != 0 || r.Changes == nil {
			t.Errorf("Revision(%q) = %+v, want a baseline at the tip %s (number %d)", since, r, tip, count)
		}
	}
	r := must[Revision](t)(m.Revision(tip))
	if r.Baseline || r.Since != r.Tip || len(r.Changes) != 0 {
		t.Errorf("Revision(tip) = %+v, want no changes and no baseline", r)
	}
	ct, _ := strconv.ParseInt(gittest.Git(t, dir, "log", "-1", "--format=%ct", tip), 10, 64)
	if want := time.Unix(ct, 0).UTC().Format(time.RFC3339); r.Tip.At != want {
		t.Errorf("tip time = %q, want %q (UTC)", r.Tip.At, want)
	}

	bare := must[*Mull](t)(Open(gittest.NewRepo(t)))
	_, err := bare.Revision("")
	wantCode(t, err, CodeNotInitialized)
	_, err = bare.RevisionAt("")
	wantCode(t, err, CodeNotInitialized)
}

func TestRevisionChanges(t *testing.T) {
	m, dir := setup(t)
	seen := head(t, dir)

	ph := must[*model.Phase](t)(m.PhaseAdd("general", "Billing", "Stripe first.", nil))
	it := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Phase: ph.ID, Title: "Webhook"}))
	must[StartResult](t)(m.Start(it.ID))
	bug := must[ItemView](t)(m.CaptureItem(NewCapture{Kind: model.ItemBug, Title: "Footer"}))
	q := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "Which provider?", Item: it.ID}))
	must[DocResult](t)(m.SetPlanDoc("general", "## The picture\nOne.\n"))
	// A change undone before you look is no change.
	must[SetResult](t)(m.Set(bug.ID, "title", "Footer links"))
	must[SetResult](t)(m.Set(bug.ID, "title", "Footer"))

	r := must[Revision](t)(m.Revision(seen))
	if r.Baseline || r.Since.Commit != seen || r.Tip.Commit != head(t, dir) || r.Since.Number >= r.Tip.Number {
		t.Fatalf("revision points: since %+v, tip %+v", r.Since, r.Tip)
	}
	got := byRecord(r.Changes)
	want := map[string]Change{
		"phase:" + ph.ID:   {Type: RecordPhase, ID: ph.ID, Title: "Billing", Plan: "general", Change: ChangeAdded},
		"item:" + it.ID:    {Type: RecordItem, ID: it.ID, Title: "Webhook", Plan: "general", Phase: ph.ID, Change: ChangeAdded, State: model.StateActive},
		"item:" + bug.ID:   {Type: RecordItem, ID: bug.ID, Title: "Footer", Kind: model.ItemBug, Change: ChangeAdded, State: model.StateCaptured},
		"entry:" + q.ID:    {Type: RecordEntry, ID: q.ID, Title: "Which provider?", Plan: "general", Kind: model.KindQuestion, Change: ChangeAdded, State: "open"},
		"plan_doc:general": {Type: RecordPlanDoc, ID: "general", Title: "General", Plan: "general", Change: ChangeAdded},
	}
	if len(got) != len(want) {
		t.Errorf("changes = %+v, want %d", r.Changes, len(want))
	}
	for k, w := range want {
		g, ok := got[k]
		if !ok {
			t.Errorf("no change for %s", k)
			continue
		}
		if g.At == "" || g.At > r.Tip.At || g.At < r.Since.At {
			t.Errorf("%s at %q, want between %s and %s", k, g.At, r.Since.At, r.Tip.At)
		}
		g.At = ""
		if g.Type != w.Type || g.ID != w.ID || g.Title != w.Title || g.Plan != w.Plan || g.Phase != w.Phase || g.Kind != w.Kind || g.Change != w.Change || g.State != w.State || len(g.Fields) != 0 {
			t.Errorf("%s = %+v, want %+v", k, g, w)
		}
	}
	for i := 1; i < len(r.Changes); i++ {
		if r.Changes[i-1].At < r.Changes[i].At {
			t.Errorf("changes not newest first: %+v", r.Changes)
		}
	}

	// Look again, then change what's there.
	seen = head(t, dir)
	must[ItemView](t)(m.Release(it.ID))
	must[*model.Entry](t)(m.Decide(q.ID, "Stripe", SettleOpts{}))
	must[DocResult](t)(m.SetPhaseDoc(ph.ID, "Webhooks first."))
	must[DocResult](t)(m.SetPlanDoc("general", ""))
	must[SetResult](t)(m.Set(it.ID, "notes", "Long notes."))
	must[SetResult](t)(m.Set(ph.ID, "title", "Payments"))

	r = must[Revision](t)(m.Revision(seen))
	got = byRecord(r.Changes)
	if len(got) != 5 {
		t.Errorf("changes = %+v, want 5", r.Changes)
	}
	item := got["item:"+it.ID]
	if item.Change != ChangeChanged || item.State != model.StateCommitted {
		t.Errorf("item = %+v", item)
	}
	if f, _ := fieldOf(item, "state"); f.From != model.StateActive || f.To != model.StateCommitted {
		t.Errorf("item state change = %+v, want active to committed", f)
	}
	if f, ok := fieldOf(item, "claim"); !ok || f.From != m.Worktree() || f.To != "" {
		t.Errorf("item claim change = %+v", f)
	}
	if f, ok := fieldOf(item, "notes"); !ok || f.From != "" || f.To != "" {
		t.Errorf("item notes change = %+v, want the field without its text", f)
	}
	if e := got["entry:"+q.ID]; e.Change != ChangeChanged || e.State != model.ActDecide {
		t.Errorf("entry = %+v", e)
	} else if f, _ := fieldOf(e, "settled"); f.From != "open" || f.To != model.ActDecide {
		t.Errorf("entry settled change = %+v", f)
	}
	if c := got["phase:"+ph.ID]; c.Change != ChangeChanged || c.Title != "Payments" {
		t.Errorf("phase = %+v", c)
	} else if f, _ := fieldOf(c, "title"); f.From != "Billing" || f.To != "Payments" {
		t.Errorf("phase title change = %+v", f)
	}
	if c := got["phase_doc:"+ph.ID]; c.Change != ChangeAdded || c.Title != "Payments" || c.Plan != "general" {
		t.Errorf("phase doc = %+v", c)
	}
	if c := got["plan_doc:general"]; c.Change != ChangeRemoved {
		t.Errorf("plan doc = %+v", c)
	}
}

// A migration rewrites every record's format; only the ones whose meaning
// changed show as changed.
func TestRevisionAcrossMigration(t *testing.T) {
	m, dir := setupFormat1(t)
	seen := head(t, dir)
	must[MigrateResult](t)(m.Migrate(false, true))
	got := byRecord(must[Revision](t)(m.Revision(seen)).Changes)
	for _, k := range []string{"item:aa01", "item:aa02", "item:aa03", "phase:1a2b", "phase_doc:1a2b", "plan:old", "plan:launch", "entry:bb04"} {
		if c, ok := got[k]; ok {
			t.Errorf("%s changed only in format, but listed: %+v", k, c)
		}
	}
	if c := got["plan:general"]; c.Change != ChangeChanged || c.State != model.PlanDone {
		t.Errorf("retired general plan = %+v", c)
	}
	if f, _ := fieldOf(got["item:aa04"], "plan"); f.From != "general" || f.To != "" {
		t.Errorf("unplanned item = %+v", got["item:aa04"])
	}
	if f, _ := fieldOf(got["phase:3c4d"], "needs"); f.From != "aa01,aa02" || f.To != "1a2b" {
		t.Errorf("converted needs = %+v", got["phase:3c4d"])
	}
	if c := got["plan_doc:launch"]; c.Change != ChangeAdded {
		t.Errorf("doc stub = %+v", c)
	}
	if c := got["entry:bb01"]; c.State != model.ActDo {
		t.Errorf("captured loose end = %+v", c)
	}
}

func TestRevisionAt(t *testing.T) {
	m, dir := setup(t)
	first := head(t, dir)
	must[ItemView](t)(m.ItemAdd(NewItem{Plan: "general", Title: "Webhook"}))
	tip := head(t, dir)

	if p := must[RevisionPoint](t)(m.RevisionAt("")); p.Commit != tip {
		t.Errorf("RevisionAt(\"\") = %+v, want the tip %s", p, tip)
	}
	p := must[RevisionPoint](t)(m.RevisionAt(first))
	if p.Commit != first || p.Number != must[RevisionPoint](t)(m.RevisionAt(tip)).Number-1 {
		t.Errorf("RevisionAt(first) = %+v", p)
	}
	for _, bad := range []string{"nope", first[:7], gittest.Git(t, dir, "rev-parse", "main")} {
		_, err := m.RevisionAt(bad)
		wantCode(t, err, CodeInvalid)
	}
}
