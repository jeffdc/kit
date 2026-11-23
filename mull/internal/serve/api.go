package serve

import (
	"net/http"

	"mull/internal/mull"
	"mull/internal/seen"
)

// registerAPI adds the read-only endpoints for one project's pages. Each
// reads one snapshot through internal/mull; the logic (ranking, now and
// next) lives there.
func registerAPI(mux *http.ServeMux, repos Repos) {
	page := func(pattern string, read func(m *mull.Mull, r *http.Request) (any, error)) {
		mux.HandleFunc("GET "+pattern, func(w http.ResponseWriter, r *http.Request) {
			path, err := resolve(repos, r.PathValue("project"))
			if err != nil {
				writeMullError(w, err)
				return
			}
			m, err := mull.Open(path)
			if err != nil {
				writeMullError(w, err)
				return
			}
			v, err := read(m, r)
			if err != nil {
				writeMullError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, v)
		})
	}
	page("/api/projects/{project}", func(m *mull.Mull, r *http.Request) (any, error) {
		return m.Overview()
	})
	page("/api/projects/{project}/tree", func(m *mull.Mull, r *http.Request) (any, error) {
		return m.TreePage()
	})
	page("/api/projects/{project}/plans/{plan}", func(m *mull.Mull, r *http.Request) (any, error) {
		return m.PlanPage(r.PathValue("plan"))
	})
	page("/api/projects/{project}/phases/{phase}", func(m *mull.Mull, r *http.Request) (any, error) {
		return m.PhasePage(r.PathValue("phase"))
	})
	page("/api/projects/{project}/items/{item}", func(m *mull.Mull, r *http.Request) (any, error) {
		return m.ItemPage(r.PathValue("item"))
	})
	page("/api/projects/{project}/unplanned", func(m *mull.Mull, r *http.Request) (any, error) {
		return m.UnplannedPage(r.URL.Query().Get("done") == "all")
	})
	page("/api/projects/{project}/backlog", func(m *mull.Mull, r *http.Request) (any, error) {
		return m.BacklogPage(r.URL.Query().Get("plan"))
	})
	// The surface: what changed since the human last looked here.
	page("/api/projects/{project}/surface", func(m *mull.Mull, r *http.Request) (any, error) {
		since := r.URL.Query().Get("since")
		if since == "" {
			if look, ok, err := seen.Get(m.MainWorktree()); err == nil && ok {
				since = look.At
			}
		}
		return m.Surface(since)
	})
	page("/api/projects/{project}/rules", func(m *mull.Mull, r *http.Request) (any, error) {
		return m.Rules()
	})
}
