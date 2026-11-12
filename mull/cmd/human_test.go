package cmd

import (
	"strings"
	"testing"

	"mull/internal/gittest"
)

// mull name sets this machine's name for the human. mull context and mull's
// messages then use it, and tell agents to write it; the guides, which are
// copied into shared files, stay as written, and nothing goes on the mull
// branch. Unset, the words are as before.
func TestHumanName(t *testing.T) {
	r := run{t, gittest.NewRepo(t)}
	r.ok("init")
	guide := r.ok("guide", "agents")["text"].(string)
	cfgBefore := r.ok("config")

	if v := r.ok("name"); v["name"] != "" || v["file"] == "" {
		t.Fatalf("unset name = %v", v)
	}
	if v := r.ok("name", "Jeff"); v["name"] != "Jeff" {
		t.Fatalf("mull name Jeff = %v", v)
	}
	if g := r.ok("guide", "agents")["text"].(string); g != guide {
		t.Error("the agents guide changed with a name set; it's copied into shared files")
	}
	if cfg := r.ok("config"); len(cfg) != len(cfgBefore) || cfg["human"] != nil {
		t.Errorf("the name reached the repository's config: %v", cfg)
	}

	proto := r.ok("context")["protocol"].([]any)
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
	r.ok("name", "")
	if proto := r.ok("context")["protocol"].([]any); strings.Contains(proto[0].(string), "Jeff") {
		t.Errorf("cleared, the protocol still names Jeff: %q", proto[0])
	}
	if msg := r.fails("not_allowed", "plan", "new", "pilot", "Pilot")["error"].(string); !strings.Contains(msg, "only the human creates a plan") {
		t.Errorf("cleared, error = %q", msg)
	}
}
