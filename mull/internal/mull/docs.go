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
