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

// ok runs the CLI, requires success, and decodes the JSON output.
func (r run) ok(args ...string) map[string]any {
	r.t.Helper()
	out, errOut, code := r.mull("", args...)
	if code != 0 {
		r.t.Fatalf("mull %v exited %d: %s", args, code, errOut)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		r.t.Fatalf("mull %v: output is not a JSON object: %v\n%s", args, err, out)
	}
	return v
}

func TestCLIEndToEnd(t *testing.T) {
	r := run{t: t, dir: gittest.NewRepo(t)}

	_, errOut, code := r.mull("", "plan", "list")
	if code != 1 || !strings.Contains(errOut, `"error"`) || !strings.Contains(errOut, "mull init") {
		t.Fatalf("before init: code %d, stderr %s", code, errOut)
	}

	r.ok("init", "--integration", "taxogall")
	r.ok("plan", "new", "taxogall", "Taxogall")
	epic := r.ok("epic", "add", "Phase 2", "--plan", "taxogall")["id"].(string)

	out, errOut, code := r.mull("Read 8567 §3 first.\n", "add", "Tree edits become Facts", "--plan", "taxogall", "--epic", epic, "--notes", "-")
	if code != 0 {
		t.Fatal(errOut)
	}
	var it map[string]any
	_ = json.Unmarshal([]byte(out), &it)
	id := it["id"].(string)
	if it["notes"] != "Read 8567 §3 first." || it["ready"] != true {
		t.Fatalf("item = %v", it)
	}

	second := r.ok("add", "Admin writes through Curation", "--plan", "taxogall", "--needs", id)["id"].(string)
	ready := r.ok("ready", "--plan", "taxogall")["items"].([]any)
	if len(ready) != 1 || ready[0].(map[string]any)["id"] != id {
		t.Fatalf("ready = %v", ready)
	}
	if r.ok("show", second)["ready"] != false {
		t.Fatal("second item should be blocked")
	}

	start := r.ok("start", id)
	if start["item"].(map[string]any)["state"] != "active" {
		t.Fatalf("start = %v", start)
	}

	r.ok("decided", "Any admin can undo anyone's decision", "--item", id, "--why", "Simplest rule")
	q := r.ok("ask", "Did split undos run on preview?", "--item", id)["id"].(string)
	loose := r.ok("loose", "Canonical URL emitted as meta", "--source", "2c8c")["id"].(string)

	text, _, code := r.mull("", "inbox")
	if code != 0 || !strings.Contains(text, "DECISIONS (1)") || !strings.Contains(text, "QUESTIONS (1)") || !strings.Contains(text, "LOOSE ENDS (1)") {
		t.Fatalf("inbox text:\n%s", text)
	}

	r.ok("decide", q, "No compatibility path needed", "--on-request")
	r.ok("park", loose, "after release")
	if n := len(r.ok("decisions")["decisions"].([]any)); n != 1 {
		t.Fatalf("decision log has %d entries", n)
	}
	if n := len(r.ok("inbox", "--json")["entries"].([]any)); n != 1 {
		t.Fatalf("open inbox has %d entries, want the one decision", n)
	}

	_, errOut, code = r.mull("", "need", id, second)
	if code != 1 || !strings.Contains(errOut, "dependency cycle") {
		t.Fatalf("cycle not refused: %s", errOut)
	}

	plan := r.ok("show", "taxogall")
	if plan["progress"].(map[string]any)["active"].(float64) != 1 {
		t.Fatalf("plan progress = %v", plan["progress"])
	}

	// Finish the item through the CLI: config, checks, done, report, review.
	r.ok("config", "checks", "true")
	r.ok("config", "layer", "ui", "web/**")
	gittest.Commit(t, r.dir, "web/tree.html", "x\n", "Tree edits", "Mull: "+id)
	_, errOut, code = r.mull("", "done", id, "--summary", "Tree edits become Facts.")
	if code != 1 || !strings.Contains(errOut, "UI tour") {
		t.Fatalf("done without a tour: %d %s", code, errOut)
	}
	out, errOut, code = r.mull("1. Open the tree\n2. Rename a genus\n", "done", id, "--summary", "Tree edits become Facts.", "--tour", "-")
	if code != 0 {
		t.Fatal(errOut)
	}
	var done map[string]any
	_ = json.Unmarshal([]byte(out), &done)
	if tour := done["report"].(map[string]any)["tour"].([]any); len(tour) != 2 || tour[0] != "Open the tree" {
		t.Fatalf("tour = %v", tour)
	}
	report, _, code := r.mull("", "report", id)
	if code != 0 || !strings.Contains(report, "Status:** PASSED") || !strings.Contains(report, "2. Rename a genus") {
		t.Fatalf("report:\n%s", report)
	}

	board, _, code := r.mull("", "view")
	if code != 0 || !strings.Contains(board, "TAXOGALL") || !strings.Contains(board, "NEEDS YOU  1 review · 1 decision") {
		t.Fatalf("view:\n%s", board)
	}
	if v := r.ok("view", "--json"); len(v["plans"].([]any)) != 1 {
		t.Fatalf("view --json = %v", v)
	}

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

	// The working tree is untouched by all of this.
	if st := gittest.Git(t, r.dir, "status", "--porcelain"); st != "" {
		t.Fatalf("working tree changed: %q", st)
	}
}
