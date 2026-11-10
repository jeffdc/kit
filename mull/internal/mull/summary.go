package mull

import (
	"sort"

	"mull/internal/model"
)

// Summary is one project's line in mull serve's project list: enough to
// say where it stands without opening it. It reads the records and a few
// refs, never the item links the view derives from every branch.
type Summary struct {
	Integration      string        `json:"integration"`
	IntegrationFound bool          `json:"integration_found"`
	Release          string        `json:"release"`
	AheadOfRelease   int           `json:"ahead_of_release"`
	Plans            []SummaryPlan `json:"plans"` // plans not done: active, then shaping
	NeedsYou         NeedsYou      `json:"needs_you"`
	Blocking         int           `json:"blocking"` // open questions on active or ready work: Needs you's first tier
	Agents           int           `json:"agents"`   // active items
	Untriaged        int           `json:"untriaged"`
	Commit           string        `json:"commit"`      // the mull branch's tip
	LastChange       string        `json:"last_change"` // its commit time
	Format           int           `json:"format"`
	// MigrationRequired: see View.
	MigrationRequired bool `json:"migration_required,omitempty"`
}

// SummaryPlan is a plan in a project summary.
type SummaryPlan struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	State string `json:"state"`
}

// Summary computes the project's summary.
func (m *Mull) Summary() (Summary, error) {
	var out Summary
	err := m.view(func(s *state) error {
		var err error
		out, err = m.summary(s)
		return err
	})
	return out, err
}

func (m *Mull) summary(s *state) (Summary, error) {
	out := Summary{Plans: []SummaryPlan{}, Release: s.config.Release, Format: s.config.Format,
		MigrationRequired: s.writable() != nil, Commit: s.tx.Commit()}
	for _, slug := range sortedKeys(s.plans) {
		p := s.plans[slug]
		if p.State == model.PlanActive || p.State == model.PlanShaping {
			out.Plans = append(out.Plans, SummaryPlan{Slug: p.Slug, Title: p.Title, State: p.State})
		}
	}
	sort.SliceStable(out.Plans, func(a, b int) bool {
		return out.Plans[a].State == model.PlanActive && out.Plans[b].State != model.PlanActive
	})
	for _, i := range s.items {
		if i.State == model.StateActive {
			out.Agents++
		}
		if untriaged(i) {
			out.Untriaged++
		}
	}
	now := m.Now().UTC()
	for _, e := range s.entries {
		out.NeedsYou.add(e, now)
	}
	out.Blocking = s.countBlocking(nil)
	var err error
	out.LastChange, err = m.repo.Output(nil, nil, "log", "-1", "--format=%cI", out.Commit)
	if err != nil {
		return out, err
	}
	return out, m.summaryRefs(s, &out)
}

// summaryRefs fills in the integration branch and how far it is ahead of
// the release branch.
func (m *Mull) summaryRefs(s *state, out *Summary) error {
	out.Integration = s.config.Integration
	if out.Integration == "" {
		out.Integration = "main"
	}
	integ, err := m.repo.RefCommit("refs/heads/" + out.Integration)
	if err != nil || integ == "" {
		return err
	}
	out.IntegrationFound = true
	out.AheadOfRelease = m.aheadOfRelease(s, out.Integration, integ)
	return nil
}
