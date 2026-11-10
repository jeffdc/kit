package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"mull/internal/guide"
	"mull/internal/model"
	"mull/internal/mull"
)

// ---- Argument validators that report usage errors with a code ----

func usage(err error) error { return &mull.Error{Code: mull.CodeUsage, Msg: err.Error()} }

func wrapArgs(v cobra.PositionalArgs) cobra.PositionalArgs {
	return func(c *cobra.Command, args []string) error {
		if err := v(c, args); err != nil {
			return usage(err)
		}
		return nil
	}
}

var (
	noArgs  = wrapArgs(cobra.NoArgs)
	exact   = func(n int) cobra.PositionalArgs { return wrapArgs(cobra.ExactArgs(n)) }
	atLeast = func(n int) cobra.PositionalArgs { return wrapArgs(cobra.MinimumNArgs(n)) }
	upTo    = func(n int) cobra.PositionalArgs { return wrapArgs(cobra.MaximumNArgs(n)) }
)

// write runs one op and prints its result.
func (a *app) write(op string, in mull.Input) error {
	m, err := a.open()
	if err != nil {
		return err
	}
	res, err := m.Exec(op, in)
	if err != nil {
		return err
	}
	return a.json(res)
}

// read opens mull, runs fn and prints its result.
func (a *app) read(fn func(m *mull.Mull) (any, error)) error {
	m, err := a.open()
	if err != nil {
		return err
	}
	v, err := fn(m)
	if err != nil {
		return err
	}
	return a.json(v)
}

func onRequestFlag(c *cobra.Command, p *bool) {
	c.Flags().BoolVar(p, "on-request", false, "an agent settling this on the human's explicit instruction")
}

// ---- Setup ----

func initCmd(a *app) *cobra.Command {
	var integration, release string
	c := &cobra.Command{
		Use:   "init",
		Short: "Create the mull branch in this repository",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.read(func(m *mull.Mull) (any, error) {
				cfg, err := m.Init(integration, release)
				if err != nil {
					return nil, err
				}
				return map[string]any{"initialized": true, "branch": "mull", "config": cfg}, nil
			})
		},
	}
	c.Flags().StringVar(&integration, "integration", "main", "branch where work lands")
	c.Flags().StringVar(&release, "release", "main", "branch where work ships")
	return c
}

func versionCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "Print mull's version", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.json(map[string]string{"version": Version})
		},
	}
}

func syncCmd(a *app) *cobra.Command {
	var dryRun bool
	c := &cobra.Command{
		Use:   "sync",
		Short: "Exchange the mull branch with the remote (pushes only the mull branch, never forced)",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.read(func(m *mull.Mull) (any, error) { return m.Sync(dryRun) })
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show what would happen, change nothing")
	return c
}

func configCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use: "config", Short: "Show or change this repository's mull configuration (human-owned)", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.read(func(m *mull.Mull) (any, error) { return m.Config() })
		},
	}
	set := &cobra.Command{
		Use: "set <integration|release|remote> <value>", Short: "Set a branch or the remote", Args: exact(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.write("config set", &mull.ConfigSetIn{Key: args[0], Value: args[1]})
		},
	}
	checks := &cobra.Command{
		Use: "checks [command...]", Short: "Replace the hard checks done runs (each argument is one shell command; none clears)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.write("config checks", &mull.ConfigChecksIn{Commands: args})
		},
	}
	layer := &cobra.Command{
		Use:   "layer <name> [glob...]",
		Short: "Set a layer's path globs for done reports (no globs removes it; a layer named ui makes tours required)",
		Args:  atLeast(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.write("config layer", &mull.ConfigLayerIn{Name: args[0], Globs: args[1:]})
		},
	}
	c.AddCommand(set, checks, layer)
	return c
}

// ---- Batch ----

func batchCmd(a *app) *cobra.Command {
	var dryRun bool
	c := &cobra.Command{
		Use:   "batch [file|-]",
		Short: "Run many writes from JSON Lines in one commit: all or nothing (see mull guide json)",
		Long: `Reads JSON Lines from stdin (or a file). Each line is
  {"op": "<op>", "ref": "<optional name>", ...fields}
with field names exactly the command's flag names. A later line may use
"@<ref>" in any ID field to mean the record an earlier line created.
Every line is validated and run in one transaction: if any line fails,
nothing is written and every failed line is reported. Ops: ` + strings.Join(mull.OpNames(), ", ") + `.`,
		Args: upTo(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var r io.Reader = a.stdin
			if len(args) == 1 && args[0] != "-" {
				f, err := os.Open(args[0])
				if err != nil {
					return &mull.Error{Code: mull.CodeInvalid, Msg: err.Error()}
				}
				defer f.Close()
				r = f
			}
			return a.read(func(m *mull.Mull) (any, error) { return m.Batch(r, dryRun) })
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "validate and run without writing (IDs in the results are throwaway)")
	return c
}

// ---- Plans and phases ----

func planCmd(a *app) *cobra.Command {
	c := &cobra.Command{Use: "plan", Short: "Plans: the top level of the work"}
	var in mull.PlanNewIn
	newC := &cobra.Command{
		Use:   "new <slug> <title>",
		Short: "Create a plan (slug: lowercase words joined by hyphens)",
		Args:  exact(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			in.Slug, in.Title = args[0], args[1]
			var err error
			if in.Summary, err = a.text(in.Summary); err != nil {
				return err
			}
			return a.write("plan new", &in)
		},
	}
	newC.Flags().StringVar(&in.Summary, "summary", "", "high-level summary (- reads stdin)")
	list := &cobra.Command{
		Use: "list", Short: "List plans with progress", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.read(func(m *mull.Mull) (any, error) {
				ps, err := m.PlanList()
				return map[string]any{"plans": ps}, err
			})
		},
	}
	show := &cobra.Command{
		Use: "show <slug>", Short: "Show a plan with its phases and items", Args: exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.read(func(m *mull.Mull) (any, error) { return m.PlanShow(args[0]) })
		},
	}
	closeC := &cobra.Command{
		Use: "close <slug>", Short: "Close a plan", Args: exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.write("plan close", &mull.PlanCloseIn{Slug: args[0]})
		},
	}
	c.AddCommand(newC, list, show, closeC)
	return c
}

func phaseCmd(a *app) *cobra.Command {
	c := &cobra.Command{Use: "phase", Short: "Phases: big pieces of a plan"}
	var in mull.PhaseAddIn
	var needs []string
	add := &cobra.Command{
		Use: "add <title>", Short: "Add a phase to a plan", Args: exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in.Title, in.Needs = args[0], mull.SplitIDs(needs)
			var err error
			if in.Summary, err = a.text(in.Summary); err != nil {
				return err
			}
			return a.write("phase add", &in)
		},
	}
	add.Flags().StringVar(&in.Plan, "plan", "", "plan slug")
	add.Flags().StringVar(&in.Summary, "summary", "", "summary (- reads stdin)")
	add.Flags().StringSliceVar(&needs, "needs", nil, "items or phases this phase needs")

	var listPlan string
	list := &cobra.Command{
		Use: "list", Short: "List phases with progress", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.read(func(m *mull.Mull) (any, error) {
				es, err := m.PhaseList(listPlan)
				return map[string]any{"phases": es}, err
			})
		},
	}
	list.Flags().StringVar(&listPlan, "plan", "", "only this plan")
	c.AddCommand(add, list)
	return c
}

// ---- Items ----

func addCmd(a *app) *cobra.Command {
	var in mull.AddIn
	var needs, docs []string
	c := &cobra.Command{
		Use:   "add <title>",
		Short: "Add an item: a unit of work one agent completes in one session (--phase implies its plan)",
		Args:  exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in.Title, in.Needs, in.Doc = args[0], mull.SplitIDs(needs), docs
			var err error
			if in.Notes, err = a.text(in.Notes); err != nil {
				return err
			}
			return a.write("add", &in)
		},
	}
	c.Flags().StringVar(&in.Plan, "plan", "", "plan slug (not needed with --phase; none: unplanned work)")
	c.Flags().StringVar(&in.Phase, "phase", "", "phase ID")
	c.Flags().StringSliceVar(&needs, "needs", nil, "items or phases this item needs")
	c.Flags().StringVar(&in.Notes, "notes", "", "short notes (- reads stdin)")
	c.Flags().StringArrayVar(&docs, "doc", nil, "path of a related document (repeatable)")
	return c
}

func showCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use: "show <slug|id>", Short: "Show a plan, phase, item or inbox entry", Args: exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.read(func(m *mull.Mull) (any, error) { return m.Show(args[0]) })
		},
	}
}

func listCmd(a *app) *cobra.Command {
	var f mull.ItemFilter
	c := &cobra.Command{
		Use: "list", Short: "List items", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.read(func(m *mull.Mull) (any, error) {
				items, err := m.ItemList(f)
				return map[string]any{"items": items}, err
			})
		},
	}
	c.Flags().StringVar(&f.Plan, "plan", "", "only this plan (none: items with no plan)")
	c.Flags().StringVar(&f.Phase, "phase", "", "only this phase")
	c.Flags().StringVar(&f.State, "state", "", "only this stage (captured, committed, active, in-review, done, dropped)")
	c.Flags().StringVar(&f.Kind, "kind", "", "only this kind of captured item (bug, idea)")
	return c
}

func readyCmd(a *app) *cobra.Command {
	var f mull.ItemFilter
	c := &cobra.Command{
		Use: "ready", Short: "List items ready to start (committed, with every need finished)", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			f.ReadyOnly = true
			return a.read(func(m *mull.Mull) (any, error) {
				items, err := m.ItemList(f)
				return map[string]any{"items": items}, err
			})
		},
	}
	c.Flags().StringVar(&f.Plan, "plan", "", "only this plan")
	return c
}

func setCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "set <slug|id> <field> <value>",
		Short: "Change a field (plan/phase: title, summary; item: title, notes, phase, doc). Value - reads stdin",
		Args:  exact(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			val, err := a.text(args[2])
			if err != nil {
				return err
			}
			return a.write("set", &mull.SetIn{ID: args[0], Field: args[1], Value: val})
		},
	}
}

func needCmd(a *app, add bool) *cobra.Command {
	use, short, op := "need <id> <needs...>", "Record that an item or phase needs other items or phases", "need"
	if !add {
		use, short, op = "unneed <id> <needs...>", "Remove needs", "unneed"
	}
	return &cobra.Command{
		Use: use, Short: short, Args: atLeast(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			in, _ := mull.NewInput(op)
			n := in.(*mull.NeedIn)
			n.ID, n.Needs = args[0], mull.SplitIDs(args[1:])
			return a.write(op, n)
		},
	}
}

func startCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use: "start <id>", Short: "Claim an item for this worktree and mark it active", Args: exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.write("start", &mull.StartIn{ID: args[0]})
		},
	}
}

func releaseCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use: "release <id>", Short: "Give back a claim; the item returns to committed", Args: exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.write("release", &mull.ReleaseIn{ID: args[0]})
		},
	}
}

// ---- Capture ----

func captureCmd(a *app, op string) *cobra.Command {
	short := map[string]string{
		"decided": "Record a decision you made that the human didn't (--detail: why)",
		"ask":     "Ask the human something",
	}[op]
	var item, plan, source, detail string
	c := &cobra.Command{
		Use: op + " <text>", Short: short, Args: exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in, _ := mull.NewInput(op)
			ci := in.(*mull.CaptureIn)
			var err error
			if ci.Detail, err = a.text(detail); err != nil {
				return err
			}
			ci.Text, ci.Item, ci.Plan, ci.Source = args[0], item, plan, source
			return a.write(op, ci)
		},
	}
	c.Flags().StringVar(&item, "item", "", "the item this concerns (implies the plan)")
	c.Flags().StringVar(&plan, "plan", "", "the plan, when there is no item (required when the repo has more than one plan)")
	c.Flags().StringVar(&source, "source", "", "where this came from (file, matter, commit)")
	c.Flags().StringVar(&detail, "detail", "", "more detail; for a decision, why (- reads stdin)")
	return c
}

func captureItemCmd(a *app, op string) *cobra.Command {
	short := map[string]string{
		"bug":  "Capture a bug into the backlog: a captured item, not committed work",
		"idea": "Capture an idea into the backlog: a captured item, not committed work",
	}[op]
	var in mull.CaptureItemIn
	c := &cobra.Command{
		Use: op + " <title>", Short: short + " (only the human commits it: mull commit)", Args: exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ci, _ := mull.NewInput(op)
			c := ci.(*mull.CaptureItemIn)
			var err error
			if c.Detail, err = a.text(in.Detail); err != nil {
				return err
			}
			c.Text, c.Plan, c.Phase, c.Category, c.Source = args[0], in.Plan, in.Phase, in.Category, in.Source
			return a.write(op, c)
		},
	}
	c.Flags().StringVar(&in.Plan, "plan", "", "the plan it belongs to (optional)")
	c.Flags().StringVar(&in.Phase, "phase", "", "the phase it belongs to (optional; implies the plan)")
	c.Flags().StringVar(&in.Category, "category", "", "a free-text category")
	c.Flags().StringVar(&in.Source, "source", "", "where it came from (a conversation, a file, a commit)")
	c.Flags().StringVar(&in.Detail, "detail", "", "a short note, kept as the item's notes (- reads stdin)")
	return c
}

func commitCmd(a *app) *cobra.Command {
	var in mull.CommitIn
	c := &cobra.Command{
		Use:   "commit <id>",
		Short: "Commit a captured item as work, optionally moving it to a plan or phase (--plan none: unplanned). Only the human commits",
		Args:  exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in.ID = args[0]
			return a.write("commit", &in)
		},
	}
	c.Flags().StringVar(&in.Plan, "plan", "", "move it to this plan (none: unplanned work)")
	c.Flags().StringVar(&in.Phase, "phase", "", "move it to this phase (implies the plan)")
	c.Flags().BoolVar(&in.OnRequest, "on-request", false, "an agent committing this on the human's explicit instruction")
	return c
}

// ---- Inbox ----

func inboxCmd(a *app) *cobra.Command {
	var f mull.InboxFilter
	var asText bool
	c := &cobra.Command{
		Use: "inbox", Short: "What needs the human, oldest first", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			es, err := m.Inbox(f)
			if err != nil {
				return err
			}
			if asText {
				return printInbox(a, es)
			}
			return a.json(map[string]any{"entries": es})
		},
	}
	c.Flags().StringVar(&f.Plan, "plan", "", "only this plan")
	c.Flags().StringVar(&f.Kind, "kind", "", "only this kind (decision, question, review; or the retired loose-end, idea)")
	c.Flags().BoolVar(&f.Parked, "parked", false, "include parked entries")
	c.Flags().BoolVar(&f.All, "all", false, "include settled entries")
	c.Flags().BoolVar(&asText, "text", false, "render for a human instead of JSON")
	return c
}

// retiredCmd stands in for a retired command: it accepts anything and
// fails with code retired and a pointer to the replacement.
func retiredCmd(use, pointer string) *cobra.Command {
	return &cobra.Command{
		Use: use, Short: "Retired: " + pointer, Hidden: true, DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return &mull.Error{Code: mull.CodeRetired, Msg: cmd.Name() + " is retired: " + pointer}
		},
	}
}

func printInbox(a *app, es []*model.Entry) error {
	if len(es) == 0 {
		_, err := fmt.Fprintln(a.stdout, "Inbox empty.")
		return err
	}
	order := []string{model.KindReview, model.KindDecision, model.KindQuestion, model.KindLoose}
	titles := map[string]string{
		model.KindReview: "REVIEWS", model.KindDecision: "DECISIONS", model.KindQuestion: "QUESTIONS", model.KindLoose: "LOOSE ENDS",
	}
	var b strings.Builder
	for _, kind := range order {
		var rows []*model.Entry
		for _, e := range es {
			if e.Kind == kind {
				rows = append(rows, e)
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s (%d)\n", titles[kind], len(rows))
		for _, e := range rows {
			where := e.Plan
			if e.Item != "" {
				where += "/" + e.Item
			}
			status, src := "", ""
			if e.Settled != nil {
				status = "  [" + e.Settled.Action + "]"
			}
			if e.Source != "" {
				src = "  (from " + e.Source + ")"
			}
			fmt.Fprintf(&b, "  %s  %-14s %s  %s%s%s\n", e.ID, where, e.Created[:10], e.Text, src, status)
		}
		b.WriteString("\n")
	}
	_, err := fmt.Fprint(a.stdout, strings.TrimRight(b.String(), "\n")+"\n")
	return err
}

func acceptCmd(a *app) *cobra.Command {
	var in mull.AcceptIn
	c := &cobra.Command{
		Use: "accept <id...>", Short: "Ratify decisions, or accept reviewed work (entry or item IDs)", Args: atLeast(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in.IDs = mull.SplitIDs(args)
			return a.write("accept", &in)
		},
	}
	c.Flags().StringVar(&in.FollowUp, "follow-up", "", "also create a follow-up item (one reviewed item only)")
	onRequestFlag(c, &in.OnRequest)
	return c
}

func decideCmd(a *app) *cobra.Command {
	var in mull.DecideIn
	c := &cobra.Command{
		Use: "decide <id> <decision>", Short: "Settle an entry with your decision; it joins the decision log", Args: exact(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := a.text(args[1])
			if err != nil {
				return err
			}
			in.ID, in.Decision = args[0], text
			return a.write("decide", &in)
		},
	}
	onRequestFlag(c, &in.OnRequest)
	return c
}

func doCmd(a *app) *cobra.Command {
	var in mull.DoIn
	c := &cobra.Command{
		Use:   "do <id>",
		Short: "Turn a question, loose end or idea into an item (or --as phase), or send reviewed work back (--reason)",
		Args:  exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in.ID = args[0]
			return a.write("do", &in)
		},
	}
	c.Flags().StringVar(&in.Plan, "plan", "", "plan for the new item (default: the phase's plan, else the entry's)")
	c.Flags().StringVar(&in.Phase, "phase", "", "phase for the new item (implies its plan)")
	c.Flags().StringVar(&in.As, "as", "item", "what it becomes: item or phase")
	c.Flags().StringVar(&in.Reason, "reason", "", "why reviewed work is sent back")
	onRequestFlag(c, &in.OnRequest)
	return c
}

func parkCmd(a *app) *cobra.Command {
	var in mull.ParkIn
	c := &cobra.Command{
		Use: "park <id> <reason>", Short: "Set an entry aside; it stays listed as parked", Args: exact(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			in.ID, in.Reason = args[0], args[1]
			return a.write("park", &in)
		},
	}
	onRequestFlag(c, &in.OnRequest)
	return c
}

func dropCmd(a *app) *cobra.Command {
	var in mull.DropIn
	c := &cobra.Command{
		Use: "drop <id...>", Short: "Drop inbox entries (gone for good, recorded) or items", Args: atLeast(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in.IDs = mull.SplitIDs(args)
			return a.write("drop", &in)
		},
	}
	c.Flags().StringVar(&in.Reason, "reason", "", "why (recorded)")
	onRequestFlag(c, &in.OnRequest)
	return c
}

func decisionsCmd(a *app) *cobra.Command {
	var plan string
	c := &cobra.Command{
		Use: "decisions", Short: "The decision log, newest first: read it before deciding anything", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.read(func(m *mull.Mull) (any, error) {
				ds, err := m.Decisions(plan)
				return map[string]any{"decisions": ds}, err
			})
		},
	}
	c.Flags().StringVar(&plan, "plan", "", "only this plan")
	return c
}

// ---- Board, context, guides ----

func viewCmd(a *app) *cobra.Command {
	var plan string
	var asText bool
	c := &cobra.Command{
		Use:   "view",
		Short: "The board: each plan's phases, problems, work in flight, and what needs you",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			v, err := m.Board(plan)
			if err != nil {
				return err
			}
			if asText {
				_, err = fmt.Fprint(a.stdout, v.Text(m.Now()))
				return err
			}
			return a.json(v)
		},
	}
	c.Flags().StringVar(&plan, "plan", "", "only this plan (closed plans too)")
	c.Flags().BoolVar(&asText, "text", false, "render for a human instead of JSON")
	return c
}

func contextCmd(a *app) *cobra.Command {
	var item string
	c := &cobra.Command{
		Use:   "context",
		Short: "Session-start briefing for an agent: protocol, plans, ready work, claims, decisions",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.read(func(m *mull.Mull) (any, error) { return m.Context(item) })
		},
	}
	c.Flags().StringVar(&item, "item", "", "the item this session works on")
	return c
}

func guideCmd(a *app) *cobra.Command {
	var asText bool
	c := &cobra.Command{
		Use:   "guide <" + strings.Join(guide.Names(), "|") + ">",
		Short: "A procedure or reference: agents (for AGENTS.md), supervisor, intake, json (output shapes)",
		Args:  exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := guide.Get(args[0])
			if err != nil {
				return &mull.Error{Code: mull.CodeNotFound, Msg: err.Error()}
			}
			if asText {
				_, err = fmt.Fprint(a.stdout, g)
				return err
			}
			return a.json(map[string]string{"guide": args[0], "text": g})
		},
	}
	c.Flags().BoolVar(&asText, "text", false, "print the guide as plain text instead of JSON")
	return c
}

// ---- Finishing work ----

func checkCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use: "check", Short: "Run the hard checks against the clean HEAD and record the result", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			out, err := m.Check()
			if err != nil {
				return err
			}
			if !out.Passed {
				return &mull.Error{Code: mull.CodeChecksFailed, Msg: "hard checks failed: " + failedCommand(out.Results)}
			}
			return a.json(out)
		},
	}
}

func failedCommand(rs []model.CheckResult) string {
	for _, r := range rs {
		if r.Exit != 0 {
			return fmt.Sprintf("%s exited %d; last output:\n%s", r.Command, r.Exit, r.Tail)
		}
	}
	return "unknown"
}

func doneCmd(a *app) *cobra.Command {
	var in mull.DoneInput
	var summary, tour string
	c := &cobra.Command{
		Use:   "done <id>",
		Short: "Finish an item: run the hard checks, write the done report, and file it for review",
		Args:  exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if summary == "-" && tour == "-" {
				return &mull.Error{Code: mull.CodeUsage, Msg: "only one of --summary and --tour can read stdin"}
			}
			var err error
			if in.Summary, err = a.text(summary); err != nil {
				return err
			}
			t, err := a.text(tour)
			if err != nil {
				return err
			}
			for _, line := range strings.Split(t, "\n") {
				line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*0123456789.)"))
				if line != "" && !strings.EqualFold(line, "none") {
					in.Tour = append(in.Tour, line)
				}
			}
			return a.read(func(m *mull.Mull) (any, error) { return m.Done(args[0], in) })
		},
	}
	c.Flags().StringVar(&summary, "summary", "", "what changed and how it fits (- reads stdin)")
	c.Flags().StringVar(&tour, "tour", "", "UI tour, one step per line: what to click and what to see (- reads stdin)")
	c.Flags().StringVar(&in.Override, "override", "", "HUMAN ONLY: finish despite failing or missing checks, with the reason")
	return c
}

func reportCmd(a *app) *cobra.Command {
	var n int
	var asText bool
	c := &cobra.Command{
		Use: "report <item-id>", Short: "An item's done report (the latest by default)", Args: exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			text, rep, err := m.ReportText(args[0], n)
			if err != nil {
				return err
			}
			if asText {
				_, err = fmt.Fprintln(a.stdout, text)
				return err
			}
			return a.json(map[string]any{"report": rep, "text": text})
		},
	}
	c.Flags().IntVar(&n, "n", 0, "which attempt (default: latest)")
	c.Flags().BoolVar(&asText, "text", false, "print the report as markdown instead of JSON")
	return c
}
