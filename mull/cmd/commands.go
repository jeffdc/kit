package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"mull/internal/core"
	"mull/internal/guide"
	"mull/internal/human"
	"mull/internal/seen"
)

func usage(err error) error { return &core.Error{Code: core.CodeUsage, Msg: err.Error()} }

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

// run opens mull, runs fn and prints its result.
func (a *app) run(fn func(m *core.Mull) (any, error)) error {
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

// cmd makes a command whose RunE runs fn with the args.
func (a *app) cmd(use, short string, args cobra.PositionalArgs, fn func(m *core.Mull, args []string) (any, error)) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Args: args, RunE: func(c *cobra.Command, as []string) error {
		return a.run(func(m *core.Mull) (any, error) { return fn(m, as) })
	}}
}

func onRequest(c *cobra.Command, p *bool) {
	c.Flags().BoolVar(p, "on-request", false, "an agent doing this on the human's explicit instruction")
}

// ---- Setup ----

func initCmd(a *app) *cobra.Command {
	var integration, release string
	c := a.cmd("init", "Create the mull branch in this repository", noArgs, func(m *core.Mull, _ []string) (any, error) {
		cfg, err := m.Init(integration, release)
		return map[string]any{"initialized": err == nil, "config": cfg}, err
	})
	c.Flags().StringVar(&integration, "integration", "next", "where unplanned work lands (never the release branch)")
	c.Flags().StringVar(&release, "release", "main", "where work ships")
	return c
}

func versionCmd(a *app) *cobra.Command {
	return &cobra.Command{Use: "version", Short: "Print mull's version", Args: noArgs,
		RunE: func(*cobra.Command, []string) error { return a.json(map[string]string{"version": Version}) }}
}

func nameCmd(a *app) *cobra.Command {
	return &cobra.Command{Use: "name [<name>]", Short: "Show or set the name you go by on this machine (\"\" clears it)", Args: upTo(1),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 1 {
				if err := human.SetName(args[0]); err != nil {
					return &core.Error{Code: core.CodeInvalid, Msg: err.Error()}
				}
			}
			name, err := human.Name()
			if err != nil {
				return err
			}
			file, _ := human.File()
			return a.json(map[string]string{"name": name, "file": file})
		}}
}

func syncCmd(a *app) *cobra.Command {
	var dry bool
	c := a.cmd("sync", "Exchange the mull branch with the remote (only the mull branch, never forced)", noArgs,
		func(m *core.Mull, _ []string) (any, error) { return m.Sync(dry) })
	c.Flags().BoolVar(&dry, "dry-run", false, "show what would happen")
	return c
}

func configCmd(a *app) *cobra.Command {
	c := a.cmd("config", "Show this repository's configuration", noArgs, func(m *core.Mull, _ []string) (any, error) { return m.Config() })
	var or bool
	set := a.cmd("set <integration|release|remote|worktrees|parallel> <value>", "Set a branch, the remote, where worktrees go, or the parallel cap (the human's)", exact(2),
		func(m *core.Mull, args []string) (any, error) { return m.ConfigSet(args[0], args[1], or) })
	onRequest(set, &or)
	checks := a.cmd("checks [command...]", "Replace the hard checks done runs (the human's)", nil,
		func(m *core.Mull, args []string) (any, error) { return m.ConfigChecks(args, or) })
	onRequest(checks, &or)
	setup := a.cmd("setup [command...]", "Replace the commands start runs in a new worktree; MULL_MAIN is the main worktree (the human's)", nil,
		func(m *core.Mull, args []string) (any, error) { return m.ConfigSetup(args, or) })
	onRequest(setup, &or)
	var edges, allowed string
	mp := a.cmd("map", "Set the commands printing the code's connections (--edges) and the declared ones (--allowed), one 'from -> to' per line (the human's)", noArgs,
		func(m *core.Mull, _ []string) (any, error) { return m.ConfigMap(edges, allowed, or) })
	mp.Flags().StringVar(&edges, "edges", "", "command printing the connections the code has")
	mp.Flags().StringVar(&allowed, "allowed", "", "command printing the declared connections")
	onRequest(mp, &or)
	var ri core.RiskInput
	var noSample bool
	risk := a.cmd("risk", "Change how done scores risk: high-risk paths, the size that makes work medium, the percent of low-risk work sampled (the human's)", noArgs,
		func(m *core.Mull, _ []string) (any, error) {
			if noSample {
				ri.Sample = -1
			}
			ri.OnRequest = or
			return m.ConfigRisk(ri)
		})
	risk.Flags().StringSliceVar(&ri.Add, "add", nil, "add high-risk path globs")
	risk.Flags().StringSliceVar(&ri.Remove, "remove", nil, "remove high-risk path globs")
	risk.Flags().IntVar(&ri.MaxLines, "max-lines", 0, "lines above which work is medium risk")
	risk.Flags().IntVar(&ri.MaxFiles, "max-files", 0, "files above which work is medium risk")
	risk.Flags().IntVar(&ri.Sample, "sample", 0, "percent of low-risk work shown to the human")
	risk.Flags().BoolVar(&noSample, "no-sample", false, "sample no low-risk work")
	onRequest(risk, &or)
	c.AddCommand(set, checks, setup, mp, risk)
	return c
}

func batchCmd(a *app) *cobra.Command {
	var dry bool
	c := &cobra.Command{Use: "batch [file|-]", Args: upTo(1),
		Short: "Run JSON Lines of writes in one commit, all or nothing. Ops: " + strings.Join(core.Ops, ", "),
		RunE: func(c *cobra.Command, args []string) error {
			var r io.Reader = a.stdin
			if len(args) == 1 && args[0] != "-" {
				f, err := os.Open(args[0])
				if err != nil {
					return &core.Error{Code: core.CodeInvalid, Msg: err.Error()}
				}
				defer f.Close()
				r = f
			}
			return a.run(func(m *core.Mull) (any, error) { return m.Batch(r, dry) })
		}}
	c.Flags().BoolVar(&dry, "dry-run", false, "run every line, write nothing")
	return c
}

func guideCmd(a *app) *cobra.Command {
	var asText bool
	c := &cobra.Command{Use: "guide <" + strings.Join(guide.Names(), "|") + ">", Short: "Print a guide: agents, supervisor, auditor, intake, json", Args: exact(1),
		RunE: func(c *cobra.Command, args []string) error {
			g, err := guide.Get(args[0])
			if err != nil {
				return &core.Error{Code: core.CodeNotFound, Msg: err.Error()}
			}
			if asText {
				_, err = fmt.Fprint(a.stdout, g)
				return err
			}
			return a.json(map[string]string{"guide": args[0], "text": g})
		}}
	c.Flags().BoolVar(&asText, "text", false, "plain text")
	return c
}

// ---- Plans and phases ----

func planCmd(a *app) *cobra.Command {
	c := &cobra.Command{Use: "plan", Short: "Plans: the top level of the work"}
	var summary, target string
	var or bool
	nw := a.cmd("new <slug> <title>", "Create a plan, shaping until the human agrees it (the human's)", exact(2),
		func(m *core.Mull, args []string) (any, error) {
			return m.PlanNew(args[0], args[1], summary, target, or)
		})
	nw.Flags().StringVar(&summary, "summary", "", "a line or two")
	nw.Flags().StringVar(&target, "target", "own", "where its work lands: own (plan/<slug>), none (the integration branch), or a branch")
	onRequest(nw, &or)
	agree := a.cmd("agree <slug>", "Agree a shaping plan (the human's)", exact(1),
		func(m *core.Mull, args []string) (any, error) { return m.PlanState(args[0], core.PlanActive, or) })
	onRequest(agree, &or)
	closeC := a.cmd("close <slug>", "Close a plan (the human's)", exact(1),
		func(m *core.Mull, args []string) (any, error) { return m.PlanState(args[0], core.PlanDone, or) })
	onRequest(closeC, &or)
	set := a.cmd("set <slug> <title|summary|target|preview|doc> <value|->", "Change a plan; - reads stdin", exact(3),
		func(m *core.Mull, args []string) (any, error) {
			v, err := a.text(args[2])
			if err != nil {
				return nil, err
			}
			return m.PlanSet(args[0], args[1], v)
		})
	c.AddCommand(nw, agree, closeC, set)
	return c
}

func phaseCmd(a *app) *cobra.Command {
	c := &cobra.Command{Use: "phase", Short: "Phases: big pieces of a plan"}
	var plan, summary string
	var needs []string
	add := a.cmd("add <title>", "Add a phase to a plan", exact(1),
		func(m *core.Mull, args []string) (any, error) { return m.PhaseAdd(plan, args[0], summary, needs) })
	add.Flags().StringVar(&plan, "plan", "", "the plan")
	add.Flags().StringVar(&summary, "summary", "", "a line or two, with when it's done")
	add.Flags().StringSliceVar(&needs, "needs", nil, "phases it needs first")
	set := a.cmd("set <id> <title|summary|doc|needs> <value|->", "Change a phase; - reads stdin", exact(3),
		func(m *core.Mull, args []string) (any, error) {
			v, err := a.text(args[2])
			if err != nil {
				return nil, err
			}
			return m.PhaseSet(args[0], args[1], v)
		})
	c.AddCommand(add, set)
	return c
}

// ---- Items ----

func addCmds(a *app) []*cobra.Command {
	var out []*cobra.Command
	for _, kind := range []string{"", core.Bug, core.Idea} {
		kind := kind
		var in core.NewItem
		use, short := "add <title>", "Add committed work"
		switch kind {
		case core.Bug:
			use, short = "bug <title>", "Queue a bug in one line (fix a small one in the area you're changing instead)"
		case core.Idea:
			use, short = "idea <title>", "Queue an idea: a possibility that isn't work yet; it feeds planning"
		}
		c := a.cmd(use, short, exact(1), func(m *core.Mull, args []string) (any, error) {
			in.Title, in.Kind = args[0], kind
			return m.Add(in)
		})
		c.Flags().StringVar(&in.Plan, "plan", "", "its plan (none: unplanned)")
		c.Flags().StringVar(&in.Phase, "phase", "", "its phase")
		c.Flags().StringVar(&in.Area, "area", "", "where in the code it is: a path or glob")
		c.Flags().StringVar(&in.Notes, "notes", "", "a short note")
		if kind == "" {
			c.Flags().StringVar(&in.From, "from", "", "the done item it follows up")
		} else {
			c.Flags().StringVar(&in.Source, "source", "", "where it came from")
		}
		out = append(out, c)
	}
	return out
}

func itemCmds(a *app) []*cobra.Command {
	var f core.Filter
	list := a.cmd("list", "List items", noArgs, func(m *core.Mull, _ []string) (any, error) {
		items, err := m.List(f)
		return map[string]any{"items": items}, err
	})
	list.Flags().StringVar(&f.Plan, "plan", "", "only this plan (none: unplanned)")
	list.Flags().StringVar(&f.Phase, "phase", "", "only this phase")
	list.Flags().StringVar(&f.State, "state", "", "only this stage (captured is the queue)")
	list.Flags().StringVar(&f.Kind, "kind", "", "only bugs or ideas")
	ready := a.cmd("ready", "Committed work whose phase can start", noArgs, func(m *core.Mull, _ []string) (any, error) {
		items, err := m.Ready()
		return map[string]any{"items": items}, err
	})
	show := a.cmd("show <slug|id>", "Show a plan, phase, item (with its reports) or inbox entry", exact(1),
		func(m *core.Mull, args []string) (any, error) { return m.Show(args[0]) })
	set := a.cmd("set <id> <title|notes|area|kind|plan|phase> <value|->", "Change an item; moving is a move, for work nobody's on", exact(3),
		func(m *core.Mull, args []string) (any, error) {
			v, err := a.text(args[2])
			if err != nil {
				return nil, err
			}
			return m.Set(args[0], args[1], v)
		})
	var plan, phase, reason string
	commit := a.cmd("commit <id>", "Commit queued work, optionally moving it", exact(1),
		func(m *core.Mull, args []string) (any, error) { return m.Commit(args[0], plan, phase) })
	commit.Flags().StringVar(&plan, "plan", "", "move it to this plan (none: unplanned)")
	commit.Flags().StringVar(&phase, "phase", "", "move it to this phase")
	drop := a.cmd("drop <id...>", "Drop items that aren't done", atLeast(1),
		func(m *core.Mull, args []string) (any, error) { return m.DropItems(args, reason) })
	drop.Flags().StringVar(&reason, "reason", "", "why")
	return []*cobra.Command{list, ready, show, set, commit, drop}
}

// ---- Working ----

func workCmds(a *app) []*cobra.Command {
	var item string
	ctx := a.cmd("context", "An agent's brief: the protocol, the rulebook, and the item's target, docs and queue", noArgs,
		func(m *core.Mull, _ []string) (any, error) { return m.Brief(item) })
	ctx.Flags().StringVar(&item, "item", "", "the item this session works on")
	var here bool
	start := a.cmd("start <id>", "Make the item's branch and worktree from its target, run setup there, and claim it (--here: claim it here)", exact(1),
		func(m *core.Mull, args []string) (any, error) { return m.StartWork(args[0], here) })
	start.Flags().BoolVar(&here, "here", false, "claim it for this worktree, on the branch checked out")
	release := a.cmd("release <id>", "Give back a claim", exact(1), func(m *core.Mull, args []string) (any, error) { return m.Release(args[0]) })
	check := a.cmd("check", "Run the hard checks against the clean HEAD (a pass for the tree is reused)", noArgs,
		func(m *core.Mull, _ []string) (any, error) { return m.Check() })
	var in core.DoneInput
	var look string
	done := a.cmd("done <id>", "Finish: run the checks once, score risk, and land the work (low, medium) or file it for review (high)", exact(1),
		func(m *core.Mull, args []string) (any, error) {
			var err error
			if in.Design, err = a.text(in.Design); err != nil {
				return nil, err
			}
			for _, l := range strings.Split(look, "\n") {
				if l = strings.TrimSpace(l); l != "" && !strings.EqualFold(l, "none") {
					in.Look = append(in.Look, l)
				}
			}
			return m.Done(args[0], in)
		})
	done.Flags().StringVar(&in.Summary, "summary", "", "required: one line on what changed")
	done.Flags().StringVar(&in.Rules, "rules", "", "required: none, or which rules the work touched and how")
	done.Flags().StringVar(&in.Slowed, "slowed", "", "what slowed the work, if anything")
	done.Flags().StringSliceVar(&in.Covers, "covers", nil, "queued or committed items this work also did")
	done.Flags().StringVar(&look, "look", "", "what to look at, one step per line")
	done.Flags().StringVar(&in.Design, "design", "", "how and why, when it isn't obvious (- reads stdin)")
	var n int
	var asText bool
	report := &cobra.Command{Use: "report <id>", Short: "An item's done report (the latest)", Args: exact(1),
		RunE: func(c *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			r, err := m.ReportOf(args[0], n)
			if err != nil {
				return err
			}
			if asText {
				_, err = fmt.Fprintln(a.stdout, r.Text)
				return err
			}
			return a.json(r)
		}}
	report.Flags().IntVar(&n, "n", 0, "which attempt")
	report.Flags().BoolVar(&asText, "text", false, "markdown")
	land := a.cmd("land <id>", "Put done work on its target when landing missed (its tip must hold the target's tip and pass mull check)", exact(1),
		func(m *core.Mull, args []string) (any, error) { return m.Land(args[0]) })
	var prune bool
	worktrees := a.cmd("worktrees", "Remove worktrees whose work is all on a target, and their branches (start and done do too)", noArgs,
		func(m *core.Mull, _ []string) (any, error) {
			removed, err := m.Prune()
			if removed == nil {
				removed = []string{}
			}
			return map[string]any{"removed": removed}, err
		})
	worktrees.Flags().BoolVar(&prune, "prune", true, "kept for older habits; pruning is all it does")
	hooks := a.cmd("hooks", "Install a pre-push hook refusing pushes to the release branch", noArgs,
		func(m *core.Mull, _ []string) (any, error) { return m.InstallHook() })
	return []*cobra.Command{ctx, start, release, check, done, report, land, worktrees, hooks}
}

// ---- The inbox ----

func inboxCmds(a *app) []*cobra.Command {
	var item, plan, def, by, detail string
	var human, or bool
	ask := a.cmd("ask <question>", "Ask the human about a rule conflict or something hard to undo, with your default", exact(1),
		func(m *core.Mull, args []string) (any, error) { return m.Ask(args[0], item, plan, def, by, detail) })
	ask.Flags().StringVar(&item, "item", "", "the item it's about")
	ask.Flags().StringVar(&plan, "plan", "", "the plan, with no item")
	ask.Flags().StringVar(&def, "default", "", "required: what you'll do if nobody answers")
	ask.Flags().StringVar(&by, "by", "", "when your default stands: 36h, 2d or a date")
	ask.Flags().StringVar(&detail, "detail", "", "more detail")
	decided := a.cmd("decided <decision>", "Note a decision: it goes in your report, not to the human (--human --on-request: one the human made)", exact(1),
		func(m *core.Mull, args []string) (any, error) {
			return m.Decided(args[0], item, plan, detail, human, or)
		})
	decided.Flags().StringVar(&item, "item", "", "the item it's about")
	decided.Flags().StringVar(&plan, "plan", "", "the plan, with no item")
	decided.Flags().StringVar(&detail, "detail", "", "why")
	decided.Flags().BoolVar(&human, "human", false, "the human made it")
	onRequest(decided, &or)
	inbox := a.cmd("inbox", "What needs the human: questions and reviews", noArgs, func(m *core.Mull, _ []string) (any, error) {
		e, err := m.Inbox()
		return map[string]any{"entries": e}, err
	})
	decide := a.cmd("decide <id> <decision>", "Answer a question with the human's decision", exact(2),
		func(m *core.Mull, args []string) (any, error) { return m.Decide(args[0], args[1], or) })
	onRequest(decide, &or)
	accept := a.cmd("accept <id...>", "Accept reviews (by entry or item) and looks, or take a question's default", atLeast(1),
		func(m *core.Mull, args []string) (any, error) { return m.Accept(args, or) })
	onRequest(accept, &or)
	var reason string
	back := a.cmd("send-back <id>", "Send reviewed work back with a reason (landed work gets a follow-up)", exact(1),
		func(m *core.Mull, args []string) (any, error) { return m.SendBack(args[0], reason, or) })
	back.Flags().StringVar(&reason, "reason", "", "required: why")
	onRequest(back, &or)
	dismiss := a.cmd("dismiss <id...>", "Drop inbox entries for good", atLeast(1),
		func(m *core.Mull, args []string) (any, error) { return m.DropEntries(args, reason, or) })
	dismiss.Flags().StringVar(&reason, "reason", "", "why")
	onRequest(dismiss, &or)
	decisions := a.cmd("decisions", "The decision log, newest first", noArgs, func(m *core.Mull, _ []string) (any, error) {
		d, err := m.Decisions()
		return map[string]any{"decisions": d}, err
	})
	return []*cobra.Command{ask, decided, inbox, decide, accept, back, dismiss, decisions}
}

// ---- The human's view, rules, map, audit ----

func viewCmds(a *app) []*cobra.Command {
	var since string
	var asText bool
	status := &cobra.Command{Use: "status", Short: "What changed since the human last looked: what needs them, what's worse, the plans, the flow", Args: noArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			from := since
			if look, ok, err := seen.Get(m.MainWorktree()); from == "" && err == nil && ok {
				from = look.At
			}
			v, err := m.Surface(from)
			if err != nil {
				return err
			}
			if asText {
				_, err = fmt.Fprintln(a.stdout, v.Text())
				return err
			}
			return a.json(v)
		}}
	status.Flags().StringVar(&since, "since", "", "from this time (RFC 3339) instead of the last look")
	status.Flags().BoolVar(&asText, "text", false, "for a terminal")
	rules := a.cmd("rules", "The rulebook (RULES.md): each rule, its scope and check, advisory ones marked", noArgs,
		func(m *core.Mull, _ []string) (any, error) { return m.Rules() })
	rules.AddCommand(a.cmd("check", "Run the rulebook's checks here now", noArgs, func(m *core.Mull, _ []string) (any, error) { return m.CheckRules() }))
	var grow, or bool
	ratchet := a.cmd("ratchet <id>", "Shrink a rule's ratchet file to the violations found now (growing it: --grow --on-request)", exact(1),
		func(m *core.Mull, args []string) (any, error) { return m.Ratchet(args[0], grow, or) })
	ratchet.Flags().BoolVar(&grow, "grow", false, "let it grow, or set its baseline (the human's)")
	onRequest(ratchet, &or)
	rules.AddCommand(ratchet)
	var record bool
	mp := a.cmd("map", "The code's connections against the declared ones and the last recording (--record --on-request sets it)", noArgs,
		func(m *core.Mull, _ []string) (any, error) { return m.Map(record, or) })
	mp.Flags().BoolVar(&record, "record", false, "record them as the baseline (the human's)")
	onRequest(mp, &or)
	audit := a.cmd("audit", "The latest audit (mull guide auditor)", noArgs, func(m *core.Mull, _ []string) (any, error) { return m.Audit() })
	audit.AddCommand(a.cmd("record -", "Record an audit from stdin", exact(1), func(m *core.Mull, args []string) (any, error) {
		text, err := a.text(args[0])
		if err != nil {
			return nil, err
		}
		return m.RecordAudit(text)
	}))
	return []*cobra.Command{status, rules, mp, audit}
}
