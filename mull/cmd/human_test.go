package cmd

import (
	"strings"
	"testing"

	"mull/internal/gittest"
)

// With mull config set human, mull's own words use the name: the guides,
// the session protocol and its messages, and agents are told to write it.
// Unset, they say "the human" as before.
func TestHumanName(t *testing.T) {
	r := run{t, gittest.NewRepo(t)}
	r.ok("init")
	before := r.ok("guide", "agents")["text"].(string)
	if !strings.Contains(before, "the human commits them") || strings.Contains(before, "The human here is") {
		t.Fatal("unnamed, the agents guide should be as written")
	}

	if cfg := r.ok("config", "set", "human", "Jeff"); cfg["human"] != "Jeff" {
		t.Fatalf("config set human = %v", cfg)
	}
	g := r.ok("guide", "agents")["text"].(string)
	if !strings.Contains(g, "The human here is Jeff.") || !strings.Contains(g, "until Jeff commits them") ||
		strings.Contains(g, "until the human commits") {
		t.Fatalf("named, the agents guide should name Jeff:\n%s", g[:min(len(g), 1200)])
	}
	if !strings.Contains(g, "the human-feedback") && !strings.Contains(g, "Feedback Jeff hands over") {
		t.Error("the guide's feedback rule went missing")
	}
	if tri := r.ok("guide", "triage")["text"].(string); strings.Contains(tri, "the human") {
		t.Error("the triage guide still says the human")
	}

	ctx := r.ok("context")
	proto := ctx["protocol"].([]any)
	if !strings.HasPrefix(proto[0].(string), "The human here is Jeff.") {
		t.Errorf("protocol starts %q", proto[0])
	}
	for _, line := range proto[1:] {
		if strings.Contains(line.(string), "the human") {
			t.Errorf("protocol line still says the human: %q", line)
		}
	}

	e := r.fails("not_allowed", "plan", "new", "pilot", "Pilot")
	if msg := e["error"].(string); !strings.Contains(msg, "only Jeff creates a plan") {
		t.Errorf("error = %q", msg)
	}

	// Clearing the name puts the words back.
	r.ok("config", "set", "human", "")
	if g := r.ok("guide", "agents")["text"].(string); g != before {
		t.Error("clearing the name didn't restore the guide")
	}
}
