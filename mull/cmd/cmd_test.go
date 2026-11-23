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
	r.fails("not_initialized", "status")
	r.ok("init", "--integration", "main", "--release", "release")
	r.fails("not_allowed", "config", "checks", "true")
	r.ok("config", "checks", "true", "--on-request")
	r.fails("not_allowed", "plan", "new", "web", "Web")
	if p := r.ok("plan", "new", "web", "Web", "--target", "none", "--on-request"); p["state"] != "shaping" {
		t.Fatalf("plan = %v", p)
	}
	r.ok("plan", "agree", "web", "--on-request")
	ph := id(r.ok("phase", "add", "Pages", "--plan", "web", "--summary", "Done when: the pages load."))
	it := id(r.ok("add", "Home page", "--phase", ph, "--area", "web"))
	bug := id(r.ok("bug", "Footer link 404s", "--area", "web/footer", "--source", "a session"))
	r.fails("invalid_argument", "ask", "Centre it?", "--item", it)
	r.ok("ask", "Centre it?", "--item", it, "--default", "Centred", "--by", "2d")

	if ctx := r.ok("context", "--item", it); ctx["item"].(map[string]any)["queue"] == nil {
		t.Fatalf("context = %v", ctx)
	}
	r.ok("start", it, "--here")
	r.ok("decided", "Kept the old class", "--item", it, "--detail", "fewer changes")
	gittest.Commit(t, r.dir, "web/footer/home.html", "home\n", "Home", "Mull: "+it)
	r.fails("rules_required", "done", it, "--summary", "Home.")
	done := r.ok("done", it, "--summary", "Home page.", "--rules", "none", "--covers", bug, "--slowed", "nothing")
	if done["landed"] != true || done["report"].(map[string]any)["risk"] != "low" {
		t.Fatalf("done = %v", done)
	}
	if out, _, _ := r.mull("", "report", it, "--text"); !strings.Contains(out, "Home page.") || !strings.Contains(out, "Also covered:** "+bug) {
		t.Fatalf("report:\n%s", out)
	}
	text, _, code := r.mull("", "status", "--text")
	if code != 0 || !strings.Contains(text, "NEEDS YOU (1)") || !strings.Contains(text, "1 landed") {
		t.Fatalf("status:\n%s", text)
	}
	inbox := r.ok("inbox")["entries"].([]any)
	q := inbox[0].(map[string]any)["id"].(string)
	r.fails("not_allowed", "accept", q)
	r.ok("accept", q, "--on-request")
	if d := r.ok("decisions")["decisions"].([]any); len(d) != 2 {
		t.Fatalf("decisions = %v", d)
	}
	for _, g := range []string{"agents", "supervisor", "auditor", "intake", "json"} {
		if gv := r.ok("guide", g); !strings.Contains(gv["text"].(string), "mull") {
			t.Fatalf("guide %s = %v", g, gv)
		}
	}
	r.fails("usage", "frobnicate")
	r.fails("usage", "done")
}

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
		t.Fatal("no code block")
	}
	if out, errOut, code := r.mull(strings.TrimSpace(parts[1])+"\n", "batch"); code != 0 {
		t.Fatalf("example batch exited %d: %s\n%s", code, errOut, out)
	}
}
