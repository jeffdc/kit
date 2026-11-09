// Package model defines mull's records and their file format: YAML
// frontmatter followed by a markdown body.
package model

import (
	"bytes"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Item states.
const (
	StateOpen     = "open"
	StateActive   = "active"
	StateDone     = "done"
	StateAccepted = "accepted"
	StateDropped  = "dropped"
)

// Plan states.
const (
	PlanActive = "active"
	PlanClosed = "closed"
)

// Inbox entry kinds.
const (
	KindDecision = "decision"
	KindQuestion = "question"
	KindLoose    = "loose-end"
	KindReview   = "review"
)

// Inbox settle actions.
const (
	ActAccept = "accept"
	ActDecide = "decide"
	ActDo     = "do"
	ActDrop   = "drop"
	ActPark   = "park"
)

// GeneralPlan is the standing plan for work outside any big effort.
const GeneralPlan = "general"

var (
	idPattern   = regexp.MustCompile(`^[0-9a-f]{4}$`)
	slugPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
)

// IsID reports whether s looks like an epic, item or inbox ID.
func IsID(s string) bool { return idPattern.MatchString(s) }

// ValidSlug reports whether s can name a plan. Slugs that look like IDs are
// refused so a plan and a record can never be confused.
func ValidSlug(s string) bool { return slugPattern.MatchString(s) && !IsID(s) }

// Plan is a large effort. It holds only the high-level.
type Plan struct {
	Slug    string `yaml:"-" json:"slug"`
	Title   string `yaml:"title" json:"title"`
	State   string `yaml:"state" json:"state"`
	Created string `yaml:"created" json:"created"`
	Updated string `yaml:"updated" json:"updated"`
	Summary string `yaml:"-" json:"summary,omitempty"`
}

// Epic is a big piece of a plan: a set of items.
type Epic struct {
	ID      string   `yaml:"-" json:"id"`
	Plan    string   `yaml:"plan" json:"plan"`
	Title   string   `yaml:"title" json:"title"`
	Needs   []string `yaml:"needs,omitempty" json:"needs,omitempty"`
	Created string   `yaml:"created" json:"created"`
	Updated string   `yaml:"updated" json:"updated"`
	Summary string   `yaml:"-" json:"summary,omitempty"`
}

// Claim records which worktree is working on an item.
type Claim struct {
	Worktree string `yaml:"worktree" json:"worktree"`
	Branch   string `yaml:"branch" json:"branch"`
	At       string `yaml:"at" json:"at"`
}

// Item is a unit of work one agent completes in one session.
type Item struct {
	ID      string   `yaml:"-" json:"id"`
	Plan    string   `yaml:"plan" json:"plan"`
	Epic    string   `yaml:"epic,omitempty" json:"epic,omitempty"`
	Title   string   `yaml:"title" json:"title"`
	State   string   `yaml:"state" json:"state"`
	Needs   []string `yaml:"needs,omitempty" json:"needs,omitempty"`
	Docs    []string `yaml:"docs,omitempty" json:"docs,omitempty"`
	Claim   *Claim   `yaml:"claim,omitempty" json:"claim,omitempty"`
	Created string   `yaml:"created" json:"created"`
	Updated string   `yaml:"updated" json:"updated"`
	Notes   string   `yaml:"-" json:"notes,omitempty"`
}

// Settlement records how an inbox entry was settled.
type Settlement struct {
	Action    string `yaml:"action" json:"action"`
	Note      string `yaml:"note,omitempty" json:"note,omitempty"`
	Item      string `yaml:"item,omitempty" json:"item,omitempty"` // the item created by "do"
	By        string `yaml:"by" json:"by"`
	OnRequest bool   `yaml:"on_request,omitempty" json:"on_request,omitempty"`
	At        string `yaml:"at" json:"at"`
}

// Entry is an inbox entry: something that needs the human.
type Entry struct {
	ID      string      `yaml:"-" json:"id"`
	Kind    string      `yaml:"kind" json:"kind"`
	Text    string      `yaml:"text" json:"text"`
	Plan    string      `yaml:"plan" json:"plan"`
	Item    string      `yaml:"item,omitempty" json:"item,omitempty"`
	Source  string      `yaml:"source,omitempty" json:"source,omitempty"`
	By      string      `yaml:"by" json:"by"`
	Created string      `yaml:"created" json:"created"`
	Settled *Settlement `yaml:"settled,omitempty" json:"settled,omitempty"`
	Detail  string      `yaml:"-" json:"detail,omitempty"`
}

// Config is per-repository configuration.
type Config struct {
	Integration string              `yaml:"integration" json:"integration"`
	Release     string              `yaml:"release" json:"release"`
	Remote      string              `yaml:"remote" json:"remote"`
	DoneChecks  []string            `yaml:"done_checks" json:"done_checks"`
	Layers      map[string][]string `yaml:"layers" json:"layers"`
}

// Paths of records on the mull branch.
func PlanPath(slug string) string { return "plans/" + slug + ".md" }
func EpicPath(id string) string   { return "epics/" + id + ".md" }
func ItemPath(id string) string   { return "items/" + id + ".md" }
func EntryPath(id string) string  { return "inbox/" + id + ".md" }

const ConfigPath = "config.yml"

// Encode writes frontmatter v followed by body.
func Encode(v any, body string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	buf.WriteString("---\n")
	if body != "" {
		buf.WriteString("\n")
		buf.WriteString(body)
		if body[len(body)-1] != '\n' {
			buf.WriteString("\n")
		}
	}
	return buf.Bytes(), nil
}

// Decode reads frontmatter into v and returns the body.
func Decode(data []byte, v any) (string, error) {
	rest, ok := bytes.CutPrefix(data, []byte("---\n"))
	if !ok {
		return "", fmt.Errorf("missing frontmatter")
	}
	front, body, ok := bytes.Cut(rest, []byte("\n---\n"))
	if !ok {
		return "", fmt.Errorf("unterminated frontmatter")
	}
	if err := yaml.Unmarshal(front, v); err != nil {
		return "", err
	}
	body = bytes.TrimPrefix(body, []byte("\n"))
	return string(bytes.TrimRight(body, "\n")), nil
}
