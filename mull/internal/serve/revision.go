package serve

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"time"

	"mull/internal/mull"
	"mull/internal/seen"
)

// Revisions: what changed since the human last looked. The last look is
// kept per machine by package seen; the changes come from mull.Revision.
// The first time mull serve reads a project, its tip becomes the baseline,
// so changes from then on are clouded.

// RevisionReport is GET /api/projects/{id}/revision: the revision schedule.
type RevisionReport struct {
	ID string `json:"id"`
	mull.Revision
	// SeenAt is when Since was marked seen, or when mull serve first read
	// the project (the baseline).
	SeenAt string `json:"seen_at"`
}

// ProjectRevision is a project's revision in the project list.
type ProjectRevision struct {
	Number  int    `json:"number"`  // the tip's revision number
	Changes int    `json:"changes"` // records changed since the last look
	SeenAt  string `json:"seen_at"`
}

func registerRevision(mux *http.ServeMux, repos Repos) {
	mux.HandleFunc("GET /api/projects/{id}/revision", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		path, err := resolve(repos, id)
		if err != nil {
			writeRevisionError(w, err)
			return
		}
		rev, at, err := revisionSince(path)
		if err != nil {
			writeRevisionError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, RevisionReport{ID: id, Revision: rev, SeenAt: at})
	})
	mux.HandleFunc("POST /api/projects/{id}/seen", func(w http.ResponseWriter, r *http.Request) {
		if status, e := writeAllowed(r); e != nil {
			writeError(w, status, e)
			return
		}
		var body struct {
			Commit string `json:"commit"` // the revision the page showed; the tip if empty
		}
		dec := json.NewDecoder(io.LimitReader(r.Body, 4096))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, &mull.Error{Code: mull.CodeInvalid, Msg: "want a JSON body {\"commit\"?: \"<mull branch commit>\"}: " + err.Error()})
			return
		}
		id := r.PathValue("id")
		path, err := resolve(repos, id)
		if err != nil {
			writeRevisionError(w, err)
			return
		}
		m, err := mull.Open(path)
		if err != nil {
			writeRevisionError(w, err)
			return
		}
		pt, err := m.RevisionAt(body.Commit)
		if err != nil {
			writeRevisionError(w, err)
			return
		}
		at := time.Now().UTC().Format(time.RFC3339)
		if err := seen.Mark(path, seen.Look{Commit: pt.Commit, At: at}); err != nil {
			writeRevisionError(w, err)
			return
		}
		rev, at, err := revisionSince(path)
		if err != nil {
			writeRevisionError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, RevisionReport{ID: id, Revision: rev, SeenAt: at})
	})
}

// revisionSince is the project's revision since its last look, and when that
// look was. With no usable last look (never looked at, or a commit the
// repository no longer has), the tip is recorded as the baseline.
func revisionSince(path string) (mull.Revision, string, error) {
	m, err := mull.Open(path)
	if err != nil {
		return mull.Revision{}, "", err
	}
	look, _, err := seen.Get(path)
	if err != nil {
		return mull.Revision{}, "", err
	}
	rev, err := m.Revision(look.Commit)
	if err != nil || !rev.Baseline {
		return rev, look.At, err
	}
	base := seen.Look{Commit: rev.Tip.Commit, At: time.Now().UTC().Format(time.RFC3339)}
	kept, err := seen.Baseline(path, base, look.Commit)
	if err != nil {
		return mull.Revision{}, "", err
	}
	if kept.Commit != base.Commit { // marked meanwhile
		rev, err = m.Revision(kept.Commit)
	}
	return rev, kept.At, err
}

// projectRevision is the project list's revision column, or nil if it
// can't be read (the summary carries the reason).
func projectRevision(path string) *ProjectRevision {
	rev, at, err := revisionSince(path)
	if err != nil {
		return nil
	}
	return &ProjectRevision{Number: rev.Tip.Number, Changes: len(rev.Changes), SeenAt: at}
}

// writeAllowed refuses a write a web page on another site could make. The
// Host check (localOnly) stops a page reading the API, but a cross-site form
// can still POST to localhost, so writes must come from the UI itself (or a
// tool such as curl): a JSON body, which a form can't send without a CORS
// preflight this server never grants, and no Origin or Sec-Fetch-Site
// naming another site.
func writeAllowed(r *http.Request) (int, *mull.Error) {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return http.StatusForbidden, &mull.Error{Code: mull.CodeNotAllowed, Msg: "mull serve takes writes only from its own pages (Sec-Fetch-Site: " + site + ")"}
	}
	if o := r.Header.Get("Origin"); o != "" {
		u, err := url.Parse(o)
		if err != nil || u.Scheme != "http" || u.Host != r.Host {
			return http.StatusForbidden, &mull.Error{Code: mull.CodeNotAllowed, Msg: "mull serve takes writes only from its own pages, not from origin " + o}
		}
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return http.StatusUnsupportedMediaType, &mull.Error{Code: mull.CodeNotAllowed, Msg: "a write must have Content-Type: application/json"}
	}
	return 0, nil
}

// writeRevisionError writes err with the HTTP status its code means.
func writeRevisionError(w http.ResponseWriter, err error) {
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
		status = http.StatusConflict // the project is registered but can't be read
	}
	writeError(w, status, e)
}
