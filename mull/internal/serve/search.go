package serve

import (
	"net/http"
	"sort"
	"sync"

	"mull/internal/mull"
)

// SearchHit is a record search found, in a project.
type SearchHit struct {
	Project string `json:"project"` // the project's ID
	mull.Hit
}

// SearchResult is what GET /api/search returns: the best hits across every
// project, best first.
type SearchResult struct {
	Query string      `json:"query"`
	Hits  []SearchHit `json:"hits"`
}

// searchLimit bounds the hits returned, across projects.
const searchLimit = 30

// registerSearch adds GET /api/search?q=: records whose ID or title matches
// q, in every project mull serve reads. A project that can't be read is
// left out.
func registerSearch(mux *http.ServeMux, repos Repos) {
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		paths, err := repos()
		if err != nil {
			writeError(w, http.StatusInternalServerError, mull.AsError(err))
			return
		}
		ids := projectIDs(paths)
		found := make([][]SearchHit, len(paths))
		var wg sync.WaitGroup
		for i, p := range paths {
			wg.Add(1)
			go func() {
				defer wg.Done()
				m, err := mull.Open(p)
				if err != nil {
					return
				}
				hits, err := m.Search(q, searchLimit)
				if err != nil {
					return
				}
				for _, h := range hits {
					found[i] = append(found[i], SearchHit{Project: ids[i], Hit: h})
				}
			}()
		}
		wg.Wait()
		out := SearchResult{Query: q, Hits: []SearchHit{}}
		for _, hs := range found {
			out.Hits = append(out.Hits, hs...)
		}
		// Each project's hits are in order; across projects, by rank (a
		// stable sort keeps project order among equals).
		sort.SliceStable(out.Hits, func(a, b int) bool { return out.Hits[a].Rank < out.Hits[b].Rank })
		if len(out.Hits) > searchLimit {
			out.Hits = out.Hits[:searchLimit]
		}
		writeJSON(w, http.StatusOK, out)
	})
}
