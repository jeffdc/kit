package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ---- Checks ----

// cleanTree is HEAD's commit and tree; uncommitted or untracked changes
// are refused, so results belong to a commit.
func (m *Mull) cleanTree() (commit, tree string, err error) {
	st, err := m.repo.Output(nil, nil, "status", "--porcelain")
	if err != nil {
		return "", "", err
	}
	if st != "" {
		return "", "", errf(CodeDirty, "the working tree has uncommitted or untracked changes; commit them first:\n%s", st)
	}
	if commit, err = m.repo.Output(nil, nil, "rev-parse", "HEAD"); err != nil {
		return "", "", err
	}
	tree, err = m.repo.Output(nil, nil, "rev-parse", "HEAD^{tree}")
	return commit, tree, err
}

// CheckOut is a check run, and whether a pass for the tree was reused.
type CheckOut struct {
	CheckRun
	Cached bool `json:"cached"`
}

// Check runs the hard checks against the clean HEAD, reusing a pass for
// the same tree, and records the run.
func (m *Mull) Check() (CheckOut, error) {
	cfg, err := m.Config()
	if err != nil {
		return CheckOut{}, err
	}
	if len(cfg.Checks) == 0 {
		return CheckOut{}, errf(CodeNoChecks, "no hard checks: the human sets them with mull config checks <command>... --on-request")
	}
	return m.runChecks(cfg.Checks)
}

func (m *Mull) runChecks(commands []string) (CheckOut, error) {
	commit, tree, err := m.cleanTree()
	if err != nil {
		return CheckOut{}, err
	}
	if run, ok := m.cachedPass(tree, commands); ok {
		return CheckOut{CheckRun: run, Cached: true}, nil
	}
	run := CheckRun{Tree: tree, Commit: commit, At: m.now(), Passed: true}
	for _, c := range commands {
		r := runIn(m.repo.Dir, c, nil)
		run.Results = append(run.Results, r)
		if r.Exit != 0 {
			run.Passed = false
			break
		}
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return CheckOut{}, err
	}
	err = m.update("check "+tree[:12], func(s *state) error {
		s.tx.Put(checkPath(tree), append(data, '\n'))
		return nil
	})
	return CheckOut{CheckRun: run}, err
}

func (m *Mull) cachedPass(tree string, commands []string) (CheckRun, bool) {
	var run CheckRun
	ok := false
	_ = m.view(func(s *state) error {
		b, found := s.tx.Get(checkPath(tree))
		if !found || json.Unmarshal(b, &run) != nil || !run.Passed || len(run.Results) != len(commands) {
			return nil
		}
		for k, r := range run.Results {
			if r.Command != commands[k] {
				return nil
			}
		}
		ok = true
		return nil
	})
	return run, ok
}

// ---- Done ----

// DoneInput is what an agent gives when it finishes.
type DoneInput struct {
	Summary string   // one line: what changed
	Rules   string   // "none", or which rules the work touched and how
	Slowed  string   // what slowed it, if anything
	Covers  []string // queued or committed items this work also did
	Look    []string // what to look at, optional
	Design  string   // how and why, optional
}

// DoneOut is what done did.
type DoneOut struct {
	Report     Report   `json:"report"`
	Item       *Item    `json:"item"`
	Review     string   `json:"review_entry,omitempty"`
	Landed     bool     `json:"landed"`
	Merged     bool     `json:"merged"`
	MergeError string   `json:"merge_error,omitempty"`
	Pruned     []string `json:"pruned,omitempty"`
	Reminder   string   `json:"reminder"`
}

// Done finishes an item: it runs the hard checks once (a pass for the
// tree is reused) and the rulebook's, scores risk from the diff alone,
// writes the report, and either lands the work on its target (low and
// medium risk; a look is filed for medium and for sampled low work) or
// files it for review (high). Failed checks keep the item active.
func (m *Mull) Done(id string, in DoneInput) (DoneOut, error) {
	var out DoneOut
	if strings.TrimSpace(in.Summary) == "" {
		return out, errf(CodeInvalid, "done needs --summary: one line on what changed")
	}
	if strings.TrimSpace(in.Rules) == "" {
		return out, errf(CodeRulesRequired, "done needs --rules: none, or which rules the work touched and how (touching one makes it high risk)")
	}
	var cfg Config
	var claim Claim
	var target, release string
	var snap *MapSnapshot
	err := m.view(func(s *state) error {
		i, err := s.item(id)
		if err != nil {
			return err
		}
		if err := m.ownActive(i); err != nil {
			return err
		}
		cfg, claim, target, release, snap = s.config, *i.Claim, s.targetOf(i.Plan), s.release(), s.snapshot()
		return nil
	})
	if err != nil {
		return out, err
	}
	if len(cfg.Checks) == 0 {
		return out, errf(CodeNoChecks, "no hard checks, so nothing can prove this done: the human sets them with mull config checks --on-request")
	}
	if target != release && claim.Branch != target {
		head, _ := m.repo.RefCommit("HEAD")
		if behind, err := m.behindTarget(target, head); err != nil {
			return out, err
		} else if behind {
			return out, errf(CodeBehindTarget, "%s has moved on since this branch was made: merge it in (git merge %s), then finish again", target, target)
		}
	}
	checks, err := m.runChecks(cfg.Checks)
	if err != nil {
		return out, err
	}
	base := claim.Base
	if base == "" || !m.hasCommit(base) {
		base, _ = m.repo.Output(nil, nil, "merge-base", "refs/heads/"+target, checks.Commit)
	}
	files, err := m.diffFiles(base, checks.Commit)
	if err != nil {
		return out, err
	}
	rulebook, err := m.readRules()
	if err != nil {
		return out, err
	}
	var ruleRuns []RuleResult
	passed := checks.Passed
	if passed {
		ruleRuns = m.runRules(rulebook, cfg.Checks)
		for _, r := range ruleRuns {
			passed = passed && r.Passed
		}
	}
	edges, err := m.newEdges(cfg.Map, snap)
	if err != nil {
		return out, err
	}
	risk := scoreRisk(cfg.Risk.Effective(), files, in.Rules, id, checks.Commit, edges)
	if grown := m.grownRatchets(rulebook, base); len(grown) > 0 {
		risk = Risk{Band: RiskHigh, Why: append(risk.Why, "a ratchet grew: "+strings.Join(grown, ", ")+" (only the human grows one)")}
	}

	err = m.update("done "+id, func(s *state) error {
		i, err := s.item(id)
		if err != nil {
			return err
		}
		if err := m.ownActive(i); err != nil {
			return err
		}
		rep := Report{Item: id, N: len(s.tx.List("reports/"+id+"-")) + 1, Passed: passed, Commit: checks.Commit, Base: base,
			At: m.now(), By: m.by(), Summary: strings.TrimSpace(in.Summary), Rules: strings.TrimSpace(in.Rules),
			Slowed: strings.TrimSpace(in.Slowed), Look: in.Look, Design: strings.TrimSpace(in.Design),
			Risk: risk.Band, RiskWhy: risk.Why, Sampled: risk.Sampled, Checks: checks.CheckRun, RuleChecks: ruleRuns,
			Files: files, NewEdges: edges}
		for _, e := range s.entries {
			if e.Item == id && e.Kind == KindDecision && e.Created >= claim.At {
				rep.Decisions = append(rep.Decisions, e.Text)
			}
		}
		sort.Strings(rep.Decisions)
		if passed {
			if rep.Covered, err = m.cover(s, id, in.Covers); err != nil {
				return err
			}
			skip := map[string]bool{}
			for _, c := range rep.Covered {
				skip[c] = true
			}
			for _, q := range s.queueNear(changedPaths(files), skip) {
				rep.Nearby = append(rep.Nearby, q.ID)
			}
		}
		rep.Text = renderReport(rep, i.Title, rulesInScope(rulebook, files))
		if err := s.put(reportPath(id, rep.N), rep, rep.Text); err != nil {
			return err
		}
		out.Report, out.Item = rep, i
		if !passed {
			return nil
		}
		out.Landed = risk.Band != RiskHigh
		i.State, i.Updated = InReview, m.now()
		if out.Landed {
			i.State = Done
		}
		if err := s.putItem(i); err != nil {
			return err
		}
		if out.Landed && !risk.Sampled && risk.Band == RiskLow {
			return nil
		}
		e, err := m.newEntry(s, KindReview, i.Title, id, "", "")
		if err != nil {
			return err
		}
		e.Source = reportPath(id, rep.N)
		if out.Landed {
			e.Landed = risk.Band
			if risk.Sampled {
				e.Landed = "sampled"
			}
		}
		out.Review = e.ID
		return s.putEntry(e)
	})
	if err != nil {
		return out, err
	}
	if !passed {
		return out, errf(CodeChecksFailed, "the checks failed; the item stays active. See mull report %s", id)
	}
	switch {
	case out.Landed:
		out.Merged, out.MergeError = m.landItem(target, release, checks.Commit)
		out.Pruned, _ = m.Prune()
		out.Reminder = "Done, " + risk.Band + " risk: on " + target + ". mull sync."
		if !out.Merged {
			out.Reminder = "Done, " + risk.Band + " risk, but not on " + target + " yet: " + out.MergeError
		}
	default:
		out.Reminder = m.personal("High risk, so it waits for the human's review: " + strings.Join(risk.Why, "; ") + ". mull sync.")
	}
	return out, nil
}

func (m *Mull) ownActive(i *Item) error {
	switch {
	case i.State != Active:
		return errf(CodeState, "item %s is %s, not active: mull start %s", i.ID, i.State, i.ID)
	case i.Claim == nil || i.Claim.Worktree != m.Worktree():
		return errf(CodeClaimed, "item %s is claimed by another worktree", i.ID)
	}
	return nil
}

func (m *Mull) hasCommit(sha string) bool {
	_, code, err := m.repo.RunStatus(nil, nil, "cat-file", "-e", sha+"^{commit}")
	return err == nil && code == 0
}

// diffFiles lists the files changed from base to commit.
func (m *Mull) diffFiles(base, commit string) ([]FileChange, error) {
	if base == "" || base == commit {
		return nil, nil
	}
	names, err := m.repo.Output(nil, nil, "diff", "--name-status", "-M", base, commit)
	if err != nil {
		return nil, err
	}
	nums, err := m.repo.Output(nil, nil, "diff", "--numstat", "-M", base, commit)
	if err != nil {
		return nil, err
	}
	counts := map[string][2]int{}
	for _, l := range strings.Split(nums, "\n") {
		if f := strings.Split(l, "\t"); len(f) >= 3 {
			a, _ := strconv.Atoi(f[0])
			d, _ := strconv.Atoi(f[1])
			counts[f[len(f)-1]] = [2]int{a, d}
		}
	}
	var out []FileChange
	for _, l := range strings.Split(names, "\n") {
		if f := strings.Split(l, "\t"); len(f) >= 2 {
			path := f[len(f)-1]
			out = append(out, FileChange{Status: f[0][:1], Path: path, Added: counts[path][0], Removed: counts[path][1]})
		}
	}
	return out, nil
}

// renderReport is a report as the human reads it: short.
func renderReport(r Report, title string, inScope []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (%s)\n\n%s\n\n", title, r.Item, r.Summary)
	status := "checks passed"
	if !r.Passed {
		status = "checks FAILED"
	}
	if r.Risk != "" {
		status += ", " + r.Risk + " risk"
		if r.Sampled {
			status += ", sampled"
		}
	}
	added, removed := 0, 0
	for _, f := range r.Files {
		added, removed = added+f.Added, removed+f.Removed
	}
	fmt.Fprintf(&b, "- **Status:** %s; %d files, +%d −%d; commit %.12s\n", status, len(r.Files), added, removed, r.Commit)
	for _, w := range r.RiskWhy {
		fmt.Fprintf(&b, "- **Why %s:** %s\n", r.Risk, w)
	}
	fmt.Fprintf(&b, "- **Rules:** %s\n", r.Rules)
	line := func(label string, xs []string) {
		if len(xs) > 0 {
			fmt.Fprintf(&b, "- **%s:** %s\n", label, strings.Join(xs, ", "))
		}
	}
	line("Rules in scope", inScope)
	line("Also covered", r.Covered)
	line("Queued nearby", r.Nearby)
	line("New connections", r.NewEdges)
	if r.Slowed != "" {
		fmt.Fprintf(&b, "- **Slowed by:** %s\n", r.Slowed)
	}
	for _, c := range r.Checks.Results {
		if c.Exit != 0 {
			fmt.Fprintf(&b, "\n`%s` exited %d:\n\n```\n%s\n```\n", c.Command, c.Exit, c.Tail)
		}
	}
	for _, rc := range r.RuleChecks {
		if !rc.Passed {
			fmt.Fprintf(&b, "\nRule %s failed:\n\n```\n%s\n```\n", rc.ID, rc.Tail)
		}
	}
	if len(r.Look) > 0 {
		b.WriteString("\n**Look at:**\n\n")
		for k, s := range r.Look {
			fmt.Fprintf(&b, "%d. %s\n", k+1, s)
		}
	}
	if len(r.Decisions) > 0 {
		b.WriteString("\n**Decisions:**\n\n")
		for _, d := range r.Decisions {
			fmt.Fprintf(&b, "- %s\n", d)
		}
	}
	if r.Design != "" {
		fmt.Fprintf(&b, "\n**Design:** %s\n", r.Design)
	}
	return strings.TrimRight(b.String(), "\n")
}

// reports are an item's done reports, oldest first.
func (s *state) reports(id string) []Report {
	var out []Report
	for _, p := range s.tx.List("reports/" + id + "-") {
		if r, ok := s.report(p); ok {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].N < out[b].N })
	return out
}

func (s *state) report(path string) (Report, bool) {
	b, ok := s.tx.Get(path)
	if !ok {
		return Report{}, false
	}
	var r Report
	body, err := decode(b, &r)
	if err != nil {
		return Report{}, false
	}
	r.Text = body
	return r, true
}

func (s *state) latestReport(id string) *Report {
	rs := s.reports(id)
	if len(rs) == 0 {
		return nil
	}
	return &rs[len(rs)-1]
}

// ReportOf is an item's latest done report (n > 0: that attempt).
func (m *Mull) ReportOf(id string, n int) (Report, error) {
	var out Report
	err := m.view(func(s *state) error {
		rs := s.reports(id)
		for _, r := range rs {
			if n == 0 || r.N == n {
				out = r
			}
		}
		if out.Item == "" {
			return errf(CodeNotFound, "item %s has no report", id)
		}
		return nil
	})
	return out, err
}

// globRegexp turns a path glob into a regexp: ** crosses directories, *
// and ? stay within one.
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
