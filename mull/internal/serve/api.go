package serve

import (
	"net/http"

	"mull/internal/mull"
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
		return m.UnplannedPage()
	})
	page("/api/projects/{project}/backlog", func(m *mull.Mull, r *http.Request) (any, error) {
		return m.BacklogPage(r.URL.Query().Get("plan"))
	})
}
