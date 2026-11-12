package guide

import (
	"regexp"
	"strings"
)

// theHuman is "the human" or "the human's" as mull's own text says it, not
// part of a longer word or a hyphened name ("the human-feedback rule").
var theHuman = regexp.MustCompile(`\b([Tt])he human('s)?([^\w-]|$)`)

// Personal returns text with "the human" put as name, when the repository
// names its human (mull config set human <name>). Only mull's own words go
// through it (guides, the protocol, messages), never what's stored.
func Personal(text, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return text
	}
	return theHuman.ReplaceAllStringFunc(text, func(m string) string {
		sub := theHuman.FindStringSubmatch(m)
		return name + sub[2] + sub[3]
	})
}

// NameRule is the instruction that goes with a named human: agents write
// the name, not "the human", in what they write to mull. "" with no name.
func NameRule(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return "The human here is " + name + ". In everything you write to mull (titles, notes, details, decisions, questions, reports), write " +
		name + ", not \"the human\"."
}

// For returns the named guide in human's name (see Personal). The agents
// guide also gets NameRule, after its opening paragraph.
func For(guideName, human string) (string, error) {
	g, err := Get(guideName)
	if err != nil || strings.TrimSpace(human) == "" {
		return g, err
	}
	g = Personal(g, human)
	if guideName == "agents" {
		// The heading, then the opening paragraph, then the rule.
		if i := strings.Index(g, "\n\n"); i >= 0 {
			if j := strings.Index(g[i+2:], "\n\n"); j >= 0 {
				at := i + 2 + j + 2
				g = g[:at] + NameRule(human) + "\n\n" + g[at:]
			}
		}
	}
	return g, nil
}
