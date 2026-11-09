package mull

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"mull/internal/model"
)

// ---- Config ----

// ConfigSet changes one scalar config key: integration, release or remote.
func (m *Mull) ConfigSet(key, value string) (model.Config, error) {
	return m.changeConfig("config set "+key, func(c *model.Config) error {
		switch key {
		case "integration":
			c.Integration = value
		case "release":
			c.Release = value
		case "remote":
			c.Remote = value
		default:
			return fmt.Errorf("settable keys are integration, release and remote (use 'config checks' and 'config layer' for the rest)")
		}
		if value == "" {
			return fmt.Errorf("%s cannot be empty", key)
		}
		return nil
	})
}

// ConfigChecks replaces the list of hard-check commands done runs.
func (m *Mull) ConfigChecks(commands []string) (model.Config, error) {
	return m.changeConfig("config checks", func(c *model.Config) error {
		c.DoneChecks = []string{}
		for _, cmd := range commands {
			if cmd = strings.TrimSpace(cmd); cmd != "" {
				c.DoneChecks = append(c.DoneChecks, cmd)
			}
		}
		return nil
	})
}

// ConfigLayer sets a layer's path globs; no globs removes the layer.
func (m *Mull) ConfigLayer(name string, globs []string) (model.Config, error) {
	return m.changeConfig("config layer "+name, func(c *model.Config) error {
		if c.Layers == nil {
			c.Layers = map[string][]string{}
		}
		if len(globs) == 0 {
			delete(c.Layers, name)
			return nil
		}
		for _, g := range globs {
			if _, err := globRegexp(g); err != nil {
				return err
			}
		}
		c.Layers[name] = globs
		return nil
	})
}

func (m *Mull) changeConfig(msg string, fn func(c *model.Config) error) (model.Config, error) {
	var out model.Config
	err := m.update(msg, func(s *state) error {
		c := s.config
		if err := fn(&c); err != nil {
			return err
		}
		b, err := yaml.Marshal(c)
		if err != nil {
			return err
		}
		s.tx.Put(model.ConfigPath, b)
		out = c
		return nil
	})
	return out, err
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
		return "", "", fmt.Errorf("the working tree has uncommitted or untracked changes; commit them first so the checks belong to a commit:\n%s", st)
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

// Check runs the configured hard checks against HEAD (which must be clean),
// reusing a cached pass for the same tree, and records the run.
func (m *Mull) Check() (CheckOutcome, error) {
	cfg, err := m.Config()
	if err != nil {
		return CheckOutcome{}, err
	}
	if len(cfg.DoneChecks) == 0 {
		return CheckOutcome{}, fmt.Errorf("no hard checks configured; the human sets them with: mull config checks \"<command>\" ...")
	}
	commit, tree, err := m.cleanTree()
	if err != nil {
		return CheckOutcome{}, err
	}
	if cached, ok := m.cachedPass(tree, cfg.DoneChecks); ok {
		return CheckOutcome{CheckRun: cached, Cached: true}, nil
	}
	run := model.CheckRun{Tree: tree, Commit: commit, At: m.now(), By: m.Worktree(), Passed: true}
	for _, c := range cfg.DoneChecks {
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
	err = m.update("check "+tree[:12], func(s *state) error {
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
	Override string   // human only: finish despite failing or missing checks, with a reason
}

// DoneResult reports a done attempt.
type DoneResult struct {
	Report         model.Report `json:"report"`
	ReportPath     string       `json:"report_path"`
	Item           ItemView     `json:"item"`
	Review         string       `json:"review_entry,omitempty"`
	LooseEndsFiled int          `json:"loose_ends_filed"`
	Reminder       string       `json:"reminder,omitempty"`
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
		return out, fmt.Errorf("done needs a summary: a few sentences on what changed and how it fits")
	}
	wt := m.Worktree()
	var cfg model.Config
	var claim model.Claim
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
		return nil
	})
	if err != nil {
		return out, err
	}

	var checks model.CheckRun
	if len(cfg.DoneChecks) == 0 {
		if in.Override == "" {
			return out, fmt.Errorf("no hard checks configured, so nothing can prove this item done; the human sets them with: mull config checks \"<command>\" ...")
		}
		commit, tree, err := m.cleanTree()
		if err != nil {
			return out, err
		}
		checks = model.CheckRun{Tree: tree, Commit: commit, At: m.now(), By: wt}
	} else {
		outcome, err := m.Check()
		if err != nil {
			return out, err
		}
		checks = outcome.CheckRun
	}

	changes, base, err := m.changes(cfg, claim.Base, checks.Commit)
	if err != nil {
		return out, err
	}
	if len(in.Tour) == 0 {
		for _, lc := range changes {
			if lc.Layer == "ui" {
				return out, fmt.Errorf("UI files changed (%d); give a short UI tour with --tour: what to click and what to see", len(lc.Files))
			}
		}
	}

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
		rep := model.Report{
			Item: id, N: n, Passed: checks.Passed, Override: in.Override, Commit: checks.Commit, Base: base,
			At: m.now(), By: wt, Summary: strings.TrimSpace(in.Summary), Tour: in.Tour, Checks: checks, Changes: changes,
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
		i.State, i.Updated = model.StateDone, m.now()
		if err := s.putItem(i); err != nil {
			return err
		}
		e, err := m.newEntry(s, model.KindReview, "Review: "+i.Title, id, "", out.ReportPath, "")
		if err != nil {
			return err
		}
		out.Review = e.ID
		out.Item = s.itemView(i)
		for _, eid := range sortedKeys(s.entries) {
			if en := s.entries[eid]; en.Item == id && en.Kind == model.KindLoose && en.Created >= claim.At {
				out.LooseEndsFiled++
			}
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	if !finished {
		return out, &ErrChecksFailed{Report: out.ReportPath}
	}
	out.Reminder = "Show the human (or the supervisor) the report and the UI tour; they accept or send it back. File any loose ends you noticed with: mull loose \"...\" --item " + id
	return out, nil
}

func ownActive(i *model.Item, wt string) error {
	switch {
	case i.State == model.StateOpen:
		return fmt.Errorf("item %s is not started; claim it with: mull start %s", i.ID, i.ID)
	case i.State != model.StateActive:
		return fmt.Errorf("item %s is %s", i.ID, i.State)
	case i.Claim == nil || i.Claim.Worktree != wt:
		owner := "nobody"
		if i.Claim != nil {
			owner = i.Claim.Worktree
		}
		return fmt.Errorf("item %s is claimed by %s, not this worktree", i.ID, owner)
	}
	return nil
}

// changes lists files changed from the claim's base (or the merge base with
// the integration branch) to commit, grouped by configured layer.
func (m *Mull) changes(cfg model.Config, claimBase, commit string) ([]model.LayerChanges, string, error) {
	base := claimBase
	if base == "" || !m.hasCommit(base) {
		integ := cfg.Integration
		if integ == "" {
			integ = "main"
		}
		mb, err := m.repo.Output(nil, nil, "merge-base", "refs/heads/"+integ, commit)
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
	fmt.Fprintf(&b, "## Summary\n\n%s\n\n", r.Summary)
	b.WriteString("## UI tour\n\n")
	if len(r.Tour) == 0 {
		b.WriteString("None given.\n\n")
	} else {
		for k, step := range r.Tour {
			fmt.Fprintf(&b, "%d. %s\n", k+1, step)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Checks (run by mull)\n\n")
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
	b.WriteString("\n## Changes by layer\n\n")
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

// ReportText returns an item's done report (the latest when n is 0) as
// markdown, plus the structured report.
func (m *Mull) ReportText(id string, n int) (string, model.Report, error) {
	var text string
	var rep model.Report
	err := m.view(func(s *state) error {
		if _, err := s.item(id); err != nil {
			return err
		}
		paths := s.tx.List(fmt.Sprintf("reports/%s-", id))
		if len(paths) == 0 {
			return fmt.Errorf("item %s has no done report", id)
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
			return fmt.Errorf("no report %s", path)
		}
		body, err := model.Decode(b, &rep)
		text = body
		return err
	})
	return text, rep, err
}
