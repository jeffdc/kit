package cmd

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"mull/internal/gittest"
)

// keys describes a documented JSON object: keys that must be present and
// keys that may be (omitted when empty). See internal/guide/json.md.
type keys struct{ req, opt []string }

func (k keys) plus(o keys) keys {
	return keys{append(append([]string{}, k.req...), o.req...), append(append([]string{}, k.opt...), o.opt...)}
}

var (
	planKeys      = keys{[]string{"slug", "title", "state", "created", "updated"}, []string{"summary", "doc"}}
	phaseKeys     = keys{[]string{"id", "plan", "title", "created", "updated"}, []string{"needs", "from", "summary", "doc"}}
	itemKeys      = keys{[]string{"id", "title", "state", "created", "updated"}, []string{"plan", "phase", "kind", "category", "source", "needs", "docs", "claim", "from", "triaged", "notes"}}
	itemViewKeys  = itemKeys.plus(keys{[]string{"ready"}, []string{"blocked_by"}})
	phaseViewKeys = phaseKeys.plus(keys{[]string{"progress", "complete"}, []string{"blocked_by"}})
	entryKeys     = keys{[]string{"id", "kind", "text", "plan", "by", "created"}, []string{"item", "source", "settled", "decisions", "detail"}}
	configKeys    = keys{[]string{"integration", "release", "remote", "done_checks", "layers"}, nil}
	progressKeys  = keys{[]string{"total", "captured", "untriaged", "committed", "active", "in_review", "done", "dropped", "ready"}, nil}
	reportKeys    = keys{[]string{"item", "n", "passed", "commit", "at", "by", "summary", "checks"}, []string{"override", "base", "tour", "changes", "design", "decisions"}}
)

func checkKeys(t *testing.T, what string, v any, k keys) {
	t.Helper()
	obj, ok := v.(map[string]any)
	if !ok {
		t.Errorf("%s: not a JSON object: %v", what, v)
		return
	}
	allowed := map[string]bool{}
	for _, key := range append(append([]string{}, k.req...), k.opt...) {
		allowed[key] = true
	}
	for _, key := range k.req {
		if _, ok := obj[key]; !ok {
			t.Errorf("%s: missing documented key %q (has %v)", what, key, sortedKeysOf(obj))
		}
	}
	for key := range obj {
		if !allowed[key] {
			t.Errorf("%s: undocumented key %q", what, key)
		}
	}
}

func sortedKeysOf(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func first(t *testing.T, what string, v any) any {
	t.Helper()
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		t.Fatalf("%s: want a non-empty list, got %v", what, v)
	}
	return list[0]
}

func TestOutputShapes(t *testing.T) {
	r := run{t: t, dir: gittest.NewRepo(t)}
	shape := func(k keys, args ...string) map[string]any {
		t.Helper()
		v := r.ok(args...)
		checkKeys(t, strings.Join(args, " "), v, k)
		return v
	}
	shapeIn := func(stdin string, k keys, args ...string) map[string]any {
		t.Helper()
		v := r.okIn(stdin, args...)
		checkKeys(t, strings.Join(args, " "), v, k)
		return v
	}

	init := shape(keys{[]string{"initialized", "branch", "config"}, nil}, "init")
	checkKeys(t, "init.config", init["config"], configKeys)
	shape(keys{[]string{"version"}, nil}, "version")
	shape(configKeys, "config")
	shape(configKeys, "config", "set", "integration", "main")
	shape(configKeys, "config", "checks", "true")
	shape(configKeys, "config", "layer", "ui", "web/**")

	shape(planKeys, "plan", "new", "launch", "Launch", "--summary", "Ship it.")
	shape(planKeys, "plan", "agree", "launch", "--on-request")
	pl := shape(keys{[]string{"plans"}, nil}, "plan", "list")
	checkKeys(t, "plan list[0]", first(t, "plans", pl["plans"]), planKeys.plus(keys{[]string{"progress"}, nil}))

	phase := shape(phaseKeys, "phase", "add", "Billing", "--plan", "launch")
	el := shape(keys{[]string{"phases"}, nil}, "phase", "list")
	checkKeys(t, "phase list[0]", first(t, "phases", el["phases"]), phaseViewKeys)

	docKeys := keys{[]string{"type", "id", "doc"}, nil}
	shapeIn("The picture.\n", docKeys, "plan", "doc", "launch", "-")
	shape(docKeys, "plan", "doc", "launch")
	shapeIn("Its design.\n", docKeys, "phase", "doc", id(phase), "-")
	shape(docKeys, "phase", "doc", id(phase))
	dl := shape(keys{[]string{"type", "id", "versions"}, nil}, "phase", "doc", id(phase), "--log")
	checkKeys(t, "phase doc --log versions[0]", first(t, "versions", dl["versions"]), keys{[]string{"commit", "at", "doc"}, nil})
	shape(keys{[]string{"type", "id", "versions"}, nil}, "plan", "doc", "launch", "--log")

	a := shape(itemViewKeys, "add", "Webhook", "--phase", id(phase), "--doc", "docs/a.md")
	b := shape(itemViewKeys, "add", "Invoices", "--phase", id(phase))
	ps := shape(planKeys.plus(keys{[]string{"progress", "phases", "items"}, nil}), "plan", "show", "launch")
	checkKeys(t, "plan show.progress", ps["progress"], progressKeys)
	checkKeys(t, "plan show.phases[0]", first(t, "phases", ps["phases"]), phaseViewKeys)
	checkKeys(t, "plan show.items[0]", first(t, "items", ps["items"]), itemViewKeys)

	for _, ref := range []string{"launch", id(phase), id(a)} {
		v := shape(keys{[]string{"type"}, []string{"plan", "phase", "item", "entry"}}, "show", ref)
		inner := map[string]keys{
			"plan":  planKeys.plus(keys{[]string{"progress", "phases", "items"}, nil}),
			"phase": phaseViewKeys.plus(keys{[]string{"items"}, nil}),
			"item":  itemViewKeys,
		}[v["type"].(string)]
		checkKeys(t, "show "+ref, v[v["type"].(string)], inner)
	}
	l := shape(keys{[]string{"items"}, nil}, "list")
	checkKeys(t, "list[0]", first(t, "items", l["items"]), itemViewKeys)
	rd := shape(keys{[]string{"items"}, nil}, "ready")
	checkKeys(t, "ready[0]", first(t, "items", rd["items"]), itemViewKeys)

	set := shape(keys{[]string{"type"}, []string{"plan", "phase", "item"}}, "set", id(a), "title", "Stripe webhook")
	checkKeys(t, "set.item", set["item"], itemViewKeys)
	set = shape(keys{[]string{"type"}, []string{"plan", "phase", "item"}}, "set", "launch", "title", "Public launch")
	checkKeys(t, "set.plan", set["plan"], planKeys)
	set = shape(keys{[]string{"type"}, []string{"plan", "phase", "item"}}, "set", id(phase), "summary", "Stripe first")
	checkKeys(t, "set.phase", set["phase"], phaseViewKeys)

	c := shape(itemViewKeys, "add", "Copy", "--plan", "launch")
	golive := shape(phaseKeys, "phase", "add", "Go live", "--plan", "launch")
	nd := shape(keys{[]string{"type", "phase"}, nil}, "need", id(golive), id(phase))
	checkKeys(t, "need.phase", nd["phase"], phaseViewKeys)
	nd = shape(keys{[]string{"type", "phase"}, nil}, "unneed", id(golive), id(phase))
	checkKeys(t, "unneed.phase", nd["phase"], phaseViewKeys)

	st := shape(keys{[]string{"item", "warnings"}, nil}, "start", id(b))
	checkKeys(t, "start.item", st["item"], itemViewKeys)
	shape(itemViewKeys, "release", id(b))

	dec := shape(entryKeys, "decided", "Store raw payloads", "--item", id(a), "--detail", "replay")
	q := shape(entryKeys, "ask", "Annual plans?", "--plan", "launch")
	lo := shape(entryKeys, "ask", "Fix the footer's old pricing links?", "--plan", "launch")
	lo2 := shape(entryKeys, "ask", "Two h1s: which one stays?", "--plan", "launch")
	inb := shape(keys{[]string{"entries"}, nil}, "inbox")
	checkKeys(t, "inbox[0]", first(t, "entries", inb["entries"]), entryKeys)

	acc := shape(keys{[]string{"entries", "items", "follow_up"}, nil}, "accept", id(dec))
	checkKeys(t, "accept.entries[0]", first(t, "entries", acc["entries"]), entryKeys)
	shape(entryKeys, "decide", id(q), "Monthly first")
	do := shape(keys{[]string{"entry"}, []string{"item", "phase"}}, "do", id(lo))
	checkKeys(t, "do.entry", do["entry"], entryKeys)
	checkKeys(t, "do.item", do["item"], itemViewKeys)

	// Captures: bugs and ideas are captured items; only the human commits them.
	shape(itemViewKeys, "idea", "Mobile ID tool", "--plan", "launch", "--detail", "Phones in the field.", "--source", "conversation")
	bug := shape(itemViewKeys, "bug", "Flaky login test", "--category", "tests")
	h1s := shape(itemViewKeys, "bug", "Two h1s on home", "--phase", id(phase))
	shape(itemViewKeys, "commit", id(bug), "--on-request")
	tr := shape(keys{[]string{"items"}, nil}, "triage", id(h1s), "--on-request")
	checkKeys(t, "triage.items[0]", first(t, "items", tr["items"]), itemViewKeys)
	shape(itemViewKeys, "bug", "Untriaged for the view", "--plan", "launch")
	lo3 := shape(entryKeys, "ask", "A pricing page?", "--plan", "launch")
	promo := shape(keys{[]string{"entry"}, []string{"item", "phase"}}, "do", id(lo3), "--as", "phase")
	checkKeys(t, "do --as phase .phase", promo["phase"], phaseViewKeys)
	shape(entryKeys, "park", id(lo2), "after launch")
	drop := shape(keys{[]string{"entries", "items"}, nil}, "drop", id(c))
	checkKeys(t, "drop.items[0]", first(t, "items", drop["items"]), itemViewKeys)
	ds := shape(keys{[]string{"decisions"}, nil}, "decisions")
	checkKeys(t, "decisions[0]", first(t, "decisions", ds["decisions"]),
		keys{[]string{"id", "plan", "kind", "context", "decision", "by", "at"}, []string{"item", "on_request"}})

	// Finish work.
	r.ok("start", id(a))
	ck := shape(keys{[]string{"tree", "commit", "at", "by", "passed", "results", "cached"}, nil}, "check")
	checkKeys(t, "check.results[0]", first(t, "results", ck["results"]), keys{[]string{"command", "exit", "duration_ms"}, []string{"tail"}})
	r.ok("decided", "Retry three times", "--item", id(a), "--detail", "Stripe's advice")
	dn := shapeIn("One handler.\n", keys{[]string{"report", "report_path", "item"}, []string{"review_entry", "reminder"}},
		"done", id(a), "--summary", "Webhook handler.", "--tour", "none", "--design", "-")
	checkKeys(t, "done.item", dn["item"], itemViewKeys)
	rp := shape(keys{[]string{"report", "text"}, nil}, "report", id(a))
	checkKeys(t, "report.report", rp["report"], reportKeys)
	checkKeys(t, "report.decisions[0]", first(t, "decisions", rp["report"].(map[string]any)["decisions"]),
		keys{[]string{"id", "text"}, []string{"detail"}})
	rv := shape(keys{[]string{"type"}, []string{"entry"}}, "show", dn["review_entry"].(string))
	checkKeys(t, "show review", rv["entry"], entryKeys)
	if ds, _ := rv["entry"].(map[string]any)["decisions"].([]any); len(ds) != 1 {
		t.Errorf("review entry decisions = %v", ds)
	}

	v := shape(keys{[]string{"generated", "integration", "integration_found", "release", "ahead_of_release", "plans", "unlinked_commits_this_week", "untriaged"},
		[]string{"unlinked_recent", "unplanned", "legacy", "last_triage"}}, "view")
	board := first(t, "view.plans", v["plans"])
	boardKeys := keys{[]string{"slug", "title", "progress", "phases", "unfiled", "problems", "in_flight", "needs_you"}, []string{"state"}}
	checkKeys(t, "view.plans[0]", board, boardKeys)
	checkKeys(t, "view.unplanned", v["unplanned"], boardKeys)
	checkKeys(t, "view.needs_you", board.(map[string]any)["needs_you"],
		keys{[]string{"reviews", "decisions", "questions", "older_than_a_week", "parked"}, nil})
	shape(keys{[]string{"worktree", "branch", "protocol", "my_claims", "plans", "ready", "active", "decisions"}, []string{"item"}}, "context")
	shape(keys{[]string{"guide", "text"}, nil}, "guide", "json")

	remote := t.TempDir() + "/remote.git"
	gittest.Git(t, r.dir, "init", "-q", "--bare", remote)
	gittest.Git(t, r.dir, "remote", "add", "origin", remote)
	shape(keys{[]string{"remote", "action", "local", "pushed_commits"}, []string{"remote_before", "dry_run", "commands"}}, "sync", "--dry-run")

	// batch prints an array, one result per line.
	out, errOut, code := r.mull(`{"op": "ask", "plan": "launch", "text": "x", "ref": "x"}`+"\n", "batch")
	if code != 0 {
		t.Fatal(errOut)
	}
	var results []any
	if err := json.Unmarshal([]byte(out), &results); err != nil || len(results) != 1 {
		t.Fatalf("batch output = %s", out)
	}
	checkKeys(t, "batch[0]", results[0], keys{[]string{"line", "op", "result"}, []string{"ref", "id"}})
	checkKeys(t, "batch[0].result", results[0].(map[string]any)["result"], entryKeys)

	// Errors: a JSON object with a code on stderr, nothing on stdout.
	e := r.fails("not_found", "show", "ffff")
	checkKeys(t, "error", e, keys{[]string{"code", "error"}, []string{"lines"}})
}
