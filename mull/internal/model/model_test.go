package model

import (
	"reflect"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	in := Item{
		Plan: "taxogall", Epic: "1a2b", Title: "Tree edits: become Facts", State: StateActive,
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

func TestAllDigitIDsStayStrings(t *testing.T) {
	data, err := Encode(Item{Needs: []string{"7877"}, Epic: "0123"}, "")
	if err != nil {
		t.Fatal(err)
	}
	var out Item
	if _, err := Decode(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Epic != "0123" || out.Needs[0] != "7877" {
		t.Errorf("digit IDs not preserved: %+v", out)
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
