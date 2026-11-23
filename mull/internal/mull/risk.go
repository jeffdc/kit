package mull

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"

	"mull/internal/model"
)

// Risk is done's score for a finished item, from mechanical signals only:
// the paths its diff touches, how big the diff is, and whether the agent
// says it touched a rule. Never from what the agent writes about its work.
type Risk struct {
	Band    string   `json:"band"`
	Why     []string `json:"why,omitempty"`
	Sampled bool     `json:"sampled,omitempty"`
}

// scoreRisk scores changes against rc (already Effective). rules is the
// agent's rule-contact line; anything but "none" makes the work high risk.
// The sample draw is fixed by the item and its commit, so the same work
// always draws the same.
func scoreRisk(rc model.RiskConfig, changes []model.LayerChanges, rules, item, commit string, newEdges []string) Risk {
	var why []string
	if !noRules(rules) {
		why = append(why, "the work touched a rule: "+strings.TrimSpace(rules))
	}
	for _, e := range newEdges {
		why = append(why, "a new connection between parts: "+e)
	}
	files, lines := 0, 0
	var high []string
	for _, lc := range changes {
		for _, f := range lc.Files {
			files++
			lines += f.Added + f.Removed
			if g := matchGlob(rc.High, f.Path); g != "" {
				high = append(high, fmt.Sprintf("%s (high-risk: %s)", f.Path, g))
			}
		}
	}
	why = append(why, high...)
	if len(why) > 0 {
		return Risk{Band: model.RiskHigh, Why: why}
	}
	if lines > rc.MaxLines {
		why = append(why, fmt.Sprintf("%d lines changed (over %d)", lines, rc.MaxLines))
	}
	if files > rc.MaxFiles {
		why = append(why, fmt.Sprintf("%d files changed (over %d)", files, rc.MaxFiles))
	}
	if len(why) > 0 {
		return Risk{Band: model.RiskMedium, Why: why}
	}
	return Risk{Band: model.RiskLow, Sampled: drawn(item+commit, rc.Sample)}
}

// noRules reports whether the agent's rule-contact line says no rule was
// touched.
func noRules(rules string) bool {
	r := strings.ToLower(strings.TrimRight(strings.TrimSpace(rules), "."))
	return r == "none" || r == "no"
}

// matchGlob returns the first of globs that path matches, or "".
func matchGlob(globs []string, path string) string {
	for _, g := range globs {
		if re, err := globRegexp(g); err == nil && re.MatchString(path) {
			return g
		}
	}
	return ""
}

// drawn reports whether key falls in the first percent of 100 buckets.
func drawn(key string, percent int) bool {
	if percent <= 0 {
		return false
	}
	sum := sha256.Sum256([]byte(key))
	return int(binary.BigEndian.Uint64(sum[:8])%100) < percent
}
