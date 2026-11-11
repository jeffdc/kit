package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
	"mull/internal/mull"
)

// decision files an open decision in the repository dir is in.
func decision(t *testing.T, dir, text string) string {
	t.Helper()
	m, err := mull.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	e, err := m.Capture(mull.CaptureInput{Kind: model.KindDecision, Text: text})
	if err != nil {
		t.Fatal(err)
	}
	return e.ID
}

func opsBody(t *testing.T, commit string, ops ...map[string]any) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"commit": commit, "ops": ops})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func wantMullError(t *testing.T, what string, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var e mull.Error
	if rec.Code != status || json.Unmarshal(rec.Body.Bytes(), &e) != nil || e.Code != code {
		t.Errorf("%s: %d %s, want %d %s", what, rec.Code, rec.Body, status, code)
	}
}

func TestOpsApproveDecision(t *testing.T) {
	dir := onMull(t)
	gittest.Git(t, dir, "config", "user.name", "Jeff")
	d := decision(t, dir, "use sqlite")
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	url := "/api/projects/" + filepath.Base(dir) + "/ops"

	rec := post(t, h, url, opsBody(t, tipOf(t, dir), map[string]any{"op": "accept", "ids": []string{d}}), nil)
	if rec.Code != 200 {
		t.Fatalf("POST ops: %d %s", rec.Code, rec.Body)
	}
	var rep struct {
		Results []struct {
			Line   int    `json:"line"`
			Op     string `json:"op"`
			Result struct {
				Entries []model.Entry `json:"entries"`
			} `json:"result"`
		} `json:"results"`
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Commit != tipOf(t, dir) || len(rep.Results) != 1 || rep.Results[0].Op != "accept" || len(rep.Results[0].Result.Entries) != 1 {
		t.Fatalf("report = %s", rec.Body)
	}
	if s := rep.Results[0].Result.Entries[0].Settled; s == nil || s.By != "Jeff" || !s.Human || s.OnRequest {
		t.Errorf("settled = %+v, want the human's, by name", s)
	}

	// The page showed the decision open; it has been approved since.
	rec = post(t, h, url, opsBody(t, tipOf(t, dir), map[string]any{"op": "accept", "ids": []string{d}}), nil)
	wantMullError(t, "approving again from the new tip", rec, 400, mull.CodeBatchInvalid)
	shown := tipOf(t, dir)
	d2 := decision(t, dir, "use postgres")
	m, _ := mull.Open(dir)
	if _, err := m.Accept([]string{d2}, "", mull.SettleOpts{OnRequest: true}); err != nil {
		t.Fatal(err)
	}
	rec = post(t, h, url, opsBody(t, shown, map[string]any{"op": "accept", "ids": []string{d2}}), nil)
	wantMullError(t, "a stale page", rec, http.StatusConflict, mull.CodeConflict)
}

func TestOpsRefused(t *testing.T) {
	dir := onMull(t)
	d := decision(t, dir, "d")
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	url := "/api/projects/" + filepath.Base(dir) + "/ops"
	accept := map[string]any{"op": "accept", "ids": []string{d}}
	tip := tipOf(t, dir)

	wantMullError(t, "no git user.name", post(t, h, url, opsBody(t, tip, accept), nil), http.StatusForbidden, mull.CodeNotAllowed)
	gittest.Git(t, dir, "config", "user.name", "Jeff")

	wantMullError(t, "an op the UI doesn't offer", post(t, h, url, opsBody(t, tip, map[string]any{"op": "plan close", "slug": "x"}), nil), http.StatusForbidden, mull.CodeNotAllowed)
	wantMullError(t, "on-request", post(t, h, url, opsBody(t, tip, map[string]any{"op": "accept", "ids": []string{d}, "on-request": true}), nil), 400, mull.CodeInvalid)
	wantMullError(t, "no commit", post(t, h, url, opsBody(t, "", accept), nil), 400, mull.CodeInvalid)
	wantMullError(t, "no ops", post(t, h, url, `{"commit": "`+tip+`", "ops": []}`, nil), 400, mull.CodeInvalid)
	wantMullError(t, "an unknown field", post(t, h, url, `{"commit": "`+tip+`", "ops": [], "dry": true}`, nil), 400, mull.CodeInvalid)
	wantMullError(t, "another site", post(t, h, url, opsBody(t, tip, accept), map[string]string{"Origin": "http://evil.example", "Sec-Fetch-Site": "cross-site"}), http.StatusForbidden, mull.CodeNotAllowed)
	wantMullError(t, "a form", post(t, h, url, opsBody(t, tip, accept), map[string]string{"Content-Type": "application/x-www-form-urlencoded"}), http.StatusUnsupportedMediaType, mull.CodeNotAllowed)
	wantMullError(t, "an unknown project", post(t, h, "/api/projects/nope/ops", opsBody(t, tip, accept), nil), 404, mull.CodeNotFound)
	if tipOf(t, dir) != tip {
		t.Fatal("a refused write wrote")
	}
}
