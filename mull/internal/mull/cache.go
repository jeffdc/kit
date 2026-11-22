package mull

import (
	"sync"

	"mull/internal/model"
	"mull/internal/store"
)

// Reads share parsed states. A state read at a mull commit is the same for
// as long as anyone asks (a commit names its content exactly), so one parse
// serves every read of that commit, across requests and repositories, until
// it falls out of the cache. Writes never use it: they load the branch
// afresh inside their compare-and-swap.
//
// A cached state is shared, so it must not change: its transaction refuses
// writes (store.ReadOnly), and a read must not modify the records it gets.
// The cache tests check both.

// cachedStates bounds the states kept: enough for the tip and the last-looked
// commit of a few repositories.
const cachedStates = 8

var states = newLRU[*state](cachedStates)

// lru is a small least-recently-used cache whose values, once loaded, never
// change.
type lru[V any] struct {
	mu      sync.Mutex
	max     int
	entries map[string]*lruEntry[V]
	order   []string // keys, least recently used first
}

type lruEntry[V any] struct {
	done chan struct{} // closed once v and err are set
	v    V
	err  error
}

func newLRU[V any](max int) *lru[V] {
	return &lru[V]{max: max, entries: map[string]*lruEntry[V]{}}
}

// get returns the value for key, loading it with load if no caller has.
// Concurrent callers for the same key share one load. A failed load isn't
// kept, so the next caller tries again.
func (c *lru[V]) get(key string, load func() (V, error)) (V, error) {
	c.mu.Lock()
	e, ok := c.entries[key]
	if ok {
		c.order = append(remove(c.order, key), key)
		c.mu.Unlock()
		<-e.done
		return e.v, e.err
	}
	e = &lruEntry[V]{done: make(chan struct{})}
	c.entries[key] = e
	c.order = append(c.order, key)
	for len(c.order) > c.max {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
	c.mu.Unlock()

	e.v, e.err = load()
	close(e.done)
	if e.err != nil {
		c.mu.Lock()
		if c.entries[key] == e {
			delete(c.entries, key)
			c.order = remove(c.order, key)
		}
		c.mu.Unlock()
	}
	return e.v, e.err
}

func remove(xs []string, x string) []string {
	for i, y := range xs {
		if y == x {
			return append(xs[:i:i], xs[i+1:]...)
		}
	}
	return xs
}

// latest is the state each repository (by working tree) loaded last: a new
// commit is read as the difference from it, reading and parsing only the
// files that changed.
var latest = struct {
	sync.Mutex
	by map[string]*state
}{by: map[string]*state{}}

// cachedState returns the state at commit from the cache, loading it if
// needed.
func (m *Mull) cachedState(commit string) (*state, error) {
	return states.get(commit, func() (*state, error) {
		latest.Lock()
		prev := latest.by[m.repo.Dir]
		latest.Unlock()
		var prevTx *store.Tx
		if prev != nil {
			prevTx = prev.tx
		}
		tx, err := m.store.ReadOnly(commit, prevTx)
		if err != nil {
			return nil, err
		}
		s, err := loadFrom(tx, prev)
		if err != nil {
			return nil, err
		}
		s.index()
		latest.Lock()
		latest.by[m.repo.Dir] = s
		latest.Unlock()
		return s, nil
	})
}

// index builds the lookups a read-only state answers from, so a read of
// every item doesn't scan every item for each one.
func (s *state) index() {
	s.byPhase = map[string][]*model.Item{}
	for _, id := range sortedKeys(s.items) {
		i := s.items[id]
		s.byPhase[i.Phase] = append(s.byPhase[i.Phase], i)
	}
	s.byItem = map[string][]*model.Entry{}
	for _, id := range sortedKeys(s.entries) {
		if e := s.entries[id]; e.Item != "" {
			s.byItem[e.Item] = append(s.byItem[e.Item], e)
		}
	}
	s.search = s.searchDocs() // after byItem: whether an item is archived reads its entries
}

// itemEntries are the inbox entries on item id, in ID order.
func (s *state) itemEntries(id string) []*model.Entry {
	if s.byItem != nil {
		return s.byItem[id]
	}
	var out []*model.Entry
	for _, eid := range sortedKeys(s.entries) {
		if e := s.entries[eid]; e.Item == id {
			out = append(out, e)
		}
	}
	return out
}

// headState returns the state at the mull branch's tip.
func (m *Mull) headState() (*state, error) {
	commit, err := m.store.Head()
	if err != nil {
		return nil, err
	}
	return m.cachedState(commit)
}

// Git history is cached the same way: a log or count named by commit IDs
// alone (never by branch name) gives the same answer for as long as the
// commits exist. Keys carry the working tree, so repositories don't share.
var gitLogs = newLRU[[]commitInfo](64)

var gitCounts = newLRU[int](64)

// targetHistories are target branches' histories, indexed (see history),
// keyed by tip like gitLogs. Shared, so never changed once built.
var targetHistories = newLRU[*history](16)

// Preload reads the state at commit, a full commit ID on the mull branch,
// into the read cache, so the next read of it is answered from memory. The
// cache is keyed by commit, so a ref (whose commit moves) is refused.
func (m *Mull) Preload(commit string) error {
	if !commitPattern.MatchString(commit) {
		return errf(CodeInvalid, "preload takes a full commit ID, not %q", commit)
	}
	_, err := m.cachedState(commit)
	return err
}
