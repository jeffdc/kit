package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
	"mull/internal/mull"
	"mull/internal/seen"
)

// post sends body to path as the UI does: JSON, from its own page.
func post(t *testing.T, h http.Handler, path, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Host = "localhost:6855"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:6855")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	for k, v := range header {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("%v\n%s", err, rec.Body)
	}
	return v
}

func capture(t *testing.T, dir, title string) string {
	t.Helper()
	m, err := mull.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	it, err := m.CaptureItem(mull.NewCapture{Kind: model.ItemBug, Title: title})
	if err != nil {
		t.Fatal(err)
	}
	return it.ID
}

func mullTip(t *testing.T, dir string) string {
	t.Helper()
	return gittest.Git(t, dir, "rev-parse", "refs/heads/mull")
}

func TestRevisionAndSeen(t *testing.T) {
	dir := onMull(t)
	id := filepath.Base(dir)
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	revURL, seenURL := "/api/projects/"+id+"/revision", "/api/projects/"+id+"/seen"

	// The first look is the baseline: nothing is new yet, and it is kept.
	rec := get(t, h, revURL)
	rev := decode[RevisionReport](t, rec)
	if rec.Code != 200 || rev.ID != id || !rev.Baseline || rev.Tip.Commit != mullTip(t, dir) || rev.Since != rev.Tip || len(rev.Changes) != 0 || rev.SeenAt == "" {
		t.Fatalf("first GET revision: %d %+v", rec.Code, rev)
	}
	if l, ok, _ := seen.Get(dir); !ok || l.Commit != rev.Tip.Commit || l.At != rev.SeenAt {
		t.Errorf("baseline not recorded: %+v", l)
	}
	first := rev.Tip

	bug := capture(t, dir, "Footer")
	rev = decode[RevisionReport](t, get(t, h, revURL))
	if rev.Baseline || rev.Since != first || rev.Tip.Number != first.Number+1 || len(rev.Changes) != 1 || rev.Changes[0].ID != bug || rev.Changes[0].Change != mull.ChangeAdded {
		t.Errorf("GET revision after a capture: %+v", rev)
	}
	list := decode[ProjectList](t, get(t, h, "/api/projects"))
	if r := list.Projects[0].Revision; r == nil || r.Number != rev.Tip.Number || r.Changes != 1 || r.SeenAt != rev.SeenAt {
		t.Errorf("project list revision = %+v", r)
	}

	// Marking seen clears the changes, and never writes the mull branch.
	before := mullTip(t, dir)
	rec = post(t, h, seenURL, "", nil)
	rev = decode[RevisionReport](t, rec)
	if rec.Code != 200 || rev.Since.Commit != before || len(rev.Changes) != 0 || rev.Baseline {
		t.Errorf("POST seen: %d %+v", rec.Code, rev)
	}
	if mullTip(t, dir) != before {
		t.Error("POST seen moved the mull branch")
	}
	if rev := decode[RevisionReport](t, get(t, h, revURL)); len(rev.Changes) != 0 || rev.Since.Commit != before {
		t.Errorf("GET revision after seen: %+v", rev)
	}

	// The page can say which revision it showed; a later write stays new.
	shown := mullTip(t, dir)
	later := capture(t, dir, "Header")
	rev = decode[RevisionReport](t, post(t, h, seenURL, `{"commit": "`+shown+`"}`, nil))
	if rev.Since.Commit != shown || len(rev.Changes) != 1 || rev.Changes[0].ID != later {
		t.Errorf("POST seen at the shown commit: %+v", rev)
	}
	// Going back to the first look clouds both again.
	if rev := decode[RevisionReport](t, post(t, h, seenURL, `{"commit": "`+first.Commit+`"}`, nil)); len(rev.Changes) != 2 {
		t.Errorf("POST seen at an earlier commit: %+v", rev)
	}

	for body, code := range map[string]string{
		`{"commit": "nope"}`:                   mull.CodeInvalid,
		`{"commit": "` + headOf(t, dir) + `"}`: mull.CodeInvalid, // not on the mull branch
		`{"other": 1}`:                         mull.CodeInvalid,
		`not json`:                             mull.CodeInvalid,
	} {
		rec := post(t, h, seenURL, body, nil)
		if e := decode[mull.Error](t, rec); rec.Code != 400 || e.Code != code {
			t.Errorf("POST seen %s: %d %s", body, rec.Code, rec.Body)
		}
	}
	for _, path := range []string{"/api/projects/nope/revision"} {
		rec := get(t, h, path)
		if e := decode[mull.Error](t, rec); rec.Code != 404 || e.Code != mull.CodeNotFound {
			t.Errorf("GET %s: %d %s", path, rec.Code, rec.Body)
		}
	}
	rec = post(t, h, "/api/projects/nope/seen", "", nil)
	if e := decode[mull.Error](t, rec); rec.Code != 404 || e.Code != mull.CodeNotFound {
		t.Errorf("POST seen for an unknown project: %d %s", rec.Code, rec.Body)
	}
}

func headOf(t *testing.T, dir string) string {
	t.Helper()
	return gittest.Git(t, dir, "rev-parse", "main")
}

// A web page on another site can't mark a project seen: the POST must be
// JSON, and not come from another origin.
func TestSeenRefusesCrossOrigin(t *testing.T) {
	dir := onMull(t)
	id := filepath.Base(dir)
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	seenURL := "/api/projects/" + id + "/seen"
	get(t, h, "/api/projects/"+id+"/revision") // the baseline
	capture(t, dir, "Footer")
	look, _, _ := seen.Get(dir)

	for name, c := range map[string]struct {
		header map[string]string
		status int
	}{
		"another origin":           {map[string]string{"Origin": "http://evil.example"}, 403},
		"another port":             {map[string]string{"Origin": "http://localhost:3000"}, 403},
		"null origin":              {map[string]string{"Origin": "null"}, 403},
		"cross-site fetch":         {map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": ""}, 403},
		"same-site fetch":          {map[string]string{"Sec-Fetch-Site": "same-site", "Origin": ""}, 403},
		"a form post":              {map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Origin": "", "Sec-Fetch-Site": ""}, 415},
		"a text/plain post":        {map[string]string{"Content-Type": "text/plain", "Origin": "", "Sec-Fetch-Site": ""}, 415},
		"no content type":          {map[string]string{"Content-Type": "", "Origin": "", "Sec-Fetch-Site": ""}, 415},
		"another host (rebinding)": {map[string]string{"Host": "evil.example"}, 403},
	} {
		req := httptest.NewRequest("POST", seenURL, strings.NewReader("{}"))
		req.Host = "localhost:6855"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://localhost:6855")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		for k, v := range c.header {
			switch {
			case k == "Host":
				req.Host = v
			case v == "":
				req.Header.Del(k)
			default:
				req.Header.Set(k, v)
			}
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if e := decode[mull.Error](t, rec); rec.Code != c.status || e.Code != mull.CodeNotAllowed {
			t.Errorf("%s: %d %s, want %d not_allowed", name, rec.Code, rec.Body, c.status)
		}
		if l, _, _ := seen.Get(dir); l != look {
			t.Fatalf("%s: the look changed to %+v", name, l)
		}
	}

	// curl from a terminal sends no Origin or Sec-Fetch-Site; a browser's
	// direct navigation is "none". Both may write.
	for name, header := range map[string]map[string]string{
		"curl":       {"Origin": "", "Sec-Fetch-Site": ""},
		"navigation": {"Origin": "", "Sec-Fetch-Site": "none"},
		"127.0.0.1":  {"Origin": "http://127.0.0.1:6855"},
	} {
		req := httptest.NewRequest("POST", seenURL, nil)
		req.Host = "localhost:6855"
		if name == "127.0.0.1" {
			req.Host = "127.0.0.1:6855"
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		for k, v := range header {
			if v != "" {
				req.Header.Set(k, v)
			}
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
}

func TestRevisionUnreadableProject(t *testing.T) {
	bare := gittest.NewRepo(t)
	gone := filepath.Join(t.TempDir(), "gone")
	h := Handler(func() ([]string, error) { return []string{bare, gone}, nil })
	for _, p := range []string{bare, gone} {
		rec := get(t, h, "/api/projects/"+filepath.Base(p)+"/revision")
		if e := decode[mull.Error](t, rec); rec.Code != 409 || (e.Code != mull.CodeNotInitialized && e.Code != mull.CodeNotRepo) {
			t.Errorf("GET revision of %s: %d %s", p, rec.Code, rec.Body)
		}
	}
	list := decode[ProjectList](t, get(t, h, "/api/projects"))
	for _, p := range list.Projects {
		if p.Revision != nil {
			t.Errorf("unreadable project has a revision: %+v", p)
		}
	}
	if f, _ := seen.File(); fileExists(f) {
		t.Error("reading unreadable projects wrote last-looked state")
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
