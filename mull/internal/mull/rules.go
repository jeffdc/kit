package mull

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"mull/internal/git"
	"mull/internal/model"
)

// The rulebook is RULES.md at the root of the project's repository: the
// human's rules for the code, versioned with it, so a rule lands with its
// check. Each rule is a level-two heading with an ID, then optional fields,
// then why:
//
//	## R1: Pages use only library components
//
//	- scope: lib/oaks_web/**, assets/**
//	- check: scripts/check-components
//	- ratchet: yes
//
//	Why it matters, in a sentence or two.
//
// A rule with no check is advisory: a hope, not a guard, and shown as one.
// A check passes when it exits 0. A ratcheted check instead prints its
// violations, one per line; the ratchet file (.mull/ratchets/<ID>.txt)
// lists the known ones, and may only shrink: a violation not in it fails,
// and one fixed but still listed fails until the file is shrunk (mull rules
// ratchet <ID>), so the count can only go down.

// RulesPath is the rulebook's path in a repository.
const RulesPath = "RULES.md"

// Rule is one rule of the rulebook.
type Rule struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Scope    []string `json:"scope,omitempty"`
	Check    string   `json:"check,omitempty"`
	Ratchet  string   `json:"ratchet,omitempty"` // the ratchet file's path, when ratcheted
	Advisory bool     `json:"advisory,omitempty"`
	Why      string   `json:"why,omitempty"`
}

var (
	ruleHead  = regexp.MustCompile(`^##\s+([A-Z][A-Z0-9]*[0-9]+)\s*[:.]\s*(.+?)\s*$`)
	ruleField = regexp.MustCompile(`^[-*]\s+(scope|check|ratchet)\s*:\s*(.*?)\s*$`)
)

// ParseRules reads a rulebook. Headings that don't look like rules (no ID)
// are prose and are skipped; a repeated ID is an error.
func ParseRules(text string) ([]Rule, error) {
	var rules []Rule
	seen := map[string]bool{}
	var cur *Rule
	var why []string
	flush := func() {
		if cur == nil {
			return
		}
		cur.Why = strings.TrimSpace(strings.Join(why, "\n"))
		cur.Advisory = cur.Check == ""
		rules = append(rules, *cur)
		cur, why = nil, nil
	}
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if m := ruleHead.FindStringSubmatch(line); m != nil {
			flush()
			if seen[m[1]] {
				return nil, errf(CodeInvalid, "%s: rule %s appears twice", RulesPath, m[1])
			}
			seen[m[1]] = true
			cur = &Rule{ID: m[1], Title: m[2]}
			continue
		}
		if strings.HasPrefix(line, "#") {
			flush() // another heading ends the rule
			continue
		}
		if cur == nil {
			continue
		}
		if m := ruleField.FindStringSubmatch(strings.TrimSpace(line)); m != nil && len(why) == 0 {
			switch m[1] {
			case "scope":
				for _, g := range strings.Split(m[2], ",") {
					if g = strings.Trim(strings.TrimSpace(g), "`"); g != "" {
						cur.Scope = append(cur.Scope, g)
					}
				}
			case "check":
				cur.Check = strings.Trim(m[2], "`")
			case "ratchet":
				switch v := strings.Trim(m[2], "`"); strings.ToLower(v) {
				case "", "no", "none":
				case "yes":
					cur.Ratchet = ratchetPath(cur.ID)
				default:
					cur.Ratchet = v
				}
			}
			continue
		}
		if strings.TrimSpace(line) != "" || len(why) > 0 {
			why = append(why, line)
		}
	}
	flush()
	return rules, sc.Err()
}

func ratchetPath(id string) string { return ".mull/ratchets/" + id + ".txt" }

// readRules reads the rulebook in this worktree; none is no rules.
func (m *Mull) readRules() ([]Rule, error) {
	b, err := os.ReadFile(filepath.Join(m.repo.Dir, RulesPath))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ParseRules(string(b))
}

// readRulesAt reads the rulebook as committed at rev; none is no rules.
func readRulesAt(r *git.Repo, rev string) ([]Rule, error) {
	out, code, err := r.RunStatus(nil, nil, "show", rev+":"+RulesPath)
	if err != nil || code != 0 {
		return nil, nil
	}
	return ParseRules(string(out))
}

// RulesOut is mull rules: the rulebook, and how many rules have a check.
type RulesOut struct {
	Path     string `json:"path"`
	Rules    []Rule `json:"rules"`
	Checked  int    `json:"checked"`
	Advisory int    `json:"advisory"`
}

// Rules lists this worktree's rulebook.
func (m *Mull) Rules() (RulesOut, error) {
	rules, err := m.readRules()
	out := RulesOut{Path: RulesPath, Rules: rules}
	if out.Rules == nil {
		out.Rules = []Rule{}
	}
	for _, r := range rules {
		if r.Advisory {
			out.Advisory++
		} else {
			out.Checked++
		}
	}
	return out, err
}

// runRules runs every checked rule's check in this worktree, comparing a
// ratcheted one's violations with its ratchet file.
func (m *Mull) runRules(rules []Rule, passed []string) []model.RuleResult {
	var out []model.RuleResult
	for _, r := range rules {
		if r.Advisory {
			continue
		}
		if r.Ratchet == "" && slices.Contains(passed, r.Check) {
			// The hard checks ran it already, and it passed.
			out = append(out, model.RuleResult{ID: r.ID, Passed: true})
			continue
		}
		rr := model.RuleResult{ID: r.ID}
		if r.Ratchet == "" {
			res := runIn(m.repo.Dir, r.Check, []string{"MULL_RULE=" + r.ID})
			rr.Exit, rr.DurationMS, rr.Passed, rr.Tail = res.Exit, res.DurationMS, res.Exit == 0, res.Tail
			out = append(out, rr)
			continue
		}
		start := time.Now()
		found, err := ruleViolations(m.repo.Dir, r.Check, r.ID)
		rr.DurationMS = time.Since(start).Milliseconds()
		if err != nil {
			rr.Tail = err.Error()
			out = append(out, rr)
			continue
		}
		known := readRatchet(filepath.Join(m.repo.Dir, r.Ratchet))
		rr.Count = len(found)
		for _, v := range found {
			if !known[v] {
				rr.New = append(rr.New, v)
			}
		}
		have := map[string]bool{}
		for _, v := range found {
			have[v] = true
		}
		for v := range known {
			if !have[v] {
				rr.Fixed++
			}
		}
		rr.Passed = len(rr.New) == 0 && rr.Fixed == 0
		switch {
		case len(rr.New) > 0:
			rr.Tail = fmt.Sprintf("%d new violations of %s (fix them; only the human grows a ratchet):\n%s", len(rr.New), r.ID, strings.Join(first(rr.New, 20), "\n"))
		case rr.Fixed > 0:
			rr.Tail = fmt.Sprintf("%d violations of %s are fixed: shrink its ratchet with mull rules ratchet %s, and commit it", rr.Fixed, r.ID, r.ID)
		}
		out = append(out, rr)
	}
	return out
}

func first(xs []string, n int) []string {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

// ruleViolations runs a ratcheted check and returns the violations it
// prints, sorted and without repeats. Its exit status doesn't count: what
// counts is what it prints.
func ruleViolations(dir, check, id string) ([]string, error) {
	cmd := exec.Command("sh", "-c", check)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "MULL_RULE="+id)
	b, err := cmd.Output()
	if _, exited := err.(*exec.ExitError); err != nil && !exited {
		return nil, fmt.Errorf("%s: %v", check, err)
	}
	seen := map[string]bool{}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out, nil
}

// readRatchet reads a ratchet file's violations; none is an empty set.
func readRatchet(path string) map[string]bool {
	out := map[string]bool{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			out[l] = true
		}
	}
	return out
}

// RatchetResult is what mull rules ratchet did.
type RatchetResult struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Before int    `json:"before"`
	After  int    `json:"after"`
}

// Ratchet rewrites a rule's ratchet file to the violations its check finds
// now. It may only shrink the file: growing it, or starting one (the
// baseline), is the human's call (grow, with onRequest for an agent). The
// file is in the working tree; commit it.
func (m *Mull) Ratchet(id string, grow, onRequest bool) (RatchetResult, error) {
	rules, err := m.readRules()
	if err != nil {
		return RatchetResult{}, err
	}
	var rule *Rule
	for k := range rules {
		if rules[k].ID == id {
			rule = &rules[k]
		}
	}
	switch {
	case rule == nil:
		return RatchetResult{}, errf(CodeNotFound, "no rule %s in %s", id, RulesPath)
	case rule.Ratchet == "":
		return RatchetResult{}, errf(CodeInvalid, "rule %s has no ratchet (add '- ratchet: yes' to it)", id)
	}
	path := filepath.Join(m.repo.Dir, rule.Ratchet)
	known := readRatchet(path)
	found, err := ruleViolations(m.repo.Dir, rule.Check, rule.ID)
	if err != nil {
		return RatchetResult{}, err
	}
	var grown []string
	for _, v := range found {
		if !known[v] {
			grown = append(grown, v)
		}
	}
	_, exists := os.Stat(path)
	if (len(grown) > 0 || exists != nil) && !(grow && m.asked(onRequest)) {
		return RatchetResult{}, errf(CodeNotAllowed, "%d violations of %s aren't in its ratchet: fix them. Only the human grows a ratchet or sets its baseline (mull rules ratchet %s --grow --on-request)", len(grown), id, id)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return RatchetResult{}, err
	}
	body := fmt.Sprintf("# Known violations of %s, one per line. This list may only shrink (mull rules ratchet %s).\n", id, id)
	if len(found) > 0 {
		body += strings.Join(found, "\n") + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return RatchetResult{}, err
	}
	return RatchetResult{ID: id, Path: rule.Ratchet, Before: len(known), After: len(found)}, nil
}

// RuleCheckOut is mull rules check: each checked rule's result.
type RuleCheckOut struct {
	Passed  bool               `json:"passed"`
	Results []model.RuleResult `json:"results"`
}

// CheckRules runs the rulebook's checks in this worktree now.
func (m *Mull) CheckRules() (RuleCheckOut, error) {
	rules, err := m.readRules()
	if err != nil {
		return RuleCheckOut{}, err
	}
	out := RuleCheckOut{Passed: true, Results: m.runRules(rules, nil)}
	if out.Results == nil {
		out.Results = []model.RuleResult{}
	}
	for _, r := range out.Results {
		out.Passed = out.Passed && r.Passed
	}
	return out, nil
}

// grownRatchets lists the ratcheted rules whose ratchet file gained a
// violation between base and the worktree: hiding a new violation by
// editing the file, which only the human may do. Shrinking one is fine.
func (m *Mull) grownRatchets(rules []Rule, base string) []string {
	var out []string
	for _, r := range rules {
		if r.Ratchet == "" {
			continue
		}
		before := map[string]bool{}
		if b, code, err := m.repo.RunStatus(nil, nil, "show", base+":"+r.Ratchet); err == nil && code == 0 {
			for _, l := range strings.Split(string(b), "\n") {
				if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
					before[l] = true
				}
			}
		} else {
			continue // a new ratchet: RULES.md changed too, which is high risk already
		}
		for v := range readRatchet(filepath.Join(m.repo.Dir, r.Ratchet)) {
			if !before[v] {
				out = append(out, r.ID)
				break
			}
		}
	}
	return out
}

// rulesInScope lists the rules whose scope matches a changed file.
func rulesInScope(rules []Rule, changes []model.LayerChanges) []string {
	var out []string
	for _, r := range rules {
	files:
		for _, lc := range changes {
			for _, f := range lc.Files {
				if matchGlob(r.Scope, f.Path) != "" {
					out = append(out, r.ID)
					break files
				}
			}
		}
	}
	return out
}
