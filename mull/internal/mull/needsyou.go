package mull

import (
	"sort"

	"mull/internal/model"
)

// Needs you is ranked by what an answer unblocks, in six tiers. Each tier
// keeps its rank when others are empty, so the UI can number them 01–06.
const (
	TierBlocking = "blocking" // a question on active or ready committed work
	TierReview   = "review"   // finished work waiting for review
	TierNow      = "now"      // a question on an item in a phase that can start (or has)
	TierLater    = "later"    // a question on an item in a phase still waiting for its needs
	TierPlan     = "plan"     // a question about a whole plan, the project, or an item in no phase
	TierDecide   = "decide"   // decisions agents made, to approve, grouped by source
)

var tierInfo = []struct{ key, title, hint string }{
	{TierBlocking, "Blocking work", "Committed work that can't finish without your answer"},
	{TierReview, "Waiting for your review", "Not done until you accept it"},
	{TierNow, "Questions for the current phases", "Will block soon"},
	{TierLater, "Questions for later phases", "Can wait for their phase"},
	{TierPlan, "Questions about the whole plan", "Shape the plan, not one phase"},
	{TierDecide, "Decisions to approve", "Choices agents made; approve, or decide differently"},
}

// NeedsYouTiers is the ranked Needs you schedule: only the tiers that have
// entries, in rank order. Parked entries are counted, not listed.
type NeedsYouTiers struct {
	Open   int    `json:"open"`
	Parked int    `json:"parked"`
	Tiers  []Tier `json:"tiers"`
}

// Tier is one rank of Needs you. The decide tier holds groups; every other
// tier holds entries. Within a tier (and a group) entries are oldest first.
type Tier struct {
	Key     string          `json:"key"`
	Rank    int             `json:"rank"` // 1 (blocking) to 6 (decide)
	Title   string          `json:"title"`
	Hint    string          `json:"hint"`
	Count   int             `json:"count"`
	Entries []NeedRow       `json:"entries,omitempty"`
	Groups  []DecisionGroup `json:"groups,omitempty"`
}

// NeedRow is an open inbox entry placed in the schedule, with where it is.
type NeedRow struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Text       string   `json:"text"`
	Detail     string   `json:"detail,omitempty"`
	Plan       string   `json:"plan,omitempty"`
	PlanTitle  string   `json:"plan_title,omitempty"`
	Phase      string   `json:"phase,omitempty"` // the item's phase
	PhaseTitle string   `json:"phase_title,omitempty"`
	Item       string   `json:"item,omitempty"`
	ItemTitle  string   `json:"item_title,omitempty"`
	ItemState  string   `json:"item_state,omitempty"`
	ItemReady  bool     `json:"item_ready,omitempty"`
	Source     string   `json:"source,omitempty"`
	Created    string   `json:"created"`
	Decisions  []string `json:"decisions,omitempty"` // a review's carried decisions
}

// DecisionGroup is decisions from one source: the item they were made in,
// else their source text, else their plan, else the whole project. Review
// is the open review carrying them, when there is one: accepting it
// ratifies them.
type DecisionGroup struct {
	Key     string    `json:"key"` // item:<id>, source:<text>, plan:<slug> or project
	Title   string    `json:"title"`
	Item    string    `json:"item,omitempty"`
	Plan    string    `json:"plan,omitempty"`
	Review  string    `json:"review,omitempty"`
	Entries []NeedRow `json:"entries"`
}

// tier ranks an open entry. Questions are placed by their item: active or
// ready work is blocking; otherwise by whether the item's phase can start.
// A question with no item, or on an item in no phase that isn't ready, is
// about the whole plan (or, with no plan, the project).
func (s *state) tier(e *model.Entry) string {
	switch e.Kind {
	case model.KindReview:
		return TierReview
	case model.KindDecision:
		return TierDecide
	}
	i := s.items[e.Item]
	if i == nil {
		return TierPlan
	}
	if i.State == model.StateActive || s.itemView(i).Ready {
		return TierBlocking
	}
	if ph := s.phases[i.Phase]; ph != nil {
		if len(s.blockedBy(ph.Needs)) > 0 {
			return TierLater
		}
		return TierNow
	}
	return TierPlan
}

// blocking reports whether e is an open question in the blocking tier.
func (s *state) blocking(e *model.Entry) bool {
	return e.Settled == nil && e.Kind == model.KindQuestion && s.tier(e) == TierBlocking
}

// countBlocking counts the open blocking questions keep selects.
func (s *state) countBlocking(keep func(*model.Entry) bool) int {
	n := 0
	for _, e := range s.entries {
		if s.blocking(e) && (keep == nil || keep(e)) {
			n++
		}
	}
	return n
}

// entryPhase is the phase an entry is about: its item's.
func (s *state) entryPhase(e *model.Entry) string {
	if i := s.items[e.Item]; i != nil {
		return i.Phase
	}
	return ""
}

func (s *state) needRow(e *model.Entry) NeedRow {
	r := NeedRow{ID: e.ID, Kind: e.Kind, Text: e.Text, Detail: e.Detail, Plan: e.Plan, Item: e.Item,
		Source: e.Source, Created: e.Created, Decisions: e.Decisions}
	if p := s.plans[e.Plan]; p != nil {
		r.PlanTitle = p.Title
	}
	if i := s.items[e.Item]; i != nil {
		v := s.itemView(i)
		r.ItemTitle, r.ItemState, r.ItemReady = i.Title, i.State, v.Ready
		if ph := s.phases[i.Phase]; ph != nil {
			r.Phase, r.PhaseTitle = ph.ID, ph.Title
		}
	}
	return r
}

// openEntries returns the open entries keep selects, of the kinds that need
// the human, oldest first, and how many of them are parked.
func (s *state) openEntries(keep func(*model.Entry) bool) (open []*model.Entry, parked int) {
	for _, id := range sortedKeys(s.entries) {
		e := s.entries[id]
		if retiredKind(e.Kind) || (keep != nil && !keep(e)) {
			continue
		}
		if e.Settled != nil {
			if e.Settled.Action == model.ActPark {
				parked++
			}
			continue
		}
		open = append(open, e)
	}
	sort.SliceStable(open, func(a, b int) bool { return open[a].Created < open[b].Created })
	return open, parked
}

// needsYou ranks the open entries keep selects (nil: all of them).
func (s *state) needsYou(keep func(*model.Entry) bool) NeedsYouTiers {
	open, parked := s.openEntries(keep)
	out := NeedsYouTiers{Open: len(open), Parked: parked, Tiers: []Tier{}}

	// Decisions carried by an open review, so their group can point at it.
	carriedBy := map[string]string{}
	for _, e := range open {
		if e.Kind == model.KindReview {
			for _, d := range e.Decisions {
				carriedBy[d] = e.ID
			}
		}
	}

	byTier := map[string][]*model.Entry{}
	for _, e := range open {
		t := s.tier(e)
		byTier[t] = append(byTier[t], e)
	}
	for rank, info := range tierInfo {
		list := byTier[info.key]
		if len(list) == 0 {
			continue
		}
		t := Tier{Key: info.key, Rank: rank + 1, Title: info.title, Hint: info.hint, Count: len(list)}
		if info.key == TierDecide {
			t.Groups = s.decisionGroups(list, carriedBy)
		} else {
			for _, e := range list {
				t.Entries = append(t.Entries, s.needRow(e))
			}
		}
		out.Tiers = append(out.Tiers, t)
	}
	return out
}

// decisionGroups groups decisions by source, groups ordered by their oldest
// decision.
func (s *state) decisionGroups(list []*model.Entry, carriedBy map[string]string) []DecisionGroup {
	var groups []DecisionGroup
	index := map[string]int{}
	for _, e := range list {
		var g DecisionGroup
		switch i := s.items[e.Item]; {
		case i != nil:
			g = DecisionGroup{Key: "item:" + i.ID, Title: i.Title, Item: i.ID, Plan: i.Plan}
		case e.Source != "":
			g = DecisionGroup{Key: "source:" + e.Source, Title: e.Source, Plan: e.Plan}
		case s.plans[e.Plan] != nil:
			g = DecisionGroup{Key: "plan:" + e.Plan, Title: s.plans[e.Plan].Title, Plan: e.Plan}
		default:
			g = DecisionGroup{Key: "project", Title: "The whole project"}
		}
		k, ok := index[g.Key]
		if !ok {
			k = len(groups)
			index[g.Key] = k
			groups = append(groups, g)
		}
		groups[k].Entries = append(groups[k].Entries, s.needRow(e))
		if r := carriedBy[e.ID]; r != "" && groups[k].Review == "" {
			groups[k].Review = r
		}
	}
	return groups
}
