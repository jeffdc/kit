package mull

import (
	"path/filepath"
	"sort"

	"mull/internal/model"
)

// Context is the compact briefing an agent reads at the start of a session.
type Context struct {
	Worktree  string        `json:"worktree"`
	Branch    string        `json:"branch"`
	Protocol  []string      `json:"protocol"`
	Item      *ContextItem  `json:"item,omitempty"`
	MyClaims  []BriefItem   `json:"my_claims"`
	Plans     []BriefPlan   `json:"plans"`
	Ready     []BriefItem   `json:"ready"`
	Active    []BriefItem   `json:"active"`
	Decisions []BriefDecide `json:"decisions"`
}

// ContextItem is the item a session works on, with what bears on it.
type ContextItem struct {
	ItemView
	PhaseTitle     string        `json:"phase_title,omitempty"`
	PhaseSummary   string        `json:"phase_summary,omitempty"`
	OpenQuestions  []BriefEntry  `json:"open_questions,omitempty"`
	ItemDecisions  []BriefDecide `json:"item_decisions,omitempty"`
	LastReport     string        `json:"last_report,omitempty"`
	LastReportPass *bool         `json:"last_report_passed,omitempty"`
}

// BriefItem, BriefPlan, BriefEntry and BriefDecide are compact records.
type BriefItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Plan     string `json:"plan,omitempty"`
	Phase    string `json:"phase,omitempty"`
	Worktree string `json:"worktree,omitempty"`
}

type BriefPlan struct {
	Slug   string       `json:"slug"`
	Title  string       `json:"title"`
	Phases []BriefPhase `json:"phases,omitempty"`
}

type BriefPhase struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Progress  string   `json:"progress"`
	BlockedBy []string `json:"blocked_by,omitempty"`
}

type BriefEntry struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type BriefDecide struct {
	ID       string `json:"id"`
	Item     string `json:"item,omitempty"`
	Context  string `json:"context"`
	Decision string `json:"decision"`
}

const (
	contextReadyLimit     = 15
	contextDecisionsLimit = 20
)

var protocol = []string{
	"Use only the mull CLI; never read or edit the mull branch or its files.",
	"One item per session: mull start <id>; branch <id>-<name>; end each commit message with 'Mull: <id>'.",
	"Read the decisions below before deciding anything they settle.",
	"Capture as you go: mull decided / mull ask with --item <id>; anything out of scope with mull bug / mull idea --source \"item <id>\".",
	"Don't change plans or phases or add items unless the human asks; propose with mull ask.",
	"Finish only with mull done <id> --summary ... --tour ... --design - (design notes on stdin: how and why); it runs the hard checks. Never claim done in prose.",
	"Settle inbox entries only on the human's explicit instruction, with --on-request. mull sync after done.",
}

// Context builds the session-start briefing, focused on item when given.
func (m *Mull) Context(itemID string) (Context, error) {
	wt := m.Worktree()
	c := Context{Worktree: wt, Branch: m.repo.Branch(), Protocol: protocol,
		MyClaims: []BriefItem{}, Plans: []BriefPlan{}, Ready: []BriefItem{}, Active: []BriefItem{}, Decisions: []BriefDecide{}}
	decisions, err := m.Decisions("")
	if err != nil {
		return c, err
	}
	err = m.view(func(s *state) error {
		brief := func(i *model.Item) BriefItem {
			b := BriefItem{ID: i.ID, Title: i.Title, Plan: i.Plan, Phase: i.Phase}
			if i.Claim != nil {
				b.Worktree = filepath.Base(i.Claim.Worktree)
			}
			return b
		}
		for _, slug := range sortedKeys(s.plans) {
			p := s.plans[slug]
			if p.State != model.PlanActive {
				continue
			}
			bp := BriefPlan{Slug: slug, Title: p.Title}
			for _, id := range sortedKeys(s.phases) {
				if e := s.phases[id]; e.Plan == slug {
					ev := s.phaseView(e)
					bp.Phases = append(bp.Phases, BriefPhase{ID: id, Title: e.Title, Progress: progressText(ev.Progress), BlockedBy: ev.BlockedBy})
				}
			}
			c.Plans = append(c.Plans, bp)
		}
		for _, id := range sortedKeys(s.items) {
			i := s.items[id]
			if p := s.plans[i.Plan]; i.Plan != "" && (p == nil || p.State != model.PlanActive) {
				continue // unplanned work is always in view
			}
			switch {
			case i.State == model.StateActive:
				c.Active = append(c.Active, brief(i))
				if i.Claim != nil && i.Claim.Worktree == wt {
					c.MyClaims = append(c.MyClaims, brief(i))
				}
			case s.itemView(i).Ready && len(c.Ready) < contextReadyLimit:
				c.Ready = append(c.Ready, brief(i))
			}
		}
		if itemID != "" {
			i, err := s.item(itemID)
			if err != nil {
				return err
			}
			ci := &ContextItem{ItemView: s.itemView(i)}
			if e := s.phases[i.Phase]; e != nil {
				ci.PhaseTitle, ci.PhaseSummary = e.Title, e.Summary
			}
			for _, id := range sortedKeys(s.entries) {
				e := s.entries[id]
				if e.Item == itemID && e.Kind == model.KindQuestion && e.Settled == nil {
					ci.OpenQuestions = append(ci.OpenQuestions, BriefEntry{ID: e.ID, Text: e.Text})
				}
			}
			if rep := s.latestReport(itemID); rep != nil {
				ci.LastReport = model.ReportPath(itemID, rep.N)
				passed := rep.Passed
				ci.LastReportPass = &passed
			}
			c.Item = ci
		}
		return nil
	})
	if err != nil {
		return c, err
	}
	sort.SliceStable(decisions, func(a, b int) bool { return decisions[a].At > decisions[b].At })
	for _, d := range decisions {
		bd := BriefDecide{ID: d.ID, Item: d.Item, Context: d.Context, Decision: d.Decision}
		if c.Item != nil && d.Item == itemID {
			c.Item.ItemDecisions = append(c.Item.ItemDecisions, bd)
			continue
		}
		if len(c.Decisions) < contextDecisionsLimit {
			c.Decisions = append(c.Decisions, bd)
		}
	}
	return c, nil
}
