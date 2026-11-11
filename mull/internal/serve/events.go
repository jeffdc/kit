package serve

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"mull/internal/mull"
)

// Option adds an endpoint to Handler.
type Option func(*http.ServeMux)

// WithEvents serves GET /api/events from w: the live updates.
func WithEvents(w *Watcher) Option {
	return func(mux *http.ServeMux) { mux.Handle("GET /api/events", w) }
}

// Event is one "project" event on GET /api/events: the project whose mull
// branch moved, or that was added to or removed from the registry.
type Event struct {
	ID string `json:"id"`
}

// subscriber is one open event stream. Announcements gather in pending
// until the stream writes them, so a slow client never blocks the watcher
// and never misses a project: repeats of one project collapse into one event.
type subscriber struct {
	pending map[string]bool // guarded by Watcher.mu
	wake    chan struct{}   // one slot; closed when the Watcher closes
}

func (w *Watcher) subscribe() *subscriber {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	s := &subscriber{pending: map[string]bool{}, wake: make(chan struct{}, 1)}
	w.subs[s] = true
	return s
}

func (w *Watcher) unsubscribe(s *subscriber) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.subs, s)
}

// publish announces project id to every open stream.
func (w *Watcher) publish(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for s := range w.subs {
		s.pending[id] = true
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// take returns and clears s's pending projects, sorted.
func (w *Watcher) take(s *subscriber) []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	ids := make([]string, 0, len(s.pending))
	for id := range s.pending {
		ids = append(ids, id)
	}
	clear(s.pending)
	sort.Strings(ids)
	return ids
}

// ServeHTTP is GET /api/events: a text/event-stream that opens with a
// comment, then sends a "project" event (data: Event) for each project that
// changed, and a keepalive comment when idle. It ends when the client goes
// away or the Watcher closes.
func (w *Watcher) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	s := w.subscribe()
	if s == nil {
		writeError(rw, http.StatusServiceUnavailable, &mull.Error{Code: mull.CodeInternal, Msg: "mull serve is shutting down"})
		return
	}
	defer w.unsubscribe(s)
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-store")
	rc := http.NewResponseController(rw)
	send := func(msg string) bool {
		if _, err := io.WriteString(rw, msg); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !send(": mull serve\n\n") {
		return
	}
	tick := time.NewTicker(w.keepalive)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case _, ok := <-s.wake:
			if !ok {
				return
			}
			var b strings.Builder
			for _, id := range w.take(s) {
				data, _ := json.Marshal(Event{ID: id})
				b.WriteString("event: project\ndata: " + string(data) + "\n\n")
			}
			if !send(b.String()) {
				return
			}
		case <-tick.C:
			if !send(": keepalive\n\n") {
				return
			}
		}
	}
}
