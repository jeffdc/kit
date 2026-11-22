// Package bench measures mull's reads: it seeds large repositories and times
// every page mull serve shows, in-process and over HTTP, counting the git
// processes each one starts.
package bench

import (
	"bufio"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"mull/internal/model"
)

// Size is the shape of a seeded repository.
type Size struct {
	Plans         int // the last two are active and one more is shaping; the rest are done
	PhasesPerPlan int
	Items         int // planned, unplanned and untagged captures together
	Span          time.Duration
	// Targets is how many of the active plans (at most two) get a target
	// branch of their own, cut from main when the plan starts; 0 in the
	// named sizes, so every plan lands on integration.
	Targets int
}

// Sizes are the named sizes Seed accepts.
var Sizes = map[string]Size{
	"small":  {Plans: 4, PhasesPerPlan: 3, Items: 80, Span: 60 * 24 * time.Hour},
	"medium": {Plans: 12, PhasesPerPlan: 5, Items: 1500, Span: 365 * 24 * time.Hour},
	"large":  {Plans: 25, PhasesPerPlan: 7, Items: 5000, Span: 2 * 365 * 24 * time.Hour},
}

// Seed creates a git repository at dir holding a mull branch of the given
// size, as mull's own writes would have built it: one commit per write, with
// plans agreed and closed, items captured, committed, started, finished with
// a report and a review, and accepted, decisions and questions filed and
// settled, and the integration branch (main) carrying each item's work with
// a Mull trailer. History ends at end. The same seed gives the same records.
//
// It writes through git fast-import rather than mull's ops: each op reloads
// the whole branch, so building tens of thousands of commits that way takes
// most of an hour.
func Seed(dir string, size Size, seed uint64, end time.Time) error {
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s already exists", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := gitIn(dir, nil, "init", "-q", "-b", "main"); err != nil {
		return err
	}
	g := newGen(size, seed, end)
	g.plan()
	cmd := exec.Command("git", "fast-import", "--quiet", "--done")
	cmd.Dir = dir
	cmd.Env = isolated()
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	werr := g.write(bufio.NewWriterSize(in, 1<<20))
	in.Close()
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git fast-import: %v: %s", err, stderr.String())
	}
	if werr != nil {
		return werr
	}
	return gitIn(dir, nil, "reset", "-q", "--hard")
}

// isolated is the environment for git commands Seed runs: no global or
// system config, so the seeded repository is the same on every machine.
func isolated() []string {
	return append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
}

func gitIn(dir string, stdin io.Reader, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = isolated()
	cmd.Stdin = stdin
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return nil
}

// An event is one commit: to the mull branch (apply changes the records and
// returns what it wrote) or, with work set, to main.
type event struct {
	at    time.Time
	seq   int
	msg   string
	apply func(c *commit)
	work  string // main: the item whose work this commit carries
	// branch, when set, is no commit: it creates that branch where main is.
	branch string
}

// commit collects one mull commit's file writes.
type commit struct{ files map[string][]byte }

func (c *commit) put(path string, v any, body string) {
	b, err := model.Encode(v, body)
	if err != nil {
		panic(err) // records here always encode
	}
	c.files[path] = b
}

func (c *commit) doc(path, doc string) { c.files[path] = []byte(doc + "\n") }

type gen struct {
	size   Size
	rnd    *rand.Rand
	start  time.Time
	end    time.Time
	events []event
	used   map[string]bool
	wt     string // the worktree records name as writer
}

func newGen(size Size, seed uint64, end time.Time) *gen {
	end = end.UTC().Truncate(time.Second)
	return &gen{
		size:  size,
		rnd:   rand.New(rand.NewPCG(seed, seed^0x6d756c6c)),
		start: end.Add(-size.Span),
		end:   end,
		used:  map[string]bool{},
		wt:    "/work/bench",
	}
}

func (g *gen) id() string {
	for {
		id := fmt.Sprintf("%04x", g.rnd.IntN(1<<16))
		if !g.used[id] {
			g.used[id] = true
			return id
		}
	}
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func (g *gen) at(t time.Time, msg string, apply func(c *commit)) {
	if t.After(g.end) {
		t = g.end
	}
	g.events = append(g.events, event{at: t, seq: len(g.events), msg: msg, apply: apply})
}

func (g *gen) work(t time.Time, item, title string) {
	if t.After(g.end) {
		t = g.end
	}
	g.events = append(g.events, event{at: t, seq: len(g.events), msg: title, work: item})
}

// after returns t plus a random duration between lo and hi.
func (g *gen) after(t time.Time, lo, hi time.Duration) time.Time {
	return t.Add(lo + time.Duration(g.rnd.Int64N(int64(hi-lo)+1)))
}

var words = []string{"search", "billing", "accounts", "mail", "reports", "sync", "export", "import",
	"auth", "onboarding", "admin", "mobile", "offline", "audit", "alerts", "uploads", "sharing",
	"payments", "feeds", "pricing", "calendar", "archive", "metrics", "settings", "teams", "docs"}

var nouns = []string{"page", "endpoint", "job", "migration", "index", "cache", "form", "list",
	"filter", "import", "export", "retry", "webhook", "dialog", "report", "schema", "queue", "test"}

var verbs = []string{"Add", "Fix", "Rework", "Split", "Speed up", "Remove", "Document", "Validate",
	"Paginate", "Rename", "Harden", "Wire up"}

func (g *gen) pick(xs []string) string { return xs[g.rnd.IntN(len(xs))] }

func (g *gen) title(area string) string {
	return fmt.Sprintf("%s the %s %s", g.pick(verbs), area, g.pick(nouns))
}

func (g *gen) prose(n int) string {
	var b strings.Builder
	for i := range n {
		if i > 0 {
			b.WriteString(" ")
		}
		fmt.Fprintf(&b, "%s the %s %s so the %s %s keeps working when it grows.",
			g.pick(verbs), g.pick(words), g.pick(nouns), g.pick(words), g.pick(nouns))
	}
	return b.String()
}

// planSlot is one plan's place in history.
type planSlot struct {
	slug, area string
	from, to   time.Time
	state      string // its state at the end
	target     string // its own target branch, if it has one
	phases     []string
	items      int
}

// plan lays out every event.
func (g *gen) plan() {
	s := g.size
	g.at(g.start, "init", func(c *commit) {
		b, err := yaml.Marshal(model.Config{Format: model.Format, Integration: "main", Release: "main",
			Remote: "origin", DoneChecks: []string{"true"}})
		if err != nil {
			panic(err)
		}
		c.files[model.ConfigPath] = b
	})
	slots := make([]*planSlot, s.Plans)
	window := s.Span / time.Duration(max(s.Plans-2, 1)) * 2
	for k := range slots {
		area := words[k%len(words)]
		from := g.start.Add(s.Span * time.Duration(k) / time.Duration(s.Plans+1))
		st := model.PlanDone
		switch {
		case k == s.Plans-1:
			st = model.PlanShaping
		case k >= s.Plans-3:
			st = model.PlanActive
		}
		slots[k] = &planSlot{slug: fmt.Sprintf("%s-%d", area, k+1), area: area, from: from, to: from.Add(window), state: st}
		if st == model.PlanActive && k >= s.Plans-1-s.Targets {
			slots[k].target = slots[k].slug + "-next"
			g.events = append(g.events, event{at: from, seq: len(g.events), branch: slots[k].target})
		}
		g.planEvents(slots[k])
	}
	// Items: 90% planned (to the plans that take work), 5% unplanned work, 5% untagged captures.
	var working []*planSlot
	for _, p := range slots {
		if p.state != model.PlanShaping {
			working = append(working, p)
		}
	}
	for n := range s.Items {
		switch r := g.rnd.Float64(); {
		case r < 0.90:
			p := working[n%len(working)]
			p.items++
			g.itemEvents(p, g.planFate(p.state))
		case r < 0.95:
			g.itemEvents(nil, g.openFate(0.8))
		default:
			g.captureEvents(slots[len(slots)-1])
		}
	}
	for _, p := range slots {
		if p.state == model.PlanDone {
			// Closed once its last items are accepted (an item's life runs up to about three weeks).
			closed := p.to.Add(30 * 24 * time.Hour)
			g.at(closed, "plan close "+p.slug, func(c *commit) { g.putPlan(c, p, model.PlanDone, closed) })
		}
	}
}

func (g *gen) putPlan(c *commit, p *planSlot, state string, at time.Time) {
	c.put(model.PlanPath(p.slug), &model.Plan{Title: "The " + p.area + " plan", State: state, Target: p.target,
		Created: ts(p.from), Updated: ts(at)}, "Make "+p.area+" ready for the next year of growth.")
}

func (g *gen) planEvents(p *planSlot) {
	g.at(p.from, "plan new "+p.slug, func(c *commit) { g.putPlan(c, p, model.PlanShaping, p.from) })
	doc := g.prose(6) + "\n\n## Phases\n\n" + g.prose(4)
	g.at(p.from.Add(time.Hour), "plan doc "+p.slug, func(c *commit) { c.doc(model.PlanDocPath(p.slug), doc) })
	// A few planning-session decisions, the last unsettled while the plan is open.
	for d := range 3 {
		at := p.from.Add(time.Duration(d+1) * 90 * time.Minute)
		e := &model.Entry{ID: g.id(), Kind: model.KindDecision, Text: "Keep " + p.area + " " + g.pick(nouns) + "s behind one interface",
			Plan: p.slug, Source: "planning session", By: g.wt, Created: ts(at)}
		g.at(at, "decided "+e.ID, func(c *commit) { c.put(model.EntryPath(e.ID), e, "Fewer moving parts.") })
		if p.state == model.PlanDone || d < 2 {
			done := g.after(at, time.Hour, 72*time.Hour)
			g.at(done, "accept "+e.ID, func(c *commit) {
				e.Settled = &model.Settlement{Action: model.ActAccept, By: "Human", Human: true, At: ts(done)}
				c.put(model.EntryPath(e.ID), e, "Fewer moving parts.")
			})
		}
	}
	if p.state == model.PlanShaping {
		return
	}
	agreed := p.from.Add(6 * time.Hour)
	g.at(agreed, "plan agree "+p.slug, func(c *commit) { g.putPlan(c, p, model.PlanActive, agreed) })
	for k := range g.size.PhasesPerPlan {
		ph := &model.Phase{ID: g.id(), Plan: p.slug, Title: fmt.Sprintf("%s %d", strings.ToUpper(p.area[:1])+p.area[1:], k+1),
			Created: ts(agreed), Updated: ts(agreed), Summary: g.prose(1)}
		if k > 0 {
			ph.Needs = []string{p.phases[k-1]}
		}
		p.phases = append(p.phases, ph.ID)
		doc := g.prose(3) + "\n\nDone when " + g.prose(1)
		at := agreed.Add(time.Duration(k+1) * time.Minute)
		g.at(at, "phase add "+ph.ID, func(c *commit) {
			c.put(model.PhasePath(ph.ID), ph, ph.Summary)
			c.doc(model.PhaseDocPath(ph.ID), doc)
		})
	}
}

// A fate is where an item's life stops.
type fate int

const (
	fateCaptured fate = iota
	fateCommitted
	fateActive
	fateInReview
	fateDone
	fateDropped
)

func (g *gen) planFate(state string) fate {
	if state == model.PlanDone {
		if g.rnd.Float64() < 0.9 {
			return fateDone
		}
		return fateDropped
	}
	return g.openFate(0.5)
}

// openFate is a fate in open work: done with probability done, the rest
// spread over every other stage.
func (g *gen) openFate(done float64) fate {
	r := g.rnd.Float64()
	switch {
	case r < done:
		return fateDone
	case r < done+(1-done)*0.2:
		return fateInReview
	case r < done+(1-done)*0.36:
		return fateActive
	case r < done+(1-done)*0.7:
		return fateCommitted
	case r < done+(1-done)*0.9:
		return fateCaptured
	}
	return fateDropped
}

// itemEvents lays out one item's life in plan p (nil: unplanned work).
func (g *gen) itemEvents(p *planSlot, f fate) {
	from, to := g.start, g.end
	plan, phase, area := "", "", g.pick(words)
	if p != nil {
		from, to, plan, area = p.from.Add(7*time.Hour), p.to, p.slug, p.area
		phase = p.phases[g.rnd.IntN(len(p.phases))]
	}
	if f != fateDone && f != fateDropped && to.Before(g.end) {
		to = g.end // open work lives up to now
	}
	created := from.Add(time.Duration(g.rnd.Int64N(int64(to.Sub(from)) * 8 / 10)))
	i := &model.Item{ID: g.id(), Plan: plan, Phase: phase, Title: g.title(area), Created: ts(created)}
	notes := g.prose(1 + g.rnd.IntN(3))
	put := func(c *commit, at time.Time) {
		i.Updated = ts(at)
		c.put(model.ItemPath(i.ID), i, notes)
	}
	if f == fateCaptured || f == fateDropped || g.rnd.Float64() < 0.4 {
		// Captured first, as a bug or idea, then perhaps committed.
		i.State, i.Kind, i.Source = model.StateCaptured, g.pick([]string{model.ItemBug, model.ItemIdea}), "item review"
		g.at(created, "capture "+i.ID, func(c *commit) { put(c, created) })
		if f == fateDropped {
			at := g.after(created, time.Hour, 30*24*time.Hour)
			g.at(at, "drop "+i.ID, func(c *commit) { i.State = model.StateDropped; put(c, at) })
			return
		}
		if f == fateCaptured {
			if g.rnd.Float64() < 0.5 {
				at := g.after(created, time.Hour, 7*24*time.Hour)
				g.at(at, "triage "+i.ID, func(c *commit) { i.Triaged = ts(at); put(c, at) })
			}
			return
		}
		created = g.after(created, time.Hour, 5*24*time.Hour)
		g.at(created, "commit "+i.ID, func(c *commit) { i.State = model.StateCommitted; put(c, created) })
	} else {
		i.State = model.StateCommitted
		g.at(created, "add "+i.ID, func(c *commit) { put(c, created) })
	}
	if f == fateCommitted {
		return
	}
	started := g.after(created, time.Hour, 10*24*time.Hour)
	base := hex(g.rnd, 40)
	g.at(started, "start "+i.ID, func(c *commit) {
		i.State = model.StateActive
		i.Claim = &model.Claim{Worktree: g.wt, Branch: i.ID + "-work", At: ts(started), Base: base}
		put(c, started)
	})
	var decisions []*model.Entry
	for range g.rnd.IntN(4) {
		at := g.after(started, time.Minute, 3*time.Hour)
		e := &model.Entry{ID: g.id(), Kind: model.KindDecision, Text: "Use the existing " + g.pick(nouns) + " for " + area,
			Plan: plan, Item: i.ID, By: g.wt, Created: ts(at)}
		decisions = append(decisions, e)
		g.at(at, "decided "+e.ID, func(c *commit) { c.put(model.EntryPath(e.ID), e, "It already handles the edge cases.") })
	}
	if g.rnd.Float64() < 0.15 {
		at := g.after(started, time.Minute, 2*time.Hour)
		q := &model.Entry{ID: g.id(), Kind: model.KindQuestion, Text: "Should " + area + " keep the old " + g.pick(nouns) + "?",
			Plan: plan, Item: i.ID, By: g.wt, Created: ts(at)}
		g.at(at, "ask "+q.ID, func(c *commit) { c.put(model.EntryPath(q.ID), q, "") })
		if f != fateActive {
			ans := g.after(at, time.Hour, 24*time.Hour)
			g.at(ans, "decide "+q.ID, func(c *commit) {
				q.Settled = &model.Settlement{Action: model.ActDecide, Note: "Yes, for one more release.", By: "Human", Human: true, At: ts(ans)}
				c.put(model.EntryPath(q.ID), q, "")
			})
		}
	}
	finished := g.after(started, 3*time.Hour, 3*24*time.Hour)
	if f == fateActive {
		if g.rnd.Float64() < 0.5 {
			g.work(g.after(started, time.Hour, 2*time.Hour), i.ID, i.Title)
		}
		return
	}
	g.work(g.after(started, time.Hour, 2*time.Hour), i.ID, i.Title)
	review := &model.Entry{ID: g.id(), Kind: model.KindReview, Text: "Review: " + i.Title, Plan: plan, Item: i.ID,
		Source: model.ReportPath(i.ID, 1), By: g.wt, Created: ts(finished)}
	for _, d := range decisions {
		review.Decisions = append(review.Decisions, d.ID)
	}
	rep := g.report(i, base, finished, decisions)
	g.at(finished, "done "+i.ID, func(c *commit) {
		i.State = model.StateInReview
		put(c, finished)
		c.put(model.ReportPath(i.ID, 1), rep, rep.Summary+"\n\n## Design\n\n"+rep.Design)
		c.put(model.EntryPath(review.ID), review, "")
	})
	if f == fateInReview {
		return
	}
	accepted := g.after(finished, time.Hour, 4*24*time.Hour)
	g.at(accepted, "accept "+i.ID, func(c *commit) {
		i.State = model.StateDone
		put(c, accepted)
		review.Settled = &model.Settlement{Action: model.ActAccept, By: "Human", Human: true, At: ts(accepted)}
		c.put(model.EntryPath(review.ID), review, "")
		for _, d := range decisions {
			d.Settled = &model.Settlement{Action: model.ActAccept, By: "Human", Human: true, At: ts(accepted)}
			c.put(model.EntryPath(d.ID), d, "It already handles the edge cases.")
		}
	})
}

// captureEvents files an untagged or shaping-plan capture.
func (g *gen) captureEvents(shaping *planSlot) {
	at := g.start.Add(time.Duration(g.rnd.Int64N(int64(g.size.Span))))
	i := &model.Item{ID: g.id(), Title: g.title(g.pick(words)), State: model.StateCaptured,
		Kind: g.pick([]string{model.ItemBug, model.ItemIdea}), Source: "human", Created: ts(at), Updated: ts(at)}
	if at.After(shaping.from) && g.rnd.Float64() < 0.3 {
		i.Plan = shaping.slug
	}
	notes := g.prose(1)
	g.at(at, "capture "+i.ID, func(c *commit) { c.put(model.ItemPath(i.ID), i, notes) })
	if g.rnd.Float64() < 0.3 {
		dropped := g.after(at, time.Hour, 60*24*time.Hour)
		g.at(dropped, "drop "+i.ID, func(c *commit) {
			i.State, i.Updated = model.StateDropped, ts(dropped)
			c.put(model.ItemPath(i.ID), i, notes)
		})
	}
}

func (g *gen) report(i *model.Item, base string, at time.Time, decisions []*model.Entry) *model.Report {
	commit := hex(g.rnd, 40)
	rep := &model.Report{Item: i.ID, N: 1, Passed: true, Commit: commit, Base: base, At: ts(at), By: g.wt,
		Summary: g.prose(2), Design: g.prose(4),
		Tour: []string{"Open the " + g.pick(nouns) + " page", "See the new " + g.pick(nouns)},
		Checks: model.CheckRun{Tree: hex(g.rnd, 40), Commit: commit, At: ts(at), By: g.wt, Passed: true,
			Results: []model.CheckResult{{Command: "make done-checks", Exit: 0, DurationMS: 30000 + g.rnd.Int64N(60000)}}},
		Changes: []model.LayerChanges{{Layer: "other", Files: []model.FileChange{
			{Status: "M", Path: "internal/" + g.pick(words) + "/" + g.pick(nouns) + ".go", Added: g.rnd.IntN(200), Removed: g.rnd.IntN(80)},
			{Status: "A", Path: "internal/" + g.pick(words) + "/" + g.pick(nouns) + "_test.go", Added: g.rnd.IntN(150)},
		}}},
	}
	for _, d := range decisions {
		rep.Decisions = append(rep.Decisions, model.ReportDecision{ID: d.ID, Text: d.Text, Detail: "It already handles the edge cases."})
	}
	return rep
}

func hex(r *rand.Rand, n int) string {
	const digits = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = digits[r.IntN(16)]
	}
	return string(b)
}

// write runs every event in time order and streams the commits to
// fast-import: mull commits on refs/heads/mull, work commits on main.
func (g *gen) write(w *bufio.Writer) error {
	sort.SliceStable(g.events, func(a, b int) bool {
		if !g.events[a].at.Equal(g.events[b].at) {
			return g.events[a].at.Before(g.events[b].at)
		}
		return g.events[a].seq < g.events[b].seq
	})
	data := func(b []byte) { fmt.Fprintf(w, "data %d\n", len(b)); w.Write(b); w.WriteString("\n") }
	who := func(at time.Time) string { return fmt.Sprintf("Bench <bench@example.com> %d +0000", at.Unix()) }
	main := 0
	commitMain := func(at time.Time, msg, path, content string) {
		w.WriteString("commit refs/heads/main\n")
		fmt.Fprintf(w, "author %s\ncommitter %s\n", who(at), who(at))
		data([]byte(msg))
		fmt.Fprintf(w, "M 100644 inline %s\n", path)
		data([]byte(content))
		w.WriteString("\n")
		main++
	}
	commitMain(g.start, "Start the project", "README.md", "# bench\n")
	for _, ev := range g.events {
		if ev.branch != "" {
			fmt.Fprintf(w, "reset refs/heads/%s\nfrom refs/heads/main\n\n", ev.branch)
			continue
		}
		if ev.work != "" {
			commitMain(ev.at, ev.msg+"\n\nMull: "+ev.work, "work/"+ev.work+".txt", ev.msg+"\n")
			continue
		}
		c := &commit{files: map[string][]byte{}}
		ev.apply(c)
		w.WriteString("commit " + "refs/heads/mull\n")
		fmt.Fprintf(w, "author %s\ncommitter %s\n", who(ev.at), who(ev.at))
		data([]byte("mull: " + ev.msg))
		paths := make([]string, 0, len(c.files))
		for p := range c.files {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			if !model.ValidDataPath(p) {
				return fmt.Errorf("seed wrote %s, which isn't a mull data path", p)
			}
			fmt.Fprintf(w, "M 100644 inline %s\n", p)
			data(c.files[p])
		}
		w.WriteString("\n")
	}
	w.WriteString("done\n")
	return w.Flush()
}
