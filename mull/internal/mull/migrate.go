package mull

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"mull/internal/model"
	"mull/internal/store"
)

// mull migrate converts a repository's mull data to Model v2 (format 2, see
// model.Format) in one commit:
//
//   - format: phases move from epics/ to phases/ (their docs from docs/epics/
//     to docs/phases/), items store their phase under phase: and their stage
//     under stage: with its Model v2 name, a done plan is stored as done, a
//     settlement's phase under phase:, and config.yml records format: 2;
//   - open and parked loose ends and inbox ideas become captured ideas (the
//     human re-kinds the bugs among them with mull set <id> kind bug), and
//     the entries are settled as do, pointing at the new item;
//   - needs: a phase's need on an item becomes a need on that item's phase
//     (or is dropped when it is met, or can't be one), and items' needs are
//     dropped (kept in the notes of unfinished items);
//   - on the first migration only (format 1 to 2): the standing general plan
//     is retired, its items unplanned and its entries made project-wide, and
//     each plan's and phase's summary becomes its doc where it has none.
//
// It is idempotent: on migrated data it changes nothing. Its writes are the
// human's call, so a real run needs OnRequest; a dry run only reads.

// GeneralPlan is the standing plan mull init created before Model v2.
const GeneralPlan = "general"

// MigrateResult is what migrate did, or with a dry run would do.
type MigrateResult struct {
	DryRun       bool             `json:"dry_run"`
	FormatBefore int              `json:"format_before"`
	Format       int              `json:"format"`
	Changed      bool             `json:"changed"`
	Commit       string           `json:"commit,omitempty"` // the mull branch after a real run that wrote
	Counts       MigrateCounts    `json:"counts"`
	Records      []MigrateRecord  `json:"records"`
	Captured     []MigrateCapture `json:"captured"`
	Needs        []MigrateNeed    `json:"needs"`
	Warnings     []string         `json:"warnings"`
}

// MigrateCounts counts the records migrate changes, by type, plus the doc
// files it writes or moves, the items it captures and the needs it converts
// or drops.
type MigrateCounts struct {
	Plans    int `json:"plans"`
	Phases   int `json:"phases"`
	Items    int `json:"items"`
	Entries  int `json:"entries"`
	Docs     int `json:"docs"`
	Captured int `json:"captured"`
	Needs    int `json:"needs"`
}

// MigrateRecord is one existing record migrate changes, and how.
type MigrateRecord struct {
	Type    string   `json:"type"` // config, plan, phase, item, entry
	ID      string   `json:"id"`
	Path    string   `json:"path"`           // where it is stored after
	From    string   `json:"from,omitempty"` // where it was, when it moves
	Changes []string `json:"changes"`
}

// MigrateCapture is a loose end or inbox idea that becomes a captured item.
type MigrateCapture struct {
	Item      string `json:"item"` // the new item (throwaway ID in a dry run)
	Kind      string `json:"kind"` // idea; re-kind bugs with mull set <id> kind bug
	Title     string `json:"title"`
	Plan      string `json:"plan,omitempty"`
	Entry     string `json:"entry"`
	EntryKind string `json:"entry_kind"` // loose-end or idea
	Was       string `json:"was"`        // open or parked
}

// MigrateNeed is a need migrate converts or drops.
type MigrateNeed struct {
	Type   string `json:"type"` // the record that had it: phase or item
	ID     string `json:"id"`
	Need   string `json:"need"`         // the item it needed
	Action string `json:"action"`       // converted or dropped
	To     string `json:"to,omitempty"` // converted: the phase it needs now
	Reason string `json:"reason"`
}

// Migrate converts the repository's mull data to the current format. With
// dryRun it only reads, and reports what it would change.
func (m *Mull) Migrate(dryRun, onRequest bool) (MigrateResult, error) {
	var out MigrateResult
	if dryRun {
		err := m.store.View(func(tx *store.Tx) error {
			s, err := load(tx)
			if err != nil {
				return err
			}
			out, err = m.migrate(s)
			return err
		})
		out.DryRun = true
		return out, err
	}
	if !onRequest {
		return out, errf(CodeNotAllowed, "migrating rewrites the record and settles inbox entries, so it's the human's call: "+
			"show the human mull migrate --dry-run, and run mull migrate --on-request when they ask")
	}
	err := m.updateAny(fmt.Sprintf("migrate to Model v2 (format %d)", model.Format), func(s *state) error {
		var err error
		out, err = m.migrate(s)
		return err
	})
	if err != nil {
		return out, err
	}
	if out.Changed {
		if out.Commit, err = m.repo.RefCommit(store.Ref); err != nil {
			return out, err
		}
	}
	return out, nil
}

// migration collects what migrate changes, record by record.
type migration struct {
	m       *Mull
	s       *state
	now     string
	res     MigrateResult
	records map[string]*MigrateRecord // by type/id
	plans   map[string]bool           // records to write back
	phases  map[string]bool
	items   map[string]bool
	entries map[string]bool
	retired bool // the general plan was retired
}

func (g *migration) note(typ, id, path, change string) *MigrateRecord {
	key := typ + "/" + id
	r := g.records[key]
	if r == nil {
		r = &MigrateRecord{Type: typ, ID: id, Path: path, Changes: []string{}}
		g.records[key] = r
	}
	if change != "" {
		r.Changes = append(r.Changes, change)
	}
	return r
}

func (g *migration) plan(slug, change string) {
	g.plans[slug] = true
	g.note("plan", slug, model.PlanPath(slug), change)
}

func (g *migration) phase(id, change string) {
	g.phases[id] = true
	g.note("phase", id, model.PhasePath(id), change)
}

func (g *migration) item(id, change string) {
	g.items[id] = true
	g.note("item", id, model.ItemPath(id), change)
}

func (g *migration) entry(id, change string) {
	g.entries[id] = true
	g.note("entry", id, model.EntryPath(id), change)
}

func (m *Mull) migrate(s *state) (MigrateResult, error) {
	g := &migration{
		m: m, s: s, now: m.now(), records: map[string]*MigrateRecord{},
		plans: map[string]bool{}, phases: map[string]bool{}, items: map[string]bool{}, entries: map[string]bool{},
		res: MigrateResult{FormatBefore: s.config.Format, Format: model.Format,
			Records: []MigrateRecord{}, Captured: []MigrateCapture{}, Needs: []MigrateNeed{}, Warnings: []string{}},
	}
	first := s.config.Format < model.Format
	g.formatChanges()
	if first {
		g.retireGeneral()
	}
	if err := g.captureLegacyEntries(); err != nil {
		return g.res, err
	}
	if err := g.convertPhaseNeeds(); err != nil {
		return g.res, err
	}
	g.dropItemNeeds()
	if err := g.write(first); err != nil {
		return g.res, err
	}
	g.finish()
	return g.res, nil
}

// formatChanges notes every record stored in format 1 and how it changes.
func (g *migration) formatChanges() {
	s := g.s
	for _, slug := range sortedKeys(s.plans) {
		if s.plans[slug].Legacy {
			g.plan(slug, "state: closed → state: done")
		}
	}
	for _, id := range sortedKeys(s.phases) {
		e := s.phases[id]
		if e.Legacy {
			g.phase(id, "")
			g.records["phase/"+id].From = model.LegacyPhasePath(id)
		}
	}
	for _, id := range sortedKeys(s.items) {
		if !s.items[id].Legacy {
			continue
		}
		var raw struct {
			State string `yaml:"state"`
			Stage string `yaml:"stage"`
			Epic  string `yaml:"epic"`
		}
		b, _ := s.tx.Get(model.ItemPath(id))
		_, _ = model.Decode(b, &raw)
		i := s.items[id]
		if raw.Stage == "" {
			g.item(id, fmt.Sprintf("state: %s → stage: %s", raw.State, i.State))
		} else {
			g.item(id, "state: "+raw.State+" removed (stage: "+raw.Stage+" kept)")
		}
		if raw.Epic != "" {
			g.item(id, "epic: "+raw.Epic+" → phase: "+i.Phase)
		}
	}
	for _, id := range sortedKeys(s.entries) {
		if e := s.entries[id]; e.Settled != nil && e.Settled.Legacy {
			g.entry(id, "settled epic: "+e.Settled.Phase+" → settled phase: "+e.Settled.Phase)
		}
	}
}

// retireGeneral retires the standing general plan, which Model v2 doesn't
// have: its items become unplanned (or untagged backlog), its entries
// project-wide, and the plan done. A general plan with phases is real work,
// so it stays as it is.
func (g *migration) retireGeneral() {
	s := g.s
	p := s.plans[GeneralPlan]
	if p == nil || p.State == model.PlanDone {
		return
	}
	for _, e := range s.phases {
		if e.Plan == GeneralPlan {
			g.res.Warnings = append(g.res.Warnings, "plan general has phases, so it stays a plan, with its items and entries")
			return
		}
	}
	for _, id := range sortedKeys(s.items) {
		if i := s.items[id]; i.Plan == GeneralPlan {
			i.Plan, i.Updated = "", g.now
			g.item(id, "plan general → unplanned")
		}
	}
	for _, id := range sortedKeys(s.entries) {
		if e := s.entries[id]; e.Plan == GeneralPlan {
			e.Plan = ""
			g.entry(id, "plan general → project-wide")
		}
	}
	p.State, p.Updated = model.PlanDone, g.now
	g.retired = true
	g.plan(GeneralPlan, "retired (state: done): Model v2 has no standing plan; its work is unplanned now")
}

// captureLegacyEntries turns open and parked loose ends and inbox ideas into
// captured ideas, and settles the entries as do, pointing at them.
func (g *migration) captureLegacyEntries() error {
	s := g.s
	for _, id := range sortedKeys(s.entries) {
		e := s.entries[id]
		if !retiredKind(e.Kind) || (e.Settled != nil && e.Settled.Action != model.ActPark) {
			continue
		}
		was := "open"
		notes := e.Detail
		if e.Settled != nil {
			was = "parked"
			if e.Settled.Note != "" {
				notes = joinNotes(notes, "Parked in the inbox: "+e.Settled.Note)
			}
		}
		if e.Item != "" {
			notes = joinNotes(notes, "Filed on item "+e.Item+".")
		}
		plan := e.Plan
		if p := s.plans[plan]; plan != "" && (p == nil || p.State == model.PlanDone) {
			g.res.Warnings = append(g.res.Warnings, fmt.Sprintf("%s %s was on plan %q, which is done or missing, so its captured item is untagged", e.Kind, e.ID, plan))
			plan = ""
		}
		itemID, err := s.newID()
		if err != nil {
			return err
		}
		i := &model.Item{
			ID: itemID, Plan: plan, Title: e.Text, State: model.StateCaptured, Kind: model.ItemIdea,
			Source: e.Source, From: e.ID, Created: e.Created, Updated: g.now, Notes: notes,
		}
		if err := s.putItem(i); err != nil {
			return err
		}
		e.Settled = &model.Settlement{Action: model.ActDo, Note: "mull migrate: captured as idea " + itemID,
			Item: itemID, By: g.m.Worktree(), OnRequest: true, At: g.now}
		g.entry(id, fmt.Sprintf("%s %s: settled do, captured as idea %s", was, e.Kind, itemID))
		g.res.Captured = append(g.res.Captured, MigrateCapture{Item: itemID, Kind: model.ItemIdea, Title: e.Text,
			Plan: plan, Entry: e.ID, EntryKind: e.Kind, Was: was})
	}
	return nil
}

// convertPhaseNeeds turns a phase's needs on items, from before needs were
// between phases only, into needs on those items' phases. A need already
// met, on an item with no phase or in the same phase, or one that would make
// a cycle, is dropped.
func (g *migration) convertPhaseNeeds() error {
	s := g.s
	for _, id := range sortedKeys(s.phases) {
		e := s.phases[id]
		orig := e.Needs
		var keep []string
		changed := false
		for k, n := range orig {
			i := s.items[n]
			if i == nil {
				keep = append(keep, n)
				continue
			}
			changed = true
			need := MigrateNeed{Type: "phase", ID: id, Need: n, Action: "dropped"}
			switch {
			case finished(i):
				need.Reason = "item " + n + " is " + i.State + ", so the need is met"
			case i.Phase == "":
				need.Reason = "item " + n + " is in no phase"
			case i.Phase == id:
				need.Reason = "item " + n + " is in this phase"
			case containsStr(keep, i.Phase) || containsStr(orig, i.Phase):
				need.Reason = "the phase already needs phase " + i.Phase
			default:
				// Try it with the needs kept so far and those still to come.
				e.Needs = append(append(append([]string{}, keep...), i.Phase), orig[k+1:]...)
				if err := s.checkCycles(); err != nil {
					need.Reason = "needing phase " + i.Phase + " would make a cycle"
				} else {
					keep = append(keep, i.Phase)
					need.Action, need.To = "converted", i.Phase
					need.Reason = "needs exist between phases only, so it needs item " + n + "'s phase"
				}
			}
			g.res.Needs = append(g.res.Needs, need)
			g.phase(id, fmt.Sprintf("need on item %s %s", n, map[string]string{
				"dropped": "dropped", "converted": "→ need on phase " + need.To}[need.Action]))
		}
		e.Needs = keep
		if changed {
			e.Updated = g.now
		}
	}
	return nil
}

// dropItemNeeds removes items' needs (items have no needs in Model v2). An
// unfinished item keeps them in its notes.
func (g *migration) dropItemNeeds() {
	s := g.s
	for _, id := range sortedKeys(s.items) {
		i := s.items[id]
		if len(i.Needs) == 0 {
			continue
		}
		for _, n := range i.Needs {
			g.res.Needs = append(g.res.Needs, MigrateNeed{Type: "item", ID: id, Need: n, Action: "dropped",
				Reason: "items have no needs; order work with needs between phases"})
		}
		change := "needs " + strings.Join(i.Needs, ", ") + " dropped"
		if !finished(i) {
			i.Notes = joinNotes(i.Notes, "Before Model v2 this item needed "+strings.Join(i.Needs, ", ")+
				". Items have no needs now; order work with needs between phases.")
			change += " (kept in its notes)"
		}
		i.Needs, i.Updated = nil, g.now
		g.item(id, change)
	}
}

// write writes back every changed record in format 2, moves phases and
// their docs, adds doc stubs (first migration only) and records the format.
func (g *migration) write(first bool) error {
	s := g.s
	for slug := range g.plans {
		if err := s.putPlan(s.plans[slug]); err != nil {
			return err
		}
	}
	for _, id := range sortedKeys(g.phases) {
		e := s.phases[id]
		e.Legacy = false // it is written to phases/
		if err := s.putPhase(e); err != nil {
			return err
		}
	}
	// Format 1 paths left behind: the moved phases, and copies of phases
	// also in phases/ (stragglers from an older mull: phases/ wins), then
	// docs.
	for _, p := range s.tx.List("epics/") {
		id := baseName(p)
		if r := g.records["phase/"+id]; r == nil || r.From == "" {
			g.res.Warnings = append(g.res.Warnings, fmt.Sprintf("phase %s was in both epics/ and phases/; kept %s", id, model.PhasePath(id)))
			g.note("phase", id, model.PhasePath(id), "format 1 copy "+p+" removed ("+model.PhasePath(id)+" kept)")
		}
		s.tx.Delete(p)
	}
	for _, p := range s.tx.List("docs/epics/") {
		id := baseName(p)
		b, _ := s.tx.Get(p)
		r := g.note("phase", id, model.PhasePath(id), "")
		if _, ok := s.tx.Get(model.PhaseDocPath(id)); ok {
			g.res.Warnings = append(g.res.Warnings, fmt.Sprintf("phase %s's doc was in both docs/epics/ and docs/phases/; kept docs/phases/%s.md", id, id))
			r.Changes = append(r.Changes, "doc "+p+" removed (docs/phases/ kept)")
		} else {
			s.tx.Put(model.PhaseDocPath(id), b)
			r.Changes = append(r.Changes, "doc "+p+" → "+model.PhaseDocPath(id))
			g.res.Counts.Docs++
		}
		s.tx.Delete(p)
	}
	for id := range g.items {
		if err := s.putItem(s.items[id]); err != nil {
			return err
		}
	}
	for id := range g.entries {
		if err := s.putEntry(s.entries[id]); err != nil {
			return err
		}
	}
	if first {
		g.docStubs()
		c := s.config
		c.Format = model.Format
		b, err := yaml.Marshal(c)
		if err != nil {
			return err
		}
		s.tx.Put(model.ConfigPath, b)
		s.config = c
		g.note("config", "config", model.ConfigPath, fmt.Sprintf("format: %d → format: %d", g.res.FormatBefore, model.Format))
	}
	return nil
}

// docStubs gives every plan and phase with a summary and no doc a doc stub:
// its summary, for the human to grow into the doc. The summary stays. The
// general plan, if retired, gets none.
func (g *migration) docStubs() {
	s := g.s
	for _, slug := range sortedKeys(s.plans) {
		p := s.plans[slug]
		if p.Summary == "" || p.Doc != "" || (slug == GeneralPlan && g.retired) {
			continue
		}
		s.putDoc(model.PlanDocPath(slug), p.Summary)
		p.Doc = getDoc(s.tx, model.PlanDocPath(slug))
		g.note("plan", slug, model.PlanPath(slug), "doc stub from its summary ("+model.PlanDocPath(slug)+")")
		g.res.Counts.Docs++
	}
	for _, id := range sortedKeys(s.phases) {
		e := s.phases[id]
		if e.Summary == "" || e.Doc != "" {
			continue
		}
		s.putDoc(model.PhaseDocPath(id), e.Summary)
		e.Doc = getDoc(s.tx, model.PhaseDocPath(id))
		g.note("phase", id, model.PhasePath(id), "doc stub from its summary ("+model.PhaseDocPath(id)+")")
		g.res.Counts.Docs++
	}
}

var migrateTypeOrder = map[string]int{"config": 0, "plan": 1, "phase": 2, "item": 3, "entry": 4}

func (g *migration) finish() {
	for _, r := range g.records {
		if r.Type == "phase" && r.From != "" {
			r.Changes = append([]string{r.From + " → " + r.Path}, r.Changes...)
		}
		g.res.Records = append(g.res.Records, *r)
		switch r.Type {
		case "plan":
			g.res.Counts.Plans++
		case "phase":
			g.res.Counts.Phases++
		case "item":
			g.res.Counts.Items++
		case "entry":
			g.res.Counts.Entries++
		}
	}
	sort.Slice(g.res.Records, func(a, b int) bool {
		ra, rb := g.res.Records[a], g.res.Records[b]
		if ra.Type != rb.Type {
			return migrateTypeOrder[ra.Type] < migrateTypeOrder[rb.Type]
		}
		return ra.ID < rb.ID
	})
	g.res.Counts.Captured = len(g.res.Captured)
	g.res.Counts.Needs = len(g.res.Needs)
	g.res.Changed = len(g.res.Records) > 0 || len(g.res.Captured) > 0
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Text renders the result for the human.
func (r MigrateResult) Text() string {
	var b strings.Builder
	switch {
	case !r.Changed:
		fmt.Fprintf(&b, "Nothing to migrate: the data is format %d.\n", r.Format)
		return b.String()
	case r.DryRun:
		fmt.Fprintf(&b, "mull migrate --dry-run: format %d → %d. Nothing was written.\n", r.FormatBefore, r.Format)
	default:
		fmt.Fprintf(&b, "mull migrate: format %d → %d, in commit %s.\n", r.FormatBefore, r.Format, short(r.Commit))
	}
	c := r.Counts
	entries := fmt.Sprintf("%d inbox entries", c.Entries)
	if c.Entries == 1 {
		entries = "1 inbox entry"
	}
	fmt.Fprintf(&b, "%s · %s · %s · %s · %s · %d captured · %s\n",
		plural(c.Plans, "plan"), plural(c.Phases, "phase"), plural(c.Items, "item"), entries,
		plural(c.Docs, "doc"), c.Captured, plural(c.Needs, "need"))
	titles := map[string]string{"config": "CONFIG", "plan": "PLANS", "phase": "PHASES", "item": "ITEMS", "entry": "ENTRIES"}
	last := ""
	for _, rec := range r.Records {
		if rec.Type != last {
			fmt.Fprintf(&b, "\n%s\n", titles[rec.Type])
			last = rec.Type
		}
		fmt.Fprintf(&b, "  %-10s %s\n", rec.ID, strings.Join(rec.Changes, "; "))
	}
	if len(r.Captured) > 0 {
		fmt.Fprintf(&b, "\nCAPTURED as ideas (re-kind the bugs: mull set <id> kind bug)\n")
		for _, cp := range r.Captured {
			plan := cp.Plan
			if plan == "" {
				plan = "untagged"
			}
			fmt.Fprintf(&b, "  %s  from %s %s (%s)  %-12s %s\n", cp.Item, cp.Was, cp.EntryKind, cp.Entry, plan, cp.Title)
		}
	}
	if len(r.Needs) > 0 {
		b.WriteString("\nNEEDS\n")
		for _, n := range r.Needs {
			what := n.Action
			if n.To != "" {
				what += " to phase " + n.To
			}
			fmt.Fprintf(&b, "  %s %s needed item %s: %s (%s)\n", n.Type, n.ID, n.Need, what, n.Reason)
		}
	}
	if len(r.Warnings) > 0 {
		b.WriteString("\nWARNINGS\n")
		for _, w := range r.Warnings {
			fmt.Fprintf(&b, "  %s\n", w)
		}
	}
	if r.DryRun {
		b.WriteString("\nIDs of captured items are throwaway in a dry run. Migrating is the human's call: mull migrate --on-request.\n")
	}
	return b.String()
}
