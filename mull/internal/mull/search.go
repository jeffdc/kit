package mull

import (
	"sort"
	"strings"

	"mull/internal/model"
)

// Hit is a record search found: enough to show it and to open it.
type Hit struct {
	Type  string `json:"type"` // plan, phase, item or entry
	ID    string `json:"id"`   // a plan's slug, or the record's ID
	Title string `json:"title"`
	Plan  string `json:"plan,omitempty"`
	Item  string `json:"item,omitempty"` // an entry's item
	Kind  string `json:"kind,omitempty"` // an entry's kind, or a captured item's
	State string `json:"state,omitempty"`
	Rank  int    `json:"rank"` // lower is better: 0 the ID, 1 starts the ID, 2 starts words of the title, 3 inside it
	// Archived means it's in the archive: a closed plan, or a record in one;
	// finished unplanned work (archivedUnplanned), or an entry on it.
	Archived bool   `json:"archived,omitempty"`
	at       string // last change, newest first among equal ranks
}

// Found is what Search found: the hits, and how many more matches the
// archive holds when it wasn't searched.
type Found struct {
	Hits     []Hit `json:"hits"`
	Archived int   `json:"archived"` // matches in the archive left out
}

// searchDoc is one record as search reads it: its hit, and the ID and title
// lower-cased once.
type searchDoc struct {
	hit       Hit
	id, title string
}

// searchDocs lists every record for search, in type order (plans, phases,
// items, entries) then by ID.
func (s *state) searchDocs() []searchDoc {
	if s.search != nil {
		return s.search
	}
	var out []searchDoc
	add := func(h Hit) {
		if p := s.plans[h.Plan]; p != nil && p.State == model.PlanDone {
			h.Archived = true
		}
		if i := s.items[h.Item]; i != nil && s.archivedUnplanned(i) {
			h.Archived = true // an entry on archived unplanned work
		}
		out = append(out, searchDoc{hit: h, id: strings.ToLower(h.ID), title: strings.ToLower(h.Title)})
	}
	for _, slug := range sortedKeys(s.plans) {
		p := s.plans[slug]
		add(Hit{Type: RecordPlan, ID: slug, Title: p.Title, Plan: slug, State: p.State, at: p.Updated})
	}
	for _, id := range sortedKeys(s.phases) {
		p := s.phases[id]
		add(Hit{Type: RecordPhase, ID: id, Title: p.Title, Plan: p.Plan, at: p.Updated})
	}
	for _, id := range sortedKeys(s.items) {
		i := s.items[id]
		add(Hit{Type: RecordItem, ID: id, Title: i.Title, Plan: i.Plan, Kind: i.Kind, State: i.State, at: i.Updated, Archived: s.archivedUnplanned(i)})
	}
	for _, id := range sortedKeys(s.entries) {
		e := s.entries[id]
		h := Hit{Type: RecordEntry, ID: id, Title: e.Text, Plan: e.Plan, Item: e.Item, Kind: e.Kind, State: "open", at: e.Created}
		if e.Settled != nil {
			h.State, h.at = e.Settled.Action, later(e.Created, e.Settled.At)
		}
		if i := s.items[e.Item]; i != nil && h.Plan == "" {
			h.Plan = i.Plan
		}
		add(h)
	}
	return out
}

// Search finds records by ID or title: every word of q must start the ID or
// appear in the title. Best first: the ID itself, then IDs it starts, then
// titles whose words start with q's, then titles containing them; among
// equals, the most recently changed first. At most limit hits. The archive
// (closed plans and what's in them, and finished unplanned work and the
// entries on it) is searched only with archive; without it, Found counts
// the matches it holds.
func (m *Mull) Search(q string, limit int, archive bool) (Found, error) {
	found := Found{Hits: []Hit{}}
	words := strings.Fields(strings.ToLower(q))
	if len(words) == 0 || limit <= 0 {
		return found, nil
	}
	var out []Hit
	err := m.view(func(s *state) error {
		for _, d := range s.searchDocs() {
			rank, ok := match(d, words)
			if !ok {
				continue
			}
			if d.hit.Archived && !archive {
				found.Archived++
				continue
			}
			h := d.hit
			h.Rank = rank
			out = append(out, h)
		}
		return nil
	})
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Rank != out[b].Rank {
			return out[a].Rank < out[b].Rank
		}
		return out[a].at > out[b].at
	})
	if len(out) > limit {
		out = out[:limit]
	}
	if out != nil {
		found.Hits = out
	}
	return found, err
}

// match ranks d against the query's words (see Hit.Rank).
func match(d searchDoc, words []string) (int, bool) {
	if len(words) == 1 {
		switch w := words[0]; {
		case d.id == w:
			return 0, true
		case strings.HasPrefix(d.id, w):
			return 1, true
		}
	}
	rank := 2
	for _, w := range words {
		i := strings.Index(d.title, w)
		if i < 0 {
			return 0, false
		}
		if !startsWord(d.title, w) {
			rank = 3
		}
	}
	return rank, true
}

// startsWord reports whether w starts a word of title.
func startsWord(title, w string) bool {
	for i := 0; i+len(w) <= len(title); {
		j := strings.Index(title[i:], w)
		if j < 0 {
			return false
		}
		at := i + j
		if at == 0 || !isWordByte(title[at-1]) {
			return true
		}
		i = at + 1
	}
	return false
}

func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b >= 0x80
}
