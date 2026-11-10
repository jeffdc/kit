package mull

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"mull/internal/model"
	"mull/internal/store"
)

// Revisions: what changed on the mull branch since the human last looked.
// mull serve keeps the commit last looked at (per machine, never on the mull
// branch) and asks Revision for the records that differ between it and the
// tip: the revision schedule, which the UI clouds in red.

// RevisionPoint is one revision of the mull branch: a commit, its number
// and its time. The number counts the commits on the mull branch up to and
// including it, so it grows with every write.
type RevisionPoint struct {
	Commit string `json:"commit"`
	Number int    `json:"number"`
	At     string `json:"at"` // the commit's time
}

// Revision is what changed between the revision last looked at (Since) and
// the tip.
type Revision struct {
	Tip   RevisionPoint `json:"tip"`
	Since RevisionPoint `json:"since"`
	// Baseline means there was nothing to compare from (never looked at, or
	// a commit this repository doesn't have), so Since is the tip and nothing
	// has changed yet. The caller records the tip as the baseline.
	Baseline bool     `json:"baseline,omitempty"`
	Changes  []Change `json:"changes"` // newest first
}

// Change kinds.
const (
	ChangeAdded   = "added"
	ChangeChanged = "changed"
	ChangeRemoved = "removed"
)

// Types of record a change is about.
const (
	RecordPlan     = "plan"
	RecordPlanDoc  = "plan_doc"
	RecordPhase    = "phase"
	RecordPhaseDoc = "phase_doc"
	RecordItem     = "item"
	RecordEntry    = "entry"
)

// Change is one record that differs between two revisions. A doc is its own
// record (plan_doc, phase_doc, with the plan's slug or the phase's ID), as
// it is its own file.
type Change struct {
	Type   string `json:"type"`
	ID     string `json:"id"`              // a plan's slug, or a phase, item or entry ID
	Title  string `json:"title"`           // the record's title (its plan's or phase's for a doc); an entry's text
	Plan   string `json:"plan,omitempty"`  // the plan it belongs to (a plan's own slug for a plan)
	Phase  string `json:"phase,omitempty"` // an item's phase
	Kind   string `json:"kind,omitempty"`  // an entry's kind, or a captured item's
	Change string `json:"change"`          // added, changed or removed
	// State is the record's state now (before, if removed): an item's stage,
	// a plan's state, an entry's settle action or "open". Phases and docs
	// have none.
	State string `json:"state,omitempty"`
	// Fields are the fields that changed, for a changed record.
	Fields []FieldChange `json:"fields,omitempty"`
	At     string        `json:"at"` // the latest commit since that touched it
}

// FieldChange is one changed field of a record. From and To are given for
// short fields (stage, title, plan, phase…), not for text (notes, summary,
// detail, doc).
type FieldChange struct {
	Field string `json:"field"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
}

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// Revision compares the mull branch's tip with since, the commit last looked
// at. An empty since, or one this repository doesn't have, gives a baseline:
// Since is the tip and there are no changes.
func (m *Mull) Revision(since string) (Revision, error) {
	tip, err := m.tip()
	if err != nil {
		return Revision{}, err
	}
	var out Revision
	if out.Tip, err = m.point(tip); err != nil {
		return Revision{}, err
	}
	if !m.knownCommit(since) {
		out.Since, out.Baseline, out.Changes = out.Tip, true, []Change{}
		return out, nil
	}
	if out.Since, err = m.point(since); err != nil {
		return Revision{}, err
	}
	out.Changes, err = m.recordChanges(since, tip)
	return out, err
}

// RevisionAt returns the revision commit names, which must be the mull
// branch's tip or a commit before it; "" means the tip. It is what can be
// marked as looked at.
func (m *Mull) RevisionAt(commit string) (RevisionPoint, error) {
	tip, err := m.tip()
	if err != nil {
		return RevisionPoint{}, err
	}
	if commit == "" {
		return m.point(tip)
	}
	if !m.knownCommit(commit) {
		return RevisionPoint{}, errf(CodeInvalid, "%q is not a commit in this repository (want a full commit ID on the mull branch)", commit)
	}
	if !m.isAncestor(commit, tip) {
		return RevisionPoint{}, errf(CodeInvalid, "commit %s is not on the mull branch", commit)
	}
	return m.point(commit)
}

func (m *Mull) tip() (string, error) {
	tip, err := m.repo.RefCommit(store.Ref)
	if err != nil {
		return "", err
	}
	if tip == "" {
		return "", store.ErrNotInitialized
	}
	return tip, nil
}

// knownCommit reports whether c is a full commit ID this repository has.
func (m *Mull) knownCommit(c string) bool {
	return commitPattern.MatchString(c) && m.hasCommit(c)
}

func (m *Mull) point(commit string) (RevisionPoint, error) {
	n, err := m.repo.Output(nil, nil, "rev-list", "--count", commit)
	if err != nil {
		return RevisionPoint{}, err
	}
	num, err := strconv.Atoi(n)
	if err != nil {
		return RevisionPoint{}, errf(CodeGit, "unexpected rev-list count %q", n)
	}
	at, err := m.commitTime(commit)
	if err != nil {
		return RevisionPoint{}, err
	}
	return RevisionPoint{Commit: commit, Number: num, At: at}, nil
}

// commitTime is a commit's time in UTC, RFC 3339.
func (m *Mull) commitTime(commit string) (string, error) {
	ts, err := m.repo.Output(nil, nil, "log", "-1", "--format=%ct", commit)
	if err != nil {
		return "", err
	}
	secs, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return "", errf(CodeGit, "unexpected commit time %q", ts)
	}
	return unixUTC(secs), nil
}

func unixUTC(secs int64) string { return time.Unix(secs, 0).UTC().Format(time.RFC3339) }

// stateAt loads every record as it was at commit.
func (m *Mull) stateAt(commit string) (*state, error) {
	var s *state
	err := m.store.ViewAt(commit, func(tx *store.Tx) error {
		var err error
		s, err = load(tx)
		return err
	})
	return s, err
}

// recordChanges lists the records that differ between the states at from and to,
// newest first. Records are compared as read, so a change of storage format
// alone (mull migrate moving a phase, renaming a stage) is no change, and
// nor is a record rewritten with only a new updated time.
func (m *Mull) recordChanges(from, to string) ([]Change, error) {
	out := []Change{}
	if from == to {
		return out, nil
	}
	old, err := m.stateAt(from)
	if err != nil {
		return nil, err
	}
	cur, err := m.stateAt(to)
	if err != nil {
		return nil, err
	}
	for _, slug := range unionKeys(old.plans, cur.plans) {
		a, b := old.plans[slug], cur.plans[slug]
		if c, ok := planChange(a, b); ok {
			out = append(out, c)
		}
		if c, ok := docChange(RecordPlanDoc, slug, planDoc(a), planDoc(b), planTitle(a, b), slug); ok {
			out = append(out, c)
		}
	}
	for _, id := range unionKeys(old.phases, cur.phases) {
		a, b := old.phases[id], cur.phases[id]
		if c, ok := phaseChange(a, b); ok {
			out = append(out, c)
		}
		title, plan := phaseTitle(a, b)
		if c, ok := docChange(RecordPhaseDoc, id, phaseDoc(a), phaseDoc(b), title, plan); ok {
			out = append(out, c)
		}
	}
	for _, id := range unionKeys(old.items, cur.items) {
		if c, ok := itemChange(old.items[id], cur.items[id]); ok {
			out = append(out, c)
		}
	}
	for _, id := range unionKeys(old.entries, cur.entries) {
		if c, ok := entryChange(old.entries[id], cur.entries[id]); ok {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return out, nil
	}
	times, err := m.touched(from, to)
	if err != nil {
		return nil, err
	}
	tipAt := ""
	for i := range out {
		for _, p := range changePaths(out[i]) {
			if at, ok := times[p]; ok && at > out[i].At {
				out[i].At = at
			}
		}
		if out[i].At == "" { // not touched by a commit in from..to (from isn't an ancestor)
			if tipAt == "" {
				if tipAt, err = m.commitTime(to); err != nil {
					return nil, err
				}
			}
			out[i].At = tipAt
		}
	}
	order := map[string]int{RecordPlan: 0, RecordPlanDoc: 1, RecordPhase: 2, RecordPhaseDoc: 3, RecordItem: 4, RecordEntry: 5}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].At != out[b].At {
			return out[a].At > out[b].At
		}
		if out[a].Type != out[b].Type {
			return order[out[a].Type] < order[out[b].Type]
		}
		return out[a].ID < out[b].ID
	})
	return out, nil
}

// touched maps each path changed by a commit in from..to to the time of the
// newest such commit, in UTC RFC 3339 so the times compare as strings.
func (m *Mull) touched(from, to string) (map[string]string, error) {
	log, err := m.repo.Output(nil, nil, "log", "--format=@%ct", "--name-only", "--no-renames", from+".."+to)
	if err != nil {
		return nil, err
	}
	times, at := map[string]string{}, ""
	for _, line := range strings.Split(log, "\n") {
		line = strings.TrimSpace(line)
		if ts, ok := strings.CutPrefix(line, "@"); ok {
			secs, err := strconv.ParseInt(ts, 10, 64)
			if err != nil {
				return nil, errf(CodeGit, "unexpected git log line %q", line)
			}
			at = unixUTC(secs)
			continue
		}
		if line != "" && times[line] == "" { // log is newest first
			times[line] = at
		}
	}
	return times, nil
}

// changePaths are the paths a changed record may be stored at, in either
// format.
func changePaths(c Change) []string {
	switch c.Type {
	case RecordPlan:
		return []string{model.PlanPath(c.ID)}
	case RecordPlanDoc:
		return []string{model.PlanDocPath(c.ID)}
	case RecordPhase:
		return []string{model.PhasePath(c.ID), model.LegacyPhasePath(c.ID)}
	case RecordPhaseDoc:
		return []string{model.PhaseDocPath(c.ID), model.LegacyPhaseDocPath(c.ID)}
	case RecordItem:
		return []string{model.ItemPath(c.ID)}
	case RecordEntry:
		return []string{model.EntryPath(c.ID)}
	}
	return nil
}

func unionKeys[V any](a, b map[string]V) []string {
	keys := sortedKeys(a)
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// fields collects the changed fields of a record.
type fields []FieldChange

// short records a short field's change with its values.
func (f *fields) short(name, from, to string) {
	if from != to {
		*f = append(*f, FieldChange{Field: name, From: from, To: to})
	}
}

// text records a text field's change without its values.
func (f *fields) text(name, from, to string) {
	if from != to {
		*f = append(*f, FieldChange{Field: name})
	}
}

func list(ids []string) string { return strings.Join(ids, ",") }

// change makes the Change for a record that was a (nil: added) and is b
// (nil: removed); ok is false when nothing changed.
func change[T any](a, b *T, diff func(a, b *T) fields, describe func(r *T, c *Change)) (Change, bool) {
	var c Change
	switch {
	case a == nil && b == nil:
		return c, false
	case a == nil:
		c.Change = ChangeAdded
		describe(b, &c)
	case b == nil:
		c.Change = ChangeRemoved
		describe(a, &c)
	default:
		f := diff(a, b)
		if len(f) == 0 {
			return c, false
		}
		c.Change, c.Fields = ChangeChanged, f
		describe(b, &c)
	}
	return c, true
}

func planChange(a, b *model.Plan) (Change, bool) {
	return change(a, b, func(a, b *model.Plan) fields {
		var f fields
		f.short("title", a.Title, b.Title)
		f.short("state", a.State, b.State)
		f.text("summary", a.Summary, b.Summary)
		return f
	}, func(p *model.Plan, c *Change) {
		c.Type, c.ID, c.Title, c.Plan, c.State = RecordPlan, p.Slug, p.Title, p.Slug, p.State
	})
}

func phaseChange(a, b *model.Phase) (Change, bool) {
	return change(a, b, func(a, b *model.Phase) fields {
		var f fields
		f.short("plan", a.Plan, b.Plan)
		f.short("title", a.Title, b.Title)
		f.short("needs", list(a.Needs), list(b.Needs))
		f.short("from", a.From, b.From)
		f.text("summary", a.Summary, b.Summary)
		return f
	}, func(p *model.Phase, c *Change) {
		c.Type, c.ID, c.Title, c.Plan = RecordPhase, p.ID, p.Title, p.Plan
	})
}

func claimOf(i *model.Item) string {
	if i.Claim == nil {
		return ""
	}
	return i.Claim.Worktree
}

func itemChange(a, b *model.Item) (Change, bool) {
	return change(a, b, func(a, b *model.Item) fields {
		var f fields
		f.short("state", a.State, b.State)
		f.short("title", a.Title, b.Title)
		f.short("plan", a.Plan, b.Plan)
		f.short("phase", a.Phase, b.Phase)
		f.short("kind", a.Kind, b.Kind)
		f.short("category", a.Category, b.Category)
		f.short("source", a.Source, b.Source)
		f.short("claim", claimOf(a), claimOf(b))
		f.short("triaged", a.Triaged, b.Triaged)
		f.short("docs", list(a.Docs), list(b.Docs))
		f.short("needs", list(a.Needs), list(b.Needs))
		f.short("from", a.From, b.From)
		f.text("notes", a.Notes, b.Notes)
		return f
	}, func(i *model.Item, c *Change) {
		c.Type, c.ID, c.Title, c.Plan, c.Phase, c.State = RecordItem, i.ID, i.Title, i.Plan, i.Phase, i.State
		if i.State == model.StateCaptured {
			c.Kind = i.Kind
		}
	})
}

// settledAs is an entry's state: its settle action, or open.
func settledAs(e *model.Entry) string {
	if e.Settled == nil {
		return "open"
	}
	return e.Settled.Action
}

func entryChange(a, b *model.Entry) (Change, bool) {
	return change(a, b, func(a, b *model.Entry) fields {
		var f fields
		f.short("settled", settledAs(a), settledAs(b))
		f.short("kind", a.Kind, b.Kind)
		f.text("text", a.Text, b.Text)
		f.short("plan", a.Plan, b.Plan)
		f.short("item", a.Item, b.Item)
		f.short("source", a.Source, b.Source)
		f.short("decisions", list(a.Decisions), list(b.Decisions))
		f.text("detail", a.Detail, b.Detail)
		if a.Settled != nil && b.Settled != nil && a.Settled.Action == b.Settled.Action && a.Settled.Note != b.Settled.Note {
			f.text("settled_note", a.Settled.Note, b.Settled.Note)
		}
		return f
	}, func(e *model.Entry, c *Change) {
		c.Type, c.ID, c.Title, c.Plan, c.Kind, c.State = RecordEntry, e.ID, e.Text, e.Plan, e.Kind, settledAs(e)
	})
}

// docChange compares a doc's text; an empty doc is no doc.
func docChange(typ, id, from, to, title, plan string) (Change, bool) {
	c := Change{Type: typ, ID: id, Title: title, Plan: plan}
	switch {
	case from == to:
		return c, false
	case from == "":
		c.Change = ChangeAdded
	case to == "":
		c.Change = ChangeRemoved
	default:
		c.Change = ChangeChanged
	}
	return c, true
}

func planDoc(p *model.Plan) string {
	if p == nil {
		return ""
	}
	return p.Doc
}

func phaseDoc(p *model.Phase) string {
	if p == nil {
		return ""
	}
	return p.Doc
}

// planTitle is the plan's title now, or before if it was removed.
func planTitle(a, b *model.Plan) string {
	if b != nil {
		return b.Title
	}
	if a != nil {
		return a.Title
	}
	return ""
}

func phaseTitle(a, b *model.Phase) (title, plan string) {
	if b != nil {
		return b.Title, b.Plan
	}
	if a != nil {
		return a.Title, a.Plan
	}
	return "", ""
}
