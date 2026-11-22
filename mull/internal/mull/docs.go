package mull

import (
	"sort"
	"strings"
	"time"

	"mull/internal/model"
	"mull/internal/store"
)

// Plans and phases each have a doc: markdown holding intent only (a plan's
// picture, done-when and why the order; a phase's design and done-when),
// never status, which comes from the records. A doc is its own file on the
// mull branch, so its history is the commits that changed that file.

// DocResult is a plan's or phase's doc, as plan doc and phase doc print it.
type DocResult struct {
	Type string `json:"type"` // plan or phase
	ID   string `json:"id"`   // the plan's slug or the phase's ID
	Doc  string `json:"doc"`  // "" when there is none
}

// DocVersion is one version of a doc: its text after a commit that changed
// it ("" when the commit removed it).
type DocVersion struct {
	Commit string `json:"commit"`
	At     string `json:"at"`
	Doc    string `json:"doc"`
}

// DocLog is a doc's history, newest first.
type DocLog struct {
	Type     string       `json:"type"`
	ID       string       `json:"id"`
	Versions []DocVersion `json:"versions"`
}

// PlanDoc returns a plan's doc.
func (m *Mull) PlanDoc(slug string) (DocResult, error) {
	var out DocResult
	err := m.view(func(s *state) error {
		p, err := s.plan(slug)
		if err != nil {
			return err
		}
		out = DocResult{Type: "plan", ID: p.Slug, Doc: p.Doc}
		return nil
	})
	return out, err
}

// PhaseDoc returns a phase's doc.
func (m *Mull) PhaseDoc(id string) (DocResult, error) {
	var out DocResult
	err := m.view(func(s *state) error {
		e, err := s.phase(id)
		if err != nil {
			return err
		}
		out = DocResult{Type: "phase", ID: e.ID, Doc: e.Doc}
		return nil
	})
	return out, err
}

// SetPlanDoc replaces a plan's doc; an empty doc removes it. Any plan takes
// a doc, shaping ones included: a shaping plan is its doc and questions.
func (m *Mull) SetPlanDoc(slug, doc string) (DocResult, error) {
	var out DocResult
	err := m.update("plan doc "+slug, func(s *state) error {
		var err error
		out, err = m.setPlanDoc(s, slug, doc)
		return err
	})
	return out, err
}

func (m *Mull) setPlanDoc(s *state, slug, doc string) (DocResult, error) {
	p, err := s.plan(slug)
	if err != nil {
		return DocResult{}, err
	}
	s.putDoc(model.PlanDocPath(slug), doc)
	p.Doc = getDoc(s.tx, model.PlanDocPath(slug))
	return DocResult{Type: "plan", ID: slug, Doc: p.Doc}, nil
}

// SetPhaseDoc replaces a phase's doc; an empty doc removes it.
func (m *Mull) SetPhaseDoc(id, doc string) (DocResult, error) {
	var out DocResult
	err := m.update("phase doc "+id, func(s *state) error {
		var err error
		out, err = m.setPhaseDoc(s, id, doc)
		return err
	})
	return out, err
}

func (m *Mull) setPhaseDoc(s *state, id, doc string) (DocResult, error) {
	e, err := s.phase(id)
	if err != nil {
		return DocResult{}, err
	}
	s.putDoc(model.PhaseDocPath(id), doc)
	e.Doc = getDoc(s.tx, model.PhaseDocPath(id))
	return DocResult{Type: "phase", ID: id, Doc: e.Doc}, nil
}

// PlanDocLog returns every version of a plan's doc, newest first.
func (m *Mull) PlanDocLog(slug string) (DocLog, error) {
	if _, err := m.PlanDoc(slug); err != nil {
		return DocLog{}, err
	}
	return m.docLog("plan", slug, model.PlanDocPath(slug))
}

// PhaseDocLog returns every version of a phase's doc, newest first. Its
// versions from before mull migrate moved it, under docs/epics/, are part
// of its history too.
func (m *Mull) PhaseDocLog(id string) (DocLog, error) {
	if _, err := m.PhaseDoc(id); err != nil {
		return DocLog{}, err
	}
	return m.docLog("phase", id, model.PhaseDocPath(id), model.LegacyPhaseDocPath(id))
}

// docLog reads a doc's history at its path, then at any older paths it had.
// A commit that moved the doc shows once, as its version at the newer path.
func (m *Mull) docLog(typ, id string, paths ...string) (DocLog, error) {
	out := DocLog{Type: typ, ID: id, Versions: []DocVersion{}}
	var vs []store.Version
	seen := map[string]bool{}
	for _, path := range paths {
		h, err := m.store.History(path)
		if err != nil {
			return out, err
		}
		for _, v := range h {
			if !seen[v.Commit] {
				seen[v.Commit] = true
				vs = append(vs, v)
			}
		}
	}
	sort.SliceStable(vs, func(a, b int) bool { return vs[a].At.After(vs[b].At) })
	for _, v := range vs {
		out.Versions = append(out.Versions, DocVersion{
			Commit: v.Commit, At: v.At.UTC().Format(time.RFC3339), Doc: strings.TrimRight(string(v.Data), "\n"),
		})
	}
	return out, nil
}

// Plans and phases may also have rules: the human's standing rules for the
// agents working in them (how to set up, what to use, what not to touch).
// They are plain markdown kept as a doc is, in their own file, and
// context --item prints the item's plan's rules, then its phase's.

// RulesResult is a plan's or phase's rules, as plan rules and phase rules
// print them.
type RulesResult struct {
	Type  string `json:"type"`  // plan or phase
	ID    string `json:"id"`    // the plan's slug or the phase's ID
	Rules string `json:"rules"` // "" when there are none
}

// RulesVersion is one version of a plan's or phase's rules ("" when the
// commit removed them).
type RulesVersion struct {
	Commit string `json:"commit"`
	At     string `json:"at"`
	Rules  string `json:"rules"`
}

// RulesLog is the history of a plan's or phase's rules, newest first.
type RulesLog struct {
	Type     string         `json:"type"`
	ID       string         `json:"id"`
	Versions []RulesVersion `json:"versions"`
}

// PlanRules returns a plan's rules.
func (m *Mull) PlanRules(slug string) (RulesResult, error) {
	var out RulesResult
	err := m.view(func(s *state) error {
		p, err := s.plan(slug)
		if err != nil {
			return err
		}
		out = RulesResult{Type: "plan", ID: p.Slug, Rules: p.Rules}
		return nil
	})
	return out, err
}

// PhaseRules returns a phase's rules.
func (m *Mull) PhaseRules(id string) (RulesResult, error) {
	var out RulesResult
	err := m.view(func(s *state) error {
		e, err := s.phase(id)
		if err != nil {
			return err
		}
		out = RulesResult{Type: "phase", ID: e.ID, Rules: e.Rules}
		return nil
	})
	return out, err
}

// SetPlanRules replaces a plan's rules; empty rules remove them. Any plan
// takes rules, as any plan takes a doc.
func (m *Mull) SetPlanRules(slug, rules string) (RulesResult, error) {
	var out RulesResult
	err := m.update("plan rules "+slug, func(s *state) error {
		var err error
		out, err = m.setPlanRules(s, slug, rules)
		return err
	})
	return out, err
}

func (m *Mull) setPlanRules(s *state, slug, rules string) (RulesResult, error) {
	p, err := s.plan(slug)
	if err != nil {
		return RulesResult{}, err
	}
	s.putDoc(model.PlanRulesPath(slug), rules)
	p.Rules = getDoc(s.tx, model.PlanRulesPath(slug))
	return RulesResult{Type: "plan", ID: slug, Rules: p.Rules}, nil
}

// SetPhaseRules replaces a phase's rules; empty rules remove them.
func (m *Mull) SetPhaseRules(id, rules string) (RulesResult, error) {
	var out RulesResult
	err := m.update("phase rules "+id, func(s *state) error {
		var err error
		out, err = m.setPhaseRules(s, id, rules)
		return err
	})
	return out, err
}

func (m *Mull) setPhaseRules(s *state, id, rules string) (RulesResult, error) {
	e, err := s.phase(id)
	if err != nil {
		return RulesResult{}, err
	}
	s.putDoc(model.PhaseRulesPath(id), rules)
	e.Rules = getDoc(s.tx, model.PhaseRulesPath(id))
	return RulesResult{Type: "phase", ID: id, Rules: e.Rules}, nil
}

// PlanRulesLog returns every version of a plan's rules, newest first.
func (m *Mull) PlanRulesLog(slug string) (RulesLog, error) {
	if _, err := m.PlanRules(slug); err != nil {
		return RulesLog{}, err
	}
	return m.rulesLog("plan", slug, model.PlanRulesPath(slug))
}

// PhaseRulesLog returns every version of a phase's rules, newest first.
func (m *Mull) PhaseRulesLog(id string) (RulesLog, error) {
	if _, err := m.PhaseRules(id); err != nil {
		return RulesLog{}, err
	}
	return m.rulesLog("phase", id, model.PhaseRulesPath(id))
}

// rulesLog is docLog for rules: the same history, with rules for doc.
func (m *Mull) rulesLog(typ, id, path string) (RulesLog, error) {
	out := RulesLog{Type: typ, ID: id, Versions: []RulesVersion{}}
	l, err := m.docLog(typ, id, path)
	if err != nil {
		return out, err
	}
	for _, v := range l.Versions {
		out.Versions = append(out.Versions, RulesVersion{Commit: v.Commit, At: v.At, Rules: v.Doc})
	}
	return out, nil
}
