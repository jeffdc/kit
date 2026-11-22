package mull

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"mull/internal/model"
)

// ---- Config ----

// ConfigSet changes one scalar config key: integration, release or remote.
func (m *Mull) ConfigSet(key, value string) (model.Config, error) {
	return m.configTx("config set "+key, func(s *state) (model.Config, error) { return m.configSet(s, key, value) })
}

// ConfigChecks replaces the list of hard-check commands done runs. The
// checks judge agents' work, so only the human changes them: an agent must
// say they asked (onRequest).
func (m *Mull) ConfigChecks(commands []string, onRequest bool) (model.Config, error) {
	return m.configTx("config checks", func(s *state) (model.Config, error) { return m.configChecks(s, commands, onRequest) })
}

// ConfigLayer sets a layer's path globs; no globs removes the layer.
func (m *Mull) ConfigLayer(name string, globs []string) (model.Config, error) {
	return m.configTx("config layer "+name, func(s *state) (model.Config, error) { return m.configLayer(s, name, globs) })
}

func (m *Mull) configTx(msg string, fn func(s *state) (model.Config, error)) (model.Config, error) {
	var out model.Config
	err := m.update(msg, func(s *state) error {
		var err error
		out, err = fn(s)
		return err
	})
	return out, err
}

func (m *Mull) configSet(s *state, key, value string) (model.Config, error) {
	return s.changeConfig(func(c *model.Config) error {
		switch key {
		case "integration":
			c.Integration = value
		case "release":
			c.Release = value
		case "remote":
			c.Remote = value
		default:
			return errf(CodeInvalid, "settable keys are integration, release and remote (use 'config checks' and 'config layer' for the rest)")
		}
		if value == "" {
			return errf(CodeInvalid, "%s cannot be empty", key)
		}
		return nil
	})
}

func (m *Mull) configChecks(s *state, commands []string, onRequest bool) (model.Config, error) {
	if !m.asked(onRequest) {
		return model.Config{}, errf(CodeNotAllowed, "only the human changes the hard checks, since they judge agents' work: pass --on-request when the human has asked for this change")
	}
	return s.changeConfig(func(c *model.Config) error {
		c.DoneChecks = append([]string{}, trimCommands(commands)...)
		return nil
	})
}

// trimCommands trims each check command and leaves out empty ones.
func trimCommands(commands []string) []string {
	var out []string
	for _, cmd := range commands {
		if cmd = strings.TrimSpace(cmd); cmd != "" {
			out = append(out, cmd)
		}
	}
	return out
}

// PlanChecks replaces a plan's own hard checks, which done runs for the
// plan's items after config's; no commands removes them. Like config's, only
// the human changes them: an agent must say they asked (onRequest).
func (m *Mull) PlanChecks(slug string, commands []string, onRequest bool) (*model.Plan, error) {
	var out *model.Plan
	err := m.update("plan checks "+slug, func(s *state) error {
		var err error
		out, err = m.planChecks(s, slug, commands, onRequest)
		return err
	})
	return out, err
}

func (m *Mull) planChecks(s *state, slug string, commands []string, onRequest bool) (*model.Plan, error) {
	p, err := s.plan(slug)
	if err != nil {
		return nil, err
	}
	if !m.asked(onRequest) {
		return nil, errf(CodeNotAllowed, "only the human changes a plan's hard checks, since they judge agents' work: pass --on-request when the human has asked for this change to %q", slug)
	}
	checks := trimCommands(commands)
	if slices.Equal(checks, p.Checks) {
		return p, nil // unchanged: write nothing
	}
	p.Checks, p.Updated = checks, m.now()
	return p, s.putPlan(p)
}

// PlanShared replaces a plan's shared globs: the files its parallel work
// shares, whose changes done flags in the report. No globs removes them.
func (m *Mull) PlanShared(slug string, globs []string) (*model.Plan, error) {
	var out *model.Plan
	err := m.update("plan shared "+slug, func(s *state) error {
		var err error
		out, err = m.planShared(s, slug, globs)
		return err
	})
	return out, err
}

func (m *Mull) planShared(s *state, slug string, globs []string) (*model.Plan, error) {
	p, err := s.plan(slug)
	if err != nil {
		return nil, err
	}
	var shared []string
	for _, g := range globs {
		g = strings.TrimSpace(g)
		switch {
		case g == "" || slices.Contains(shared, g):
			continue
		case strings.HasPrefix(g, "/") || strings.HasPrefix(g, "./"):
			return nil, errf(CodeInvalid, "shared glob %q: give it relative to the repository's root, without a leading / or ./ (such as app/layout.css or assets/css/**)", g)
		}
		shared = append(shared, g)
	}
	if slices.Equal(shared, p.Shared) {
		return p, nil // unchanged: write nothing
	}
	p.Shared, p.Updated = shared, m.now()
	return p, s.putPlan(p)
}

// sharedChanges lists the changed files that match one of globs, by path,
// each with the first glob it matches.
func sharedChanges(changes []model.LayerChanges, globs []string) []model.SharedChange {
	if len(globs) == 0 {
		return nil
	}
	var res []*regexp.Regexp
	for _, g := range globs {
		re, err := globRegexp(g)
		if err != nil {
			return nil
		}
		res = append(res, re)
	}
	var out []model.SharedChange
	for _, lc := range changes {
		for _, f := range lc.Files {
			for k, re := range res {
				if re.MatchString(f.Path) {
					out = append(out, model.SharedChange{Status: f.Status, Path: f.Path, Glob: globs[k]})
					break
				}
			}
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Path < out[b].Path })
	return out
}

// hardChecks is the commands done runs for an item in plan p (nil for an
// item with no plan): config's, then the plan's own, leaving out any that
// config already runs.
func hardChecks(cfg model.Config, p *model.Plan) []string {
	out := append([]string{}, cfg.DoneChecks...)
	if p != nil {
		for _, c := range p.Checks {
			if !slices.Contains(out, c) {
				out = append(out, c)
			}
		}
	}
	return out
}

const noChecksHint = `the human sets them with: mull config checks "<command>" ... --on-request, and a plan's own with: mull plan checks <slug> "<command>" ... --on-request`

func (m *Mull) configLayer(s *state, name string, globs []string) (model.Config, error) {
	return s.changeConfig(func(c *model.Config) error {
		if name == "" {
			return errf(CodeInvalid, "a layer needs a name")
		}
		if c.Layers == nil {
			c.Layers = map[string][]string{}
		}
		if len(globs) == 0 {
			delete(c.Layers, name)
			return nil
		}
		for _, g := range globs {
			if _, err := globRegexp(g); err != nil {
				return errf(CodeInvalid, "bad glob %q: %v", g, err)
			}
		}
		c.Layers[name] = globs
		return nil
	})
}

func (s *state) changeConfig(fn func(c *model.Config) error) (model.Config, error) {
	c := s.config
	c.Layers = map[string][]string{}
	for k, v := range s.config.Layers {
		c.Layers[k] = v
	}
	if c.DoneChecks == nil {
		c.DoneChecks = []string{}
	}
	if err := fn(&c); err != nil {
		return model.Config{}, err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return model.Config{}, err
	}
	s.tx.Put(model.ConfigPath, b)
	s.config = c
	return c, nil
}

// globRegexp turns a path glob into a regexp: ** matches across directories,
// * and ? within one.
func globRegexp(glob string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(glob); i++ {
		switch c := glob[i]; c {
		case '*':
			if i+1 < len(glob) && glob[i+1] == '*' {
				i++
				if i+1 < len(glob) && glob[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// layerOf returns the first layer (by name) whose globs match path, or "other".
func layerOf(layers map[string][]string, path string) string {
	for _, name := range sortedKeys(layers) {
		for _, g := range layers[name] {
			if re, err := globRegexp(g); err == nil && re.MatchString(path) {
				return name
			}
		}
	}
	return "other"
}

// ---- Checks ----

const tailLines = 40

// cleanTree returns HEAD's commit and tree, or an error if the working tree
// has uncommitted or untracked changes: check results must belong to a commit.
func (m *Mull) cleanTree() (commit, tree string, err error) {
	st, err := m.repo.Output(nil, nil, "status", "--porcelain")
	if err != nil {
		return "", "", err
	}
	if st != "" {
		return "", "", errf(CodeDirty, "the working tree has uncommitted or untracked changes; commit them first so the checks belong to a commit:\n%s", st)
	}
	commit, err = m.repo.Output(nil, nil, "rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	tree, err = m.repo.Output(nil, nil, "rev-parse", "HEAD^{tree}")
	return commit, tree, err
}

// CheckOutcome is the result of running (or reusing) the hard checks.
type CheckOutcome struct {
	model.CheckRun
	Cached bool `json:"cached"`
}

// Check runs the hard checks against HEAD (which must be clean), reusing a
// cached pass for the same tree and command list, and records the run. With
// an item, they're the checks done would run for it: config's, then its
// plan's own. Without one, only config's.
func (m *Mull) Check(itemID string) (CheckOutcome, error) {
	var commands []string
	err := m.view(func(s *state) error {
		var p *model.Plan
		if itemID != "" {
			i, err := s.item(itemID)
			if err != nil {
				return err
			}
			p = s.plans[i.Plan]
		}
		commands = hardChecks(s.config, p)
		return nil
	})
	if err != nil {
		return CheckOutcome{}, err
	}
	if len(commands) == 0 {
		return CheckOutcome{}, errf(CodeNoChecks, "no hard checks configured; "+noChecksHint)
	}
	return m.runChecks(commands)
}

// runChecks runs commands as the hard checks against the clean HEAD,
// reusing a cached pass for the same tree and command list.
func (m *Mull) runChecks(commands []string) (CheckOutcome, error) {
	commit, tree, err := m.cleanTree()
	if err != nil {
		return CheckOutcome{}, err
	}
	if cached, ok := m.cachedPass(tree, commands); ok {
		return CheckOutcome{CheckRun: cached, Cached: true}, nil
	}
	run := model.CheckRun{Tree: tree, Commit: commit, At: m.now(), By: m.Worktree(), Passed: true}
	for _, c := range commands {
		r := m.runCheck(c)
		run.Results = append(run.Results, r)
		if r.Exit != 0 {
			run.Passed = false
			break // later checks would run against a known failure
		}
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return CheckOutcome{}, err
	}
	err = m.updateAny("check "+tree[:12], func(s *state) error { // a cache, not a record
		s.tx.Put(model.CheckPath(tree), append(data, '\n'))
		return nil
	})
	return CheckOutcome{CheckRun: run}, err
}

func (m *Mull) cachedPass(tree string, commands []string) (model.CheckRun, bool) {
	var run model.CheckRun
	found := false
	_ = m.view(func(s *state) error {
		b, ok := s.tx.Get(model.CheckPath(tree))
		if !ok || json.Unmarshal(b, &run) != nil || !run.Passed || len(run.Results) != len(commands) {
			return nil
		}
		for k, r := range run.Results {
			if r.Command != commands[k] {
				return nil
			}
		}
		found = true
		return nil
	})
	return run, found
}

func (m *Mull) runCheck(command string) model.CheckResult {
	start := time.Now()
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = m.repo.Dir
	cmd.Env = os.Environ()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	res := model.CheckResult{Command: command, DurationMS: time.Since(start).Milliseconds()}
	if exit, ok := err.(*exec.ExitError); ok {
		res.Exit = exit.ExitCode()
	} else if err != nil {
		res.Exit = -1
		out.WriteString("\n" + err.Error())
	}
	if res.Exit != 0 {
		lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
		if len(lines) > tailLines {
			lines = lines[len(lines)-tailLines:]
		}
		res.Tail = strings.Join(lines, "\n")
	}
	return res
}

// ---- Done ----

// DoneInput is what the agent supplies when finishing an item.
type DoneInput struct {
	Summary  string
	Tour     []string // what to click and what to see; required when UI files changed
	Design   string   // how it was built and why; always required
	Override string   // human only: finish despite failing or missing checks, with a reason
}

// DoneResult reports a done attempt.
type DoneResult struct {
	Report     model.Report `json:"report"`
	ReportPath string       `json:"report_path"`
	Item       ItemView     `json:"item"`
	Review     string       `json:"review_entry,omitempty"`
	Warnings   []string     `json:"warnings,omitempty"` // soft: they never stop done, and the report carries them too
	Reminder   string       `json:"reminder,omitempty"`
}

// ErrChecksFailed means the hard checks failed; the item stays active.
type ErrChecksFailed struct{ Report string }

func (e *ErrChecksFailed) Error() string {
	return "hard checks failed; the item stays active. See " + e.Report
}

// Done finishes an item: it runs (or reuses) the hard checks against a clean
// HEAD, writes a done report, marks the item done and files a review entry.
// If the checks fail, the failed report is saved and the item stays active.
func (m *Mull) Done(id string, in DoneInput) (DoneResult, error) {
	var out DoneResult
	if strings.TrimSpace(in.Summary) == "" {
		return out, errf(CodeInvalid, "done needs a summary: a few sentences on what changed and how it fits")
	}
	design := strings.TrimSpace(in.Design)
	if design == "" || strings.EqualFold(strings.TrimRight(design, "."), "none") {
		return out, errf(CodeDesignRequired, "done needs design notes with --design: how it was built and why (the approach, the shape of the code, what was rejected)")
	}
	wt := m.Worktree()
	var cfg model.Config
	var claim model.Claim
	var commands []string
	var target, preview string
	err := m.view(func(s *state) error {
		cfg = s.config
		i, err := s.item(id)
		if err != nil {
			return err
		}
		if err := ownActive(i, wt); err != nil {
			return err
		}
		claim = *i.Claim
		commands = hardChecks(cfg, s.plans[i.Plan])
		target, _ = s.targetOf(i.Plan)
		if p := s.plans[i.Plan]; p != nil {
			preview = p.Preview
		}
		return nil
	})
	if err != nil {
		return out, err
	}

	var checks model.CheckRun
	if len(commands) == 0 {
		if in.Override == "" {
			return out, errf(CodeNoChecks, "no hard checks configured, so nothing can prove this item done; "+noChecksHint)
		}
		commit, tree, err := m.cleanTree()
		if err != nil {
			return out, err
		}
		checks = model.CheckRun{Tree: tree, Commit: commit, At: m.now(), By: wt}
	} else {
		outcome, err := m.runChecks(commands)
		if err != nil {
			return out, err
		}
		checks = outcome.CheckRun
	}

	changes, base, err := m.changes(cfg, target, claim.Base, checks.Commit)
	if err != nil {
		return out, err
	}
	if len(in.Tour) == 0 {
		for _, lc := range changes {
			if lc.Layer == "ui" {
				return out, errf(CodeTourRequired, "UI files changed (%d); give a short UI tour with --tour: what to click and what to see", len(lc.Files))
			}
		}
	}

	warnings := tourWarnings(in.Tour, preview)
	finished := checks.Passed || in.Override != ""
	err = m.update("done "+id, func(s *state) error {
		i, err := s.item(id)
		if err != nil {
			return err
		}
		if err := ownActive(i, wt); err != nil {
			return err
		}
		n := len(s.tx.List(fmt.Sprintf("reports/%s-", id))) + 1
		decisions := s.openDecisions(id)
		rep := model.Report{
			Item: id, N: n, Passed: checks.Passed, Override: in.Override, Commit: checks.Commit, Base: base,
			At: m.now(), By: wt, Summary: strings.TrimSpace(in.Summary), Tour: in.Tour, Checks: checks,
			Design: design, Changes: changes, Warnings: warnings,
		}
		if p := s.plans[i.Plan]; p != nil {
			rep.Shared = sharedChanges(changes, p.Shared)
		}
		for _, d := range decisions {
			rep.Decisions = append(rep.Decisions, model.ReportDecision{ID: d.ID, Text: d.Text, Detail: d.Detail})
		}
		data, err := model.Encode(rep, renderReport(rep, i.Title))
		if err != nil {
			return err
		}
		out.ReportPath = model.ReportPath(id, n)
		s.tx.Put(out.ReportPath, data)
		out.Report = rep
		if !finished {
			out.Item = s.itemView(i)
			return nil
		}
		i.State, i.Updated = model.StateInReview, m.now()
		if err := s.putItem(i); err != nil {
			return err
		}
		e, err := m.newEntry(s, model.KindReview, "Review: "+i.Title, id, "", out.ReportPath, "")
		if err != nil {
			return err
		}
		if len(decisions) > 0 {
			for _, d := range decisions {
				e.Decisions = append(e.Decisions, d.ID)
			}
			if err := s.putEntry(e); err != nil {
				return err
			}
		}
		out.Review = e.ID
		out.Item = s.itemView(i)
		return nil
	})
	if err != nil {
		return out, err
	}
	out.Warnings = warnings
	if !finished {
		return out, &ErrChecksFailed{Report: out.ReportPath}
	}
	out.Reminder = m.personal("Show the human (or the supervisor) the report and the UI tour; they accept or send it back. " +
		"Accepting the review also ratifies the decisions it carries. Capture anything out of scope you noticed with mull bug \"...\" or mull idea \"...\" --source \"item " + id + "\"")
	return out, nil
}

func ownActive(i *model.Item, wt string) error {
	switch {
	case i.State == model.StateCaptured || i.State == model.StateCommitted:
		return errf(CodeState, "item %s is not started; claim it with: mull start %s", i.ID, i.ID)
	case i.State != model.StateActive:
		return errf(CodeState, "item %s is %s", i.ID, i.State)
	case i.Claim == nil || i.Claim.Worktree != wt:
		owner := "nobody"
		if i.Claim != nil {
			owner = i.Claim.Worktree
		}
		return errf(CodeClaimed, "item %s is claimed by %s, not this worktree", i.ID, owner)
	}
	return nil
}

// changes lists files changed from the claim's base (or the merge base with
// target, the item's plan's target branch) to commit, grouped by configured
// layer.
func (m *Mull) changes(cfg model.Config, target, claimBase, commit string) ([]model.LayerChanges, string, error) {
	base := claimBase
	if base == "" || !m.hasCommit(base) {
		mb, err := m.repo.Output(nil, nil, "merge-base", "refs/heads/"+target, commit)
		if err != nil {
			return nil, "", nil // no common base: report no changes rather than fail
		}
		base = mb
	}
	if base == commit {
		return nil, base, nil
	}
	nameStatus, err := m.repo.Output(nil, nil, "diff", "--name-status", "-M", base, commit)
	if err != nil {
		return nil, "", err
	}
	numstat, err := m.repo.Output(nil, nil, "diff", "--numstat", "-M", base, commit)
	if err != nil {
		return nil, "", err
	}
	counts := map[string][2]int{}
	for _, l := range strings.Split(numstat, "\n") {
		f := strings.Split(l, "\t")
		if len(f) < 3 {
			continue
		}
		path := f[len(f)-1]
		counts[path] = [2]int{atoi(f[0]), atoi(f[1])}
	}
	byLayer := map[string][]model.FileChange{}
	for _, l := range strings.Split(nameStatus, "\n") {
		f := strings.Split(l, "\t")
		if len(f) < 2 {
			continue
		}
		status, path := f[0][:1], f[len(f)-1]
		c := counts[path]
		byLayer[layerOf(cfg.Layers, path)] = append(byLayer[layerOf(cfg.Layers, path)],
			model.FileChange{Status: status, Path: path, Added: c[0], Removed: c[1]})
	}
	var out []model.LayerChanges
	for _, layer := range sortedKeys(byLayer) {
		files := byLayer[layer]
		sort.Slice(files, func(a, b int) bool { return files[a].Path < files[b].Path })
		out = append(out, model.LayerChanges{Layer: layer, Files: files})
	}
	return out, base, nil
}

func renderReport(r model.Report, title string) string {
	var b strings.Builder
	status := "PASSED"
	switch {
	case !r.Passed && r.Override != "":
		status = "OVERRIDDEN (checks did not pass): " + r.Override
	case !r.Passed:
		status = "FAILED: the item stays active"
	}
	fmt.Fprintf(&b, "# Done report %d: %s (%s)\n\n", r.N, title, r.Item)
	fmt.Fprintf(&b, "**Status:** %s  \n**Commit:** %s  \n**By:** %s at %s\n\n", status, short(r.Commit), r.By, r.At)
	if n := len(r.Shared); n > 0 {
		fmt.Fprintf(&b, "**Shared files:** %d changed that the plan lists as shared (see Shared files changed, below).\n\n", n)
	}
	b.WriteString("## What was built\n\n")
	fmt.Fprintf(&b, "### Summary\n\n%s\n\n", r.Summary)
	b.WriteString("### UI tour\n\n")
	if len(r.Tour) == 0 {
		b.WriteString("None given.\n\n")
	} else {
		for k, step := range r.Tour {
			fmt.Fprintf(&b, "%d. %s\n", k+1, step)
		}
		b.WriteString("\n")
	}
	for _, w := range r.Warnings {
		fmt.Fprintf(&b, "**Warning:** %s\n\n", w)
	}
	b.WriteString("### Checks (run by mull)\n\n")
	if len(r.Checks.Results) == 0 {
		b.WriteString("None configured.\n\n")
	}
	for _, c := range r.Checks.Results {
		mark := "pass"
		if c.Exit != 0 {
			mark = fmt.Sprintf("FAIL (exit %d)", c.Exit)
		}
		fmt.Fprintf(&b, "- `%s`: %s, %.1fs\n", c.Command, mark, float64(c.DurationMS)/1000)
		if c.Tail != "" {
			fmt.Fprintf(&b, "\n```\n%s\n```\n", c.Tail)
		}
	}
	b.WriteString("\n## How it was built\n\n### Design notes\n\n")
	if r.Design == "" {
		b.WriteString("None given.\n\n")
	} else {
		b.WriteString(r.Design + "\n\n")
	}
	b.WriteString("### Decisions made in the session\n\n")
	if len(r.Decisions) == 0 {
		b.WriteString("None recorded.\n\n")
	} else {
		b.WriteString("Accepting the review ratifies these. To decide one differently, before accepting: `mull decide <id> \"...\"`\n\n")
		for _, d := range r.Decisions {
			fmt.Fprintf(&b, "- **%s** %s\n", d.ID, d.Text)
			if d.Detail != "" {
				fmt.Fprintf(&b, "  Why: %s\n", d.Detail)
			}
		}
		b.WriteString("\n")
	}
	if len(r.Shared) > 0 {
		b.WriteString("### Shared files changed\n\n")
		for _, f := range r.Shared {
			fmt.Fprintf(&b, "- %s %s (shared: `%s`)\n", f.Status, f.Path, f.Glob)
		}
		b.WriteString("\n")
	}
	b.WriteString("### Changes by layer\n\n")
	if len(r.Changes) == 0 {
		b.WriteString("No file changes since the claim.\n")
	}
	for _, lc := range r.Changes {
		added, removed := 0, 0
		for _, f := range lc.Files {
			added += f.Added
			removed += f.Removed
		}
		fmt.Fprintf(&b, "**%s**: %d files, +%d −%d\n", lc.Layer, len(lc.Files), added, removed)
		for _, f := range lc.Files {
			fmt.Fprintf(&b, "- %s %s (+%d −%d)\n", f.Status, f.Path, f.Added, f.Removed)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// ReportOut is an item's done report as report prints it: the structured
// report, its markdown rendering, and the preview its tour's paths resolve
// against.
type ReportOut struct {
	Report  model.Report `json:"report"`
	Text    string       `json:"text"`
	Preview string       `json:"preview,omitempty"` // the item's plan's preview, now
}

// ReportText returns an item's done report (the latest when n is 0) as
// markdown, plus the structured report. The markdown's tour steps have their
// paths resolved against the item's plan's preview as it is now; the
// structured report keeps them as written.
func (m *Mull) ReportText(id string, n int) (ReportOut, error) {
	var out ReportOut
	var text string
	var rep model.Report
	err := m.view(func(s *state) error {
		i, err := s.item(id)
		if err != nil {
			return err
		}
		if p := s.plans[i.Plan]; p != nil {
			out.Preview = p.Preview
		}
		paths := s.tx.List(fmt.Sprintf("reports/%s-", id))
		if len(paths) == 0 {
			return errf(CodeNotFound, "item %s has no done report", id)
		}
		path := ""
		if n > 0 {
			path = model.ReportPath(id, n)
		} else {
			best := 0
			for _, p := range paths {
				var k int
				fmt.Sscanf(strings.TrimSuffix(p[strings.LastIndex(p, "-")+1:], ".md"), "%d", &k)
				if k > best {
					best, path = k, p
				}
			}
		}
		b, ok := s.tx.Get(path)
		if !ok {
			return errf(CodeNotFound, "no report %s", path)
		}
		body, err := model.Decode(b, &rep)
		text = body
		return err
	})
	out.Report, out.Text = rep, resolveReportText(text, rep.Tour, out.Preview)
	return out, err
}
