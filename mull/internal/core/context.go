package core

import (
	"strings"

	"mull/internal/guide"
)

// BriefItem is an item in a brief.
type BriefItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Plan  string `json:"plan,omitempty"`
	Phase string `json:"phase,omitempty"`
}

func brief(i *Item) BriefItem {
	return BriefItem{ID: i.ID, Title: i.Title, Plan: i.Plan, Phase: i.Phase}
}

// Context is an agent's brief: the protocol, the rulebook, and either the
// item it works on or what's ready.
type Context struct {
	Protocol []string    `json:"protocol"`
	Rules    []Rule      `json:"rules"`
	Item     *ItemBrief  `json:"item,omitempty"`
	Ready    []BriefItem `json:"ready"`
	Active   []BriefItem `json:"active"`
}

// ItemBrief is the item worked on, with what bears on it.
type ItemBrief struct {
	*Item
	Target    string      `json:"target"`
	Preview   string      `json:"preview,omitempty"`
	PlanDoc   string      `json:"plan_doc,omitempty"`
	PhaseDoc  string      `json:"phase_doc,omitempty"`
	Queue     []BriefItem `json:"queue,omitempty"`     // queued work in its area or phase
	Questions []BriefItem `json:"questions,omitempty"` // its open questions (ID and text)
}

var protocol = []string{
	"Use only the mull CLI; never read or edit the mull branch.",
	"mull start <id> makes your branch and worktree and prints the path: work there. End each commit message with 'Mull: <id>'.",
	"Follow the rules below (RULES.md). If your work bends, breaks or adds one, say which in done's --rules: it waits for review. Never change RULES.md or a ratchet file unless the human asks.",
	"Decide locally; note a choice worth keeping with mull decided --item <id>. Ask (mull ask --default ... --by ...) only about a rule conflict or something hard to undo.",
	"Fix small problems in your area as part of your change; queue the rest in one line: mull bug \"<what>\" --area <path>.",
	"Finish with mull done <id> --summary \"<one line>\" --rules none (--covers <ids> for queued work you also did). If the target moved, git merge it and finish again. Then mull sync.",
}

// Brief builds an agent's brief, for item when given.
func (m *Mull) Brief(itemID string) (Context, error) {
	c := Context{Protocol: protocol, Rules: []Rule{}, Ready: []BriefItem{}, Active: []BriefItem{}}
	if name := m.Human(); name != "" {
		c.Protocol = []string{guide.NameRule(name)}
		for _, l := range protocol {
			c.Protocol = append(c.Protocol, guide.Personal(l, name))
		}
	}
	rules, err := m.readRules()
	if err != nil {
		return c, err
	}
	if rules != nil {
		c.Rules = rules
	}
	err = m.view(func(s *state) error {
		for _, id := range sortedKeys(s.items) {
			i := s.items[id]
			switch {
			case i.State == Active:
				c.Active = append(c.Active, brief(i))
			case s.ready(i) && len(c.Ready) < 15:
				c.Ready = append(c.Ready, brief(i))
			}
		}
		if itemID == "" {
			return nil
		}
		i, err := s.item(itemID)
		if err != nil {
			return err
		}
		ib := &ItemBrief{Item: i, Target: s.targetOf(i.Plan), Queue: s.queueFor(i)}
		if p := s.plans[i.Plan]; p != nil {
			ib.Preview, ib.PlanDoc = p.Preview, p.Doc
		}
		if ph := s.phases[i.Phase]; ph != nil {
			ib.PhaseDoc = strings.TrimSpace(ph.Summary + "\n\n" + ph.Doc)
		}
		for _, id := range sortedKeys(s.entries) {
			if e := s.entries[id]; e.Item == itemID && e.Kind == KindQuestion && e.Settled == nil {
				ib.Questions = append(ib.Questions, BriefItem{ID: e.ID, Title: e.Text})
			}
		}
		c.Item = ib
		return nil
	})
	return c, err
}
