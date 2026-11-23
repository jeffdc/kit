package mull

import (
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strings"

	"mull/internal/model"
)

// The map is the code's parts and the connections between them. mull stays
// language-neutral: the project gives a command that prints the connections
// the code has (config map edges), and optionally one that prints the ones
// the human declared (config map allowed). mull records the edges, diffs a
// change against the record (a new connection is high risk), and shows
// undeclared connections on the surface.

// MapOut is mull map: the connections now, against the record and the
// declared ones.
type MapOut struct {
	Configured bool     `json:"configured"`
	Edges      []string `json:"edges"`
	Allowed    []string `json:"allowed,omitempty"`
	Undeclared []string `json:"undeclared"` // connections the human didn't declare
	Unused     []string `json:"unused"`     // declared connections the code doesn't have
	New        []string `json:"new"`        // connections since the last recording
	Gone       []string `json:"gone"`
	RecordedAt string   `json:"recorded_at,omitempty"`
	Recorded   bool     `json:"recorded,omitempty"` // this run recorded them
}

// edgeLines runs command in dir and returns the edges it prints, each as
// "from -> to", sorted and without repeats. "from to" and "from,to" are
// read too.
func edgeLines(dir, command string) ([]string, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	b, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, errf(CodeState, "map command `%s` exited %d: %s", command, ee.ExitCode(), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if e := normEdge(l); e != "" && !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	sort.Strings(out)
	return out, nil
}

// normEdge reads one edge line as "from -> to", or "" if it isn't one.
func normEdge(l string) string {
	l = strings.TrimSpace(l)
	if l == "" || strings.HasPrefix(l, "#") {
		return ""
	}
	var parts []string
	switch {
	case strings.Contains(l, "->"):
		parts = strings.SplitN(l, "->", 2)
	case strings.Contains(l, ","):
		parts = strings.SplitN(l, ",", 2)
	default:
		parts = strings.Fields(l)
	}
	if len(parts) != 2 {
		return ""
	}
	from, to := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if from == "" || to == "" || from == to {
		return ""
	}
	return from + " -> " + to
}

func minus(a, b []string) []string {
	in := map[string]bool{}
	for _, x := range b {
		in[x] = true
	}
	out := []string{}
	for _, x := range a {
		if !in[x] {
			out = append(out, x)
		}
	}
	return out
}

// snapshot reads the map's last recording; nil when there's none.
func (s *state) snapshot() *model.MapSnapshot {
	b, ok := s.tx.Get(model.MapPath)
	if !ok {
		return nil
	}
	var snap model.MapSnapshot
	if json.Unmarshal(b, &snap) != nil {
		return nil
	}
	return &snap
}

// Map reads the map in this worktree. With record, it records the edges as
// the new baseline: the human's call (onRequest for an agent), since what's
// recorded is what changes are judged against.
func (m *Mull) Map(record, onRequest bool) (MapOut, error) {
	out := MapOut{Edges: []string{}, Undeclared: []string{}, Unused: []string{}, New: []string{}, Gone: []string{}}
	var cfg model.MapConfig
	var snap *model.MapSnapshot
	if err := m.view(func(s *state) error { cfg, snap = s.config.Map, s.snapshot(); return nil }); err != nil {
		return out, err
	}
	if cfg.Edges == "" {
		return out, nil
	}
	out.Configured = true
	edges, err := edgeLines(m.repo.Dir, cfg.Edges)
	if err != nil {
		return out, err
	}
	out.Edges = append(out.Edges, edges...)
	if cfg.Allowed != "" {
		if out.Allowed, err = edgeLines(m.repo.Dir, cfg.Allowed); err != nil {
			return out, err
		}
		out.Undeclared, out.Unused = minus(edges, out.Allowed), minus(out.Allowed, edges)
	}
	if snap != nil {
		out.New, out.Gone, out.RecordedAt = minus(edges, snap.Edges), minus(snap.Edges, edges), snap.At
	}
	if !record {
		return out, nil
	}
	if !m.asked(onRequest) {
		return out, errf(CodeNotAllowed, "recording the map sets what changes are judged against, so it's the human's call: pass --on-request when they've asked")
	}
	head, _ := m.repo.RefCommit("HEAD")
	err = m.recordEdges(head, edges)
	out.Recorded, out.RecordedAt = err == nil, m.now()
	return out, err
}

// recordEdges writes edges as the map's recording.
func (m *Mull) recordEdges(commit string, edges []string) error {
	data, err := json.MarshalIndent(model.MapSnapshot{Commit: commit, At: m.now(), Edges: edges}, "", "  ")
	if err != nil {
		return err
	}
	return m.updateAny("map", func(s *state) error {
		s.tx.Put(model.MapPath, append(data, '\n'))
		return nil
	})
}

// newEdges is what done asks of the map: the connections this worktree
// adds to the recording. None without a map or a recording.
func (m *Mull) newEdges(cfg model.MapConfig, snap *model.MapSnapshot) ([]string, error) {
	if cfg.Edges == "" || snap == nil {
		return nil, nil
	}
	edges, err := edgeLines(m.repo.Dir, cfg.Edges)
	if err != nil {
		return nil, err
	}
	if n := minus(edges, snap.Edges); len(n) > 0 {
		return n, nil
	}
	return nil, nil
}

// addEdges adds edges the human accepted to the map's recording.
func (m *Mull) addEdges(edges []string) error {
	if len(edges) == 0 {
		return nil
	}
	var snap *model.MapSnapshot
	if err := m.view(func(s *state) error { snap = s.snapshot(); return nil }); err != nil || snap == nil {
		return err
	}
	all := append(append([]string{}, snap.Edges...), minus(edges, snap.Edges)...)
	sort.Strings(all)
	return m.recordEdges(snap.Commit, all)
}

// ---- The audit ----

// AuditOut is the latest audit: what an auditor agent recorded.
type AuditOut struct {
	Text string `json:"text"`
	At   string `json:"at,omitempty"`
}

// Audit returns the latest audit.
func (m *Mull) Audit() (AuditOut, error) {
	var out AuditOut
	err := m.view(func(s *state) error {
		if b, ok := s.tx.Get(model.AuditPath); ok {
			out.Text = strings.TrimSpace(string(b))
			out.At = auditDate(out.Text)
		}
		return nil
	})
	return out, err
}

// RecordAudit records an audit: markdown, opening with a line "Audited:
// <RFC 3339 time>" that mull adds when it's missing. The mull branch keeps
// every earlier audit in its history.
func (m *Mull) RecordAudit(text string) (AuditOut, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return AuditOut{}, errf(CodeInvalid, "an audit needs its findings (markdown on stdin)")
	}
	if auditDate(text) == "" {
		text = "Audited: " + m.now() + "\n\n" + text
	}
	err := m.updateAny("audit", func(s *state) error {
		s.tx.Put(model.AuditPath, []byte(text+"\n"))
		return nil
	})
	return AuditOut{Text: text, At: auditDate(text)}, err
}

// auditDate is an audit's "Audited:" time, or "".
func auditDate(text string) string {
	first, _, _ := strings.Cut(text, "\n")
	if v, ok := strings.CutPrefix(strings.TrimSpace(first), "Audited:"); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
