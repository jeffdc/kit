package cmd

import (
	"testing"

	"mull/internal/gittest"
)

// The JSON agents branch on: a few keys that must stay.
func TestOutputShapes(t *testing.T) {
	r := run{t: t, dir: gittest.NewRepo(t)}
	r.ok("init")
	has := func(what string, v map[string]any, keys ...string) {
		t.Helper()
		for _, k := range keys {
			if _, ok := v[k]; !ok {
				t.Errorf("%s lacks %q: %v", what, k, v)
			}
		}
	}
	has("config", r.ok("config"), "integration", "release", "checks", "risk")
	has("status", r.ok("status"), "since", "calls", "rules", "plans", "flow")
	has("context", r.ok("context"), "protocol", "rules", "ready", "active")
	has("rules", r.ok("rules"), "path", "rules", "checked", "advisory")
	has("map", r.ok("map"), "configured", "edges")
	has("inbox", r.ok("inbox"), "entries")
	has("list", r.ok("list"), "items")
}
