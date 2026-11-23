// Package core is mull: the record of a program kept on the mull branch,
// and the runner that moves work through git. Plans hold phases, phases
// hold items; captured items are the agents' queue; the inbox holds what
// needs the human (questions with a default, and reviews of high-risk or
// landed work). Every write is one commit, made with a compare-and-swap.
package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"mull/internal/git"
	"mull/internal/guide"
	humanname "mull/internal/human"
	"mull/internal/store"
)

// Mull is mull opened in one worktree, whose path is the caller's identity
// unless it acts for the human.
type Mull struct {
	repo  *git.Repo
	store *store.Store
	human string    // set when the human acts themselves (mull serve)
	batch *state    // set inside a batch: writes go to its one transaction
	after *[]func() // inside a batch: git work to do once it commits
	// Now is the clock; tests replace it.
	Now      func() time.Time
	pushHook func(args []string) // tests only: the exact git push arguments
}

// Open opens mull in the git working tree containing dir.
func Open(dir string) (*Mull, error) {
	r, err := git.Open(dir)
	if err != nil {
		return nil, err
	}
	return &Mull{repo: r, store: store.New(r), Now: time.Now}, nil
}

// Worktree is this working tree's path: the caller's identity.
func (m *Mull) Worktree() string { return m.repo.Worktree() }

// MainWorktree is the repository's main working tree.
func (m *Mull) MainWorktree() string { return m.repo.MainWorktree() }

// AsHuman is m acting for the human, by their name (mull name, else git
// user.name): what it does needs no --on-request.
func (m *Mull) AsHuman() (*Mull, error) {
	name, err := humanname.Name()
	if err != nil {
		return nil, err
	}
	if name == "" {
		if out, code, err := m.repo.RunStatus(nil, nil, "config", "user.name"); err == nil && code == 0 {
			name = strings.TrimSpace(string(out))
		}
	}
	if name == "" {
		return nil, errf(CodeNotAllowed, "set a name with mull name: a write from mull serve is recorded by the human's name")
	}
	h := *m
	h.human = name
	return &h, nil
}

// asked reports whether the human is behind a step only they take.
func (m *Mull) asked(onRequest bool) bool { return onRequest || m.human != "" }

// by is who's writing: the human's name, or this worktree.
func (m *Mull) by() string {
	if m.human != "" {
		return m.human
	}
	return m.Worktree()
}

// Human is this machine's name for the human, or "".
func (m *Mull) Human() string {
	name, _ := humanname.Name()
	return name
}

// personal puts mull's own words in the human's name.
func (m *Mull) personal(text string) string { return guide.Personal(text, m.Human()) }

func (m *Mull) now() string { return m.Now().UTC().Format(time.RFC3339) }

// Init creates the mull branch with a config.
func (m *Mull) Init(integration, release string) (Config, error) {
	if release == "" {
		release = "main"
	}
	if integration == "" {
		integration = DefaultStanding
	}
	cfg := Config{Format: Format, Integration: integration, Release: release, Remote: "origin", Checks: []string{}}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return cfg, err
	}
	return cfg, m.store.Init("mull: init", map[string][]byte{configPath: b})
}

// state is every record, decoded.
type state struct {
	tx      *store.Tx
	config  Config
	plans   map[string]*Plan
	phases  map[string]*Phase
	items   map[string]*Item
	entries map[string]*Entry
}

func load(tx *store.Tx) (*state, error) {
	s := &state{tx: tx, plans: map[string]*Plan{}, phases: map[string]*Phase{}, items: map[string]*Item{}, entries: map[string]*Entry{}}
	b, ok := tx.Get(configPath)
	if !ok {
		return nil, store.ErrNotInitialized
	}
	if err := yaml.Unmarshal(b, &s.config); err != nil {
		return nil, fmt.Errorf("%s: %w", configPath, err)
	}
	switch {
	case s.config.Format < Format:
		return nil, errf(CodeUnsupportedFormat, "this repository's mull data is format %d, from before Model v2; read it with the old mull (mull1)", s.config.Format)
	case s.config.Format > Format:
		return nil, errf(CodeUnsupportedFormat, "this repository's mull data is format %d, newer than this mull: update mull", s.config.Format)
	}
	read := func(path string, v any) (string, error) {
		data, _ := tx.Get(path)
		body, err := decode(data, v)
		if err != nil {
			return "", fmt.Errorf("%s: %w", path, err)
		}
		return body, nil
	}
	doc := func(path string) string {
		b, _ := tx.Get(path)
		return strings.TrimSpace(string(b))
	}
	for _, p := range tx.List("plans/") {
		pl := &Plan{}
		body, err := read(p, pl)
		if err != nil {
			return nil, err
		}
		pl.Slug = strings.TrimSuffix(strings.TrimPrefix(p, "plans/"), ".md")
		pl.Summary, pl.Doc = body, doc(planDocPath(pl.Slug))
		s.plans[pl.Slug] = pl
	}
	for _, p := range tx.List("phases/") {
		ph := &Phase{}
		body, err := read(p, ph)
		if err != nil {
			return nil, err
		}
		ph.ID = strings.TrimSuffix(strings.TrimPrefix(p, "phases/"), ".md")
		ph.Summary, ph.Doc = body, doc(phaseDocPath(ph.ID))
		s.phases[ph.ID] = ph
	}
	for _, p := range tx.List("items/") {
		i := &Item{}
		body, err := read(p, i)
		if err != nil {
			return nil, err
		}
		i.ID, i.Notes = strings.TrimSuffix(strings.TrimPrefix(p, "items/"), ".md"), body
		s.items[i.ID] = i
	}
	for _, p := range tx.List("inbox/") {
		e := &Entry{}
		body, err := read(p, e)
		if err != nil {
			return nil, err
		}
		e.ID, e.Detail = strings.TrimSuffix(strings.TrimPrefix(p, "inbox/"), ".md"), body
		s.entries[e.ID] = e
	}
	return s, nil
}

func (m *Mull) view(fn func(s *state) error) error {
	if m.batch != nil {
		return fn(m.batch)
	}
	return m.store.View(func(tx *store.Tx) error {
		s, err := load(tx)
		if err != nil {
			return err
		}
		return fn(s)
	})
}

func (m *Mull) update(msg string, fn func(s *state) error) error {
	_, err := m.write(msg, fn)
	return err
}

// write is update, returning the commit written ("" when nothing changed).
// Inside a batch, it writes to the batch's transaction.
func (m *Mull) write(msg string, fn func(s *state) error) (string, error) {
	if m.batch != nil {
		return "", fn(m.batch)
	}
	return m.store.Write("mull: "+msg, func(tx *store.Tx) error {
		s, err := load(tx)
		if err != nil {
			return err
		}
		return fn(s)
	})
}

// Config is the repository's configuration.
func (m *Mull) Config() (Config, error) {
	var c Config
	err := m.view(func(s *state) error { c = s.config; return nil })
	return c, err
}

func (s *state) put(path string, v any, body string) error {
	b, err := encode(v, body)
	if err != nil {
		return err
	}
	s.tx.Put(path, b)
	return nil
}

func (s *state) putPlan(p *Plan) error {
	s.plans[p.Slug] = p
	return s.put(planPath(p.Slug), p, p.Summary)
}

func (s *state) putPhase(p *Phase) error {
	s.phases[p.ID] = p
	return s.put(phasePath(p.ID), p, p.Summary)
}

func (s *state) putItem(i *Item) error {
	s.items[i.ID] = i
	return s.put(itemPath(i.ID), i, i.Notes)
}

func (s *state) putEntry(e *Entry) error {
	s.entries[e.ID] = e
	return s.put(entryPath(e.ID), e, e.Detail)
}

func (s *state) putConfig() error {
	b, err := yaml.Marshal(s.config)
	if err != nil {
		return err
	}
	s.tx.Put(configPath, b)
	return nil
}

// newID is an unused 4-hex ID.
func (s *state) newID() (string, error) {
	for range 1000 {
		var b [2]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		id := hex.EncodeToString(b[:])
		if s.phases[id] == nil && s.items[id] == nil && s.entries[id] == nil {
			return id, nil
		}
	}
	return "", errors.New("no free ID")
}

func (s *state) plan(slug string) (*Plan, error) {
	if p := s.plans[slug]; p != nil {
		return p, nil
	}
	return nil, errf(CodeNotFound, "no plan %q", slug)
}

func (s *state) phase(id string) (*Phase, error) {
	if p := s.phases[id]; p != nil {
		return p, nil
	}
	return nil, errf(CodeNotFound, "no phase %q", id)
}

func (s *state) item(id string) (*Item, error) {
	if i := s.items[id]; i != nil {
		return i, nil
	}
	return nil, errf(CodeNotFound, "no item %q", id)
}

func (s *state) entry(id string) (*Entry, error) {
	if e := s.entries[id]; e != nil {
		return e, nil
	}
	return nil, errf(CodeNotFound, "no inbox entry %q", id)
}

// release is where work ships and mull never lands work; integration is
// where unplanned work lands: config's, unless that's the release branch,
// then the standing branch.
func (s *state) release() string {
	if s.config.Release != "" {
		return s.config.Release
	}
	return "main"
}

func (s *state) integration() string {
	b := s.config.Integration
	if b == "" || b == s.release() {
		return DefaultStanding
	}
	return b
}

// targetOf is the branch a plan's work lands on.
func (s *state) targetOf(slug string) string {
	if p := s.plans[slug]; p != nil && p.Target != "" {
		return p.Target
	}
	return s.integration()
}

func (s *state) parallel() int {
	if s.config.Parallel > 0 {
		return s.config.Parallel
	}
	return DefaultParallel
}

func sortedKeys[V any](mp map[string]V) []string {
	keys := make([]string, 0, len(mp))
	for k := range mp {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func dedupe(ids []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func parseTime(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	return t, err == nil
}

func joinNotes(a, b string) string {
	if a == "" {
		return b
	}
	return a + "\n\n" + b
}

// ---- Errors ----

// Error codes are a stable contract agents branch on.
const (
	CodeUsage              = "usage"
	CodeInvalid            = "invalid_argument"
	CodeNotRepo            = "not_a_repo"
	CodeNotInitialized     = "not_initialized"
	CodeAlreadyInitialized = "already_initialized"
	CodeNotFound           = "not_found"
	CodeExists             = "already_exists"
	CodeState              = "invalid_state"
	CodeClaimed            = "claimed"
	CodeSettled            = "already_settled"
	CodeNotAllowed         = "not_allowed"
	CodeCycle              = "dependency_cycle"
	CodeRulesRequired      = "rules_required"
	CodeAtCapacity         = "at_capacity"
	CodeSetupFailed        = "setup_failed"
	CodeBehindTarget       = "behind_target"
	CodeNoChecks           = "no_checks"
	CodeDirty              = "dirty_tree"
	CodeChecksFailed       = "checks_failed"
	CodeSyncConflict       = "sync_conflict"
	CodePushRefused        = "push_refused"
	CodeRemote             = "remote_error"
	CodeBatchInvalid       = "batch_invalid"
	CodeBusy               = "busy"
	CodeGit                = "git_error"
	CodeUnsupportedFormat  = "unsupported_format"
	CodePortInUse          = "port_in_use"
	CodeLaunchd            = "launchd_error"
	CodeInternal           = "internal"
)

// Error is an error with a stable code.
type Error struct {
	Code  string      `json:"code"`
	Msg   string      `json:"error"`
	Lines []LineError `json:"lines,omitempty"`
}

func (e *Error) Error() string { return e.Msg }

// LineError is one failed batch line.
type LineError struct {
	Line  int    `json:"line"`
	Op    string `json:"op,omitempty"`
	Code  string `json:"code"`
	Error string `json:"error"`
}

func errf(code, format string, a ...any) error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, a...)}
}

// AsError gives any error a code.
func AsError(err error) *Error {
	var me *Error
	var gerr *git.Error
	switch {
	case errors.As(err, &me):
		return me
	case errors.Is(err, store.ErrNotInitialized):
		return &Error{Code: CodeNotInitialized, Msg: err.Error()}
	case errors.Is(err, store.ErrBusy):
		return &Error{Code: CodeBusy, Msg: err.Error()}
	case errors.Is(err, store.ErrExists):
		return &Error{Code: CodeAlreadyInitialized, Msg: err.Error()}
	case errors.Is(err, git.ErrNotRepo):
		return &Error{Code: CodeNotRepo, Msg: err.Error()}
	case errors.As(err, &gerr):
		return &Error{Code: CodeGit, Msg: err.Error()}
	}
	return &Error{Code: CodeInternal, Msg: err.Error()}
}
