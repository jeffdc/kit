// Package model defines mull's records and their file format: YAML
// frontmatter followed by a markdown body.
package model

import (
	"bytes"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Item stages: captured → committed → active → in review → done, or
// dropped from anywhere. A done item is a reviewed item.
const (
	StateCaptured  = "captured"  // in the backlog, not committed to
	StateCommitted = "committed" // committed work, not yet started
	StateActive    = "active"
	StateInReview  = "in-review" // finished through mull done, waiting for review
	StateDone      = "done"      // reviewed and accepted
	StateDropped   = "dropped"
)

// Kinds of captured item.
const (
	ItemBug  = "bug"
	ItemIdea = "idea"
)

// NoPlan, given as a plan, means "no plan": unplanned work, or backlog not
// tagged to a plan.
const NoPlan = "none"

// Stages lists every item stage, in order.
var Stages = []string{StateCaptured, StateCommitted, StateActive, StateInReview, StateDone, StateDropped}

// Format is the storage format this mull writes, recorded as format in
// config.yml. Format 1 (a config with no format) is the layout from before
// Model v2: phases under epics/ and an item's phase under epic:, item stages
// under state: with the old names (open, done, accepted for committed, in
// review, done), and a done plan stored as closed. Format 2 is Model v2's:
// phases under phases/, an item's phase under phase: and its stage under
// stage:, and a done plan stored as done. mull migrate converts a repository
// from 1 to 2.
//
// Every record shows its own format (a format 1 item has a state: key, a
// format 2 item a stage: key), so a stored "done" is never ambiguous.
// Format 1 records are still read correctly, and marked Legacy, but mull
// writes only format 2.
const Format = 2

// legacyState maps a format 1 item stage to its Model v2 name.
var legacyState = map[string]string{
	"open":     StateCommitted,
	"done":     StateInReview,
	"accepted": StateDone,
}

// LegacyStage returns the Model v2 name of an item stage stored in format 1.
func LegacyStage(stored string) string {
	if st, ok := legacyState[stored]; ok {
		return st
	}
	return stored
}

// hasKey reports whether a YAML mapping node has key.
func hasKey(node *yaml.Node, key string) bool {
	if node.Kind != yaml.MappingNode {
		return false
	}
	for k := 0; k+1 < len(node.Content); k += 2 {
		if node.Content[k].Value == key {
			return true
		}
	}
	return false
}

// Plan states: shaping → active → done. A new plan is shaping until the
// human agrees it; agreeing makes it active, and closing it makes it done.
const (
	PlanShaping = "shaping" // being worked out with the human; not agreed yet
	PlanActive  = "active"  // agreed by the human
	PlanDone    = "done"
)

// PlanClosed is how format 1 stored a done plan.
const PlanClosed = "closed"

// Inbox entry kinds.
const (
	KindDecision = "decision"
	KindQuestion = "question"
	KindReview   = "review"
	// Loose ends and inbox ideas are retired: captured items (bug, idea)
	// replaced them. Existing ones are still read, listed with inbox --kind
	// and settled, until mull migrate converts them.
	KindLoose = "loose-end"
	KindIdea  = "idea"
)

// Inbox settle actions.
const (
	ActAccept = "accept"
	ActDecide = "decide"
	ActDo     = "do"
	ActDrop   = "drop"
	ActPark   = "park"
)

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
	// Doc is the plan's doc: the picture, done-when and why the order;
	// intent only, never status. It is its own file (PlanDocPath), not part
	// of the record.
	Doc string `yaml:"-" json:"doc,omitempty"`
	// Legacy means the record was read in format 1 (see Format).
	Legacy bool `yaml:"-" json:"-"`
}

// UnmarshalYAML reads a plan in either format: format 1's "closed" is done.
func (p *Plan) UnmarshalYAML(node *yaml.Node) error {
	type plain Plan
	var q plain
	if err := node.Decode(&q); err != nil {
		return err
	}
	if q.State == PlanClosed {
		q.State, q.Legacy = PlanDone, true
	}
	*p = Plan(q)
	return nil
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
	// Doc is the phase's doc: its design and done-when; intent only, never
	// status. It is its own file (PhaseDocPath), not part of the record.
	Doc string `yaml:"-" json:"doc,omitempty"`
	// Legacy means the record was read in format 1, from epics/ (see Format).
	Legacy bool `yaml:"-" json:"-"`
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

// Item is a unit of work one agent completes in one session. An item with
// no plan is unplanned work (or, captured, untagged backlog). Captured items
// carry a kind (bug or idea), and may carry a free-text category and a
// source; they keep them once committed. Triaged is when the item was last
// triaged with the human (mull triage); a captured item without it is
// untriaged. Needs are from before Model v2 only (items have no needs):
// they are read and shown, and mull migrate removes them.
type Item struct {
	ID       string   `yaml:"-" json:"id"`
	Plan     string   `yaml:"plan,omitempty" json:"plan,omitempty"`
	Phase    string   `yaml:"phase,omitempty" json:"phase,omitempty"`
	Title    string   `yaml:"title" json:"title"`
	State    string   `yaml:"stage" json:"state"`
	Kind     string   `yaml:"kind,omitempty" json:"kind,omitempty"`
	Category string   `yaml:"category,omitempty" json:"category,omitempty"`
	Source   string   `yaml:"source,omitempty" json:"source,omitempty"`
	Needs    []string `yaml:"needs,omitempty" json:"needs,omitempty"`
	Docs     []string `yaml:"docs,omitempty" json:"docs,omitempty"`
	Claim    *Claim   `yaml:"claim,omitempty" json:"claim,omitempty"`
	From     string   `yaml:"from,omitempty" json:"from,omitempty"` // the inbox entry it was made from
	Triaged  string   `yaml:"triaged,omitempty" json:"triaged,omitempty"`
	Created  string   `yaml:"created" json:"created"`
	Updated  string   `yaml:"updated" json:"updated"`
	Notes    string   `yaml:"-" json:"notes,omitempty"`
	// Legacy means the record was read in format 1 (see Format).
	Legacy bool `yaml:"-" json:"-"`
}

// UnmarshalYAML reads an item in either format. A format 1 item keeps its
// stage under state: with the old names, and its phase under epic:.
func (i *Item) UnmarshalYAML(node *yaml.Node) error {
	type plain Item
	var p plain
	if err := node.Decode(&p); err != nil {
		return err
	}
	if hasKey(node, "state") || hasKey(node, "epic") {
		var old struct {
			State string `yaml:"state"`
			Epic  string `yaml:"epic"`
		}
		if err := node.Decode(&old); err != nil {
			return err
		}
		if !hasKey(node, "stage") {
			p.State = LegacyStage(old.State)
		}
		if p.Phase == "" {
			p.Phase = old.Epic
		}
		p.Legacy = true
	}
	*i = Item(p)
	return nil
}

// Settlement records how an inbox entry was settled.
type Settlement struct {
	Action    string `yaml:"action" json:"action"`
	Note      string `yaml:"note,omitempty" json:"note,omitempty"`
	Item      string `yaml:"item,omitempty" json:"item,omitempty"`   // the item created by "do"
	Phase     string `yaml:"phase,omitempty" json:"phase,omitempty"` // the phase created by "do --as phase"
	By        string `yaml:"by" json:"by"`
	OnRequest bool   `yaml:"on_request,omitempty" json:"on_request,omitempty"`
	At        string `yaml:"at" json:"at"`
	// Legacy means it was read in format 1, with its phase under epic:.
	Legacy bool `yaml:"-" json:"-"`
}

// UnmarshalYAML reads a settlement in either format.
func (st *Settlement) UnmarshalYAML(node *yaml.Node) error {
	type plain Settlement
	var p plain
	if err := node.Decode(&p); err != nil {
		return err
	}
	if hasKey(node, "epic") {
		var old struct {
			Epic string `yaml:"epic"`
		}
		if err := node.Decode(&old); err != nil {
			return err
		}
		if p.Phase == "" {
			p.Phase = old.Epic
		}
		p.Legacy = true
	}
	*st = Settlement(p)
	return nil
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
	// Decisions, on a review, are the decision entries it carries: the
	// item's open decisions when mull done filed it. Accepting the review
	// ratifies those still unsettled.
	Decisions []string `yaml:"decisions,omitempty" json:"decisions,omitempty"`
	Detail    string   `yaml:"-" json:"detail,omitempty"`
}

// Config is per-repository configuration.
type Config struct {
	// Format is the storage format (see Format). A config without one is
	// format 1; mull reports it as 1.
	Format      int                 `yaml:"format,omitempty" json:"format"`
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

// ReportDecision is a decision made in the session, as the done report
// records it.
type ReportDecision struct {
	ID     string `yaml:"id" json:"id"`
	Text   string `yaml:"text" json:"text"`
	Detail string `yaml:"detail,omitempty" json:"detail,omitempty"`
}

// Report is a done report: the record of finishing (or failing to finish) an
// item. What was built: summary, tour, checks. How: design notes, the
// decisions made in the session, changes by layer.
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
	// Design is how it was built and why. Reports made before mull done
	// required it have none.
	Design    string           `yaml:"design,omitempty" json:"design,omitempty"`
	Decisions []ReportDecision `yaml:"decisions,omitempty" json:"decisions,omitempty"`
}

// Paths of records on the mull branch.
func PlanPath(slug string) string { return "plans/" + slug + ".md" }
func PhasePath(id string) string  { return "phases/" + id + ".md" }
func ItemPath(id string) string   { return "items/" + id + ".md" }
func EntryPath(id string) string  { return "inbox/" + id + ".md" }
func ReportPath(item string, n int) string {
	return fmt.Sprintf("reports/%s-%d.md", item, n)
}
func CheckPath(tree string) string { return "checks/" + tree + ".json" }

// LegacyPhasePath is where format 1 kept a phase, and LegacyPhaseDocPath its
// doc.
func LegacyPhasePath(id string) string    { return "epics/" + id + ".md" }
func LegacyPhaseDocPath(id string) string { return "docs/" + LegacyPhasePath(id) }

// Docs are plain markdown files, one per plan or phase, kept apart from the
// records so a doc's history holds only its own versions. A doc's path is
// "docs/" plus its record's path.
func PlanDocPath(slug string) string { return "docs/" + PlanPath(slug) }
func PhaseDocPath(id string) string  { return "docs/" + PhasePath(id) }

const ConfigPath = "config.yml"

// dataPaths are the paths records may occupy, in either format: a migrated
// branch's history still holds format 1's epics/ and docs/epics/.
var dataPaths = []*regexp.Regexp{
	regexp.MustCompile(`^config\.yml$`),
	regexp.MustCompile(`^(docs/)?plans/[a-z][a-z0-9]*(-[a-z0-9]+)*\.md$`),
	regexp.MustCompile(`^(phases|epics|items|inbox)/[0-9a-f]{4}\.md$`),
	regexp.MustCompile(`^docs/(phases|epics)/[0-9a-f]{4}\.md$`),
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
