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
	r.fails("not_allowed", "plan", "new", "taxogall", "Taxogall")
	if p := r.ok("plan", "new", "taxogall", "Taxogall", "--on-request"); p["state"] != "shaping" {
		t.Fatalf("new plan = %v", p)
	}
	// A shaping plan takes no phases or committed work until the human agrees it.
	r.fails("invalid_state", "phase", "add", "Phase 2", "--plan", "taxogall")
	r.fails("not_allowed", "plan", "agree", "taxogall")
	if p := r.ok("plan", "agree", "taxogall", "--on-request"); p["state"] != "active" {
		t.Fatalf("agreed plan = %v", p)
	}
	r.fails("invalid_state", "plan", "agree", "taxogall", "--on-request")
	phase := id(r.ok("phase", "add", "Phase 2", "--plan", "taxogall"))

	// --phase implies the plan; work in an agreed plan is ready.
	it := r.okIn("Read 8567 §3 first.\n", "add", "Tree edits become Facts", "--phase", phase, "--notes", "-")
	item := id(it)
	if it["plan"] != "taxogall" || it["notes"] != "Read 8567 §3 first." || it["ready"] != true {
		t.Fatalf("item = %v", it)
	}

	release := id(r.ok("phase", "add", "Release", "--plan", "taxogall", "--needs", phase))
	second := id(r.ok("add", "Admin writes through Curation", "--phase", release))
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
	later := id(r.ok("ask", "Should the canonical URL be a link header?", "--plan", "taxogall", "--source", "2c8c"))
	// Loose ends and inbox ideas are retired; the old commands point at captures.
	if e := r.fails("retired", "loose", "Canonical URL emitted as meta", "--plan", "taxogall"); !strings.Contains(e["error"].(string), "mull bug") {
		t.Fatalf("loose = %v", e)
	}
	r.fails("retired", "ideas")
	r.ok("bug", "Canonical URL emitted as meta", "--plan", "taxogall", "--source", "2c8c")

	// inbox is JSON by default; --text renders it.
	if n := len(r.ok("inbox")["entries"].([]any)); n != 3 {
		t.Fatalf("inbox has %d entries", n)
	}
	text, _, code := r.mull("", "inbox", "--text")
	if code != 0 || !strings.Contains(text, "DECISIONS (1)") || !strings.Contains(text, "QUESTIONS (2)") {
		t.Fatalf("inbox --text:\n%s", text)
	}

	r.ok("decide", q, "No compatibility path needed", "--on-request")
	r.ok("park", later, "after release")
	if n := len(r.ok("decisions")["decisions"].([]any)); n != 1 {
		t.Fatalf("decision log has %d entries", n)
	}
	r.fails("dependency_cycle", "need", phase, release)
	// Needs exist between phases only.
	r.fails("not_allowed", "need", second, item)
	r.fails("not_allowed", "unneed", second, item)
	r.fails("not_allowed", "need", release, item)
	r.fails("not_allowed", "add", "Docs", "--phase", release, "--needs", phase)

	// Finish the item: config, done, report, review.
	r.ok("config", "checks", "true")
	r.ok("config", "layer", "ui", "web/**")
	gittest.Commit(t, r.dir, "web/tree.html", "x\n", "Tree edits", "Mull: "+item)
	r.fails("design_required", "done", item, "--summary", "Tree edits become Facts.", "--tour", "Open the tree")
	r.fails("usage", "done", item, "--summary", "Tree edits become Facts.", "--tour", "-", "--design", "-")
	r.fails("tour_required", "done", item, "--summary", "Tree edits become Facts.", "--design", "Edits are Facts.")
	done := r.okIn("Every edit is written as a Fact.\n\nThe tree reads Facts back.\n", "done", item,
		"--summary", "Tree edits become Facts.", "--tour", "1. Open the tree\n2. Rename a genus", "--design", "-")
	if tour := done["report"].(map[string]any)["tour"].([]any); len(tour) != 2 || tour[0] != "Open the tree" {
		t.Fatalf("tour = %v", tour)
	}
	if design := done["report"].(map[string]any)["design"]; design != "Every edit is written as a Fact.\n\nThe tree reads Facts back." {
		t.Fatalf("design = %q", design)
	}
	// The decision recorded on the item rides with its review.
	if ds := done["report"].(map[string]any)["decisions"].([]any); len(ds) != 1 {
		t.Fatalf("report decisions = %v", ds)
	}
	if text, _, _ := r.mull("", "inbox", "--text"); !strings.Contains(text, "carries decisions") {
		t.Fatalf("inbox --text:\n%s", text)
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
	for _, g := range []string{"agents", "supervisor", "intake", "triage", "json"} {
		if gv := r.ok("guide", g); gv["guide"] != g || !strings.Contains(gv["text"].(string), "mull") {
			t.Fatalf("guide %s = %v", g, gv)
		}
	}
	r.fails("not_found", "guide", "nope")
	// The human-feedback rule: feedback handed over for later has its own instruction.
	if g := r.ok("guide", "agents")["text"].(string); !strings.Contains(g, "Feedback the human hands over for later") ||
		!strings.Contains(g, `--source "human,`) {
		t.Fatal("guide agents lacks the human-feedback rule")
	}
	// Model v2 rules each guide must keep: who captures and commits, how done is
	// called, where the backlog is triaged, and migrating old data first.
	for g, wants := range map[string][]string{
		"agents":     {"Anyone captures; only the human commits", "--design -", "mull guide triage", "phase_doc"},
		"supervisor": {"mull guide triage", "mull set <id> plan", "with or without `--on-request`"},
		"intake":     {"mull migrate --dry-run", "plan agree", "phase doc"},
		"triage":     {"--on-request", "mull migrate"},
	} {
		text := r.ok("guide", g)["text"].(string)
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Errorf("guide %s lacks %q", g, want)
			}
		}
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

func TestDocCommands(t *testing.T) {
	r := run{t: t, dir: gittest.NewRepo(t)}
	r.ok("init")
	r.ok("plan", "new", "launch", "Launch", "--on-request") // shaping: a doc and questions
	if d := r.okIn("# Launch\n\nDone when: it ships.\n", "plan", "doc", "launch", "-"); d["doc"] != "# Launch\n\nDone when: it ships." {
		t.Fatalf("plan doc - = %v", d)
	}
	if out, _, code := r.mull("", "plan", "doc", "launch", "--text"); code != 0 || out != "# Launch\n\nDone when: it ships.\n" {
		t.Fatalf("plan doc --text = %q", out)
	}
	if p := r.ok("show", "launch"); p["plan"].(map[string]any)["doc"] != "# Launch\n\nDone when: it ships." {
		t.Fatalf("show = %v", p)
	}
	r.fails("usage", "plan", "doc", "launch", "inline text")
	r.fails("usage", "plan", "doc", "launch", "-", "--log")
	r.fails("usage", "plan", "doc", "launch", "--log", "--text")
	r.fails("not_found", "plan", "doc", "nope")
	r.fails("not_found", "phase", "doc", "ffff")
	// Empty stdin removes the doc.
	if d := r.okIn("", "plan", "doc", "launch", "-"); d["doc"] != "" {
		t.Fatalf("emptied doc = %v", d)
	}
	if l := r.ok("plan", "doc", "launch", "--log"); len(l["versions"].([]any)) != 2 {
		t.Fatalf("log = %v", l)
	}
}

func TestScopeRules(t *testing.T) {
	r := run{t: t, dir: gittest.NewRepo(t)}
	r.ok("init")
	// With no plan, an entry without scope is about the whole project.
	if e := r.ok("ask", "no plans yet"); e["plan"] != "" {
		t.Fatalf("entry = %v", e)
	}
	r.ok("plan", "new", "alpha", "Alpha", "--on-request")
	r.ok("plan", "agree", "alpha", "--on-request")
	r.ok("plan", "new", "beta", "Beta", "--on-request")
	for _, verb := range []string{"decided", "ask"} {
		r.fails("scope_required", verb, "about something")
	}
	if e := r.ok("ask", "about beta", "--plan", "beta"); e["plan"] != "beta" {
		t.Fatalf("entry = %v", e)
	}
	phase := id(r.ok("phase", "add", "E", "--plan", "alpha"))
	it := r.ok("add", "x", "--phase", phase)
	if it["plan"] != "alpha" {
		t.Fatalf("plan not inferred from phase: %v", it)
	}
	if e := r.ok("ask", "about the item", "--item", id(it)); e["plan"] != "alpha" {
		t.Fatalf("entry plan not inferred from item: %v", e)
	}
	r.fails("invalid_argument", "add", "y", "--phase", phase, "--plan", "beta")
	r.fails("scope_required", "add", "z")
	if it := r.ok("add", "z", "--plan", "none"); it["plan"] != nil {
		t.Fatalf("unplanned item = %v", it)
	}
	// Captures need no scope.
	r.ok("bug", "untagged")
}

func TestTriageCommand(t *testing.T) {
	r := run{t: t, dir: gittest.NewRepo(t)}
	r.ok("init")
	r.ok("plan", "new", "alpha", "Alpha", "--on-request")
	a := id(r.ok("bug", "Crash on empty list"))
	b := id(r.ok("idea", "Dark mode"))
	// Tag a capture to a plan without committing it.
	if s := r.ok("set", b, "plan", "alpha"); s["item"].(map[string]any)["plan"] != "alpha" {
		t.Fatalf("set plan = %v", s)
	}
	r.fails("not_allowed", "triage", a)
	r.fails("invalid_argument", "triage", "--on-request")
	if tr := r.ok("triage", a, "--on-request"); len(tr["items"].([]any)) != 1 {
		t.Fatalf("triage = %v", tr)
	}
	if l := r.ok("list", "--untriaged")["items"].([]any); len(l) != 1 || id(l[0].(map[string]any)) != b {
		t.Fatalf("list --untriaged = %v", l)
	}
	if v := r.ok("view"); v["untriaged"] != float64(1) || v["last_triage"] == nil {
		t.Fatalf("view = %v", v)
	}
	if tr := r.ok("triage", "--all", "--plan", "alpha", "--on-request"); len(tr["items"].([]any)) != 1 {
		t.Fatalf("triage --all = %v", tr)
	}
	if v := r.ok("view"); v["untriaged"] != float64(0) {
		t.Fatalf("view = %v", v)
	}
}

// The intake guide's example batch is the one agents copy: it must run as written.
func TestIntakeExampleBatchRuns(t *testing.T) {
	r := run{t: t, dir: gittest.NewRepo(t)}
	r.ok("init")
	text := r.ok("guide", "intake")["text"].(string)
	_, after, ok := strings.Cut(text, "## An example batch")
	if !ok {
		t.Fatal("intake guide has no example batch")
	}
	parts := strings.SplitN(after, "```", 3)
	if len(parts) < 3 {
		t.Fatal("intake guide's example batch has no code block")
	}
	if out, errOut, code := r.mull(strings.TrimSpace(parts[1])+"\n", "batch"); code != 0 {
		t.Fatalf("example batch exited %d: %s\n%s", code, errOut, out)
	}
	if d := r.ok("plan", "doc", "launch"); d["doc"] == "" {
		t.Fatalf("example batch wrote no plan doc: %v", d)
	}
}
