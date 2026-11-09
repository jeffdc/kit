package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"mull/internal/model"
	"mull/internal/mull"
)

func initCmd(a *app) *cobra.Command {
	var integration, release string
	c := &cobra.Command{
		Use:   "init",
		Short: "Create the mull branch in this repository",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			cfg, err := m.Init(integration, release)
			if err != nil {
				return err
			}
			return a.json(map[string]any{"initialized": true, "branch": "mull", "config": cfg})
		},
	}
	c.Flags().StringVar(&integration, "integration", "main", "branch where work lands")
	c.Flags().StringVar(&release, "release", "main", "branch where work ships")
	return c
}

func syncCmd(a *app) *cobra.Command {
	var dryRun bool
	c := &cobra.Command{
		Use:   "sync",
		Short: "Exchange the mull branch with the remote (pushes only the mull branch, never forced)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			r, err := m.Sync(dryRun)
			if err != nil {
				return err
			}
			return a.json(r)
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show what would happen, change nothing")
	return c
}

func planCmd(a *app) *cobra.Command {
	c := &cobra.Command{Use: "plan", Short: "Plans: the top level of the work"}
	var summary string
	newC := &cobra.Command{
		Use:   "new <slug> <title>",
		Short: "Create a plan (slug: lowercase words joined by hyphens)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			s, err := a.text(summary)
			if err != nil {
				return err
			}
			p, err := m.PlanNew(args[0], args[1], s)
			if err != nil {
				return err
			}
			return a.json(p)
		},
	}
	newC.Flags().StringVar(&summary, "summary", "", "high-level summary (- reads stdin)")
	list := &cobra.Command{
		Use: "list", Short: "List plans with progress", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			ps, err := m.PlanList()
			if err != nil {
				return err
			}
			return a.json(map[string]any{"plans": ps})
		},
	}
	show := &cobra.Command{
		Use: "show <slug>", Short: "Show a plan with its epics and items", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			p, err := m.PlanShow(args[0])
			if err != nil {
				return err
			}
			return a.json(p)
		},
	}
	closeC := &cobra.Command{
		Use: "close <slug>", Short: "Close a plan", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			p, err := m.PlanClose(args[0])
			if err != nil {
				return err
			}
			return a.json(p)
		},
	}
	c.AddCommand(newC, list, show, closeC)
	return c
}

func epicCmd(a *app) *cobra.Command {
	c := &cobra.Command{Use: "epic", Short: "Epics: big pieces of a plan"}
	var plan, summary string
	var needs []string
	add := &cobra.Command{
		Use: "add <title>", Short: "Add an epic to a plan", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			s, err := a.text(summary)
			if err != nil {
				return err
			}
			e, err := m.EpicAdd(plan, args[0], s, splitIDs(needs))
			if err != nil {
				return err
			}
			return a.json(e)
		},
	}
	add.Flags().StringVar(&plan, "plan", "", "plan slug (required)")
	add.Flags().StringVar(&summary, "summary", "", "summary (- reads stdin)")
	add.Flags().StringSliceVar(&needs, "needs", nil, "items or epics this epic needs")
	_ = add.MarkFlagRequired("plan")

	var listPlan string
	list := &cobra.Command{
		Use: "list", Short: "List epics with progress", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			es, err := m.EpicList(listPlan)
			if err != nil {
				return err
			}
			return a.json(map[string]any{"epics": es})
		},
	}
	list.Flags().StringVar(&listPlan, "plan", "", "only this plan")
	c.AddCommand(add, list)
	return c
}

func addCmd(a *app) *cobra.Command {
	var in mull.NewItem
	var needs []string
	c := &cobra.Command{
		Use:   "add <title>",
		Short: "Add an item: a unit of work one agent completes in one session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			in.Title = args[0]
			in.Needs = splitIDs(needs)
			if in.Notes, err = a.text(in.Notes); err != nil {
				return err
			}
			v, err := m.ItemAdd(in)
			if err != nil {
				return err
			}
			return a.json(v)
		},
	}
	c.Flags().StringVar(&in.Plan, "plan", "", "plan slug (required)")
	c.Flags().StringVar(&in.Epic, "epic", "", "epic ID")
	c.Flags().StringSliceVar(&needs, "needs", nil, "items or epics this item needs")
	c.Flags().StringVar(&in.Notes, "notes", "", "short notes (- reads stdin)")
	c.Flags().StringSliceVar(&in.Docs, "doc", nil, "path of a related document (repeatable)")
	_ = c.MarkFlagRequired("plan")
	return c
}

func showCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use: "show <slug|id>", Short: "Show a plan, epic, item or inbox entry", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			v, err := m.Show(args[0])
			if err != nil {
				return err
			}
			return a.json(v)
		},
	}
}

func listCmd(a *app) *cobra.Command {
	var f mull.ItemFilter
	c := &cobra.Command{
		Use: "list", Short: "List items", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			items, err := m.ItemList(f)
			if err != nil {
				return err
			}
			return a.json(map[string]any{"items": items})
		},
	}
	c.Flags().StringVar(&f.Plan, "plan", "", "only this plan")
	c.Flags().StringVar(&f.Epic, "epic", "", "only this epic")
	c.Flags().StringVar(&f.State, "state", "", "only this state (open, active, done, accepted, dropped)")
	return c
}

func readyCmd(a *app) *cobra.Command {
	var f mull.ItemFilter
	c := &cobra.Command{
		Use: "ready", Short: "List items ready to start (open, with every need finished)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			f.ReadyOnly = true
			items, err := m.ItemList(f)
			if err != nil {
				return err
			}
			return a.json(map[string]any{"items": items})
		},
	}
	c.Flags().StringVar(&f.Plan, "plan", "", "only this plan")
	return c
}

func setCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "set <slug|id> <field> <value>",
		Short: "Change a field (plan/epic: title, summary; item: title, notes, epic, doc). Value - reads stdin",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			val, err := a.text(args[2])
			if err != nil {
				return err
			}
			v, err := m.Set(args[0], args[1], val)
			if err != nil {
				return err
			}
			return a.json(v)
		},
	}
}

func needCmd(a *app, add bool) *cobra.Command {
	use, short := "need <id> <target...>", "Record that an item or epic needs other items or epics"
	if !add {
		use, short = "unneed <id> <target...>", "Remove needs"
	}
	return &cobra.Command{
		Use: use, Short: short, Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			targets := splitIDs(args[1:])
			var v any
			if add {
				v, err = m.Need(args[0], targets)
			} else {
				v, err = m.Unneed(args[0], targets)
			}
			if err != nil {
				return err
			}
			return a.json(v)
		},
	}
}

func startCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use: "start <id>", Short: "Claim an item for this worktree and mark it active", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			r, err := m.Start(args[0])
			if err != nil {
				return err
			}
			return a.json(r)
		},
	}
}

func releaseCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use: "release <id>", Short: "Give back a claim; the item returns to open", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			v, err := m.Release(args[0])
			if err != nil {
				return err
			}
			return a.json(v)
		},
	}
}

func captureCmd(a *app, verb string) *cobra.Command {
	kind, short, detailName := "", "", ""
	switch verb {
	case "decided":
		kind, short, detailName = model.KindDecision, "Record a decision you made that the human didn't", "why"
	case "ask":
		kind, short, detailName = model.KindQuestion, "Ask the human something", "detail"
	case "loose":
		kind, short, detailName = model.KindLoose, "Capture a loose end: a bug, inconsistency, half-finished work or assumption", "detail"
	}
	var item, plan, source, detail string
	c := &cobra.Command{
		Use: verb + " <text>", Short: short, Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			d, err := a.text(detail)
			if err != nil {
				return err
			}
			e, err := m.Capture(kind, args[0], item, plan, source, d)
			if err != nil {
				return err
			}
			return a.json(e)
		},
	}
	c.Flags().StringVar(&item, "item", "", "the item this concerns")
	c.Flags().StringVar(&plan, "plan", "", "the plan, when there is no item (default general)")
	c.Flags().StringVar(&source, "source", "", "where this came from (file, matter, commit)")
	c.Flags().StringVar(&detail, detailName, "", "more detail (- reads stdin)")
	return c
}

func inboxCmd(a *app) *cobra.Command {
	var f mull.InboxFilter
	var asJSON bool
	c := &cobra.Command{
		Use: "inbox", Short: "What needs the human, oldest first", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			es, err := m.Inbox(f)
			if err != nil {
				return err
			}
			if asJSON {
				return a.json(map[string]any{"entries": es})
			}
			return printInbox(a, es)
		},
	}
	c.Flags().StringVar(&f.Plan, "plan", "", "only this plan")
	c.Flags().StringVar(&f.Kind, "kind", "", "only this kind (decision, question, loose-end, review)")
	c.Flags().BoolVar(&f.Parked, "parked", false, "include parked entries")
	c.Flags().BoolVar(&f.All, "all", false, "include settled entries")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
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
			status := ""
			if e.Settled != nil {
				status = "  [" + e.Settled.Action + "]"
			}
			fmt.Fprintf(&b, "  %s  %-14s %s  %s%s\n", e.ID, where, e.Created[:10], e.Text, status)
		}
		b.WriteString("\n")
	}
	_, err := fmt.Fprint(a.stdout, strings.TrimRight(b.String(), "\n")+"\n")
	return err
}

func onRequestFlag(c *cobra.Command, o *mull.SettleOpts) {
	c.Flags().BoolVar(&o.OnRequest, "on-request", false, "an agent settling this on the human's explicit instruction")
}

func acceptCmd(a *app) *cobra.Command {
	var o mull.SettleOpts
	var followUp string
	c := &cobra.Command{
		Use: "accept <id...>", Short: "Ratify decisions, or accept reviewed work (entry or item IDs)", Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			r, err := m.Accept(splitIDs(args), followUp, o)
			if err != nil {
				return err
			}
			return a.json(r)
		},
	}
	c.Flags().StringVar(&followUp, "follow-up", "", "also create a follow-up item (one reviewed item only)")
	onRequestFlag(c, &o)
	return c
}

func decideCmd(a *app) *cobra.Command {
	var o mull.SettleOpts
	c := &cobra.Command{
		Use: "decide <id> <decision>", Short: "Settle an entry with your decision; it joins the decision log", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			text, err := a.text(args[1])
			if err != nil {
				return err
			}
			e, err := m.Decide(args[0], text, o)
			if err != nil {
				return err
			}
			return a.json(e)
		},
	}
	onRequestFlag(c, &o)
	return c
}

func doCmd(a *app) *cobra.Command {
	var o mull.SettleOpts
	var plan, epic, reason string
	c := &cobra.Command{
		Use:   "do <id>",
		Short: "Turn a question or loose end into an item, or send reviewed work back (--reason)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			r, err := m.Do(args[0], plan, epic, reason, o)
			if err != nil {
				return err
			}
			return a.json(r)
		},
	}
	c.Flags().StringVar(&plan, "plan", "", "plan for the new item (default: the entry's plan)")
	c.Flags().StringVar(&epic, "epic", "", "epic for the new item")
	c.Flags().StringVar(&reason, "reason", "", "why reviewed work is sent back")
	onRequestFlag(c, &o)
	return c
}

func parkCmd(a *app) *cobra.Command {
	var o mull.SettleOpts
	c := &cobra.Command{
		Use: "park <id> <reason>", Short: "Set an entry aside; it stays listed as parked", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			e, err := m.Park(args[0], args[1], o)
			if err != nil {
				return err
			}
			return a.json(e)
		},
	}
	onRequestFlag(c, &o)
	return c
}

func dropCmd(a *app) *cobra.Command {
	var o mull.SettleOpts
	var reason string
	c := &cobra.Command{
		Use: "drop <id...>", Short: "Drop inbox entries (gone for good, recorded) or items", Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			r, err := m.Drop(splitIDs(args), reason, o)
			if err != nil {
				return err
			}
			return a.json(r)
		},
	}
	c.Flags().StringVar(&reason, "reason", "", "why (recorded)")
	onRequestFlag(c, &o)
	return c
}

func decisionsCmd(a *app) *cobra.Command {
	var plan string
	c := &cobra.Command{
		Use: "decisions", Short: "The decision log, newest first: read it before deciding anything", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := a.open()
			if err != nil {
				return err
			}
			ds, err := m.Decisions(plan)
			if err != nil {
				return err
			}
			return a.json(map[string]any{"decisions": ds})
		},
	}
	c.Flags().StringVar(&plan, "plan", "", "only this plan")
	return c
}
