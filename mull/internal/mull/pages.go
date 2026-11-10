package mull

import (
	"regexp"
	"sort"

	"mull/internal/model"
)

// The pages of mull serve's UI, each read from one snapshot of the mull
// branch. They only read.

// stageOrder sorts items in a list: work in flight first, then what waits
// for review, what's left to do, and what's finished.
var stageOrder = map[string]int{
	model.StateActive: 0, model.StateInReview: 1, model.StateCommitted: 2,
	model.StateDone: 3, model.StateDropped: 4, model.StateCaptured: 5,
}

// ItemRow is an item in a list: everything but its notes.
type ItemRow struct {
	ID         string       `json:"id"`
	Title      string       `json:"title"`
	State      string       `json:"state"`
	Ready      bool         `json:"ready,omitempty"`
	Kind       string       `json:"kind,omitempty"`
	Category   string       `json:"category,omitempty"`
	Source     string       `json:"source,omitempty"`
	Plan       string       `json:"plan,omitempty"`
	Phase      string       `json:"phase,omitempty"`
	PhaseTitle string       `json:"phase_title,omitempty"`
	Claim      *model.Claim `json:"claim,omitempty"`
	Untriaged  bool         `json:"untriaged,omitempty"`
	Triaged    string       `json:"triaged,omitempty"`
	Questions  int          `json:"questions"` // open questions on it
	Created    string       `json:"created"`
	Updated    string       `json:"updated"`
}

func (s *state) itemRow(i *model.Item) ItemRow {
	r := ItemRow{ID: i.ID, Title: i.Title, State: i.State, Ready: s.itemView(i).Ready, Kind: i.Kind,
		Category: i.Category, Source: i.Source, Plan: i.Plan, Phase: i.Phase, Claim: i.Claim,
		Untriaged: untriaged(i), Triaged: i.Triaged, Created: i.Created, Updated: i.Updated}
	if ph := s.phases[i.Phase]; ph != nil {
		r.PhaseTitle = ph.Title
	}
	for _, e := range s.entries {
		if e.Settled == nil && e.Kind == model.KindQuestion && e.Item == i.ID {
			r.Questions++
		}
	}
	return r
}

// itemRows lists the items keep selects, by stage, then oldest first.
func (s *state) itemRows(keep func(*model.Item) bool) []ItemRow {
	var items []*model.Item
	for _, id := range sortedKeys(s.items) {
		if i := s.items[id]; keep(i) {
			items = append(items, i)
		}
	}
	sort.SliceStable(items, func(a, b int) bool {
		if oa, ob := stageOrder[items[a].State], stageOrder[items[b].State]; oa != ob {
			return oa < ob
		}
		return items[a].Created < items[b].Created
	})
	out := []ItemRow{}
	for _, i := range items {
		out = append(out, s.itemRow(i))
	}
	return out
}

// notDonePlans are the plans not done: active first, then shaping, by slug.
func (s *state) notDonePlans() []*model.Plan {
	var out []*model.Plan
	for _, slug := range sortedKeys(s.plans) {
		if p := s.plans[slug]; p.State != model.PlanDone {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		return out[a].State == model.PlanActive && out[b].State != model.PlanActive
	})
	return out
}

func unplanned(i *model.Item) bool { return i.Plan == "" && i.State != model.StateCaptured }

// Counts are the project's tab counts.
type Counts struct {
	Unplanned  int    `json:"unplanned"` // committed items with no plan, any stage after captured
	Backlog    int    `json:"backlog"`   // captured items
	Untriaged  int    `json:"untriaged"`
	LastTriage string `json:"last_triage,omitempty"`
}

func (s *state) counts() Counts {
	var c Counts
	for _, i := range s.items {
		switch {
		case unplanned(i):
			c.Unplanned++
		case i.State == model.StateCaptured:
			c.Backlog++
			if untriaged(i) {
				c.Untriaged++
			}
		}
		if i.Triaged > c.LastTriage {
			c.LastTriage = i.Triaged
		}
	}
	return c
}

// Overview is the project page: its title block (the summary), the ranked
// Needs you schedule, and where each plan not done stands.
type Overview struct {
	Summary  Summary       `json:"summary"`
	Counts   Counts        `json:"counts"`
	NeedsYou NeedsYouTiers `json:"needs_you"`
	Plans    []PlanStatus  `json:"plans"`  // active, then shaping
	Agents   []AgentRow    `json:"agents"` // every active item, unplanned ones too
}

// Overview reads the project overview.
func (m *Mull) Overview() (Overview, error) {
	var out Overview
	err := m.view(func(s *state) error {
		sum, err := m.summary(s)
		if err != nil {
			return err
		}
		g, err := m.readGit(s)
		if err != nil {
			return err
		}
		out = Overview{Summary: sum, Counts: s.counts(), NeedsYou: s.needsYou(nil), Plans: []PlanStatus{}, Agents: []AgentRow{}}
		for _, p := range s.notDonePlans() {
			out.Plans = append(out.Plans, s.planStatus(p, g))
		}
		for _, id := range sortedKeys(s.items) {
			if i := s.items[id]; i.State == model.StateActive {
				out.Agents = append(out.Agents, s.agentRow(i, g))
			}
		}
		return nil
	})
	return out, err
}

// DocRef is a phase or item a doc mentions by ID, with its status: docs
// hold intent, and status shows beside them.
type DocRef struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // phase or item
	Title     string    `json:"title"`
	State     string    `json:"state"` // a phase's done, now or waits; an item's stage
	Progress  *Progress `json:"progress,omitempty"`
	Questions int       `json:"questions"`
}

var idWord = regexp.MustCompile(`\b[0-9a-f]{4}\b`)

// docRefs lists the phases and items doc names, in order of first mention.
func (s *state) docRefs(doc string) []DocRef {
	out := []DocRef{}
	seen := map[string]bool{}
	for _, id := range idWord.FindAllString(doc, -1) {
		if seen[id] {
			continue
		}
		seen[id] = true
		if e := s.phases[id]; e != nil {
			r := s.phaseRow(e, nil)
			out = append(out, DocRef{ID: id, Type: "phase", Title: e.Title, State: r.State, Progress: &r.Progress, Questions: r.Questions})
		} else if i := s.items[id]; i != nil {
			r := s.itemRow(i)
			out = append(out, DocRef{ID: id, Type: "item", Title: i.Title, State: i.State, Questions: r.Questions})
		}
	}
	return out
}

// NextPhase is a phase that can start now, with its open items (active,
// in review, then committed).
type NextPhase struct {
	PhaseRow
	Items []ItemRow `json:"items"`
}

// PlanPage is a plan: its record and doc, its phases in schedule order,
// what needs you about it, and what's next.
type PlanPage struct {
	Plan      model.Plan      `json:"plan"`
	Status    PlanStatus      `json:"status"`
	Schedule  []ScheduleGroup `json:"schedule"`
	NeedsYou  NeedsYouTiers   `json:"needs_you"`
	Next      []NextPhase     `json:"next"` // what's next: the phases that can start now
	Unfiled   []ItemRow       `json:"unfiled"`
	DocRefs   []DocRef        `json:"doc_refs"`
	Backlog   int             `json:"backlog"` // captured items tagged to the plan
	Untriaged int             `json:"untriaged"`
}

// PlanPage reads one plan's page.
func (m *Mull) PlanPage(slug string) (PlanPage, error) {
	var out PlanPage
	err := m.view(func(s *state) error {
		p, err := s.plan(slug)
		if err != nil {
			return err
		}
		g, err := m.readGit(s)
		if err != nil {
			return err
		}
		out = PlanPage{Plan: *p, Status: s.planStatus(p, g), Schedule: s.schedule(slug),
			NeedsYou: s.needsYou(func(e *model.Entry) bool { return e.Plan == slug }),
			Next:     []NextPhase{}, DocRefs: s.docRefs(p.Doc),
			Unfiled: s.itemRows(func(i *model.Item) bool {
				return i.Plan == slug && i.Phase == "" && i.State != model.StateCaptured
			})}
		for _, r := range out.Status.Now {
			out.Next = append(out.Next, NextPhase{PhaseRow: r, Items: s.itemRows(func(i *model.Item) bool {
				return i.Phase == r.ID && (i.State == model.StateActive || i.State == model.StateInReview || i.State == model.StateCommitted)
			})})
		}
		for _, i := range s.items {
			if i.Plan == slug && i.State == model.StateCaptured {
				out.Backlog++
				if untriaged(i) {
					out.Untriaged++
				}
			}
		}
		return nil
	})
	return out, err
}

// PhasePage is a phase: its record and doc, where it stands, its items,
// what's captured for it, and what needs you about it.
type PhasePage struct {
	Phase     model.Phase   `json:"phase"`
	PlanTitle string        `json:"plan_title"`
	PlanState string        `json:"plan_state"`
	Status    PhaseRow      `json:"status"`
	NeededBy  []PhaseRef    `json:"needed_by"`
	Items     []ItemRow     `json:"items"`    // committed work, by stage
	Captured  []ItemRow     `json:"captured"` // backlog tagged to the phase
	NeedsYou  NeedsYouTiers `json:"needs_you"`
	DocRefs   []DocRef      `json:"doc_refs"`
}

// PhasePage reads one phase's page.
func (m *Mull) PhasePage(id string) (PhasePage, error) {
	var out PhasePage
	err := m.view(func(s *state) error {
		e, err := s.phase(id)
		if err != nil {
			return err
		}
		out = PhasePage{Phase: *e, Status: s.phaseRow(e, s.goLive(e.Plan)), NeededBy: []PhaseRef{},
			Items:    s.itemRows(func(i *model.Item) bool { return i.Phase == id && i.State != model.StateCaptured }),
			Captured: s.itemRows(func(i *model.Item) bool { return i.Phase == id && i.State == model.StateCaptured }),
			NeedsYou: s.needsYou(func(x *model.Entry) bool { return s.entryPhase(x) == id }),
			DocRefs:  s.docRefs(e.Doc)}
		if p := s.plans[e.Plan]; p != nil {
			out.PlanTitle, out.PlanState = p.Title, p.State
		}
		for _, oid := range sortedKeys(s.phases) {
			for _, n := range s.phases[oid].Needs {
				if n == id {
					out.NeededBy = append(out.NeededBy, s.phaseRef(oid))
					break
				}
			}
		}
		return nil
	})
	return out, err
}

// ItemPage is an item: the record with its notes, where it sits, what git
// says about it, what needs you about it, and its review packet.
type ItemPage struct {
	Item       ItemView      `json:"item"`
	PlanTitle  string        `json:"plan_title,omitempty"`
	PhaseTitle string        `json:"phase_title,omitempty"`
	PhaseState string        `json:"phase_state,omitempty"`
	Git        ItemGit       `json:"git"`
	NeedsYou   NeedsYouTiers `json:"needs_you"`
	Reports    []ReportRef   `json:"reports"` // every done attempt, oldest first
	Review     *Review       `json:"review,omitempty"`
}

// ReportRef is one done attempt.
type ReportRef struct {
	N      int    `json:"n"`
	Passed bool   `json:"passed"`
	At     string `json:"at"`
}

// Review is an item's review packet: what was built (the report's summary,
// tour and checks) and how (its design notes, the session's decisions as
// they stand now, and files by layer). Entry is the review entry, settled
// or not; a packet with no entry is a done attempt that wasn't filed for
// review (its checks failed).
type Review struct {
	Entry     *model.Entry     `json:"entry,omitempty"`
	Report    model.Report     `json:"report"`
	Decisions []ReviewDecision `json:"decisions"`
}

// ReviewDecision is a decision the review carries, with what has become of
// it: settled is absent while it is open.
type ReviewDecision struct {
	model.ReportDecision
	Settled *model.Settlement `json:"settled,omitempty"`
}

// ItemPage reads one item's page.
func (m *Mull) ItemPage(id string) (ItemPage, error) {
	var out ItemPage
	err := m.view(func(s *state) error {
		i, err := s.item(id)
		if err != nil {
			return err
		}
		g, err := m.readGit(s)
		if err != nil {
			return err
		}
		out = ItemPage{Item: s.itemView(i), Reports: []ReportRef{},
			NeedsYou: s.needsYou(func(e *model.Entry) bool { return e.Item == id })}
		if p := s.plans[i.Plan]; p != nil {
			out.PlanTitle = p.Title
		}
		if ph := s.phases[i.Phase]; ph != nil {
			out.PhaseTitle, out.PhaseState = ph.Title, s.phaseState(ph)
		}
		if ig := g.Items[id]; ig != nil {
			out.Git = *ig
		}
		out.Review, out.Reports = s.review(i)
		return nil
	})
	return out, err
}

// review builds the item's review packet from its newest review entry and
// the report it names, or from its latest report when no review was filed.
func (s *state) review(i *model.Item) (*Review, []ReportRef) {
	refs := []ReportRef{}
	reports := map[string]model.Report{}
	latest := ""
	for _, p := range s.tx.List("reports/" + i.ID + "-") {
		var r model.Report
		b, _ := s.tx.Get(p)
		if _, err := model.Decode(b, &r); err != nil || r.Item != i.ID {
			continue
		}
		reports[p] = r
		refs = append(refs, ReportRef{N: r.N, Passed: r.Passed, At: r.At})
		if latest == "" || r.N > reports[latest].N {
			latest = p
		}
	}
	sort.Slice(refs, func(a, b int) bool { return refs[a].N < refs[b].N })
	var entry *model.Entry
	for _, id := range sortedKeys(s.entries) {
		e := s.entries[id]
		if e.Kind == model.KindReview && e.Item == i.ID && (entry == nil || e.Created >= entry.Created) {
			entry = e
		}
	}
	if latest == "" {
		return nil, refs
	}
	if entry != nil && entry.Source != latest {
		entry = nil // the newest review is of an older report: the latest attempt wasn't filed
	}
	rep := reports[latest]
	out := &Review{Entry: entry, Report: rep, Decisions: []ReviewDecision{}}
	for _, d := range rep.Decisions {
		rd := ReviewDecision{ReportDecision: d}
		if e := s.entries[d.ID]; e != nil {
			rd.Settled = e.Settled
		}
		out.Decisions = append(out.Decisions, rd)
	}
	return out, refs
}

// Unplanned is the project's unplanned work: committed items with no plan,
// by stage, and what needs you about them.
type Unplanned struct {
	Items    []ItemRow     `json:"items"`
	NeedsYou NeedsYouTiers `json:"needs_you"`
}

// UnplannedPage reads the project's unplanned work.
func (m *Mull) UnplannedPage() (Unplanned, error) {
	var out Unplanned
	err := m.view(func(s *state) error {
		out = Unplanned{Items: s.itemRows(unplanned), NeedsYou: s.needsYou(func(e *model.Entry) bool {
			i := s.items[e.Item]
			return i != nil && unplanned(i)
		})}
		return nil
	})
	return out, err
}

// Backlog is the captured items, untriaged first, then by category and
// age, with what the UI filters by.
type Backlog struct {
	Items      []ItemRow     `json:"items"`
	Bugs       int           `json:"bugs"`
	Ideas      int           `json:"ideas"`
	Untriaged  int           `json:"untriaged"`
	LastTriage string        `json:"last_triage,omitempty"`
	Categories []string      `json:"categories"` // every category used, sorted
	Plans      []SummaryPlan `json:"plans"`      // the plans items are tagged to
}

// BacklogPage reads the backlog: the whole project's, or with plan one
// plan's (model.NoPlan: the items tagged to no plan).
func (m *Mull) BacklogPage(plan string) (Backlog, error) {
	var out Backlog
	err := m.view(func(s *state) error {
		if plan != "" && plan != model.NoPlan {
			if _, err := s.plan(plan); err != nil {
				return err
			}
		}
		keep := func(i *model.Item) bool {
			if i.State != model.StateCaptured {
				return false
			}
			switch plan {
			case "":
				return true
			case model.NoPlan:
				return i.Plan == ""
			}
			return i.Plan == plan
		}
		out = Backlog{Items: s.itemRows(keep), Categories: []string{}, Plans: []SummaryPlan{}}
		sort.SliceStable(out.Items, func(a, b int) bool {
			x, y := out.Items[a], out.Items[b]
			if x.Untriaged != y.Untriaged {
				return x.Untriaged
			}
			return x.Category < y.Category
		})
		cats, plans := map[string]bool{}, map[string]bool{}
		for _, r := range out.Items {
			switch r.Kind {
			case model.ItemBug:
				out.Bugs++
			case model.ItemIdea:
				out.Ideas++
			}
			if r.Untriaged {
				out.Untriaged++
			}
			if r.Triaged > out.LastTriage {
				out.LastTriage = r.Triaged
			}
			cats[r.Category] = true
			plans[r.Plan] = true
		}
		for _, c := range sortedKeys(cats) {
			out.Categories = append(out.Categories, c)
		}
		for _, slug := range sortedKeys(plans) {
			if p := s.plans[slug]; p != nil {
				out.Plans = append(out.Plans, SummaryPlan{Slug: p.Slug, Title: p.Title, State: p.State})
			}
		}
		return nil
	})
	return out, err
}

// Tree is the project's sheet index: every plan not done, its phases in
// schedule order and their committed items, then unplanned work and the
// backlog, counted.
type Tree struct {
	Plans    []TreePlan `json:"plans"`
	Counts   Counts     `json:"counts"`
	Blocking int        `json:"blocking"`
}

// TreePlan is a plan in the tree.
type TreePlan struct {
	Slug     string      `json:"slug"`
	Title    string      `json:"title"`
	State    string      `json:"state"`
	Blocking int         `json:"blocking"`
	Phases   []TreePhase `json:"phases"`
}

// TreePhase is a phase in the tree.
type TreePhase struct {
	PhaseRow
	Items []TreeItem `json:"items"`
}

// TreeItem is an item in the tree.
type TreeItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	State string `json:"state"`
	Ready bool   `json:"ready,omitempty"`
}

// TreePage reads the project's sheet index.
func (m *Mull) TreePage() (Tree, error) {
	var out Tree
	err := m.view(func(s *state) error {
		out = Tree{Plans: []TreePlan{}, Counts: s.counts(), Blocking: s.countBlocking(nil)}
		for _, p := range s.notDonePlans() {
			tp := TreePlan{Slug: p.Slug, Title: p.Title, State: p.State, Phases: []TreePhase{},
				Blocking: s.countBlocking(func(e *model.Entry) bool { return e.Plan == p.Slug })}
			for _, g := range s.schedule(p.Slug) {
				for _, r := range g.Phases {
					ph := TreePhase{PhaseRow: r, Items: []TreeItem{}}
					for _, ir := range s.itemRows(func(i *model.Item) bool { return i.Phase == r.ID && i.State != model.StateCaptured }) {
						ph.Items = append(ph.Items, TreeItem{ID: ir.ID, Title: ir.Title, State: ir.State, Ready: ir.Ready})
					}
					tp.Phases = append(tp.Phases, ph)
				}
			}
			out.Plans = append(out.Plans, tp)
		}
		return nil
	})
	return out, err
}
