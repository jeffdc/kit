package mull

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"mull/internal/model"
)

// Every write is an op: a typed input whose JSON field names are exactly the
// command's flag names (positional arguments get the names documented in
// mull guide json). The CLI and mull batch run the same ops, so a command
// and a batch line can never behave differently.

// IDList is a list of IDs. In JSON it may be an array or one string of IDs
// separated by commas or spaces.
type IDList []string

func (l *IDList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*l = SplitIDs([]string{one})
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return fmt.Errorf("want a string or an array of strings")
	}
	*l = SplitIDs(many)
	return nil
}

// StrList is a list of strings. In JSON it may be an array or one string.
type StrList []string

func (l *StrList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*l = []string{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return fmt.Errorf("want a string or an array of strings")
	}
	*l = many
	return nil
}

// SplitIDs splits "a,b c" style lists.
func SplitIDs(vals []string) []string {
	var out []string
	for _, v := range vals {
		out = append(out, strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' })...)
	}
	return out
}

// Input is one op's arguments.
type Input interface {
	// refFields returns the fields that may hold @refs in a batch.
	refFields() ([]*string, []*IDList)
	// run applies the op to the state; id is what a batch ref names.
	run(m *Mull, s *state) (result any, id string, err error)
}

type opDef struct {
	new  func() Input
	desc string
}

var ops = map[string]opDef{
	"plan new":      {func() Input { return &PlanNewIn{} }, "slug, title, summary"},
	"plan agree":    {func() Input { return &PlanAgreeIn{} }, "slug, on-request"},
	"plan close":    {func() Input { return &PlanCloseIn{} }, "slug"},
	"plan doc":      {func() Input { return &PlanDocIn{} }, "slug, doc"},
	"phase add":     {func() Input { return &PhaseAddIn{} }, "title, plan, summary, needs"},
	"phase doc":     {func() Input { return &PhaseDocIn{} }, "id, doc"},
	"add":           {func() Input { return &AddIn{} }, "title, plan, phase, notes, doc"},
	"set":           {func() Input { return &SetIn{} }, "id, field, value"},
	"need":          {func() Input { return &NeedIn{add: true} }, "id, needs"},
	"unneed":        {func() Input { return &NeedIn{} }, "id, needs"},
	"decided":       {func() Input { return &CaptureIn{kind: model.KindDecision} }, "text, item, plan, source, detail"},
	"ask":           {func() Input { return &CaptureIn{kind: model.KindQuestion} }, "text, item, plan, source, detail"},
	"loose":         {func() Input { return &CaptureIn{kind: model.KindLoose} }, "retired: use bug or idea"},
	"bug":           {func() Input { return &CaptureItemIn{kind: model.ItemBug} }, "text, plan, phase, category, source, detail"},
	"idea":          {func() Input { return &CaptureItemIn{kind: model.ItemIdea} }, "text, plan, phase, category, source, detail"},
	"commit":        {func() Input { return &CommitIn{} }, "id, plan, phase, on-request"},
	"triage":        {func() Input { return &TriageIn{} }, "ids, all, plan, on-request"},
	"start":         {func() Input { return &StartIn{} }, "id"},
	"release":       {func() Input { return &ReleaseIn{} }, "id"},
	"accept":        {func() Input { return &AcceptIn{} }, "ids, follow-up, on-request"},
	"decide":        {func() Input { return &DecideIn{} }, "id, decision, on-request"},
	"do":            {func() Input { return &DoIn{} }, "id, plan, phase, as, reason, on-request"},
	"drop":          {func() Input { return &DropIn{} }, "ids, reason, on-request"},
	"park":          {func() Input { return &ParkIn{} }, "id, reason, on-request"},
	"config set":    {func() Input { return &ConfigSetIn{} }, "key, value"},
	"config checks": {func() Input { return &ConfigChecksIn{} }, "commands"},
	"config layer":  {func() Input { return &ConfigLayerIn{} }, "name, globs"},
}

// OpNames lists the ops batch accepts.
func OpNames() []string {
	names := make([]string, 0, len(ops))
	for n := range ops {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// NewInput returns an empty input for the named op.
func NewInput(op string) (Input, error) {
	d, ok := ops[op]
	if !ok {
		return nil, errf(CodeUsage, "unknown op %q (ops: %s)", op, strings.Join(OpNames(), ", "))
	}
	return d.new(), nil
}

// Exec runs one op in its own transaction and returns its result.
func (m *Mull) Exec(op string, in Input) (any, error) {
	var out any
	err := m.update(op, func(s *state) error {
		r, _, err := in.run(m, s)
		out = r
		return err
	})
	return out, err
}

// ---- Inputs ----

type PlanNewIn struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

func (in *PlanNewIn) refFields() ([]*string, []*IDList) { return nil, nil }
func (in *PlanNewIn) run(m *Mull, s *state) (any, string, error) {
	p, err := m.planNew(s, in.Slug, in.Title, in.Summary)
	if err != nil {
		return nil, "", err
	}
	return p, p.Slug, nil
}

type PlanAgreeIn struct {
	Slug      string `json:"slug"`
	OnRequest bool   `json:"on-request"`
}

func (in *PlanAgreeIn) refFields() ([]*string, []*IDList) { return []*string{&in.Slug}, nil }
func (in *PlanAgreeIn) run(m *Mull, s *state) (any, string, error) {
	p, err := m.planAgree(s, in.Slug, in.OnRequest)
	if err != nil {
		return nil, "", err
	}
	return p, p.Slug, nil
}

type PlanCloseIn struct {
	Slug string `json:"slug"`
}

func (in *PlanCloseIn) refFields() ([]*string, []*IDList) { return []*string{&in.Slug}, nil }
func (in *PlanCloseIn) run(m *Mull, s *state) (any, string, error) {
	p, err := m.planClose(s, in.Slug)
	if err != nil {
		return nil, "", err
	}
	return p, p.Slug, nil
}

type PhaseAddIn struct {
	Title   string `json:"title"`
	Plan    string `json:"plan"`
	Summary string `json:"summary"`
	Needs   IDList `json:"needs"`
}

func (in *PhaseAddIn) refFields() ([]*string, []*IDList) {
	return []*string{&in.Plan}, []*IDList{&in.Needs}
}
func (in *PhaseAddIn) run(m *Mull, s *state) (any, string, error) {
	e, err := m.phaseAdd(s, in.Plan, in.Title, in.Summary, in.Needs, "")
	if err != nil {
		return nil, "", err
	}
	return e, e.ID, nil
}

// PlanDocIn replaces a plan's doc. Doc is required ("" removes the doc), so
// a batch line can't clear a doc by leaving the field out.
type PlanDocIn struct {
	Slug string  `json:"slug"`
	Doc  *string `json:"doc"`
}

func (in *PlanDocIn) refFields() ([]*string, []*IDList) { return []*string{&in.Slug}, nil }
func (in *PlanDocIn) run(m *Mull, s *state) (any, string, error) {
	if in.Doc == nil {
		return nil, "", errf(CodeInvalid, `plan doc needs "doc": the new doc ("" removes it)`)
	}
	r, err := m.setPlanDoc(s, in.Slug, *in.Doc)
	return r, in.Slug, err
}

// PhaseDocIn replaces a phase's doc, as PlanDocIn does a plan's.
type PhaseDocIn struct {
	ID  string  `json:"id"`
	Doc *string `json:"doc"`
}

func (in *PhaseDocIn) refFields() ([]*string, []*IDList) { return []*string{&in.ID}, nil }
func (in *PhaseDocIn) run(m *Mull, s *state) (any, string, error) {
	if in.Doc == nil {
		return nil, "", errf(CodeInvalid, `phase doc needs "doc": the new doc ("" removes it)`)
	}
	r, err := m.setPhaseDoc(s, in.ID, *in.Doc)
	return r, in.ID, err
}

type AddIn struct {
	Title string  `json:"title"`
	Plan  string  `json:"plan"`
	Phase string  `json:"phase"`
	Needs IDList  `json:"needs"` // refused: items have no needs
	Notes string  `json:"notes"`
	Doc   StrList `json:"doc"`
}

func (in *AddIn) refFields() ([]*string, []*IDList) {
	return []*string{&in.Plan, &in.Phase}, nil
}
func (in *AddIn) run(m *Mull, s *state) (any, string, error) {
	if len(in.Needs) > 0 {
		return nil, "", errItemNeeds("add takes no needs")
	}
	i, err := m.addItem(s, NewItem{Plan: in.Plan, Phase: in.Phase, Title: in.Title, Notes: in.Notes, Docs: in.Doc})
	if err != nil {
		return nil, "", err
	}
	return s.itemView(i), i.ID, nil
}

type SetIn struct {
	ID    string `json:"id"`
	Field string `json:"field"`
	Value string `json:"value"`
}

func (in *SetIn) refFields() ([]*string, []*IDList) { return []*string{&in.ID}, nil }
func (in *SetIn) run(m *Mull, s *state) (any, string, error) {
	r, err := m.set(s, in.ID, in.Field, in.Value)
	return r, in.ID, err
}

type NeedIn struct {
	ID    string `json:"id"`
	Needs IDList `json:"needs"`
	add   bool
}

func (in *NeedIn) refFields() ([]*string, []*IDList) {
	return []*string{&in.ID}, []*IDList{&in.Needs}
}
func (in *NeedIn) run(m *Mull, s *state) (any, string, error) {
	r, err := m.changeNeeds(s, in.ID, in.Needs, in.add)
	return r, in.ID, err
}

type CaptureIn struct {
	Text   string `json:"text"`
	Item   string `json:"item"`
	Plan   string `json:"plan"`
	Source string `json:"source"`
	Detail string `json:"detail"`
	kind   string
}

func (in *CaptureIn) refFields() ([]*string, []*IDList) { return []*string{&in.Item, &in.Plan}, nil }
func (in *CaptureIn) run(m *Mull, s *state) (any, string, error) {
	e, err := m.capture(s, CaptureInput{Kind: in.kind, Text: in.Text, Item: in.Item, Plan: in.Plan, Source: in.Source, Detail: in.Detail})
	if err != nil {
		return nil, "", err
	}
	return e, e.ID, nil
}

// CaptureItemIn captures a bug or idea. Text is the title, as for the other
// captures, and detail becomes its notes.
type CaptureItemIn struct {
	Text     string `json:"text"`
	Plan     string `json:"plan"`
	Phase    string `json:"phase"`
	Category string `json:"category"`
	Source   string `json:"source"`
	Detail   string `json:"detail"`
	kind     string
}

func (in *CaptureItemIn) refFields() ([]*string, []*IDList) {
	return []*string{&in.Plan, &in.Phase}, nil
}
func (in *CaptureItemIn) run(m *Mull, s *state) (any, string, error) {
	i, err := m.captureItem(s, NewCapture{Kind: in.kind, Title: in.Text, Plan: in.Plan, Phase: in.Phase,
		Category: in.Category, Source: in.Source, Detail: in.Detail})
	if err != nil {
		return nil, "", err
	}
	return s.itemView(i), i.ID, nil
}

type CommitIn struct {
	ID        string `json:"id"`
	Plan      string `json:"plan"`
	Phase     string `json:"phase"`
	OnRequest bool   `json:"on-request"`
}

func (in *CommitIn) refFields() ([]*string, []*IDList) {
	return []*string{&in.ID, &in.Plan, &in.Phase}, nil
}
func (in *CommitIn) run(m *Mull, s *state) (any, string, error) {
	r, err := m.commit(s, CommitInput{ID: in.ID, Plan: in.Plan, Phase: in.Phase, OnRequest: in.OnRequest})
	return r, in.ID, err
}

// TriageIn stamps captured items as triaged: ids, or all (in plan).
type TriageIn struct {
	IDs       IDList `json:"ids"`
	All       bool   `json:"all"`
	Plan      string `json:"plan"`
	OnRequest bool   `json:"on-request"`
}

func (in *TriageIn) refFields() ([]*string, []*IDList) {
	return []*string{&in.Plan}, []*IDList{&in.IDs}
}
func (in *TriageIn) run(m *Mull, s *state) (any, string, error) {
	r, err := m.triage(s, TriageInput{IDs: in.IDs, All: in.All, Plan: in.Plan, OnRequest: in.OnRequest})
	return r, "", err
}

type StartIn struct {
	ID string `json:"id"`
}

func (in *StartIn) refFields() ([]*string, []*IDList) { return []*string{&in.ID}, nil }
func (in *StartIn) run(m *Mull, s *state) (any, string, error) {
	r, err := m.start(s, in.ID)
	return r, in.ID, err
}

type ReleaseIn struct {
	ID string `json:"id"`
}

func (in *ReleaseIn) refFields() ([]*string, []*IDList) { return []*string{&in.ID}, nil }
func (in *ReleaseIn) run(m *Mull, s *state) (any, string, error) {
	r, err := m.release(s, in.ID)
	return r, in.ID, err
}

type AcceptIn struct {
	IDs       IDList `json:"ids"`
	FollowUp  string `json:"follow-up"`
	OnRequest bool   `json:"on-request"`
}

func (in *AcceptIn) refFields() ([]*string, []*IDList) { return nil, []*IDList{&in.IDs} }
func (in *AcceptIn) run(m *Mull, s *state) (any, string, error) {
	r, err := m.accept(s, in.IDs, in.FollowUp, SettleOpts{OnRequest: in.OnRequest})
	id := ""
	if r.FollowUp != nil {
		id = r.FollowUp.ID
	}
	return r, id, err
}

type DecideIn struct {
	ID        string `json:"id"`
	Decision  string `json:"decision"`
	OnRequest bool   `json:"on-request"`
}

func (in *DecideIn) refFields() ([]*string, []*IDList) { return []*string{&in.ID}, nil }
func (in *DecideIn) run(m *Mull, s *state) (any, string, error) {
	e, err := m.decide(s, in.ID, in.Decision, SettleOpts{OnRequest: in.OnRequest})
	return e, in.ID, err
}

type DoIn struct {
	ID        string `json:"id"`
	Plan      string `json:"plan"`
	Phase     string `json:"phase"`
	As        string `json:"as"`
	Reason    string `json:"reason"`
	OnRequest bool   `json:"on-request"`
}

func (in *DoIn) refFields() ([]*string, []*IDList) {
	return []*string{&in.ID, &in.Plan, &in.Phase}, nil
}
func (in *DoIn) run(m *Mull, s *state) (any, string, error) {
	r, err := m.do(s, DoInput{ID: in.ID, Plan: in.Plan, Phase: in.Phase, As: in.As, Reason: in.Reason, SettleOpts: SettleOpts{OnRequest: in.OnRequest}})
	id := ""
	switch {
	case r.Phase != nil:
		id = r.Phase.ID
	case r.Item != nil:
		id = r.Item.ID
	}
	return r, id, err
}

type DropIn struct {
	IDs       IDList `json:"ids"`
	Reason    string `json:"reason"`
	OnRequest bool   `json:"on-request"`
}

func (in *DropIn) refFields() ([]*string, []*IDList) { return nil, []*IDList{&in.IDs} }
func (in *DropIn) run(m *Mull, s *state) (any, string, error) {
	r, err := m.drop(s, in.IDs, in.Reason, SettleOpts{OnRequest: in.OnRequest})
	return r, "", err
}

type ParkIn struct {
	ID        string `json:"id"`
	Reason    string `json:"reason"`
	OnRequest bool   `json:"on-request"`
}

func (in *ParkIn) refFields() ([]*string, []*IDList) { return []*string{&in.ID}, nil }
func (in *ParkIn) run(m *Mull, s *state) (any, string, error) {
	e, err := m.park(s, in.ID, in.Reason, SettleOpts{OnRequest: in.OnRequest})
	return e, in.ID, err
}

type ConfigSetIn struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (in *ConfigSetIn) refFields() ([]*string, []*IDList) { return nil, nil }
func (in *ConfigSetIn) run(m *Mull, s *state) (any, string, error) {
	c, err := m.configSet(s, in.Key, in.Value)
	return c, "", err
}

type ConfigChecksIn struct {
	Commands StrList `json:"commands"`
}

func (in *ConfigChecksIn) refFields() ([]*string, []*IDList) { return nil, nil }
func (in *ConfigChecksIn) run(m *Mull, s *state) (any, string, error) {
	c, err := m.configChecks(s, in.Commands)
	return c, "", err
}

type ConfigLayerIn struct {
	Name  string  `json:"name"`
	Globs StrList `json:"globs"`
}

func (in *ConfigLayerIn) refFields() ([]*string, []*IDList) { return nil, nil }
func (in *ConfigLayerIn) run(m *Mull, s *state) (any, string, error) {
	c, err := m.configLayer(s, in.Name, in.Globs)
	return c, "", err
}

// ---- Batch ----

// BatchResult is one line's outcome, in input order.
type BatchResult struct {
	Line   int    `json:"line"`
	Op     string `json:"op"`
	Ref    string `json:"ref,omitempty"`
	ID     string `json:"id,omitempty"` // what @ref names: the record created (or changed)
	Result any    `json:"result"`       // exactly what the single command prints
}

type batchLine struct {
	n     int
	op    string
	ref   string
	input Input
}

var errDryRun = errors.New("dry run")

// maxBatchLine bounds one JSON line (notes can be long).
const maxBatchLine = 16 << 20

// Batch runs JSON Lines of ops in one transaction: all of them, as a single
// commit, or none. Each line is {"op": "<name>", "ref": "<optional>", ...fields}.
// A later line may use "@ref" in an ID field (plan, phase, needs, item, id,
// ids) to mean the record an earlier line created. With dryRun everything is
// validated and run but nothing is written; IDs in a dry run's results are
// throwaway.
func (m *Mull) Batch(r io.Reader, dryRun bool) ([]BatchResult, error) {
	var lines []batchLine
	var bad []LineError
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), maxBatchLine)
	n := 0
	for sc.Scan() {
		n++
		raw := bytes.TrimSpace(sc.Bytes())
		if len(raw) == 0 {
			continue
		}
		bl, lerr := parseLine(n, raw)
		if lerr != nil {
			bad = append(bad, *lerr)
			continue
		}
		lines = append(lines, bl)
	}
	if err := sc.Err(); err != nil {
		return nil, errf(CodeInvalidJSON, "reading batch input: %v", err)
	}
	if len(lines) == 0 && len(bad) == 0 {
		return nil, errf(CodeInvalid, "the batch is empty")
	}

	var results []BatchResult
	err := m.update(fmt.Sprintf("batch of %d operations", len(lines)), func(s *state) error {
		results = make([]BatchResult, 0, len(lines))
		runBad := append([]LineError{}, bad...)
		refIDs := map[string]string{} // ref -> ID
		refLine := map[string]int{}   // ref -> line that defined it
		failedRef := map[string]int{} // ref -> line that failed to define it
		fail := func(bl batchLine, err error) {
			e := AsError(err)
			runBad = append(runBad, LineError{Line: bl.n, Op: bl.op, Ref: bl.ref, Code: e.Code, Error: e.Msg})
			if bl.ref != "" {
				failedRef[bl.ref] = bl.n
			}
		}
		for _, bl := range lines {
			if bl.ref != "" {
				if prev, ok := refLine[bl.ref]; ok {
					fail(bl, errf(CodeExists, "ref %q is already defined on line %d", bl.ref, prev))
					continue
				}
			}
			if err := resolveRefs(bl.input, refIDs, failedRef); err != nil {
				fail(bl, err)
				continue
			}
			res, id, err := bl.input.run(m, s)
			if err != nil {
				fail(bl, err)
				continue
			}
			if bl.ref != "" {
				if id == "" {
					fail(bl, errf(CodeInvalid, "op %q creates no record a ref could name", bl.op))
					continue
				}
				refIDs[bl.ref], refLine[bl.ref] = id, bl.n
			}
			results = append(results, BatchResult{Line: bl.n, Op: bl.op, Ref: bl.ref, ID: id, Result: res})
		}
		if len(runBad) > 0 {
			sort.SliceStable(runBad, func(a, b int) bool { return runBad[a].Line < runBad[b].Line })
			return &Error{Code: CodeBatchInvalid,
				Msg:   fmt.Sprintf("%d of %d batch lines failed; nothing was written", len(runBad), len(lines)+len(bad)),
				Lines: runBad}
		}
		if dryRun {
			return errDryRun
		}
		return nil
	})
	if errors.Is(err, errDryRun) {
		return results, nil
	}
	if err != nil {
		return nil, err
	}
	return results, nil
}

func parseLine(n int, raw []byte) (batchLine, *LineError) {
	bl := batchLine{n: n}
	lineErr := func(code, msg string) *LineError {
		return &LineError{Line: n, Op: bl.op, Ref: bl.ref, Code: code, Error: msg}
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return bl, lineErr(CodeInvalidJSON, "not a JSON object: "+err.Error())
	}
	if v, ok := fields["op"]; !ok || json.Unmarshal(v, &bl.op) != nil || bl.op == "" {
		return bl, lineErr(CodeInvalid, `every line needs "op": "<name>"`)
	}
	if v, ok := fields["ref"]; ok {
		if json.Unmarshal(v, &bl.ref) != nil || bl.ref == "" || strings.HasPrefix(bl.ref, "@") {
			return bl, lineErr(CodeInvalid, `"ref" must be a non-empty string without a leading @`)
		}
	}
	delete(fields, "op")
	delete(fields, "ref")
	in, err := NewInput(bl.op)
	if err != nil {
		e := AsError(err)
		return bl, lineErr(e.Code, e.Msg)
	}
	rest, _ := json.Marshal(fields)
	dec := json.NewDecoder(bytes.NewReader(rest))
	dec.DisallowUnknownFields()
	if err := dec.Decode(in); err != nil {
		return bl, lineErr(CodeInvalid, fmt.Sprintf("bad fields for %q (fields: %s): %v", bl.op, ops[bl.op].desc, err))
	}
	bl.input = in
	return bl, nil
}

func resolveRefs(in Input, ids map[string]string, failed map[string]int) error {
	resolve := func(v string) (string, error) {
		if !strings.HasPrefix(v, "@") {
			return v, nil
		}
		name := v[1:]
		if id, ok := ids[name]; ok {
			return id, nil
		}
		if line, ok := failed[name]; ok {
			return "", errf(CodeInvalid, "%s refers to line %d, which failed", v, line)
		}
		return "", errf(CodeNotFound, "unknown ref %s: refs must be defined on an earlier line", v)
	}
	singles, lists := in.refFields()
	for _, p := range singles {
		v, err := resolve(*p)
		if err != nil {
			return err
		}
		*p = v
	}
	for _, l := range lists {
		for k, v := range *l {
			r, err := resolve(v)
			if err != nil {
				return err
			}
			(*l)[k] = r
		}
	}
	return nil
}
