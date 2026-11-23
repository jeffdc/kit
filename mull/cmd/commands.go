package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"mull/internal/guide"
	"mull/internal/human"
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

// nameCmd shows or sets the name this machine's human goes by. It's per
// machine (beside the registry), never on the mull branch.
func nameCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "name [<name>]",
		Short: "Show or set your name on this machine (\"\" clears it): mull context and mull's messages use it for \"the human\", and agents write it. Never on the mull branch",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				if err := human.SetName(args[0]); err != nil {
					return &mull.Error{Code: mull.CodeInvalid, Msg: err.Error()}
				}
			}
			name, err := human.Name()
			if err != nil {
				return err
			}
			file, err := human.File()
			if err != nil {
				return err
			}
			return a.json(map[string]string{"name": name, "file": file})
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

func migrateCmd(a *app) *cobra.Command {
	var dryRun, onRequest, asText bool
	c := &cobra.Command{
		Use: "migrate",
		Short: "Convert this repository's mull data to Model v2 (format 2) in one commit. " +
			"Run --dry-run first; the real run is the human's call (--on-request)",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			r, err := m.Migrate(dryRun, onRequest)
			if err != nil {
				return err
			}
			if asText {
				_, err = fmt.Fprint(a.stdout, r.Text())
				return err
			}
			return a.json(r)
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show exactly what would change, write nothing")
	c.Flags().BoolVar(&onRequest, "on-request", false, "an agent migrating on the human's explicit instruction")
	c.Flags().BoolVar(&asText, "text", false, "render for a human instead of JSON")
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
	var checksIn mull.ConfigChecksIn
	checks := &cobra.Command{
		Use:   "checks [command...]",
		Short: "Replace the hard checks done runs (each argument is one shell command; none clears). Only the human changes them",
		RunE: func(cmd *cobra.Command, args []string) error {
			checksIn.Commands = args
			return a.write("config checks", &checksIn)
		},
	}
	checks.Flags().BoolVar(&checksIn.OnRequest, "on-request", false, "an agent changing the checks on the human's explicit instruction")
	layer := &cobra.Command{
		Use:   "layer <name> [glob...]",
		Short: "Set a layer's path globs for done reports (no globs removes it)",
		Args:  atLeast(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.write("config layer", &mull.ConfigLayerIn{Name: args[0], Globs: args[1:]})
		},
	}
	var riskIn mull.ConfigRiskIn
	var noSample bool
	risk := &cobra.Command{
		Use:   "risk",
		Short: "Change how done scores risk: the high-risk paths (wait for review), the size that makes work medium (lands, always shown), and the percent of low-risk work sampled. Only the human changes it",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if noSample {
				riskIn.Sample = -1
			}
			return a.write("config risk", &riskIn)
		},
	}
	risk.Flags().StringSliceVar((*[]string)(&riskIn.Add), "add", nil, "add high-risk path globs")
	risk.Flags().StringSliceVar((*[]string)(&riskIn.Remove), "remove", nil, "remove high-risk path globs")
	risk.Flags().IntVar(&riskIn.MaxLines, "max-lines", 0, "lines changed above which work is medium risk")
	risk.Flags().IntVar(&riskIn.MaxFiles, "max-files", 0, "files changed above which work is medium risk")
	risk.Flags().IntVar(&riskIn.Sample, "sample", 0, "percent of low-risk work shown to the human (1 to 100)")
	risk.Flags().BoolVar(&noSample, "no-sample", false, "sample no low-risk work")
	risk.Flags().BoolVar(&riskIn.OnRequest, "on-request", false, "an agent changing risk on the human's explicit instruction")
	c.AddCommand(set, checks, layer, risk)
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
		Short: "Create a plan, shaping until the human agrees it (slug: lowercase words joined by hyphens). Only the human creates a plan",
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
	newC.Flags().BoolVar(&in.OnRequest, "on-request", false, "an agent creating it on the human's explicit instruction")
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
	var agreeIn mull.PlanAgreeIn
	agree := &cobra.Command{
		Use:   "agree <slug>",
		Short: "Agree a shaping plan, making it active so its work can start. Only the human agrees a plan",
		Args:  exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agreeIn.Slug = args[0]
			return a.write("plan agree", &agreeIn)
		},
	}
	agree.Flags().BoolVar(&agreeIn.OnRequest, "on-request", false, "an agent agreeing this on the human's explicit instruction")
	closeC := &cobra.Command{
		Use: "close <slug>", Short: "Close a plan: it becomes done", Args: exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.write("plan close", &mull.PlanCloseIn{Slug: args[0]})
		},
	}
	set := &cobra.Command{
		Use: "set <slug> <title|summary|target|preview> <value>",
		Short: "Change a plan's field. target is the branch its work lands on, where seen in git is measured " +
			"(\"\" falls back to config integration); preview is the URL that serves that branch, handed to workers " +
			"(mull never runs or checks it; \"\" clears it). Value - reads stdin",
		Args: exact(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			val, err := a.text(args[2])
			if err != nil {
				return err
			}
			return a.write("plan set", &mull.PlanSetIn{Slug: args[0], Field: args[1], Value: val})
		},
	}
	doc := docCmd(a, "plan", "<slug>", "the picture, done-when and why the order",
		func(m *mull.Mull, slug string) (mull.DocResult, error) { return m.PlanDoc(slug) },
		func(m *mull.Mull, slug string) (mull.DocLog, error) { return m.PlanDocLog(slug) },
		func(ref, doc string) mull.Input { return &mull.PlanDocIn{Slug: ref, Doc: &doc} })
	rules := rulesCmd(a, "plan", "<slug>",
		func(m *mull.Mull, slug string) (mull.RulesResult, error) { return m.PlanRules(slug) },
		func(m *mull.Mull, slug string) (mull.RulesLog, error) { return m.PlanRulesLog(slug) },
		func(ref, rules string) mull.Input { return &mull.PlanRulesIn{Slug: ref, Rules: &rules} })
	var checksIn mull.PlanChecksIn
	checks := &cobra.Command{
		Use:   "checks <slug> [command...]",
		Short: "Replace the plan's own hard checks, which done runs for its items after config's (each argument is one shell command; none clears). Only the human changes them",
		Args:  atLeast(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			checksIn.Slug, checksIn.Commands = args[0], args[1:]
			return a.write("plan checks", &checksIn)
		},
	}
	checks.Flags().BoolVar(&checksIn.OnRequest, "on-request", false, "an agent changing the checks on the human's explicit instruction")
	shared := &cobra.Command{
		Use: "shared <slug> [glob...]",
		Short: "Replace the plan's shared globs: files its parallel work shares (layout, components, styles), " +
			"whose changes done flags in the report (globs as config layer's: * within a path segment, ** across them; none clears)",
		Args: atLeast(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.write("plan shared", &mull.PlanSharedIn{Slug: args[0], Globs: args[1:]})
		},
	}
	c.AddCommand(newC, agree, list, show, set, doc, rules, checks, shared, closeC)
	return c
}

// docCmd is plan doc and phase doc: print a doc, its history (--log), or
// replace it from stdin (-).
func docCmd(a *app, kind, arg, holds string,
	get func(m *mull.Mull, ref string) (mull.DocResult, error),
	history func(m *mull.Mull, ref string) (mull.DocLog, error),
	put func(ref, doc string) mull.Input) *cobra.Command {
	return textCmd(a, kind, "doc", arg, "Print a "+kind+"'s doc ("+holds+"; intent only, never status), "+
		"or replace it with - (stdin; empty removes it). --log: every version, newest first",
		func(m *mull.Mull, ref string) (any, string, error) {
			d, err := get(m, ref)
			return d, d.Doc, err
		},
		func(m *mull.Mull, ref string) (any, error) { return history(m, ref) }, put)
}

// rulesCmd is plan rules and phase rules, which work as docCmd does.
func rulesCmd(a *app, kind, arg string,
	get func(m *mull.Mull, ref string) (mull.RulesResult, error),
	history func(m *mull.Mull, ref string) (mull.RulesLog, error),
	put func(ref, rules string) mull.Input) *cobra.Command {
	return textCmd(a, kind, "rules", arg, "Print a "+kind+"'s rules (the human's standing rules for the agents working in it, "+
		"which context --item prints in full), or replace them with - (stdin; empty removes them). --log: every version, newest first",
		func(m *mull.Mull, ref string) (any, string, error) {
			r, err := get(m, ref)
			return r, r.Rules, err
		},
		func(m *mull.Mull, ref string) (any, error) { return history(m, ref) }, put)
}

// textCmd is a plan's or phase's markdown (its doc or its rules): print
// it, its history (--log), or replace it from stdin (-).
func textCmd(a *app, kind, name, arg, short string,
	get func(m *mull.Mull, ref string) (any, string, error),
	history func(m *mull.Mull, ref string) (any, error),
	put func(ref, text string) mull.Input) *cobra.Command {
	var log, asText bool
	c := &cobra.Command{
		Use:   name + " " + arg + " [-]",
		Short: short,
		Args:  wrapArgs(cobra.RangeArgs(1, 2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 2 {
				if args[1] != "-" {
					return usage(fmt.Errorf("the %s is read from stdin: give - (got %q)", name, args[1]))
				}
				if log || asText {
					return usage(fmt.Errorf("--log and --text are for reading the %s, not replacing it", name))
				}
				text, err := a.text("-")
				if err != nil {
					return err
				}
				return a.write(kind+" "+name, put(args[0], text))
			}
			m, err := a.open()
			if err != nil {
				return err
			}
			if log {
				if asText {
					return usage(fmt.Errorf("--log prints JSON only"))
				}
				l, err := history(m, args[0])
				if err != nil {
					return err
				}
				return a.json(l)
			}
			d, text, err := get(m, args[0])
			if err != nil {
				return err
			}
			if asText {
				if text == "" {
					return nil
				}
				_, err = fmt.Fprintln(a.stdout, text)
				return err
			}
			return a.json(d)
		},
	}
	c.Flags().BoolVar(&log, "log", false, "print every version of the "+name+", newest first")
	c.Flags().BoolVar(&asText, "text", false, "print the "+name+" as markdown instead of JSON")
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
	add.Flags().StringSliceVar(&needs, "needs", nil, "phases this phase needs")

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
	doc := docCmd(a, "phase", "<id>", "its design and done-when",
		func(m *mull.Mull, id string) (mull.DocResult, error) { return m.PhaseDoc(id) },
		func(m *mull.Mull, id string) (mull.DocLog, error) { return m.PhaseDocLog(id) },
		func(ref, doc string) mull.Input { return &mull.PhaseDocIn{ID: ref, Doc: &doc} })
	rules := rulesCmd(a, "phase", "<id>",
		func(m *mull.Mull, id string) (mull.RulesResult, error) { return m.PhaseRules(id) },
		func(m *mull.Mull, id string) (mull.RulesLog, error) { return m.PhaseRulesLog(id) },
		func(ref, rules string) mull.Input { return &mull.PhaseRulesIn{ID: ref, Rules: &rules} })
	var del mull.PhaseDeleteIn
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a phase no longer needed: only one with no committed work (captured items tagged to it move to its plan; needs on it are removed). Only the human deletes a phase",
		Args:  exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			del.ID = args[0]
			return a.write("phase delete", &del)
		},
	}
	onRequestFlag(deleteCmd, &del.OnRequest)
	var setIn mull.PhaseSetIn
	set := &cobra.Command{
		Use: "set <id> <title|summary|integration-review> <value>",
		Short: "Change a phase's field. integration-review on means that once its items are done it awaits an integration review, " +
			"and isn't complete (phases that need it stay blocked) until one is accepted; only the human turns it on or off. Value - reads stdin",
		Args: exact(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			val, err := a.text(args[2])
			if err != nil {
				return err
			}
			setIn.ID, setIn.Field, setIn.Value = args[0], args[1], val
			return a.write("phase set", &setIn)
		},
	}
	set.Flags().BoolVar(&setIn.OnRequest, "on-request", false, "an agent changing integration-review on the human's explicit instruction")
	c.AddCommand(add, list, set, doc, rules, deleteCmd)
	return c
}

// ---- Items ----

func addCmd(a *app) *cobra.Command {
	var in mull.AddIn
	var needs, docs []string
	c := &cobra.Command{
		Use:   "add <title>",
		Short: "Add an item: a unit of work one agent completes in one session (--phase implies its plan; --follow-up <done-id> takes the done item's)",
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
	c.Flags().StringVar(&in.FollowUp, "follow-up", "", "a done item this follows up: the new item joins its plan and phase, with from set to it")
	c.Flags().StringSliceVar(&needs, "needs", nil, "refused: items have no needs; phases need phases")
	c.Flags().StringVar(&in.Notes, "notes", "", "short notes (- reads stdin)")
	c.Flags().StringArrayVar(&docs, "doc", nil, "path of a related document (repeatable)")
	c.Flags().StringVar(&in.Before, "before", "", "rank it just before this open item in its group (default: last)")
	c.Flags().StringVar(&in.After, "after", "", "rank it just after this open item in its group (default: last)")
	_ = c.Flags().MarkHidden("needs") // kept only to refuse it with a clear message
	return c
}

func rankCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use: "rank <phase|plan|none> <ids...>",
		Short: "Set the order of a group's open work: a phase, a plan's items with no phase, or none (Unplanned). " +
			"List every committed, active and in-review item in it, first to last; mull writes the ranks",
		Args: atLeast(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.write("rank", &mull.RankIn{Group: args[0], IDs: mull.SplitIDs(args[1:])})
		},
	}
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
	c.Flags().BoolVar(&f.Untriaged, "untriaged", false, "only captured items not triaged yet")
	return c
}

func readyCmd(a *app) *cobra.Command {
	var f mull.ItemFilter
	c := &cobra.Command{
		Use: "ready", Short: "List items ready to start (committed, in a phase whose needs are done)", Args: noArgs,
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
		Short: "Change a field (plan: title, summary, target, preview; phase: title, summary, integration-review (the human's: mull phase set); item: title, notes, category, phase, doc; captured item: plan too, none untags, and kind bug or idea). Value - reads stdin",
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
	use, short, op := "need <phase> <phases...>", "Record that a phase needs other phases (items have no needs)", "need"
	if !add {
		use, short, op = "unneed <phase> <phases...>", "Remove a phase's needs", "unneed"
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
		"decided": "Note a decision you made (--detail: why): it goes in your done report, not to the human. With --human --on-request, record one the human made",
		"ask":     "Ask the human a question only they can answer: a rule conflict or something hard to undo. Give your default and when it stands",
	}[op]
	var item, plan, source, detail, deflt, by string
	var human, onRequest bool
	c := &cobra.Command{
		Use: op + " <text>", Short: short, Args: exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := a.text(detail)
			if err != nil {
				return err
			}
			in, _ := mull.NewInput(op)
			if di, ok := in.(*mull.DecidedIn); ok {
				di.Text, di.Item, di.Plan, di.Source, di.Detail = args[0], item, plan, source, d
				di.Human, di.OnRequest = human, onRequest
				return a.write(op, di)
			}
			if op == "ask" && strings.TrimSpace(deflt) == "" {
				return &mull.Error{Code: mull.CodeInvalid, Msg: "a question needs --default: what you'll do if nobody answers. Ask only about a rule conflict or something hard to undo; decide the rest yourself and note it with mull decided"}
			}
			ci := in.(*mull.CaptureIn)
			ci.Text, ci.Item, ci.Plan, ci.Source, ci.Detail = args[0], item, plan, source, d
			ci.Default, ci.By = deflt, by
			return a.write(op, ci)
		},
	}
	if op == "ask" {
		c.Flags().StringVar(&deflt, "default", "", "required: what you'll do if nobody answers")
		c.Flags().StringVar(&by, "by", "", "when your default stands: a duration (36h, 2d) or a date (2026-12-01)")
	}
	if op == "decided" {
		c.Flags().BoolVar(&human, "human", false, "the human made this decision (in conversation): settled at once, into the decision log (needs --on-request)")
		c.Flags().BoolVar(&onRequest, "on-request", false, "with --human: an agent recording the human's decision on their explicit instruction")
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
			c.Title, c.Plan, c.Phase, c.Category, c.Source = args[0], in.Plan, in.Phase, in.Category, in.Source
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

func triageCmd(a *app) *cobra.Command {
	var in mull.TriageIn
	c := &cobra.Command{
		Use:   "triage [id...]",
		Short: "Stamp captured items as triaged with the human (--all: the whole backlog, or one plan's with --plan). See mull guide triage",
		RunE: func(cmd *cobra.Command, args []string) error {
			in.IDs = mull.SplitIDs(args)
			return a.write("triage", &in)
		},
	}
	c.Flags().BoolVar(&in.All, "all", false, "every captured item in the project, or in --plan")
	c.Flags().StringVar(&in.Plan, "plan", "", "with --all: only this plan's backlog (none: captures tagged to no plan)")
	c.Flags().BoolVar(&in.OnRequest, "on-request", false, "an agent stamping these after triaging them with the human")
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
			} else if e.Phase != "" {
				where += "/" + e.Phase // an integration review
			}
			status, src := "", ""
			if e.Settled != nil {
				status = "  [" + e.Settled.Action + "]"
			}
			if e.Source != "" {
				src = "  (from " + e.Source + ")"
			}
			if len(e.Decisions) > 0 {
				src += fmt.Sprintf("  [carries decisions %s]", strings.Join(e.Decisions, " "))
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
		Use: "accept <id...>", Short: "Ratify decisions, or accept reviewed work (entry or item IDs, or a phase for its integration review)", Args: atLeast(1),
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
		Short: "Turn a question into an item (or --as phase), or send reviewed work back (--reason); also settles loose ends and ideas filed before Model v2",
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
	c.Flags().StringVar(&plan, "plan", "", "only this plan (done plans too)")
	c.Flags().BoolVar(&asText, "text", false, "render for a human instead of JSON")
	return c
}

func worktreesCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "worktrees",
		Short: "The claim worktrees, with their item, state and size, and which can be removed (read-only)",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.read(func(m *mull.Mull) (any, error) { return m.Worktrees() })
		},
	}
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
		Short: "A procedure or reference: agents (for AGENTS.md), supervisor, intake, triage (the backlog, with the human), json (output shapes)",
		Args:  exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := guide.Get(args[0]) // as written: guides are copied into shared files (AGENTS.md)
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
	var item string
	c := &cobra.Command{
		Use: "check", Short: "Run the hard checks against the clean HEAD and record the result", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			out, err := m.Check(item)
			if err != nil {
				return err
			}
			if !out.Passed {
				return &mull.Error{Code: mull.CodeChecksFailed, Msg: "hard checks failed: " + failedCommand(out.Results)}
			}
			return a.json(out)
		},
	}
	c.Flags().StringVar(&item, "item", "", "run what done would for this item: config's checks, then its plan's own")
	return c
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
	var summary, tour, design, rules string
	c := &cobra.Command{
		Use:   "done <item-id|phase-id>",
		Short: "Finish an item: run the hard checks once, score its risk, and land it (low, medium) or file it for review (high)",
		Args:  exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stdin := 0
			for _, s := range []string{summary, tour, design, rules} {
				if s == "-" {
					stdin++
				}
			}
			if stdin > 1 {
				return &mull.Error{Code: mull.CodeUsage, Msg: "only one of --summary, --rules, --tour and --design can read stdin; give the others inline"}
			}
			var err error
			if in.Summary, err = a.text(summary); err != nil {
				return err
			}
			if in.Design, err = a.text(design); err != nil {
				return err
			}
			if in.Rules, err = a.text(rules); err != nil {
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
			return a.read(func(m *mull.Mull) (any, error) { return m.Finish(args[0], in) })
		},
	}
	c.Flags().StringVar(&summary, "summary", "", "required: one line on what changed (- reads stdin)")
	c.Flags().StringVar(&rules, "rules", "", "required: none, or which rules the work touched and how (touching one makes it high risk)")
	c.Flags().StringVar(&in.Slowed, "slowed", "", "what slowed the work down, if anything (one line)")
	c.Flags().StringVar(&tour, "tour", "", "optional: what to look at and what to see, one step per line (- reads stdin)")
	c.Flags().StringVar(&design, "design", "", "optional: how it was built and why, when it isn't obvious (- reads stdin)")
	c.Flags().StringVar(&in.Override, "override", "", "HUMAN ONLY: finish despite failing or missing checks, with the reason")
	return c
}

func reportCmd(a *app) *cobra.Command {
	var n int
	var asText bool
	c := &cobra.Command{
		Use: "report <item-id|phase-id>", Short: "An item's done report, or a phase's integration review report (the latest by default)", Args: exact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			out, err := m.ReportText(args[0], n)
			if err != nil {
				return err
			}
			if asText {
				_, err = fmt.Fprintln(a.stdout, out.Text)
				return err
			}
			return a.json(out)
		},
	}
	c.Flags().IntVar(&n, "n", 0, "which attempt (default: latest)")
	c.Flags().BoolVar(&asText, "text", false, "print the report as markdown instead of JSON")
	return c
}
