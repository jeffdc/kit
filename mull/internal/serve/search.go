package serve

import (
	"net/http"
	"sort"
	"strconv"
	"sync"

	"mull/internal/mull"
)

// SearchHit is a record search found, in a project.
type SearchHit struct {
	Project string `json:"project"` // the project's ID
	mull.Hit
}

// SearchResult is what GET /api/search returns: the best hits across the
// projects searched, best first, and how many records match in each status
// (the limit aside).
type SearchResult struct {
	Query    string      `json:"query"`
	Status   string      `json:"status"`            // open, finished or all
	Project  string      `json:"project,omitempty"` // the one project searched; every project when absent
	Hits     []SearchHit `json:"hits"`
	Open     int         `json:"open"`     // open records that match
	Finished int         `json:"finished"` // finished records that match
}

// searchLimit bounds the hits returned, across projects.
const searchLimit = 30

// registerSearch adds GET /api/search?q=[&status=][&project=]: records
// whose ID or title matches q. status is open (the default: live work),
// finished (done, dropped, settled or archived) or all; project, a project's
// ID, searches just that one, else every project mull serve reads. A
// project that can't be read is left out.
func registerSearch(mux *http.ServeMux, repos Repos) {
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		v := r.URL.Query()
		q, status, project := v.Get("q"), v.Get("status"), v.Get("project")
		switch status {
		case "":
			status = mull.SearchOpen
		case mull.SearchOpen, mull.SearchFinished, mull.SearchAll:
		default:
			writeMullError(w, &mull.Error{Code: mull.CodeInvalid, Msg: "no search status " + strconv.Quote(status) + ": open, finished or all"})
			return
		}
		paths, err := repos()
		if err != nil {
			writeError(w, http.StatusInternalServerError, mull.AsError(err))
			return
		}
		ids := projectIDs(paths)
		if project != "" {
			at := -1
			for i, id := range ids {
				if id == project {
					at = i
				}
			}
			if at < 0 {
				writeMullError(w, &mull.Error{Code: mull.CodeNotFound, Msg: "no project " + strconv.Quote(project)})
				return
			}
			paths, ids = paths[at:at+1], ids[at:at+1]
		}
		found, counts := make([][]SearchHit, len(paths)), make([]mull.Found, len(paths))
		var wg sync.WaitGroup
		for i, p := range paths {
			wg.Add(1)
			go func() {
				defer wg.Done()
				m, err := mull.Open(p)
				if err != nil {
					return
				}
				res, err := m.Search(q, searchLimit, status)
				if err != nil {
					return
				}
				counts[i] = res
				for _, h := range res.Hits {
					found[i] = append(found[i], SearchHit{Project: ids[i], Hit: h})
				}
			}()
		}
		wg.Wait()
		out := SearchResult{Query: q, Status: status, Project: project, Hits: []SearchHit{}}
		for i, hs := range found {
			out.Hits = append(out.Hits, hs...)
			out.Open += counts[i].Open
			out.Finished += counts[i].Finished
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
