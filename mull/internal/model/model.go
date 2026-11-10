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
	// KindIdea is not work: it needs no sizing, is never ready, and stays
	// out of the inbox until the human promotes it.
	KindIdea = "idea"
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

// IsID reports whether s looks like a phase, item or inbox ID.
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

// Phase is a big piece of a plan: a set of items.
type Phase struct {
	ID      string   `yaml:"-" json:"id"`
	Plan    string   `yaml:"plan" json:"plan"`
	Title   string   `yaml:"title" json:"title"`
	Needs   []string `yaml:"needs,omitempty" json:"needs,omitempty"`
	From    string   `yaml:"from,omitempty" json:"from,omitempty"` // the inbox entry (e.g. an idea) it was promoted from
	Created string   `yaml:"created" json:"created"`
	Updated string   `yaml:"updated" json:"updated"`
	Summary string   `yaml:"-" json:"summary,omitempty"`
}

// Claim records which worktree is working on an item.
type Claim struct {
	Worktree string `yaml:"worktree" json:"worktree"`
	Branch   string `yaml:"branch" json:"branch"`
	At       string `yaml:"at" json:"at"`
	// Base is the commit checked out when the item was claimed; a done
	// report's changes are measured from it.
	Base string `yaml:"base,omitempty" json:"base,omitempty"`
}

// Item is a unit of work one agent completes in one session.
type Item struct {
	ID      string   `yaml:"-" json:"id"`
	Plan    string   `yaml:"plan" json:"plan"`
	Phase   string   `yaml:"epic,omitempty" json:"phase,omitempty"`
	Title   string   `yaml:"title" json:"title"`
	State   string   `yaml:"state" json:"state"`
	Needs   []string `yaml:"needs,omitempty" json:"needs,omitempty"`
	Docs    []string `yaml:"docs,omitempty" json:"docs,omitempty"`
	Claim   *Claim   `yaml:"claim,omitempty" json:"claim,omitempty"`
	From    string   `yaml:"from,omitempty" json:"from,omitempty"` // the inbox entry it was made from
	Created string   `yaml:"created" json:"created"`
	Updated string   `yaml:"updated" json:"updated"`
	Notes   string   `yaml:"-" json:"notes,omitempty"`
}

// Settlement records how an inbox entry was settled.
type Settlement struct {
	Action    string `yaml:"action" json:"action"`
	Note      string `yaml:"note,omitempty" json:"note,omitempty"`
	Item      string `yaml:"item,omitempty" json:"item,omitempty"`  // the item created by "do"
	Phase     string `yaml:"epic,omitempty" json:"phase,omitempty"` // the phase created by "do --as phase"
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

// CheckResult is one hard-check command's outcome.
type CheckResult struct {
	Command    string `yaml:"command" json:"command"`
	Exit       int    `yaml:"exit" json:"exit"`
	DurationMS int64  `yaml:"duration_ms" json:"duration_ms"`
	Tail       string `yaml:"tail,omitempty" json:"tail,omitempty"` // last lines of output, kept for failures
}

// CheckRun is a run of every hard check against one tree.
type CheckRun struct {
	Tree    string        `yaml:"tree" json:"tree"`
	Commit  string        `yaml:"commit" json:"commit"`
	At      string        `yaml:"at" json:"at"`
	By      string        `yaml:"by" json:"by"`
	Passed  bool          `yaml:"passed" json:"passed"`
	Results []CheckResult `yaml:"results" json:"results"`
}

// FileChange is one changed file in a done report.
type FileChange struct {
	Status  string `yaml:"status" json:"status"` // A, M, D, R
	Path    string `yaml:"path" json:"path"`
	Added   int    `yaml:"added" json:"added"`
	Removed int    `yaml:"removed" json:"removed"`
}

// LayerChanges groups changed files under one configured layer.
type LayerChanges struct {
	Layer string       `yaml:"layer" json:"layer"`
	Files []FileChange `yaml:"files" json:"files"`
}

// Report is a done report: the record of finishing (or failing to finish) an item.
type Report struct {
	Item     string         `yaml:"item" json:"item"`
	N        int            `yaml:"n" json:"n"`
	Passed   bool           `yaml:"passed" json:"passed"`
	Override string         `yaml:"override,omitempty" json:"override,omitempty"`
	Commit   string         `yaml:"commit" json:"commit"`
	Base     string         `yaml:"base,omitempty" json:"base,omitempty"`
	At       string         `yaml:"at" json:"at"`
	By       string         `yaml:"by" json:"by"`
	Summary  string         `yaml:"summary" json:"summary"`
	Tour     []string       `yaml:"tour,omitempty" json:"tour,omitempty"`
	Checks   CheckRun       `yaml:"checks" json:"checks"`
	Changes  []LayerChanges `yaml:"changes,omitempty" json:"changes,omitempty"`
}

// Paths of records on the mull branch.
func PlanPath(slug string) string { return "plans/" + slug + ".md" }
func PhasePath(id string) string  { return "epics/" + id + ".md" }
func ItemPath(id string) string   { return "items/" + id + ".md" }
func EntryPath(id string) string  { return "inbox/" + id + ".md" }
func ReportPath(item string, n int) string {
	return fmt.Sprintf("reports/%s-%d.md", item, n)
}
func CheckPath(tree string) string { return "checks/" + tree + ".json" }

const ConfigPath = "config.yml"

var dataPaths = []*regexp.Regexp{
	regexp.MustCompile(`^config\.yml$`),
	regexp.MustCompile(`^plans/[a-z][a-z0-9]*(-[a-z0-9]+)*\.md$`),
	regexp.MustCompile(`^(epics|items|inbox)/[0-9a-f]{4}\.md$`),
	regexp.MustCompile(`^reports/[0-9a-f]{4}-[0-9]+\.md$`),
	regexp.MustCompile(`^checks/[0-9a-f]{40}([0-9a-f]{24})?\.json$`),
}

// ValidDataPath reports whether path is one mull's records may occupy on the
// mull branch. Sync refuses to push a commit touching any other path.
func ValidDataPath(path string) bool {
	for _, re := range dataPaths {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

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
