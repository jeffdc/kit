package mull

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
	"mull/internal/store"
)

func jsonl(t *testing.T, lines ...map[string]any) string {
	t.Helper()
	var b strings.Builder
	for _, l := range lines {
		data, err := json.Marshal(l)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String()
}

func mullHead(t *testing.T, dir string) string {
	t.Helper()
	return gittest.Git(t, dir, "rev-parse", store.Ref)
}

func batchErr(t *testing.T, err error) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeBatchInvalid {
		t.Fatalf("got %v, want a batch_invalid error", err)
	}
	return e
}

// TestBatchIntakeSized is the worked example: a plan, 3 phases with needs
// between them, 10 items in those phases and 20 inbox entries of mixed kinds,
// as one batch, one commit, every ID returned.
func TestBatchIntakeSized(t *testing.T) {
	m, dir := setup(t)
	var lines []map[string]any
	lines = append(lines,
		map[string]any{"op": "plan new", "ref": "p", "slug": "taxogall", "title": "Taxogall", "summary": "Galls as catalog entries."},
		map[string]any{"op": "plan agree", "slug": "@p", "on-request": true},
		map[string]any{"op": "config checks", "commands": []string{"make done-checks"}},
		map[string]any{"op": "phase add", "ref": "e1", "plan": "@p", "title": "How the admin works"},
		map[string]any{"op": "phase add", "ref": "e2", "plan": "@p", "title": "Phase 2 and gaps", "needs": []string{"@e1"}},
		map[string]any{"op": "phase add", "ref": "e3", "plan": "@p", "title": "Gallformers notes", "needs": "@e2"},
	)
	for k := 1; k <= 10; k++ {
		phase := 1 // items 1-4 in phase 1, 5-7 in phase 2, 8-10 in phase 3
		if k > 7 {
			phase = 3
		} else if k > 4 {
			phase = 2
		}
		l := map[string]any{"op": "add", "ref": fmt.Sprintf("i%d", k), "phase": fmt.Sprintf("@e%d", phase),
			"title": fmt.Sprintf("Item %d", k), "notes": fmt.Sprintf("From matter 50ff, step %d.", k)}
		lines = append(lines, l)
	}
	kinds := []string{"decided", "ask", "bug"}
	for k := 1; k <= 20; k++ {
		l := map[string]any{"op": kinds[k%3], "text": fmt.Sprintf("Entry %d", k), "source": "matter 50ff"}
		if kinds[k%3] == "bug" {
			l["plan"] = "@p" // captures tag a plan; they aren't about an item
			lines = append(lines, l)
			continue
		}
		if k%2 == 0 {
			l["item"] = fmt.Sprintf("@i%d", (k%10)+1)
		} else {
			l["plan"] = "@p"
		}
		if kinds[k%3] == "decided" {
			l["detail"] = "why: simplest"
		}
		lines = append(lines, l)
	}

	before := mullHead(t, dir)
	results, err := m.Batch(strings.NewReader(jsonl(t, lines...)), false)
	if err != nil {
		var e *Error
		if errors.As(err, &e) {
			t.Fatalf("%v: %+v", err, e.Lines)
		}
		t.Fatal(err)
	}
	if len(results) != len(lines) {
		t.Fatalf("%d results for %d lines", len(results), len(lines))
	}
	// One commit, on top of the previous one.
	if n := gittest.Git(t, dir, "rev-list", "--count", before+".."+store.Ref); n != "1" {
		t.Fatalf("batch made %s commits, want 1", n)
	}
	created := map[string]string{}
	for k, r := range results {
		if r.Line != k+1 || r.Op != lines[k]["op"] {
			t.Errorf("result %d = line %d op %s", k, r.Line, r.Op)
		}
		if ref, _ := lines[k]["ref"].(string); ref != "" {
			if r.Ref != ref || r.ID == "" {
				t.Errorf("line %d: ref %q id %q", r.Line, r.Ref, r.ID)
			}
			created[ref] = r.ID
		} else if r.Op != "config checks" && r.ID == "" {
			t.Errorf("line %d (%s) returned no id", r.Line, r.Op)
		}
	}
	if created["p"] != "taxogall" || len(created) != 14 {
		t.Fatalf("created = %v", created)
	}

	// Refs resolved to the real records.
	e2 := must[Shown](t)(m.Show(created["e2"])).Phase
	if len(e2.Needs) != 1 || e2.Needs[0] != created["e1"] || e2.Plan != "taxogall" {
		t.Errorf("phase 2 = %+v", e2.Phase)
	}
	i4 := item(t, m, created["i4"])
	if i4.Plan != "taxogall" || i4.Phase != created["e1"] || !i4.Ready {
		t.Errorf("item 4 = %+v", i4)
	}
	// An item in phase 3 waits for phase 2, both made in this batch.
	if i8 := item(t, m, created["i8"]); i8.Phase != created["e3"] || i8.Ready || i8.BlockedBy[0] != created["e2"] {
		t.Errorf("item 8 = %+v", i8)
	}
	if n := len(must[[]*model.Entry](t)(m.Inbox(InboxFilter{Plan: "taxogall"}))); n != 13 {
		t.Errorf("inbox has %d entries for the plan, want 13", n)
	}
	if n := len(must[[]ItemView](t)(m.ItemList(ItemFilter{Plan: "taxogall", State: model.StateCaptured}))); n != 7 {
		t.Errorf("%d captured bugs for the plan, want 7", n)
	}
	cfg := must[model.Config](t)(m.Config())
	if len(cfg.DoneChecks) != 1 || cfg.DoneChecks[0] != "make done-checks" {
		t.Errorf("config = %+v", cfg)
	}
}

func TestBatchAllOrNothing(t *testing.T) {
	m, dir := setup(t)
	input := strings.Join([]string{
		`{"op": "plan new", "ref": "p", "slug": "launch", "title": "Launch"}`,
		`{"op": "add", "ref": "a", "phase": "ffff", "title": "bad phase"}`,
		``,
		`{"op": "add", "ref": "b", "plan": "@p", "title": "fine"}`,
		`{"op": "ask", "item": "@a", "text": "about a failed line"}`,
		`{"op": "ask", "plan": "@p", "text": "unknown ref", "item": "@nope"}`,
		`not json`,
		`{"title": "no op"}`,
		`{"op": "frobnicate"}`,
		`{"op": "add", "plan": "@p", "title": "x", "colour": "red"}`,
		`{"op": "add", "ref": "b", "plan": "@p", "title": "duplicate ref"}`,
		`{"op": "drop", "ref": "d", "ids": ["@b"]}`,
	}, "\n")
	before := mullHead(t, dir)
	_, err := m.Batch(strings.NewReader(input), false)
	e := batchErr(t, err)
	got := map[int]string{}
	for _, l := range e.Lines {
		got[l.Line] = l.Code
	}
	want := map[int]string{
		2:  CodeNotFound,    // no phase ffff
		5:  CodeInvalid,     // @a refers to a failed line
		6:  CodeNotFound,    // unknown ref
		7:  CodeInvalidJSON, // not json
		8:  CodeInvalid,     // no op
		9:  CodeUsage,       // unknown op
		10: CodeInvalid,     // unknown field
		11: CodeExists,      // duplicate ref
		12: CodeInvalid,     // drop creates nothing a ref could name
	}
	for line, code := range want {
		if got[line] != code {
			t.Errorf("line %d: code %q, want %q (all: %+v)", line, got[line], code, e.Lines)
		}
	}
	if len(got) != len(want) {
		t.Errorf("failed lines = %v, want exactly %v", got, want)
	}
	if !strings.Contains(e.Lines[1].Error, "line 2") {
		t.Errorf("ref-to-failed-line error should name the line: %q", e.Lines[1].Error)
	}
	if after := mullHead(t, dir); after != before {
		t.Fatal("a failed batch wrote to the mull branch")
	}
	if _, err := m.PlanShow("launch"); err == nil {
		t.Fatal("plan from a failed batch exists")
	}
}

func TestBatchDryRun(t *testing.T) {
	m, dir := setup(t)
	before := mullHead(t, dir)
	input := `{"op": "plan new", "ref": "p", "slug": "launch", "title": "Launch"}
{"op": "phase add", "ref": "e", "plan": "@p", "title": "E"}
{"op": "add", "phase": "@e", "title": "B"}`
	results, err := m.Batch(strings.NewReader(input), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || results[2].Result.(ItemView).Phase != results[1].ID {
		t.Fatalf("dry-run results = %+v", results)
	}
	if mullHead(t, dir) != before {
		t.Fatal("dry run wrote")
	}
	// A dry run reports failures exactly like a real run.
	_, err = m.Batch(strings.NewReader(`{"op": "add", "plan": "nope", "title": "x"}`), true)
	if e := batchErr(t, err); e.Lines[0].Code != CodeNotFound {
		t.Fatalf("lines = %+v", e.Lines)
	}
}

func TestBatchSettleInbox(t *testing.T) {
	m, _ := setup(t)
	d := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindDecision, Text: "d"}))
	q := must[*model.Entry](t)(m.Capture(CaptureInput{Kind: model.KindQuestion, Text: "q"}))
	l1 := must[*model.Entry](t)(m.captureLegacy(CaptureInput{Kind: model.KindLoose, Text: "l1", Plan: "general"}))
	l2 := must[*model.Entry](t)(m.captureLegacy(CaptureInput{Kind: model.KindLoose, Text: "l2", Plan: "general"}))
	l3 := must[*model.Entry](t)(m.captureLegacy(CaptureInput{Kind: model.KindLoose, Text: "l3", Plan: "general"}))
	input := jsonl(t,
		map[string]any{"op": "accept", "ids": []string{d.ID}, "on-request": true},
		map[string]any{"op": "decide", "id": q.ID, "decision": "yes", "on-request": true},
		map[string]any{"op": "drop", "ids": l1.ID + "," + l2.ID, "reason": "noise", "on-request": true},
		map[string]any{"op": "do", "ref": "new", "id": l3.ID, "on-request": true},
		map[string]any{"op": "start", "id": "@new"},
	)
	results, err := m.Batch(strings.NewReader(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(must[[]*model.Entry](t)(m.Inbox(InboxFilter{}))); n != 0 {
		t.Fatalf("inbox not cleared: %d", n)
	}
	if st := results[4].Result.(StartResult); st.Item.State != model.StateActive {
		t.Fatalf("start via @ref = %+v", st)
	}
	if !results[1].Result.(*model.Entry).Settled.OnRequest {
		t.Fatal("on-request not recorded")
	}
}

func TestBatchEmpty(t *testing.T) {
	m, _ := setup(t)
	_, err := m.Batch(strings.NewReader("\n\n"), false)
	wantCode(t, err, CodeInvalid)
}

func TestScopeForItemsAndDo(t *testing.T) {
	m, _ := setup(t)
	must[*model.Plan](t)(m.PlanNew("alpha", "Alpha", ""))
	e := must[*model.Phase](t)(m.PhaseAdd("alpha", "E", "", nil))
	// The phase implies the plan.
	if it := must[ItemView](t)(m.ItemAdd(NewItem{Phase: e.ID, Title: "x"})); it.Plan != "alpha" {
		t.Fatalf("plan = %s", it.Plan)
	}
	_, err := m.ItemAdd(NewItem{Phase: e.ID, Plan: "general", Title: "y"})
	wantCode(t, err, CodeInvalid)
	_, err = m.ItemAdd(NewItem{Title: "z"})
	wantCode(t, err, CodeScope)
	// do --phase files into the phase's plan, even though the entry is in general.
	l := must[*model.Entry](t)(m.captureLegacy(CaptureInput{Kind: model.KindLoose, Text: "fix the footer", Plan: "general"}))
	res := must[DoResult](t)(m.Do(DoInput{ID: l.ID, Phase: e.ID}))
	if res.Item.Plan != "alpha" || res.Item.Phase != e.ID {
		t.Fatalf("do item = %+v", res.Item)
	}
}
