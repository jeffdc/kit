package mull

import (
	"path/filepath"
	"sort"

	"mull/internal/guide"
	"mull/internal/model"
)

// Context is the compact briefing an agent reads at the start of a session.
type Context struct {
	Worktree string       `json:"worktree"`
	Branch   string       `json:"branch"`
	Protocol []string     `json:"protocol"`
	Item     *ContextItem `json:"item,omitempty"`
	MyClaims []BriefItem  `json:"my_claims"`
	Plans    []BriefPlan  `json:"plans"`
	Ready    []BriefItem  `json:"ready"`
	Active   []BriefItem  `json:"active"`
	// Rules are the project's rulebook (RULES.md), in full: every worker
	// follows them, and says at done whether the work touched one.
	Rules []Rule `json:"rules"`
}

// ContextItem is the item a session works on, with what bears on it.
type ContextItem struct {
	ItemView
	PhaseTitle   string `json:"phase_title,omitempty"`
	PhaseSummary string `json:"phase_summary,omitempty"`
	PhaseDoc     string `json:"phase_doc,omitempty"`
	// Target is the branch the item's work lands on (its plan's, or
	// integration), and Preview the URL its plan says serves that branch:
	// recorded, never run or checked by mull.
	Target  Target `json:"target"`
	Preview string `json:"preview,omitempty"`
	// Shared are the plan's shared globs: files its parallel work shares,
	// which done flags, so the worker knows them before touching one.
	Shared []string `json:"shared,omitempty"`
	// PlanRules and PhaseRules are the human's standing rules for work in
	// the item's plan and phase, in full: the worker follows the plan's,
	// then the phase's.
	PlanRules     string       `json:"plan_rules,omitempty"`
	PhaseRules    string       `json:"phase_rules,omitempty"`
	OpenQuestions []BriefEntry `json:"open_questions,omitempty"`
	// Queue is the queued work bearing on the item: in its area, or tagged
	// to its phase. Fix what's small as you go, and say so with done --covers.
	Queue          []BriefItem   `json:"queue,omitempty"`
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
	Slug    string       `json:"slug"`
	Title   string       `json:"title"`
	State   string       `json:"state"`             // shaping or active
	Target  Target       `json:"target"`            // where its work lands
	Preview string       `json:"preview,omitempty"` // the URL that serves the target
	Phases  []BriefPhase `json:"phases,omitempty"`
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

const contextReadyLimit = 15

var protocol = []string{
	"Use only the mull CLI; never read or edit the mull branch.",
	"One item per session: branch <id>-<name> from the item's target, then mull start <id>. End each commit message with 'Mull: <id>'.",
	"Decide locally. Note a choice worth keeping with mull decided --item <id>: it goes in your report, not to the human.",
	"Ask (mull ask --default ... --by ...) only about a rule conflict or something hard to undo, and keep working meanwhile.",
	"Fix small problems in the area you're changing as part of your change. File the rest in one line: mull bug or mull idea --source \"item <id>\".",
	"Don't change plans, phases, rules or checks, and don't settle inbox entries, unless the human asks (then add --on-request).",
	"Finish with mull done <id> --summary \"<one line>\" --rules none (or the rule and how). It runs the checks once and lands low and medium risk work; high risk waits for review. Never claim done in prose.",
	"mull sync after done.",
}

// rulesProtocol is the protocol's last line when the item's plan or phase
// has rules.
const rulesProtocol = "Follow the item's plan_rules, then its phase_rules: the human's standing rules for this work."

// rulesLine is the protocol's line when the project has a rulebook.
const rulesLine = "Follow the rules below (RULES.md). If your work bends, breaks or adds one, say which and how in done's --rules: it waits for the human's review. Never change RULES.md or a ratchet file unless the human asks."

// Context builds the session-start briefing, focused on item when given.
func (m *Mull) Context(itemID string) (Context, error) {
	wt := m.Worktree()
	c := Context{Worktree: wt, Branch: m.repo.Branch(), Protocol: protocol,
		MyClaims: []BriefItem{}, Plans: []BriefPlan{}, Ready: []BriefItem{}, Active: []BriefItem{}, Rules: []Rule{}}
	if rules, err := m.readRules(); err != nil {
		return c, err
	} else if len(rules) > 0 {
		c.Rules = rules
		c.Protocol = append(c.Protocol, rulesLine)
	}
	decisions, err := m.Decisions("")
	if err != nil {
		return c, err
	}
	err = m.view(func(s *state) error {
		if err := s.writable(); err != nil {
			c.Protocol = append([]string{"Writes are refused until the human migrates this repository's mull data (code migration_required): " +
				"tell the human; mull migrate --dry-run shows what it would change, and the real run is theirs (mull migrate --on-request)."}, protocol...)
		}
		if name := m.Human(); name != "" {
			named := []string{guide.NameRule(name)}
			for _, line := range c.Protocol {
				named = append(named, guide.Personal(line, name))
			}
			c.Protocol = named
		}
		_, heads, err := m.branchHeads()
		if err != nil {
			return err
		}
		targets := m.readTargets(s, heads)
		brief := func(i *model.Item) BriefItem {
			b := BriefItem{ID: i.ID, Title: i.Title, Plan: i.Plan, Phase: i.Phase}
			if i.Claim != nil {
				b.Worktree = filepath.Base(i.Claim.Worktree)
			}
			return b
		}
		for _, slug := range sortedKeys(s.plans) {
			p := s.plans[slug]
			if p.State == model.PlanDone {
				continue
			}
			bp := BriefPlan{Slug: slug, Title: p.Title, State: p.State, Target: s.target(slug, targets), Preview: p.Preview}
			for _, id := range sortedKeys(s.phases) {
				if e := s.phases[id]; e.Plan == slug {
					ev := s.phaseView(e)
					progress := progressText(ev.Progress)
					bp.Phases = append(bp.Phases, BriefPhase{ID: id, Title: e.Title, Progress: progress, BlockedBy: ev.BlockedBy})
				}
			}
			c.Plans = append(c.Plans, bp)
		}
		var items []*model.Item
		for _, id := range sortedKeys(s.items) {
			i := s.items[id]
			if p := s.plans[i.Plan]; i.Plan != "" && (p == nil || p.State == model.PlanDone) {
				continue // unplanned work is always in view
			}
			items = append(items, i)
		}
		s.workOrder(items) // so the ready list, cut at its limit, keeps what comes first
		for _, i := range items {
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
			ci := &ContextItem{ItemView: s.itemView(i), Target: s.target(i.Plan, targets)}
			if p := s.plans[i.Plan]; p != nil {
				ci.PlanRules, ci.Preview, ci.Shared = p.Rules, p.Preview, p.Shared
			}
			if e := s.phases[i.Phase]; e != nil {
				ci.PhaseTitle, ci.PhaseSummary, ci.PhaseDoc, ci.PhaseRules = e.Title, e.Summary, e.Doc, e.Rules
			}
			ci.Queue = s.queueFor(i)
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
			if ci.PlanRules != "" || ci.PhaseRules != "" {
				c.Protocol = append(c.Protocol, guide.Personal(rulesProtocol, m.Human()))
			}
			c.Item = ci
		}
		return nil
	})
	if err != nil {
		return c, err
	}
	// Only the item's own decisions: the whole log is for the human to pull
	// from (mull decisions), not for every session to read.
	sort.SliceStable(decisions, func(a, b int) bool { return decisions[a].At > decisions[b].At })
	for _, d := range decisions {
		if c.Item != nil && d.Item == itemID {
			c.Item.ItemDecisions = append(c.Item.ItemDecisions, BriefDecide{ID: d.ID, Item: d.Item, Context: d.Context, Decision: d.Decision})
		}
	}
	return c, nil
}
