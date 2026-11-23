package core

import (
	"bytes"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Records live on the mull branch as markdown files with YAML frontmatter,
// in the layout mull has used since format 2: fields the core no longer
// knows are ignored on reading and dropped when a record is written again.

// Format is the storage format the core reads and writes.
const Format = 2

// Item stages: queued (captured) → committed → active → in review → done,
// or dropped.
const (
	Captured  = "captured"
	Committed = "committed"
	Active    = "active"
	InReview  = "in-review"
	Done      = "done"
	Dropped   = "dropped"
)

// Kinds of queued item, of inbox entry, and settle actions.
const (
	Bug  = "bug"
	Idea = "idea"

	KindQuestion = "question"
	KindReview   = "review"
	KindDecision = "decision"

	ActAccept = "accept"
	ActDecide = "decide"
	ActDo     = "do"
	ActDrop   = "drop"
	ActNote   = "note"
)

// Plan states.
const (
	Shaping    = "shaping"
	PlanActive = "active"
	PlanDone   = "done"
)

// Risk bands.
const (
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
)

// NoPlan, given as a plan, means unplanned.
const NoPlan = "none"

// Plan is a large effort: its doc holds the goal, appetite, rules it adds
// and its phases' done-criteria.
type Plan struct {
	Slug    string `yaml:"-" json:"slug"`
	Title   string `yaml:"title" json:"title"`
	State   string `yaml:"state" json:"state"`
	Target  string `yaml:"target,omitempty" json:"target,omitempty"`
	Preview string `yaml:"preview,omitempty" json:"preview,omitempty"`
	Created string `yaml:"created" json:"created"`
	Updated string `yaml:"updated" json:"updated"`
	Summary string `yaml:"-" json:"summary,omitempty"`
	Doc     string `yaml:"-" json:"doc,omitempty"`
}

// Phase is a big piece of a plan.
type Phase struct {
	ID      string   `yaml:"-" json:"id"`
	Plan    string   `yaml:"plan" json:"plan"`
	Title   string   `yaml:"title" json:"title"`
	Needs   []string `yaml:"needs,omitempty" json:"needs,omitempty"`
	Created string   `yaml:"created" json:"created"`
	Updated string   `yaml:"updated" json:"updated"`
	Summary string   `yaml:"-" json:"summary,omitempty"`
	Doc     string   `yaml:"-" json:"doc,omitempty"`
}

// Claim records where an item is being worked.
type Claim struct {
	Worktree string `yaml:"worktree" json:"worktree"`
	Branch   string `yaml:"branch" json:"branch"`
	At       string `yaml:"at" json:"at"`
	Base     string `yaml:"base,omitempty" json:"base,omitempty"`
}

// Item is one session's work, or (captured) queued work.
type Item struct {
	ID        string `yaml:"-" json:"id"`
	Plan      string `yaml:"plan,omitempty" json:"plan,omitempty"`
	Phase     string `yaml:"phase,omitempty" json:"phase,omitempty"`
	Title     string `yaml:"title" json:"title"`
	State     string `yaml:"stage" json:"state"`
	Kind      string `yaml:"kind,omitempty" json:"kind,omitempty"`
	Area      string `yaml:"area,omitempty" json:"area,omitempty"`
	Source    string `yaml:"source,omitempty" json:"source,omitempty"`
	CoveredBy string `yaml:"covered_by,omitempty" json:"covered_by,omitempty"`
	From      string `yaml:"from,omitempty" json:"from,omitempty"`
	Claim     *Claim `yaml:"claim,omitempty" json:"claim,omitempty"`
	Created   string `yaml:"created" json:"created"`
	Updated   string `yaml:"updated" json:"updated"`
	Notes     string `yaml:"-" json:"notes,omitempty"`
}

// Settlement is how an inbox entry was settled.
type Settlement struct {
	Action    string `yaml:"action" json:"action"`
	Note      string `yaml:"note,omitempty" json:"note,omitempty"`
	Item      string `yaml:"item,omitempty" json:"item,omitempty"`
	By        string `yaml:"by" json:"by"`
	Human     bool   `yaml:"human,omitempty" json:"human,omitempty"`
	OnRequest bool   `yaml:"on_request,omitempty" json:"on_request,omitempty"`
	At        string `yaml:"at" json:"at"`
}

// Entry is a question for the human, a review of work, or a decision
// (noted by an agent, or the human's own).
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
	// Landed, on a review, means the work landed already (medium risk, or
	// sampled): a look, not a gate. Default and Due, on a question, are the
	// agent's default and when it stands.
	Landed  string `yaml:"landed,omitempty" json:"landed,omitempty"`
	Default string `yaml:"default,omitempty" json:"default,omitempty"`
	Due     string `yaml:"due,omitempty" json:"due,omitempty"`
	Detail  string `yaml:"-" json:"detail,omitempty"`
}

// CheckResult is one check command's outcome.
type CheckResult struct {
	Command    string `yaml:"command" json:"command"`
	Exit       int    `yaml:"exit" json:"exit"`
	DurationMS int64  `yaml:"duration_ms" json:"duration_ms"`
	Tail       string `yaml:"tail,omitempty" json:"tail,omitempty"`
}

// CheckRun is a run of the hard checks against one tree.
type CheckRun struct {
	Tree    string        `yaml:"tree" json:"tree"`
	Commit  string        `yaml:"commit" json:"commit"`
	At      string        `yaml:"at" json:"at"`
	Passed  bool          `yaml:"passed" json:"passed"`
	Results []CheckResult `yaml:"results" json:"results"`
}

// RuleResult is one rule's check as done ran it.
type RuleResult struct {
	ID     string   `yaml:"id" json:"id"`
	Passed bool     `yaml:"passed" json:"passed"`
	Count  int      `yaml:"count,omitempty" json:"count,omitempty"`
	New    []string `yaml:"new,omitempty" json:"new,omitempty"`
	Fixed  int      `yaml:"fixed,omitempty" json:"fixed,omitempty"`
	Tail   string   `yaml:"tail,omitempty" json:"tail,omitempty"`
}

// FileChange is one changed file.
type FileChange struct {
	Status  string `yaml:"status" json:"status"`
	Path    string `yaml:"path" json:"path"`
	Added   int    `yaml:"added" json:"added"`
	Removed int    `yaml:"removed" json:"removed"`
}

// Report is a done report.
type Report struct {
	Item       string       `yaml:"item" json:"item"`
	N          int          `yaml:"n" json:"n"`
	Passed     bool         `yaml:"passed" json:"passed"`
	Commit     string       `yaml:"commit" json:"commit"`
	Base       string       `yaml:"base,omitempty" json:"base,omitempty"`
	At         string       `yaml:"at" json:"at"`
	By         string       `yaml:"by" json:"by"`
	Summary    string       `yaml:"summary" json:"summary"`
	Rules      string       `yaml:"rules,omitempty" json:"rules,omitempty"`
	Slowed     string       `yaml:"slowed,omitempty" json:"slowed,omitempty"`
	Look       []string     `yaml:"tour,omitempty" json:"look,omitempty"`
	Design     string       `yaml:"design,omitempty" json:"design,omitempty"`
	Risk       string       `yaml:"risk,omitempty" json:"risk,omitempty"`
	RiskWhy    []string     `yaml:"risk_why,omitempty" json:"risk_why,omitempty"`
	Sampled    bool         `yaml:"sampled,omitempty" json:"sampled,omitempty"`
	Checks     CheckRun     `yaml:"checks" json:"checks"`
	RuleChecks []RuleResult `yaml:"rule_checks,omitempty" json:"rule_checks,omitempty"`
	Files      []FileChange `yaml:"files,omitempty" json:"files,omitempty"`
	Decisions  []string     `yaml:"noted,omitempty" json:"decisions,omitempty"` // older reports kept decisions as records under "decisions"
	Covered    []string     `yaml:"covered,omitempty" json:"covered,omitempty"`
	Nearby     []string     `yaml:"nearby,omitempty" json:"nearby,omitempty"`
	NewEdges   []string     `yaml:"new_edges,omitempty" json:"new_edges,omitempty"`
	Text       string       `yaml:"-" json:"text"`
}

// Config is per repository.
type Config struct {
	Format      int        `yaml:"format,omitempty" json:"format"`
	Integration string     `yaml:"integration" json:"integration"`
	Release     string     `yaml:"release" json:"release"`
	Remote      string     `yaml:"remote" json:"remote"`
	Checks      []string   `yaml:"done_checks" json:"checks"`
	Risk        RiskConfig `yaml:"risk,omitempty" json:"risk"`
	Worktrees   string     `yaml:"worktrees,omitempty" json:"worktrees,omitempty"`
	Setup       []string   `yaml:"setup,omitempty" json:"setup,omitempty"`
	Parallel    int        `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	Map         MapConfig  `yaml:"map,omitempty" json:"map"`
}

// RiskConfig is how done scores risk; unset fields take the defaults.
type RiskConfig struct {
	High     []string `yaml:"high,omitempty" json:"high"`
	MaxLines int      `yaml:"max_lines,omitempty" json:"max_lines"`
	MaxFiles int      `yaml:"max_files,omitempty" json:"max_files"`
	Sample   int      `yaml:"sample,omitempty" json:"sample"` // percent; negative samples nothing
}

// MapConfig names the commands that print the code's connections, one
// "from -> to" per line: the ones it has (Edges) and the declared ones.
type MapConfig struct {
	Edges   string `yaml:"edges,omitempty" json:"edges,omitempty"`
	Allowed string `yaml:"allowed,omitempty" json:"allowed,omitempty"`
}

// MapSnapshot is the map as last recorded.
type MapSnapshot struct {
	Commit string   `json:"commit"`
	At     string   `json:"at"`
	Edges  []string `json:"edges"`
}

// Defaults.
var DefaultHighRisk = []string{
	"RULES.md", ".github/**", ".githooks/**", "Makefile", "**/Makefile",
	"go.mod", "**/go.mod", "mix.exs", "**/mix.exs", "mix.lock", "**/mix.lock",
	"package.json", "**/package.json", "**/migrations/**", "**/auth/**",
}

const (
	DefaultMaxLines = 400
	DefaultMaxFiles = 15
	DefaultSample   = 20
	DefaultParallel = 3
	DefaultStanding = "next"
)

// Effective is rc with unset fields filled in.
func (rc RiskConfig) Effective() RiskConfig {
	if rc.High == nil {
		rc.High = append([]string{}, DefaultHighRisk...)
	}
	if rc.MaxLines == 0 {
		rc.MaxLines = DefaultMaxLines
	}
	if rc.MaxFiles == 0 {
		rc.MaxFiles = DefaultMaxFiles
	}
	switch {
	case rc.Sample == 0:
		rc.Sample = DefaultSample
	case rc.Sample < 0:
		rc.Sample = 0
	}
	return rc
}

// Paths on the mull branch.
func planPath(slug string) string    { return "plans/" + slug + ".md" }
func phasePath(id string) string     { return "phases/" + id + ".md" }
func itemPath(id string) string      { return "items/" + id + ".md" }
func entryPath(id string) string     { return "inbox/" + id + ".md" }
func planDocPath(slug string) string { return "docs/" + planPath(slug) }
func phaseDocPath(id string) string  { return "docs/" + phasePath(id) }
func reportPath(item string, n int) string {
	return fmt.Sprintf("reports/%s-%d.md", item, n)
}
func checkPath(tree string) string { return "checks/" + tree + ".json" }

const (
	configPath = "config.yml"
	mapPath    = "map.json"
	auditPath  = "audit.md"
)

// dataPaths are the paths mull's records may occupy, old layouts included,
// since the branch's history holds them.
var dataPaths = []*regexp.Regexp{
	regexp.MustCompile(`^config\.yml$`),
	regexp.MustCompile(`^(docs/)?plans/[a-z][a-z0-9]*(-[a-z0-9]+)*\.md$`),
	regexp.MustCompile(`^(phases|epics|items|inbox)/[0-9a-f]{4}\.md$`),
	regexp.MustCompile(`^docs/(phases|epics)/[0-9a-f]{4}\.md$`),
	regexp.MustCompile(`^rules/(plans/[a-z][a-z0-9]*(-[a-z0-9]+)*|phases/[0-9a-f]{4})\.md$`),
	regexp.MustCompile(`^reports/[0-9a-f]{4}-[0-9]+\.md$`),
	regexp.MustCompile(`^checks/[0-9a-f]{40}([0-9a-f]{24})?\.json$`),
	regexp.MustCompile(`^(map\.json|audit\.md)$`),
}

// ValidDataPath reports whether path is one mull's records may occupy.
func ValidDataPath(path string) bool {
	for _, re := range dataPaths {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

var (
	idPattern   = regexp.MustCompile(`^[0-9a-f]{4}$`)
	slugPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
)

// IsID reports whether s looks like a record ID.
func IsID(s string) bool { return idPattern.MatchString(s) }

func validSlug(s string) bool { return slugPattern.MatchString(s) && !IsID(s) }

// encode writes frontmatter v followed by body.
func encode(v any, body string) ([]byte, error) {
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
		buf.WriteString("\n" + body)
		if body[len(body)-1] != '\n' {
			buf.WriteString("\n")
		}
	}
	return buf.Bytes(), nil
}

// decode reads frontmatter into v and returns the body.
func decode(data []byte, v any) (string, error) {
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
	return string(bytes.TrimRight(bytes.TrimPrefix(body, []byte("\n")), "\n")), nil
}
