package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"mull/internal/gittest"
)

type run struct {
	t   *testing.T
	dir string
}

// mull runs the CLI and returns stdout, stderr and the exit code.
func (r run) mull(stdin string, args ...string) (string, string, int) {
	r.t.Helper()
	var out, errb bytes.Buffer
	code := Execute(args, r.dir, strings.NewReader(stdin), &out, &errb)
	return out.String(), errb.String(), code
}

// ok runs the CLI, requires success, and decodes the JSON object it prints.
func (r run) ok(args ...string) map[string]any {
	r.t.Helper()
	return r.okIn("", args...)
}

func (r run) okIn(stdin string, args ...string) map[string]any {
	r.t.Helper()
	out, errOut, code := r.mull(stdin, args...)
	if code != 0 {
		r.t.Fatalf("mull %v exited %d: %s", args, code, errOut)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		r.t.Fatalf("mull %v: output is not a JSON object: %v\n%s", args, err, out)
	}
	return v
}

// fails runs the CLI, requires failure with the given code, and returns the error object.
func (r run) fails(code string, args ...string) map[string]any {
	r.t.Helper()
	return r.failsIn("", code, args...)
}

func (r run) failsIn(stdin, code string, args ...string) map[string]any {
	r.t.Helper()
	out, errOut, exit := r.mull(stdin, args...)
	if exit != 1 {
		r.t.Fatalf("mull %v exited %d, want 1\nstdout: %s", args, exit, out)
	}
	if out != "" {
		r.t.Errorf("mull %v printed to stdout on failure: %s", args, out)
	}
	var e map[string]any
	if err := json.Unmarshal([]byte(errOut), &e); err != nil {
		r.t.Fatalf("mull %v: stderr is not JSON: %s", args, errOut)
	}
	if e["code"] != code || e["error"] == "" {
		r.t.Fatalf("mull %v: error %v, want code %s", args, e, code)
	}
	return e
}

func id(v map[string]any) string { return v["id"].(string) }

func TestCLIEndToEnd(t *testing.T) {
	r := run{t: t, dir: gittest.NewRepo(t)}

	e := r.fails("not_initialized", "plan", "list")
	if !strings.Contains(e["error"].(string), "mull init") {
		t.Fatalf("not-initialized error = %v", e)
	}

	r.ok("init", "--integration", "taxogall")
	r.ok("plan", "new", "taxogall", "Taxogall")
	epic := id(r.ok("epic", "add", "Phase 2", "--plan", "taxogall"))

	// --epic implies the plan.
	it := r.okIn("Read 8567 §3 first.\n", "add", "Tree edits become Facts", "--epic", epic, "--notes", "-")
	item := id(it)
	if it["plan"] != "taxogall" || it["notes"] != "Read 8567 §3 first." || it["ready"] != true {
		t.Fatalf("item = %v", it)
	}

	second := id(r.ok("add", "Admin writes through Curation", "--plan", "taxogall", "--needs", item))
	ready := r.ok("ready", "--plan", "taxogall")["items"].([]any)
	if len(ready) != 1 || ready[0].(map[string]any)["id"] != item {
		t.Fatalf("ready = %v", ready)
	}
	if shown := r.ok("show", second); shown["type"] != "item" || shown["item"].(map[string]any)["ready"] != false {
		t.Fatalf("show = %v", shown)
	}
	if r.ok("start", item)["item"].(map[string]any)["state"] != "active" {
		t.Fatal("start did not activate")
	}

	r.ok("decided", "Any admin can undo anyone's decision", "--item", item, "--detail", "Simplest rule")
	q := id(r.ok("ask", "Did split undos run on preview?", "--item", item))
	loose := id(r.ok("loose", "Canonical URL emitted as meta", "--plan", "general", "--source", "2c8c"))

	// inbox is JSON by default; --text renders it.
	if n := len(r.ok("inbox")["entries"].([]any)); n != 3 {
		t.Fatalf("inbox has %d entries", n)
	}
	text, _, code := r.mull("", "inbox", "--text")
	if code != 0 || !strings.Contains(text, "DECISIONS (1)") || !strings.Contains(text, "LOOSE ENDS (1)") {
		t.Fatalf("inbox --text:\n%s", text)
	}

	r.ok("decide", q, "No compatibility path needed", "--on-request")
	r.ok("park", loose, "after release")
	if n := len(r.ok("decisions")["decisions"].([]any)); n != 1 {
		t.Fatalf("decision log has %d entries", n)
	}
	r.fails("dependency_cycle", "need", item, second)

	// Finish the item: config, done, report, review.
	r.ok("config", "checks", "true")
	r.ok("config", "layer", "ui", "web/**")
	gittest.Commit(t, r.dir, "web/tree.html", "x\n", "Tree edits", "Mull: "+item)
	r.fails("tour_required", "done", item, "--summary", "Tree edits become Facts.")
	done := r.okIn("1. Open the tree\n2. Rename a genus\n", "done", item, "--summary", "Tree edits become Facts.", "--tour", "-")
	if tour := done["report"].(map[string]any)["tour"].([]any); len(tour) != 2 || tour[0] != "Open the tree" {
		t.Fatalf("tour = %v", tour)
	}
	rep := r.ok("report", item)
	if !strings.Contains(rep["text"].(string), "Status:** PASSED") || rep["report"].(map[string]any)["passed"] != true {
		t.Fatalf("report = %v", rep)
	}
	md, _, _ := r.mull("", "report", item, "--text")
	if !strings.Contains(md, "2. Rename a genus") {
		t.Fatalf("report --text:\n%s", md)
	}

	// view is JSON by default; --text renders it.
	v := r.ok("view")
	// general holds only a parked entry, so the board leaves it out.
	if plans := v["plans"].([]any); len(plans) != 1 {
		t.Fatalf("view plans = %v", plans)
	}
	board, _, code := r.mull("", "view", "--text")
	if code != 0 || !strings.Contains(board, "TAXOGALL") || !strings.Contains(board, "NEEDS YOU  1 review · 1 decision") {
		t.Fatalf("view --text:\n%s", board)
	}

	if ctx := r.ok("context", "--item", item); ctx["item"].(map[string]any)["id"] != item {
		t.Fatalf("context = %v", ctx)
	}
	for _, g := range []string{"agents", "supervisor", "intake", "json"} {
		if gv := r.ok("guide", g); gv["guide"] != g || !strings.Contains(gv["text"].(string), "mull") {
			t.Fatalf("guide %s = %v", g, gv)
		}
	}
	r.fails("not_found", "guide", "nope")

	// Sync to a bare remote: only the mull branch arrives there.
	remote := t.TempDir() + "/remote.git"
	gittest.Git(t, r.dir, "init", "-q", "--bare", remote)
	gittest.Git(t, r.dir, "remote", "add", "origin", remote)
	if dry := r.ok("sync", "--dry-run"); dry["dry_run"] != true || dry["action"] != "pushed" {
		t.Fatalf("dry run = %v", dry)
	}
	if res := r.ok("sync"); res["action"] != "pushed" {
		t.Fatalf("sync = %v", res)
	}
	if refs := gittest.Git(t, remote, "for-each-ref", "--format=%(refname)"); refs != "refs/heads/mull" {
		t.Fatalf("remote refs = %q", refs)
	}

	// The working tree is untouched by all of this, apart from the test's own commit.
	if st := gittest.Git(t, r.dir, "status", "--porcelain"); st != "" {
		t.Fatalf("working tree changed: %q", st)
	}
}

func TestUsageErrorsHaveACode(t *testing.T) {
	r := run{t: t, dir: gittest.NewRepo(t)}
	r.ok("init")
	r.fails("usage", "frobnicate")
	r.fails("usage", "add")
	r.fails("usage", "add", "x", "--bogus")
	// One flag name for an inbox entry's extra text: --detail. --why is gone.
	r.fails("usage", "decided", "x", "--why", "because")
	r.ok("decided", "x", "--detail", "because")
}

func TestScopeRules(t *testing.T) {
	r := run{t: t, dir: gittest.NewRepo(t)}
	r.ok("init")
	// With only the general plan, an entry without scope lands there.
	if e := r.ok("loose", "only one plan"); e["plan"] != "general" {
		t.Fatalf("entry = %v", e)
	}
	r.ok("plan", "new", "alpha", "Alpha")
	r.ok("plan", "new", "beta", "Beta")
	for _, verb := range []string{"decided", "ask", "loose"} {
		r.fails("scope_required", verb, "about something")
	}
	if e := r.ok("loose", "about beta", "--plan", "beta"); e["plan"] != "beta" {
		t.Fatalf("entry = %v", e)
	}
	epic := id(r.ok("epic", "add", "E", "--plan", "alpha"))
	it := r.ok("add", "x", "--epic", epic)
	if it["plan"] != "alpha" {
		t.Fatalf("plan not inferred from epic: %v", it)
	}
	if e := r.ok("ask", "about the item", "--item", id(it)); e["plan"] != "alpha" {
		t.Fatalf("entry plan not inferred from item: %v", e)
	}
	r.fails("invalid_argument", "add", "y", "--epic", epic, "--plan", "beta")
	r.fails("scope_required", "add", "z")
}
