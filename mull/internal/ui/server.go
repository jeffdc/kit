// Package ui is mull serve: the human's view of every repository on
// mull, as a local web server on 127.0.0.1. It serves the UI embedded in
// the binary and a small JSON API over the core. It writes mull data only
// as the human, through the ops the UI offers, and otherwise only the
// human's last look (package seen).
package ui

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"mull/internal/core"
	"mull/internal/seen"
)

// DefaultPort is mull serve's port: "MULL" on a phone keypad.
const DefaultPort = 6855

//go:embed web
var web embed.FS

// Repos lists the registered repositories.
type Repos func() ([]string, error)

// uiOps are the batch ops a page may send.
var uiOps = map[string]bool{"accept": true, "decide": true, "send back": true, "drop entry": true,
	"bug": true, "idea": true, "plan agree": true, "plan close": true}

// Handler serves the UI and the API. It reads the registry on every
// request, so new repositories show up without a restart.
func Handler(repos Repos) http.Handler {
	static, err := fs.Sub(web, "web")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects", func(w http.ResponseWriter, r *http.Request) {
		paths, err := repos()
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, ProjectList{Projects: projects(paths)})
	})
	read := func(pattern string, fn func(m *core.Mull, path string, r *http.Request) (any, error)) {
		mux.HandleFunc("GET "+pattern, func(w http.ResponseWriter, r *http.Request) {
			m, path, err := open(repos, r.PathValue("project"))
			if err != nil {
				writeError(w, err)
				return
			}
			v, err := fn(m, path, r)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, v)
		})
	}
	read("/api/projects/{project}/surface", func(m *core.Mull, path string, r *http.Request) (any, error) {
		since := r.URL.Query().Get("since")
		if look, ok, err := seen.Get(path); since == "" && err == nil && ok {
			since = look.At
		}
		return m.Surface(since)
	})
	read("/api/projects/{project}/rules", func(m *core.Mull, _ string, _ *http.Request) (any, error) { return m.Rules() })
	read("/api/projects/{project}/queue", func(m *core.Mull, _ string, _ *http.Request) (any, error) {
		items, err := m.List(core.Filter{State: core.Captured})
		return map[string]any{"items": items}, err
	})
	read("/api/projects/{project}/show/{ref}", func(m *core.Mull, _ string, r *http.Request) (any, error) {
		return m.Show(r.PathValue("ref"))
	})
	mux.HandleFunc("POST /api/projects/{project}/ops", func(w http.ResponseWriter, r *http.Request) {
		if status, e := writeAllowed(r); e != nil {
			writeJSON(w, status, e)
			return
		}
		var body struct {
			Ops []core.Line `json:"ops"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, &core.Error{Code: core.CodeInvalid, Msg: `want {"ops": [{"op": "<name>", ...}]}: ` + err.Error()})
			return
		}
		var lines []string
		for _, l := range body.Ops {
			if !uiOps[l.Op] {
				writeError(w, &core.Error{Code: core.CodeNotAllowed, Msg: "mull serve doesn't offer op " + strconv.Quote(l.Op)})
				return
			}
			l.OnRequest = false
			b, _ := json.Marshal(l)
			lines = append(lines, string(b))
		}
		m, _, err := open(repos, r.PathValue("project"))
		if err == nil {
			m, err = m.AsHuman()
		}
		if err != nil {
			writeError(w, err)
			return
		}
		res, err := m.Batch(strings.NewReader(strings.Join(lines, "\n")), false)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": res})
	})
	mux.HandleFunc("POST /api/projects/{project}/seen", func(w http.ResponseWriter, r *http.Request) {
		if status, e := writeAllowed(r); e != nil {
			writeJSON(w, status, e)
			return
		}
		_, path, err := open(repos, r.PathValue("project"))
		if err != nil {
			writeError(w, err)
			return
		}
		at := time.Now().UTC().Format(time.RFC3339)
		if err := seen.Mark(path, seen.Look{At: at}); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"seen_at": at})
	})
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, &core.Error{Code: core.CodeNotFound, Msg: "no API endpoint " + r.URL.Path})
	})
	files := http.FileServerFS(static)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
	return localOnly(mux)
}

// open opens the registered project with this ID.
func open(repos Repos, id string) (*core.Mull, string, error) {
	paths, err := repos()
	if err != nil {
		return nil, "", err
	}
	for i, pid := range projectIDs(paths) {
		if pid == id {
			m, err := core.Open(paths[i])
			return m, paths[i], err
		}
	}
	return nil, "", &core.Error{Code: core.CodeNotFound, Msg: "no project " + strconv.Quote(id)}
}

// ProjectList is GET /api/projects.
type ProjectList struct {
	Projects []Project `json:"projects"`
}

// Project is a registered repository: its summary, or why it can't be read.
type Project struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Path    string        `json:"path"`
	SeenAt  string        `json:"seen_at,omitempty"`
	Summary *core.Summary `json:"summary,omitempty"`
	Error   *core.Error   `json:"error,omitempty"`
}

func projects(paths []string) []Project {
	out := make([]Project, len(paths))
	ids := projectIDs(paths)
	var wg sync.WaitGroup
	for i, p := range paths {
		out[i] = Project{ID: ids[i], Name: filepath.Base(p), Path: p}
		wg.Add(1)
		go func(pr *Project) {
			defer wg.Done()
			m, err := core.Open(pr.Path)
			var sum core.Summary
			if err == nil {
				sum, err = m.Summarize()
			}
			if err != nil {
				pr.Error = core.AsError(err)
				return
			}
			pr.Summary = &sum
			if look, ok, _ := seen.Get(pr.Path); ok {
				pr.SeenAt = look.At
			}
		}(&out[i])
	}
	wg.Wait()
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

// projectIDs names each path by its directory, later duplicates -2, -3.
func projectIDs(paths []string) []string {
	ids := make([]string, len(paths))
	taken := map[string]bool{}
	for i, p := range paths {
		id := filepath.Base(p)
		for n := 2; taken[id]; n++ {
			id = filepath.Base(p) + "-" + strconv.Itoa(n)
		}
		taken[id], ids[i] = true, id
	}
	return ids
}

// localOnly refuses requests whose Host isn't this machine (DNS rebinding).
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "localhost" && host != "127.0.0.1" {
			writeJSON(w, http.StatusForbidden, &core.Error{Code: core.CodeNotAllowed, Msg: "mull serve answers only to localhost"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeAllowed refuses a write another site's page could make: writes come
// from the UI as JSON, never cross-site.
func writeAllowed(r *http.Request) (int, *core.Error) {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return http.StatusForbidden, &core.Error{Code: core.CodeNotAllowed, Msg: "writes come only from mull serve's own pages"}
	}
	if o := r.Header.Get("Origin"); o != "" {
		if u, err := url.Parse(o); err != nil || u.Scheme != "http" || u.Host != r.Host {
			return http.StatusForbidden, &core.Error{Code: core.CodeNotAllowed, Msg: "writes come only from mull serve's own pages"}
		}
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return http.StatusUnsupportedMediaType, &core.Error{Code: core.CodeNotAllowed, Msg: "a write is application/json"}
	}
	return 0, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes err with the HTTP status its code means.
func writeError(w http.ResponseWriter, err error) {
	e := core.AsError(err)
	status := http.StatusInternalServerError
	switch e.Code {
	case core.CodeNotFound:
		status = http.StatusNotFound
	case core.CodeInvalid, core.CodeBatchInvalid:
		status = http.StatusBadRequest
	case core.CodeNotAllowed:
		status = http.StatusForbidden
	case core.CodeBusy:
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, e)
}
