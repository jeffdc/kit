// Package serve is mull serve: the human's view of every repository on mull,
// as a local web server. It listens on 127.0.0.1 only, serves the UI
// embedded in the binary, and a JSON API over internal/mull. It never writes
// mull data; its one write is the last-looked state (package seen).
package serve

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"mull/internal/mull"
)

// DefaultPort is mull serve's fixed port: "MULL" on a phone keypad.
const DefaultPort = 6855

//go:embed web
var web embed.FS

// Repos lists the registered repositories (registry.Repos in production).
type Repos func() ([]string, error)

// Handler serves the UI and the API for the repositories repos lists. It
// reads the list on every request, so repositories registered while it runs
// show up without a restart. Options add endpoints (WithEvents).
func Handler(repos Repos, opts ...Option) http.Handler {
	static, err := fs.Sub(web, "web")
	if err != nil {
		panic(err) // the embedded tree is fixed at build time
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects", func(w http.ResponseWriter, r *http.Request) {
		paths, err := repos()
		if err != nil {
			writeError(w, http.StatusInternalServerError, mull.AsError(err))
			return
		}
		writeJSON(w, http.StatusOK, ProjectList{Projects: projects(paths)})
	})
	registerAPI(mux, repos)
	registerRevision(mux, repos)
	for _, opt := range opts {
		opt(mux)
	}
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, &mull.Error{Code: mull.CodeNotFound, Msg: "no API endpoint " + r.URL.Path})
	})
	files := http.FileServerFS(static)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache") // a new binary serves new files
		files.ServeHTTP(w, r)
	})
	return localOnly(mux)
}

// localOnly refuses requests whose Host isn't this machine, so a web page
// elsewhere can't read the API by pointing its own hostname at 127.0.0.1
// (DNS rebinding).
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		switch host {
		case "localhost", "127.0.0.1":
			next.ServeHTTP(w, r)
		default:
			writeError(w, http.StatusForbidden, &mull.Error{Code: mull.CodeNotAllowed, Msg: "mull serve answers only to localhost and 127.0.0.1, not " + strconv.Quote(r.Host)})
		}
	})
}

// ProjectList is GET /api/projects.
type ProjectList struct {
	Projects []Project `json:"projects"`
}

// Project is one registered repository. Exactly one of Summary and Error is
// set: a repository that can't be read (moved, deleted, no mull branch, a
// newer format) is listed with the reason.
type Project struct {
	ID      string        `json:"id"`   // the project's key in URLs: its name, made unique
	Name    string        `json:"name"` // the main worktree's directory name
	Path    string        `json:"path"`
	Summary *mull.Summary `json:"summary,omitempty"`
	Error   *mull.Error   `json:"error,omitempty"`
	// Revision is the tip's revision number and what changed since the
	// human last looked; with Summary only.
	Revision *ProjectRevision `json:"revision,omitempty"`
}

// projects reads every repository's summary, concurrently, sorted by ID.
func projects(paths []string) []Project {
	out := make([]Project, len(paths))
	ids := projectIDs(paths)
	var wg sync.WaitGroup
	for i, p := range paths {
		out[i] = Project{ID: ids[i], Name: filepath.Base(p), Path: p}
		wg.Add(1)
		go func(pr *Project) {
			defer wg.Done()
			sum, err := summary(pr.Path)
			if err != nil {
				pr.Error = mull.AsError(err)
				return
			}
			pr.Summary = &sum
			pr.Revision = projectRevision(pr.Path)
		}(&out[i])
	}
	wg.Wait()
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

func summary(path string) (mull.Summary, error) {
	m, err := mull.Open(path)
	if err != nil {
		return mull.Summary{}, err
	}
	return m.Summary()
}

// projectIDs names each path by its directory name. When names collide, the
// first registered keeps the plain name and later ones get -2, -3 and so on,
// so an ID stays put as more repositories are registered.
func projectIDs(paths []string) []string {
	ids := make([]string, len(paths))
	taken := map[string]bool{}
	for i, p := range paths {
		base := filepath.Base(p)
		id := base
		for n := 2; taken[id]; n++ {
			id = base + "-" + strconv.Itoa(n)
		}
		taken[id] = true
		ids[i] = id
	}
	return ids
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, e *mull.Error) {
	writeJSON(w, status, e)
}

// writeMullError writes err with the HTTP status its code means: a record
// or project that doesn't exist is 404, a bad value 400, a refused action
// 403, a registered project that can't be read 409, anything else 500.
func writeMullError(w http.ResponseWriter, err error) {
	e := mull.AsError(err)
	status := http.StatusInternalServerError
	switch e.Code {
	case mull.CodeNotFound:
		status = http.StatusNotFound
	case mull.CodeInvalid:
		status = http.StatusBadRequest
	case mull.CodeNotAllowed:
		status = http.StatusForbidden
	case mull.CodeNotRepo, mull.CodeNotInitialized, mull.CodeUnsupportedFormat:
		status = http.StatusConflict
	}
	writeError(w, status, e)
}
