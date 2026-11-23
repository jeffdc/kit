package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"mull/internal/store"
)

// A batch is JSON Lines, one op per line, run in one transaction: all or
// nothing. Field names are the commands' flag names. A line's "ref" names
// what it creates, and a later "@ref" in any ID field means that record.

// Line is one batch line: every op's fields.
type Line struct {
	Op, Ref                                                    string
	ID, Slug, Title, Summary, Target, Plan, Phase, Item, Text  string
	Default, By, Detail, Notes, Area, Kind, Source, Key, Value string
	Field, Reason, Decision, From, Edges, Allowed, State       string
	IDs, Commands, Needs, Add, Remove                          []string
	MaxLines                                                   int  `json:"max-lines"`
	MaxFiles                                                   int  `json:"max-files"`
	Sample                                                     int  `json:"sample"`
	Human                                                      bool `json:"human"`
	OnRequest                                                  bool `json:"on-request"`
}

// BatchResult is one line's outcome.
type BatchResult struct {
	Line   int    `json:"line"`
	Op     string `json:"op"`
	ID     string `json:"id,omitempty"`
	Result any    `json:"result"`
}

// Ops are the batch ops.
var Ops = []string{"plan new", "plan agree", "plan close", "plan set", "phase add", "phase set", "add", "bug", "idea", "set",
	"commit", "drop", "ask", "decided", "decide", "accept", "send back", "drop entry",
	"config set", "config checks", "config setup", "config risk", "config map"}

// Batch runs JSON Lines from r in one commit; with dryRun it writes
// nothing. Any failed line fails the batch, reporting every failure.
func (m *Mull) Batch(r io.Reader, dryRun bool) ([]BatchResult, error) {
	var lines []Line
	var errs []LineError
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	for n := 1; sc.Scan(); n++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			lines = append(lines, Line{})
			continue
		}
		var l Line
		dec := json.NewDecoder(strings.NewReader(text))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&l); err != nil {
			errs = append(errs, LineError{Line: n, Code: CodeInvalid, Error: err.Error()})
		}
		lines = append(lines, l)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(errs) > 0 {
		return nil, &Error{Code: CodeBatchInvalid, Msg: fmt.Sprintf("%d lines failed; nothing was written", len(errs)), Lines: errs}
	}
	var out []BatchResult
	var after []func()
	errDry := errors.New("dry run")
	_, err := m.store.Write("mull: batch", func(tx *store.Tx) error {
		s, err := load(tx)
		if err != nil {
			return err
		}
		b := *m
		b.batch, after = s, nil
		b.after = &after
		refs := map[string]string{}
		out = nil
		for k, l := range lines {
			if l.Op == "" {
				continue
			}
			res, id, err := b.runLine(l, refs)
			if err != nil {
				e := AsError(err)
				errs = append(errs, LineError{Line: k + 1, Op: l.Op, Code: e.Code, Error: e.Msg})
				continue
			}
			if l.Ref != "" && id != "" {
				refs[l.Ref] = id
			}
			out = append(out, BatchResult{Line: k + 1, Op: l.Op, ID: id, Result: res})
		}
		if len(errs) > 0 {
			return &Error{Code: CodeBatchInvalid, Msg: fmt.Sprintf("%d of %d lines failed; nothing was written", len(errs), len(lines)), Lines: errs}
		}
		if dryRun {
			return errDry
		}
		return nil
	})
	if errors.Is(err, errDry) {
		return out, nil
	}
	if err == nil {
		for _, f := range after {
			f()
		}
	}
	return out, err
}

func (m *Mull) runLine(l Line, refs map[string]string) (any, string, error) {
	ref := func(v string) string {
		if r, ok := strings.CutPrefix(v, "@"); ok {
			if id, ok := refs[r]; ok {
				return id
			}
		}
		return v
	}
	l.ID, l.Plan, l.Phase, l.Item, l.From = ref(l.ID), ref(l.Plan), ref(l.Phase), ref(l.Item), ref(l.From)
	for k := range l.IDs {
		l.IDs[k] = ref(l.IDs[k])
	}
	for k := range l.Needs {
		l.Needs[k] = ref(l.Needs[k])
	}
	switch l.Op {
	case "plan new":
		p, err := m.PlanNew(l.Slug, l.Title, l.Summary, l.Target, l.OnRequest)
		return p, l.Slug, err
	case "plan agree":
		p, err := m.PlanState(l.Slug, PlanActive, l.OnRequest)
		return p, l.Slug, err
	case "plan close":
		p, err := m.PlanState(l.Slug, PlanDone, l.OnRequest)
		return p, l.Slug, err
	case "plan set":
		p, err := m.PlanSet(l.Slug, l.Field, l.Value)
		return p, l.Slug, err
	case "phase add":
		p, err := m.PhaseAdd(l.Plan, l.Title, l.Summary, l.Needs)
		return p, idOf(p), err
	case "phase set":
		p, err := m.PhaseSet(l.ID, l.Field, ref(l.Value))
		return p, l.ID, err
	case "add", "bug", "idea":
		kind := ""
		if l.Op != "add" {
			kind = l.Op
		}
		notes := l.Notes
		if notes == "" {
			notes = l.Detail
		}
		i, err := m.Add(NewItem{Plan: l.Plan, Phase: l.Phase, Title: l.Title, Notes: notes, Area: l.Area, From: l.From, Kind: kind, Source: l.Source})
		return i, idOf(i), err
	case "set":
		i, err := m.Set(l.ID, l.Field, ref(l.Value))
		return i, l.ID, err
	case "commit":
		i, err := m.Commit(l.ID, l.Plan, l.Phase)
		return i, l.ID, err
	case "drop":
		i, err := m.DropItems(l.IDs, l.Reason)
		return i, "", err
	case "ask":
		e, err := m.Ask(l.Text, l.Item, l.Plan, l.Default, l.By, l.Detail)
		return e, idOf(e), err
	case "decided":
		e, err := m.Decided(l.Text, l.Item, l.Plan, l.Detail, l.Human, l.OnRequest)
		return e, idOf(e), err
	case "decide":
		e, err := m.Decide(l.ID, l.Decision, l.OnRequest)
		return e, l.ID, err
	case "accept":
		a, err := m.Accept(l.IDs, l.OnRequest)
		return a, "", err
	case "send back":
		i, err := m.SendBack(l.ID, l.Reason, l.OnRequest)
		return i, idOf(i), err
	case "drop entry":
		e, err := m.DropEntries(l.IDs, l.Reason, l.OnRequest)
		return e, "", err
	case "config set":
		c, err := m.ConfigSet(l.Key, l.Value, l.OnRequest)
		return c, "", err
	case "config checks":
		c, err := m.ConfigChecks(l.Commands, l.OnRequest)
		return c, "", err
	case "config setup":
		c, err := m.ConfigSetup(l.Commands, l.OnRequest)
		return c, "", err
	case "config risk":
		c, err := m.ConfigRisk(RiskInput{Add: l.Add, Remove: l.Remove, MaxLines: l.MaxLines, MaxFiles: l.MaxFiles, Sample: l.Sample, OnRequest: l.OnRequest})
		return c, "", err
	case "config map":
		c, err := m.ConfigMap(l.Edges, l.Allowed, l.OnRequest)
		return c, "", err
	}
	ops := append([]string{}, Ops...)
	sort.Strings(ops)
	return nil, "", errf(CodeInvalid, "unknown op %q (ops: %s)", l.Op, strings.Join(ops, ", "))
}

func idOf(v any) string {
	switch r := v.(type) {
	case *Item:
		if r != nil {
			return r.ID
		}
	case *Phase:
		if r != nil {
			return r.ID
		}
	case *Entry:
		if r != nil {
			return r.ID
		}
	}
	return ""
}
