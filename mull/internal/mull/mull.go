// Package mull implements mull's operations on top of the store: plans,
// phases, items, dependencies, claims and the inbox.
package mull

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
	"mull/internal/model"
	"mull/internal/store"
)

// Mull is mull opened in one worktree. The worktree is the caller's identity.
type Mull struct {
	repo  *git.Repo
	store *store.Store
	// Now returns the current time; tests may replace it.
	Now func() time.Time
	// pushHook, when set, sees the exact git push arguments (tests only).
	pushHook func(args []string)
}

// Open opens mull for the git working tree containing dir.
func Open(dir string) (*Mull, error) {
	r, err := git.Open(dir)
	if err != nil {
		return nil, err
	}
	return &Mull{repo: r, store: store.New(r), Now: time.Now}, nil
}

// Worktree is the caller's identity: this working tree's path.
func (m *Mull) Worktree() string { return m.repo.Worktree() }

func (m *Mull) now() string { return m.Now().UTC().Format(time.RFC3339) }

// Init creates the mull branch with a config and the standing general plan.
func (m *Mull) Init(integration, release string) (*model.Config, error) {
	if integration == "" {
		integration = "main"
	}
	if release == "" {
		release = "main"
	}
	cfg := &model.Config{
		Integration: integration,
		Release:     release,
		Remote:      "origin",
		DoneChecks:  []string{},
		Layers:      map[string][]string{},
	}
	cfgData, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	err = m.store.Init("mull: init", map[string][]byte{model.ConfigPath: cfgData})
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// state is every record on the mull branch, decoded.
type state struct {
	tx      *store.Tx
	config  model.Config
	plans   map[string]*model.Plan
	phases  map[string]*model.Phase
	items   map[string]*model.Item
	entries map[string]*model.Entry
}

func load(tx *store.Tx) (*state, error) {
	s := &state{
		tx:      tx,
		plans:   map[string]*model.Plan{},
		phases:  map[string]*model.Phase{},
		items:   map[string]*model.Item{},
		entries: map[string]*model.Entry{},
	}
	if b, ok := tx.Get(model.ConfigPath); ok {
		if err := yaml.Unmarshal(b, &s.config); err != nil {
			return nil, fmt.Errorf("%s: %w", model.ConfigPath, err)
		}
	}
	for _, p := range tx.List("plans/") {
		var v model.Plan
		body, err := decodeFile(tx, p, &v)
		if err != nil {
			return nil, err
		}
		v.Slug, v.Summary = baseName(p), body
		v.Doc = getDoc(tx, model.PlanDocPath(v.Slug))
		s.plans[v.Slug] = &v
	}
	for _, p := range tx.List("epics/") {
		var v model.Phase
		body, err := decodeFile(tx, p, &v)
		if err != nil {
			return nil, err
		}
		v.ID, v.Summary = baseName(p), body
		v.Doc = getDoc(tx, model.PhaseDocPath(v.ID))
		s.phases[v.ID] = &v
	}
	for _, p := range tx.List("items/") {
		var v model.Item
		body, err := decodeFile(tx, p, &v)
		if err != nil {
			return nil, err
		}
		v.ID, v.Notes = baseName(p), body
		s.items[v.ID] = &v
	}
	for _, p := range tx.List("inbox/") {
		var v model.Entry
		body, err := decodeFile(tx, p, &v)
		if err != nil {
			return nil, err
		}
		v.ID, v.Detail = baseName(p), body
		s.entries[v.ID] = &v
	}
	return s, nil
}

func decodeFile(tx *store.Tx, path string, v any) (string, error) {
	b, _ := tx.Get(path)
	body, err := model.Decode(b, v)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return body, nil
}

// getDoc reads a doc: plain markdown, no frontmatter. A missing doc is "".
func getDoc(tx *store.Tx, path string) string {
	b, _ := tx.Get(path)
	return strings.TrimRight(string(b), "\n")
}

// putDoc writes a doc to its own file, ending in one newline; an empty doc
// removes the file. It leaves the record alone, so a doc's history holds
// only its own versions.
func (s *state) putDoc(path, doc string) {
	old, exists := s.tx.Get(path)
	doc = strings.TrimRight(doc, "\n")
	switch {
	case strings.TrimSpace(doc) == "":
		if exists {
			s.tx.Delete(path)
		}
	case !exists || string(old) != doc+"\n": // an unchanged doc writes nothing
		s.tx.Put(path, []byte(doc+"\n"))
	}
}

func baseName(path string) string {
	name := path[strings.LastIndex(path, "/")+1:]
	return strings.TrimSuffix(name, ".md")
}

func (s *state) putPlan(p *model.Plan) error {
	b, err := model.Encode(p, p.Summary)
	if err != nil {
		return err
	}
	s.tx.Put(model.PlanPath(p.Slug), b)
	s.plans[p.Slug] = p
	return nil
}

func (s *state) putPhase(e *model.Phase) error {
	b, err := model.Encode(e, e.Summary)
	if err != nil {
		return err
	}
	s.tx.Put(model.PhasePath(e.ID), b)
	s.phases[e.ID] = e
	return nil
}

func (s *state) putItem(i *model.Item) error {
	b, err := model.Encode(i, i.Notes)
	if err != nil {
		return err
	}
	s.tx.Put(model.ItemPath(i.ID), b)
	s.items[i.ID] = i
	return nil
}

func (s *state) putEntry(e *model.Entry) error {
	b, err := model.Encode(e, e.Detail)
	if err != nil {
		return err
	}
	s.tx.Put(model.EntryPath(e.ID), b)
	s.entries[e.ID] = e
	return nil
}

// newID returns an unused 4-hex ID across phases, items and inbox entries.
func (s *state) newID() (string, error) {
	for i := 0; i < 1000; i++ {
		var b [2]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		id := hex.EncodeToString(b[:])
		if s.phases[id] == nil && s.items[id] == nil && s.entries[id] == nil {
			return id, nil
		}
	}
	return "", errors.New("could not find a free ID")
}

func (s *state) plan(slug string) (*model.Plan, error) {
	p := s.plans[slug]
	if p == nil {
		return nil, errf(CodeNotFound, "no plan %q", slug)
	}
	return p, nil
}

// openPlan returns a plan that can take new work: shaping or active, not done.
func (s *state) openPlan(slug string) (*model.Plan, error) {
	p, err := s.plan(slug)
	if err != nil {
		return nil, err
	}
	if p.State == model.PlanDone {
		return nil, errf(CodeState, "plan %q is done", slug)
	}
	return p, nil
}

// agreedPlan returns a plan that can take phases and committed work: an
// active one. A shaping plan holds a doc, questions and captures until the
// human agrees it.
func (s *state) agreedPlan(slug string) (*model.Plan, error) {
	p, err := s.openPlan(slug)
	if err != nil {
		return nil, err
	}
	if p.State == model.PlanShaping {
		return nil, errf(CodeState, "plan %q is shaping: it takes phases and committed work once the human agrees it (mull plan agree %s --on-request)", slug, slug)
	}
	return p, nil
}

func (s *state) phase(id string) (*model.Phase, error) {
	e := s.phases[id]
	if e == nil {
		return nil, errf(CodeNotFound, "no phase %q", id)
	}
	return e, nil
}

func (s *state) item(id string) (*model.Item, error) {
	i := s.items[id]
	if i == nil {
		return nil, errf(CodeNotFound, "no item %q", id)
	}
	return i, nil
}

func (s *state) entry(id string) (*model.Entry, error) {
	e := s.entries[id]
	if e == nil {
		return nil, errf(CodeNotFound, "no inbox entry %q", id)
	}
	return e, nil
}

// view runs fn on the current state without writing.
func (m *Mull) view(fn func(s *state) error) error {
	return m.store.View(func(tx *store.Tx) error {
		s, err := load(tx)
		if err != nil {
			return err
		}
		return fn(s)
	})
}

// update runs fn on the latest state and commits its changes atomically.
func (m *Mull) update(msg string, fn func(s *state) error) error {
	return m.store.Update("mull: "+msg, func(tx *store.Tx) error {
		s, err := load(tx)
		if err != nil {
			return err
		}
		return fn(s)
	})
}

// Config returns the repository's mull configuration.
func (m *Mull) Config() (model.Config, error) {
	var cfg model.Config
	err := m.view(func(s *state) error { cfg = s.config; return nil })
	return cfg, err
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
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
