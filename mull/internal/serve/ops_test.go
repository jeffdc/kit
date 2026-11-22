package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"mull/internal/gittest"
	"mull/internal/model"
	"mull/internal/mull"
	"mull/internal/seen"
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

	wantMullError(t, "an op the UI doesn't offer", post(t, h, url, opsBody(t, tip, map[string]any{"op": "add", "title": "x", "plan": "none"}), nil), http.StatusForbidden, mull.CodeNotAllowed)
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

// What the human does from the page isn't clouded as news: the records the
// write changed are acknowledged, and other changes stay clouded.
func TestOpsAcknowledge(t *testing.T) {
	dir := onMull(t)
	gittest.Git(t, dir, "config", "user.name", "Jeff")
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	base := "/api/projects/" + filepath.Base(dir)
	get(t, h, base+"/revision") // the baseline

	d := decision(t, dir, "use sqlite")
	other := capture(t, dir, "Footer")
	if rev := decode[RevisionReport](t, get(t, h, base+"/revision")); len(rev.Changes) != 2 {
		t.Fatalf("before: %+v", rev.Changes)
	}
	rec := post(t, h, base+"/ops", opsBody(t, tipOf(t, dir), map[string]any{"op": "accept", "ids": []string{d}}), nil)
	rep := decode[OpsReport](t, rec)
	if rec.Code != 200 || rep.AckError != nil || len(rep.Acknowledged) != 1 || rep.Acknowledged[0] != (mull.RecordRef{Type: mull.RecordEntry, ID: d}) {
		t.Fatalf("POST ops: %d %s", rec.Code, rec.Body)
	}
	rev := decode[RevisionReport](t, get(t, h, base+"/revision"))
	if len(rev.Changes) != 1 || rev.Changes[0].ID != other {
		t.Errorf("after approving: %+v, want only %s clouded", rev.Changes, other)
	}
	list := decode[ProjectList](t, get(t, h, "/api/projects"))
	if r := list.Projects[0].Revision; r == nil || r.Changes != 1 {
		t.Errorf("project list revision = %+v", r)
	}
	if l, _, _ := seen.Get(dir); len(l.Acked) != 1 || l.Acked[0].Commit != rep.Commit {
		t.Errorf("look = %+v", l)
	}

	// Marking seen acknowledges everything, and the list goes.
	post(t, h, base+"/seen", "", nil)
	if l, _, _ := seen.Get(dir); l.Acked != nil {
		t.Errorf("acks kept after marking seen: %+v", l.Acked)
	}
}

// The page answers questions and decides decisions differently with decide.
func TestOpsDecide(t *testing.T) {
	dir := onMull(t)
	gittest.Git(t, dir, "config", "user.name", "Jeff")
	m, _ := mull.Open(dir)
	q, err := m.Capture(mull.CaptureInput{Kind: model.KindQuestion, Text: "Which queue?"})
	if err != nil {
		t.Fatal(err)
	}
	d := decision(t, dir, "use sqlite")
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	rec := post(t, h, "/api/projects/"+filepath.Base(dir)+"/ops", opsBody(t, tipOf(t, dir),
		map[string]any{"op": "decide", "id": q.ID, "decision": "The jobs table"},
		map[string]any{"op": "decide", "id": d, "decision": "use postgres"}), nil)
	if rec.Code != 200 {
		t.Fatalf("POST ops: %d %s", rec.Code, rec.Body)
	}
	log, err := m.Decisions("")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]mull.Decision{}
	for _, e := range log {
		got[e.ID] = e
	}
	if a := got[q.ID]; a.Decision != "The jobs table" || a.By != "Jeff" || !a.Human {
		t.Errorf("answer = %+v", a)
	}
	if b := got[d]; b.Decision != "use postgres" || b.Context != "use sqlite" || !b.Human {
		t.Errorf("decided differently = %+v", b)
	}
}

// inReview puts a new unplanned item through mull done, carrying one open
// decision, and returns the item's ID and its review entry's.
func inReview(t *testing.T, dir string) (item, review, dec string) {
	t.Helper()
	m, err := mull.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Batch(strings.NewReader(`{"op": "config checks", "commands": ["true"]}`), false); err != nil {
		t.Fatal(err)
	}
	it, err := m.ItemAdd(mull.NewItem{Plan: model.NoPlan, Title: "Digest job"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(it.ID); err != nil {
		t.Fatal(err)
	}
	d, err := m.Capture(mull.CaptureInput{Kind: model.KindDecision, Text: "one digest a day", Item: it.ID})
	if err != nil {
		t.Fatal(err)
	}
	done, err := m.Done(it.ID, mull.DoneInput{Summary: "built", Design: "simply"})
	if err != nil {
		t.Fatal(err)
	}
	return it.ID, done.Review, d.ID
}

// The page accepts a review (ratifying what it carries) or sends it back
// with a reason, as the human, even for work done in the server's own
// worktree.
func TestOpsReview(t *testing.T) {
	dir := onMull(t)
	gittest.Git(t, dir, "config", "user.name", "Jeff")
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	url := "/api/projects/" + filepath.Base(dir) + "/ops"
	m, _ := mull.Open(dir)

	item, review, _ := inReview(t, dir)
	rec := post(t, h, url, opsBody(t, tipOf(t, dir), map[string]any{"op": "do", "id": review, "reason": "the digest skips Sundays"}), nil)
	if rec.Code != 200 {
		t.Fatalf("send back: %d %s", rec.Code, rec.Body)
	}
	if v, _ := m.Show(item); v.Item.State != model.StateCommitted || !strings.Contains(v.Item.Notes, "the digest skips Sundays") {
		t.Errorf("sent back item = %+v", v.Item)
	}

	item, _, dec := inReview(t, dir)
	rec = post(t, h, url, opsBody(t, tipOf(t, dir), map[string]any{"op": "accept", "ids": []string{item}}), nil)
	if rec.Code != 200 {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body)
	}
	if v, _ := m.Show(item); v.Item.State != model.StateDone {
		t.Errorf("accepted item = %+v", v.Item)
	}
	if v, _ := m.Show(dec); v.Entry == nil || v.Entry.Settled == nil || v.Entry.Settled.By != "Jeff" || !v.Entry.Settled.Human {
		t.Errorf("carried decision = %+v", v.Entry)
	}
}

// The page parks or drops a Needs you entry with a reason.
func TestOpsParkDrop(t *testing.T) {
	dir := onMull(t)
	gittest.Git(t, dir, "config", "user.name", "Jeff")
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	m, _ := mull.Open(dir)
	q, err := m.Capture(mull.CaptureInput{Kind: model.KindQuestion, Text: "Which queue?"})
	if err != nil {
		t.Fatal(err)
	}
	d := decision(t, dir, "use sqlite")
	rec := post(t, h, "/api/projects/"+filepath.Base(dir)+"/ops", opsBody(t, tipOf(t, dir),
		map[string]any{"op": "park", "id": q.ID, "reason": "after the launch"},
		map[string]any{"op": "drop", "ids": []string{d}, "reason": "superseded"}), nil)
	if rec.Code != 200 {
		t.Fatalf("POST ops: %d %s", rec.Code, rec.Body)
	}
	for id, want := range map[string]string{q.ID: model.ActPark, d: model.ActDrop} {
		v, err := m.Show(id)
		if err != nil {
			t.Fatal(err)
		}
		if s := v.Entry.Settled; s == nil || s.Action != want || s.By != "Jeff" || !s.Human || s.Note == "" {
			t.Errorf("%s settled = %+v, want %s by the human with the reason", id, s, want)
		}
	}
}

// Closing a plan from the UI archives it: it leaves the tree's plans for
// its Archive.
func TestOpsClosePlan(t *testing.T) {
	dir := onMull(t)
	gittest.Git(t, dir, "config", "user.name", "Jeff")
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	m, _ := mull.Open(dir)
	if _, err := m.PlanNew("pilot", "Pilot", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.PlanAgree("pilot", true); err != nil {
		t.Fatal(err)
	}
	id := filepath.Base(dir)
	rec := post(t, h, "/api/projects/"+id+"/ops", opsBody(t, tipOf(t, dir), map[string]any{"op": "plan close", "slug": "pilot"}), nil)
	if rec.Code != 200 {
		t.Fatalf("POST ops: %d %s", rec.Code, rec.Body)
	}
	var tree mull.Tree
	if err := json.Unmarshal(get(t, h, "/api/projects/"+id+"/tree").Body.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	for _, p := range tree.Plans {
		if p.Slug == "pilot" {
			t.Errorf("the closed plan is still among the tree's plans")
		}
	}
	if len(tree.Archive) != 1 || tree.Archive[0].Slug != "pilot" || tree.Archive[0].Closed == "" {
		t.Errorf("archive = %+v", tree.Archive)
	}
}

// The page edits text in place: a captured or committed item's title and
// notes with set, and a plan's and a phase's doc. Set takes no other field
// from the page.
func TestOpsEditText(t *testing.T) {
	dir := onMull(t)
	gittest.Git(t, dir, "config", "user.name", "Jeff")
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	base := "/api/projects/" + filepath.Base(dir)
	m, _ := mull.Open(dir)
	if _, err := m.PlanNew("pilot", "Pilot", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.PlanAgree("pilot", true); err != nil {
		t.Fatal(err)
	}
	ph, err := m.PhaseAdd("pilot", "Ingest", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	it, err := m.ItemAdd(mull.NewItem{Plan: "pilot", Phase: ph.ID, Title: "Digest job"})
	if err != nil {
		t.Fatal(err)
	}
	bug := capture(t, dir, "Footer overlaps")
	get(t, h, base+"/revision") // the baseline

	notes := "## Why\n\nThe footer covers the last row on a phone.\n\n- one\n- two"
	rec := post(t, h, base+"/ops", opsBody(t, tipOf(t, dir),
		map[string]any{"op": "set", "id": bug, "field": "title", "value": "Footer overlaps the last row"},
		map[string]any{"op": "set", "id": bug, "field": "notes", "value": notes},
		map[string]any{"op": "set", "id": it.ID, "field": "notes", "value": "Daily, at 6."},
		map[string]any{"op": "plan doc", "slug": "pilot", "doc": "# Pilot\n\nThe picture."},
		map[string]any{"op": "phase doc", "id": ph.ID, "doc": "Ingest the feeds."},
		map[string]any{"op": "plan rules", "slug": "pilot", "rules": "- Branch from main."},
		map[string]any{"op": "phase rules", "id": ph.ID, "rules": "Use the feed fixtures."}), nil)
	if rec.Code != 200 {
		t.Fatalf("POST ops: %d %s", rec.Code, rec.Body)
	}
	if v, _ := m.Show(bug); v.Item.Title != "Footer overlaps the last row" || v.Item.Notes != notes || v.Item.State != model.StateCaptured {
		t.Errorf("captured item = %+v", v.Item)
	}
	if v, _ := m.Show(it.ID); v.Item.Notes != "Daily, at 6." {
		t.Errorf("committed item = %+v", v.Item)
	}
	if d, _ := m.PlanDoc("pilot"); d.Doc != "# Pilot\n\nThe picture." {
		t.Errorf("plan doc = %q", d.Doc)
	}
	if d, _ := m.PhaseDoc(ph.ID); d.Doc != "Ingest the feeds." {
		t.Errorf("phase doc = %q", d.Doc)
	}
	if r, _ := m.PlanRules("pilot"); r.Rules != "- Branch from main." {
		t.Errorf("plan rules = %q", r.Rules)
	}
	if r, _ := m.PhaseRules(ph.ID); r.Rules != "Use the feed fixtures." {
		t.Errorf("phase rules = %q", r.Rules)
	}
	// What was edited is acknowledged, so it isn't clouded as news.
	if rev := decode[RevisionReport](t, get(t, h, base+"/revision")); len(rev.Changes) != 0 {
		t.Errorf("clouded after editing: %+v", rev.Changes)
	}

	// Set from the page takes only an item's title and notes.
	tip := tipOf(t, dir)
	for _, field := range []string{"kind", "plan", "phase", "category", "doc", "summary", ""} {
		rec := post(t, h, base+"/ops", opsBody(t, tip, map[string]any{"op": "set", "id": bug, "field": field, "value": "x"}), nil)
		wantMullError(t, "set "+field, rec, http.StatusForbidden, mull.CodeNotAllowed)
	}
	if tipOf(t, dir) != tip {
		t.Fatal("a refused set wrote")
	}

	// Notes an agent changed after the page drew them aren't overwritten;
	// a record that didn't change since is still edited.
	if _, err := m.Set(it.ID, "notes", "Daily, at 7."); err != nil {
		t.Fatal(err)
	}
	rec = post(t, h, base+"/ops", opsBody(t, tip, map[string]any{"op": "set", "id": it.ID, "field": "notes", "value": "Hourly."}), nil)
	wantMullError(t, "a stale edit", rec, http.StatusConflict, mull.CodeConflict)
	if v, _ := m.Show(it.ID); v.Item.Notes != "Daily, at 7." {
		t.Errorf("notes after a stale edit = %q", v.Item.Notes)
	}
	rec = post(t, h, base+"/ops", opsBody(t, tip, map[string]any{"op": "phase doc", "id": ph.ID, "doc": "Ingest the feeds daily."}), nil)
	if rec.Code != 200 {
		t.Errorf("an edit to a record that didn't change since: %d %s", rec.Code, rec.Body)
	}
}

// The page agrees a shaping plan as the human, with no on-request: only
// the human agrees a plan, and a write from the page is theirs.
func TestOpsAgreePlan(t *testing.T) {
	dir := onMull(t)
	gittest.Git(t, dir, "config", "user.name", "Jeff")
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	m, _ := mull.Open(dir)
	if _, err := m.PlanNew("pilot", "Pilot", "", true); err != nil {
		t.Fatal(err)
	}
	url := "/api/projects/" + filepath.Base(dir) + "/ops"
	tip := tipOf(t, dir)
	rec := post(t, h, url, opsBody(t, tip, map[string]any{"op": "plan agree", "slug": "pilot", "on-request": true}), nil)
	wantMullError(t, "on-request", rec, 400, mull.CodeInvalid)
	rec = post(t, h, url, opsBody(t, tip, map[string]any{"op": "plan agree", "slug": "pilot"}), nil)
	if rec.Code != 200 {
		t.Fatalf("POST ops: %d %s", rec.Code, rec.Body)
	}
	if v, _ := m.Show("pilot"); v.Plan == nil || v.Plan.State != model.PlanActive {
		t.Errorf("agreed plan = %+v", v.Plan)
	}
}

// The page captures bugs and ideas into the backlog as the human, said
// to be from them in mull serve: untagged, or tagged to the plan it shows.
func TestOpsCapture(t *testing.T) {
	dir := onMull(t)
	gittest.Git(t, dir, "config", "user.name", "Jeff")
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	m, _ := mull.Open(dir)
	if _, err := m.PlanNew("pilot", "Pilot", "", true); err != nil {
		t.Fatal(err)
	}
	rec := post(t, h, "/api/projects/"+filepath.Base(dir)+"/ops", opsBody(t, tipOf(t, dir),
		map[string]any{"op": "bug", "text": "The footer overlaps on a phone", "detail": "Seen at 400px."},
		map[string]any{"op": "idea", "text": "A digest by email", "plan": "pilot"}), nil)
	if rec.Code != 200 {
		t.Fatalf("POST ops: %d %s", rec.Code, rec.Body)
	}
	rep := decode[OpsReport](t, rec)
	if len(rep.Results) != 2 || rep.Results[0].ID == "" || rep.Results[1].ID == "" {
		t.Fatalf("results = %+v", rep.Results)
	}
	want := []struct{ kind, plan, notes string }{{model.ItemBug, "", "Seen at 400px."}, {model.ItemIdea, "pilot", ""}}
	for i, r := range rep.Results {
		v, err := m.Show(r.ID)
		if err != nil {
			t.Fatal(err)
		}
		it := v.Item
		if it.State != model.StateCaptured || it.Kind != want[i].kind || it.Plan != want[i].plan || it.Notes != want[i].notes || it.Source != "Jeff in mull serve" {
			t.Errorf("captured %d = %+v, want a %s for plan %q, from Jeff in mull serve", i, it.Item, want[i].kind, want[i].plan)
		}
	}
}

// The page drops a captured item with a reason, recorded as the human's.
func TestOpsDropCaptured(t *testing.T) {
	dir := onMull(t)
	gittest.Git(t, dir, "config", "user.name", "Jeff")
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	id := capture(t, dir, "Footer")
	rec := post(t, h, "/api/projects/"+filepath.Base(dir)+"/ops", opsBody(t, tipOf(t, dir),
		map[string]any{"op": "drop", "ids": []string{id}, "reason": "fixed by the redesign"}), nil)
	if rec.Code != 200 {
		t.Fatalf("POST ops: %d %s", rec.Code, rec.Body)
	}
	m, _ := mull.Open(dir)
	if v, _ := m.Show(id); v.Item == nil || v.Item.State != model.StateDropped || !strings.Contains(v.Item.Notes, "Dropped by Jeff: fixed by the redesign") {
		t.Errorf("dropped item = %+v", v.Item)
	}
}

// The page reorders a group's open work with rank, sending the whole new
// order; Jeff's own move isn't clouded.
func TestOpsRank(t *testing.T) {
	dir := onMull(t)
	gittest.Git(t, dir, "config", "user.name", "Jeff")
	h := Handler(func() ([]string, error) { return []string{dir}, nil })
	m, _ := mull.Open(dir)
	var ids []string
	for _, title := range []string{"a", "b", "c"} {
		v, err := m.ItemAdd(mull.NewItem{Plan: model.NoPlan, Title: title})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, v.ID)
	}
	url := "/api/projects/" + filepath.Base(dir) + "/ops"
	moved := []string{ids[0], ids[2], ids[1]} // c up one
	rec := post(t, h, url, opsBody(t, tipOf(t, dir), map[string]any{"op": "rank", "group": "none", "ids": moved}), nil)
	rep := decode[OpsReport](t, rec)
	if rec.Code != 200 || len(rep.Acknowledged) != 2 {
		t.Fatalf("POST ops: %d %s (want b and c acknowledged)", rec.Code, rec.Body)
	}
	got, _ := m.ItemList(mull.ItemFilter{Plan: model.NoPlan})
	var order []string
	for _, v := range got {
		order = append(order, v.ID)
	}
	if strings.Join(order, " ") != strings.Join(moved, " ") {
		t.Errorf("order = %v, want %v", order, moved)
	}

	// A list missing open work is refused, as mull rank refuses it.
	rec = post(t, h, url, opsBody(t, tipOf(t, dir), map[string]any{"op": "rank", "group": "none", "ids": ids[:2]}), nil)
	wantMullError(t, "a partial order", rec, 400, mull.CodeBatchInvalid)
}
