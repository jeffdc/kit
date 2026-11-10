package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	in := Item{
		Plan: "taxogall", Phase: "1a2b", Title: "Tree edits: become Facts", State: StateActive,
		Needs: []string{"0001", "7877"}, Docs: []string{"docs/x.md"},
		Claim:   &Claim{Worktree: "/w/a", Branch: "f923-x", At: "2026-11-09T10:00:00Z"},
		Created: "2026-11-09T09:00:00Z", Updated: "2026-11-09T10:00:00Z",
	}
	data, err := Encode(in, "Some notes.\n\n---\nwith a rule inside")
	if err != nil {
		t.Fatal(err)
	}
	var out Item
	body, err := Decode(data, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip changed the record:\n in %+v\nout %+v", in, out)
	}
	if body != "Some notes.\n\n---\nwith a rule inside" {
		t.Errorf("body = %q", body)
	}
}

func TestItemsAreStoredInFormat2(t *testing.T) {
	for _, stage := range Stages {
		data, err := Encode(Item{Title: "x", State: stage, Phase: "1a2b"}, "")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "\nstage: "+stage+"\n") || !strings.Contains(string(data), "\nphase: 1a2b\n") ||
			strings.Contains(string(data), "state:") || strings.Contains(string(data), "epic:") {
			t.Errorf("%s stored as:\n%s", stage, data)
		}
		var out Item
		if _, err := Decode(data, &out); err != nil {
			t.Fatal(err)
		}
		if out.State != stage || out.Phase != "1a2b" || out.Legacy {
			t.Errorf("stored %q read back as %+v", stage, out)
		}
	}
	data, err := Encode(Plan{Slug: "launch", Title: "x", State: PlanDone}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\nstate: done\n") {
		t.Errorf("a done plan stored as:\n%s", data)
	}
}

// Format 1 records read correctly, and are marked Legacy: a stored "done"
// is an item in review there, and done only under stage:.
func TestFormat1RecordsRead(t *testing.T) {
	for stored, stage := range map[string]string{
		"captured": StateCaptured, "open": StateCommitted, "active": StateActive,
		"done": StateInReview, "accepted": StateDone, "dropped": StateDropped,
	} {
		var out Item
		if _, err := Decode([]byte("---\ntitle: x\nstate: "+stored+"\nepic: \"0123\"\n---\n"), &out); err != nil {
			t.Fatal(err)
		}
		if out.State != stage || out.Phase != "0123" || !out.Legacy {
			t.Errorf("format 1 %q read as %+v, want %s", stored, out, stage)
		}
	}
	var done Item
	if _, err := Decode([]byte("---\ntitle: x\nstage: done\n---\n"), &done); err != nil {
		t.Fatal(err)
	}
	if done.State != StateDone || done.Legacy {
		t.Errorf("format 2 done read as %+v", done)
	}
	for stored, want := range map[string]string{"shaping": PlanShaping, "active": PlanActive, "closed": PlanDone, "done": PlanDone} {
		var p Plan
		if _, err := Decode([]byte("---\ntitle: x\nstate: "+stored+"\n---\n"), &p); err != nil {
			t.Fatal(err)
		}
		if p.State != want || p.Legacy != (stored == "closed") {
			t.Errorf("plan %q read as %+v", stored, p)
		}
	}
	var e Entry
	if _, err := Decode([]byte("---\nkind: idea\ntext: x\nplan: \"\"\nby: w\ncreated: t\nsettled:\n  action: do\n  epic: beef\n  by: w\n  at: t\n---\n"), &e); err != nil {
		t.Fatal(err)
	}
	if e.Settled == nil || e.Settled.Phase != "beef" || !e.Settled.Legacy {
		t.Errorf("format 1 settlement read as %+v", e.Settled)
	}
}

func TestAllDigitIDsStayStrings(t *testing.T) {
	data, err := Encode(Item{Needs: []string{"7877"}, Phase: "0123"}, "")
	if err != nil {
		t.Fatal(err)
	}
	var out Item
	if _, err := Decode(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Phase != "0123" || out.Needs[0] != "7877" {
		t.Errorf("digit IDs not preserved: %+v", out)
	}
}

func TestValidDataPath(t *testing.T) {
	for _, p := range []string{
		"config.yml", "plans/general.md", "plans/phase-two.md", "phases/1a2b.md", "epics/1a2b.md", "items/7877.md",
		"inbox/beef.md", "reports/1a2b-3.md", "docs/plans/general.md", "docs/phases/1a2b.md", "docs/epics/1a2b.md", "checks/" + strings.Repeat("a", 40) + ".json",
	} {
		if !ValidDataPath(p) {
			t.Errorf("%q should be a valid data path", p)
		}
	}
	for _, p := range []string{
		"", "README", "src/secret.go", "items/1a2b.md.bak", "items/12345.md", "plans/Bad.md",
		"plans/../x.md", "items/sub/1a2b.md", ".gitignore", "checks/abc.json", "config.yaml",
		"docs/items/1a2b.md", "docs/inbox/beef.md", "docs/plans/Bad.md", "docs/epics/12345.md", "docs/phases/12345.md", "phases/x.md", "docs/README.md", "docs/docs/plans/x.md",
	} {
		if ValidDataPath(p) {
			t.Errorf("%q should not be a valid data path", p)
		}
	}
}

func TestSlugs(t *testing.T) {
	for _, s := range []string{"taxogall", "general", "phase-2", "v2-launch"} {
		if !ValidSlug(s) {
			t.Errorf("%q should be a valid slug", s)
		}
	}
	for _, s := range []string{"", "Taxo", "a b", "-x", "x-", "beef", "1234", "2nd"} {
		if ValidSlug(s) {
			t.Errorf("%q should not be a valid slug", s)
		}
	}
}
