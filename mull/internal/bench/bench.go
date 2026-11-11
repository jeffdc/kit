package bench

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"mull/internal/git"
	"mull/internal/model"
	"mull/internal/mull"
	"mull/internal/seen"
	"mull/internal/serve"
	"mull/internal/store"
)

// Options say how to measure.
type Options struct {
	Runs int    // timed runs per target, after one cold run
	URL  string // a running mull serve to time too (e.g. the installed one), or ""
	// Back is how many mull commits back the revision targets look from, at
	// most the branch's length.
	Back []int
}

// Result is one target's timings. Cold is the first call; Median, Min and
// Max are over the timed runs after it. Git is the git processes one call
// starts (from the last run). Over is how it was reached: in-process,
// http (this tree's serve handler) or live (Options.URL).
type Result struct {
	Repo   string  `json:"repo"`
	Over   string  `json:"over"`
	Target string  `json:"target"`
	ColdMS float64 `json:"cold_ms"`
	MedMS  float64 `json:"median_ms"`
	MinMS  float64 `json:"min_ms"`
	MaxMS  float64 `json:"max_ms"`
	Git    int64   `json:"git_calls"` // -1 when not countable (live)
	Bytes  int     `json:"bytes,omitempty"`
}

// Shape is what a repository holds, for the report.
type Shape struct {
	Repo    string `json:"repo"`
	Files   int    `json:"files"`
	Commits int    `json:"commits"`
	Plans   int    `json:"plans"`
	Phases  int    `json:"phases"`
	Items   int    `json:"items"`
	Done    int    `json:"done_or_dropped"`
	Entries int    `json:"inbox_entries"`
}

// Report is everything one run measured.
type Report struct {
	Shapes  []Shape  `json:"repos"`
	Results []Result `json:"results"`
}

type target struct {
	name string
	call func() error // in-process, or nil when it has only an endpoint
	path string       // the API path, or "" when the page has no endpoint
}

// Run measures every repository in repos. It reads them only; the per-machine
// state the serve handler keeps (last looks) goes to XDG_CONFIG_HOME, which
// the caller points somewhere disposable.
func Run(repos []string, o Options) (*Report, error) {
	if o.Runs < 1 {
		o.Runs = 1
	}
	abs := make([]string, len(repos))
	for i, r := range repos {
		a, err := filepath.Abs(r)
		if err != nil {
			return nil, err
		}
		abs[i] = a
	}
	repos = abs
	rep := &Report{}
	ids := map[string]string{}
	handler := serve.Handler(func() ([]string, error) { return repos, nil })
	srv := httptest.NewServer(handler)
	defer srv.Close()
	var list struct {
		Projects []struct{ ID, Path string }
	}
	if err := getJSON(srv.URL+"/api/projects", &list); err != nil {
		return nil, err
	}
	for _, p := range list.Projects {
		ids[p.Path] = p.ID
	}
	live := map[string]string{}
	if o.URL != "" {
		if err := getJSON(strings.TrimRight(o.URL, "/")+"/api/projects", &list); err != nil {
			return nil, fmt.Errorf("live server: %w", err)
		}
		for _, p := range list.Projects {
			live[p.Path] = p.ID
		}
	}
	for _, abs := range repos {
		shape, targets, err := plan(abs, o.Back)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", abs, err)
		}
		rep.Shapes = append(rep.Shapes, shape)
		name := filepath.Base(abs)
		for _, t := range targets {
			if t.call == nil {
				continue
			}
			r, err := measure(o.Runs, t.call)
			if err != nil {
				return nil, fmt.Errorf("%s: %s: %w", abs, t.name, err)
			}
			r.Repo, r.Over, r.Target = name, "in-process", t.name
			rep.Results = append(rep.Results, r)
		}
		for _, t := range targets {
			if t.path == "" {
				continue
			}
			if err := addHTTP(rep, name, "http", t, srv.URL+expand(t.path, ids[abs]), o.Runs); err != nil {
				return nil, err
			}
		}
		if id, ok := live[abs]; ok {
			for _, t := range targets {
				if t.path == "" {
					continue
				}
				if err := addHTTP(rep, name, "live", t, strings.TrimRight(o.URL, "/")+expand(t.path, id), o.Runs); err != nil {
					return nil, err
				}
			}
		}
	}
	return rep, nil
}

func expand(path, id string) string { return strings.ReplaceAll(path, "{id}", url.PathEscape(id)) }

func addHTTP(rep *Report, repo, over string, t target, u string, runs int) error {
	size := 0
	r, err := measure(runs, func() error {
		n, err := get(u)
		size = n
		return err
	})
	if err != nil {
		return fmt.Errorf("%s: GET %s: %w", repo, u, err)
	}
	r.Repo, r.Over, r.Target, r.Bytes = repo, over, t.name, size
	if over == "live" {
		r.Git = -1
	}
	rep.Results = append(rep.Results, r)
	return nil
}

// measure calls f once cold, then runs more times.
func measure(runs int, f func() error) (Result, error) {
	var r Result
	t := time.Now()
	if err := f(); err != nil {
		return r, err
	}
	r.ColdMS = ms(time.Since(t))
	var times []float64
	for range runs {
		before := git.Calls()
		t := time.Now()
		if err := f(); err != nil {
			return r, err
		}
		times = append(times, ms(time.Since(t)))
		r.Git = git.Calls() - before
	}
	slices.Sort(times)
	r.MinMS, r.MaxMS, r.MedMS = times[0], times[len(times)-1], times[len(times)/2]
	return r, nil
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func get(u string) (int, error) {
	resp, err := http.Get(u)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return len(b), nil
}

// plan reads repo's shape and chooses what to time: every page, on the
// busiest open plan and a done one, a phase and items in several stages,
// and revision from several distances back.
func plan(repo string, back []int) (Shape, []target, error) {
	shape := Shape{Repo: repo}
	m, err := mull.Open(repo)
	if err != nil {
		return shape, nil, err
	}
	r, err := git.Open(repo)
	if err != nil {
		return shape, nil, err
	}
	st := store.New(r)
	snap, err := st.Load()
	if err != nil {
		return shape, nil, err
	}
	shape.Files = len(snap.Files)
	n, err := r.Output(nil, nil, "rev-list", "--count", store.Ref)
	if err != nil {
		return shape, nil, err
	}
	shape.Commits, _ = strconv.Atoi(n)
	plans, err := m.PlanList()
	if err != nil {
		return shape, nil, err
	}
	items, err := m.ItemList(mull.ItemFilter{})
	if err != nil {
		return shape, nil, err
	}
	phases, err := m.PhaseList("")
	if err != nil {
		return shape, nil, err
	}
	entries, err := m.Inbox(mull.InboxFilter{All: true})
	if err != nil {
		return shape, nil, err
	}
	shape.Plans, shape.Phases, shape.Items, shape.Entries = len(plans), len(phases), len(items), len(entries)
	for _, i := range items {
		if i.State == model.StateDone || i.State == model.StateDropped {
			shape.Done++
		}
	}

	page := func(name, path string, f func() error) target { return target{name: name, call: f, path: path} }
	ts := []target{
		{name: "load (whole branch)", call: func() error { _, err := st.Load(); return err }},
		page("projects (summary)", "/api/projects", func() error { _, err := m.Summary(); return err }),
		page("overview", "/api/projects/{id}", func() error { _, err := m.Overview(); return err }),
		page("tree", "/api/projects/{id}/tree", func() error { _, err := m.TreePage(); return err }),
		page("backlog", "/api/projects/{id}/backlog", func() error { _, err := m.BacklogPage(""); return err }),
		page("unplanned", "/api/projects/{id}/unplanned", func() error { _, err := m.UnplannedPage(false); return err }),
		{name: "board (mull view)", call: func() error { _, err := m.Board(""); return err }},
	}
	// The open plan with the most items, and the done plan with the most.
	count := map[string]int{}
	for _, i := range items {
		count[i.Plan]++
	}
	var open, done string
	for _, p := range plans {
		switch {
		case p.State == model.PlanActive && (open == "" || count[p.Slug] > count[open]):
			open = p.Slug
		case p.State == model.PlanDone && (done == "" || count[p.Slug] > count[done]):
			done = p.Slug
		}
	}
	for _, slug := range []string{open, done} {
		if slug == "" {
			continue
		}
		ts = append(ts, page("plan "+slug, "/api/projects/{id}/plans/"+slug, func() error { _, err := m.PlanPage(slug); return err }))
	}
	for _, ph := range phases {
		if ph.Plan == open {
			id := ph.ID
			ts = append(ts, page("phase "+id, "/api/projects/{id}/phases/"+id, func() error { _, err := m.PhasePage(id); return err }))
			break
		}
	}
	for _, stage := range []string{model.StateInReview, model.StateActive, model.StateDone} {
		for _, i := range items {
			if i.State == stage {
				id := i.ID
				ts = append(ts, page("item "+id+" ("+stage+")", "/api/projects/{id}/items/"+id, func() error { _, err := m.ItemPage(id); return err }))
				break
			}
		}
	}
	for _, b := range back {
		if b >= shape.Commits {
			b = shape.Commits - 1
		}
		since, err := r.Output(nil, nil, "rev-parse", fmt.Sprintf("%s~%d", store.Ref, b))
		if err != nil {
			return shape, nil, err
		}
		ts = append(ts, target{name: fmt.Sprintf("revision, %d back", b), call: func() error { _, err := m.Revision(since); return err }})
	}
	// The first read after a write: the state at a commit the cache hasn't
	// seen, read as the difference from the one before. Each call steps one
	// commit nearer the tip, so every call is a new commit.
	if shape.Commits > 2 {
		out, err := r.Output(nil, nil, "rev-list", "--first-parent", "-n", "65", store.Ref)
		if err != nil {
			return shape, nil, err
		}
		recent := strings.Fields(out) // newest first
		k := len(recent) - 1
		if err := m.Preload(recent[k]); err != nil {
			return shape, nil, err
		}
		ts = append(ts, target{name: "next commit (after a write)", call: func() error {
			if k == 0 {
				return fmt.Errorf("ran out of commits")
			}
			k--
			return m.Preload(recent[k])
		}})
	}
	// Over HTTP, revision reads the last look from per-machine state: mark
	// it at the farthest distance so the endpoint does the real work.
	if len(back) > 0 {
		b := min(slices.Max(back), shape.Commits-1)
		since, err := r.Output(nil, nil, "rev-parse", fmt.Sprintf("%s~%d", store.Ref, b))
		if err != nil {
			return shape, nil, err
		}
		if err := seen.Mark(repo, seen.Look{Commit: since, At: time.Now().UTC().Format(time.RFC3339)}); err != nil {
			return shape, nil, err
		}
		ts = append(ts, target{name: fmt.Sprintf("revision endpoint, %d back", b), path: "/api/projects/{id}/revision"})
	}
	ts = append(ts, target{name: "unmulled scan", path: "/api/unmulled"})
	return shape, ts, nil
}

func getJSON(u string, v any) error {
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// Isolate points per-machine state at a fresh temporary directory, so a run
// never touches the real registry or last looks. It returns the directory.
func Isolate() (string, error) {
	dir, err := os.MkdirTemp("", "mull-bench-")
	if err != nil {
		return "", err
	}
	return dir, os.Setenv("XDG_CONFIG_HOME", dir)
}
