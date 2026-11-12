package guide

import (
	"regexp"
	"strings"
)

// theHuman is "the human" or "the human's" as mull's own text says it, not
// part of a longer word or a hyphened name ("the human-feedback rule").
var theHuman = regexp.MustCompile(`\b([Tt])he human('s)?([^\w-]|$)`)

// Personal returns text with "the human" put as name, when this machine
// names its human (mull name <name>). Only mull's own words go through it
// (the session protocol, messages), never what's stored, and never the
// guides, which are copied into shared files.
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
