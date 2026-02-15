package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type primeMatter struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Status  string   `json:"status"`
	Tags    []string `json:"tags,omitempty"`
	Epic    string   `json:"epic,omitempty"`
	Relates []string `json:"relates,omitempty"`
	Blocks  []string `json:"blocks,omitempty"`
	Needs   []string `json:"needs,omitempty"`
	Parent  string   `json:"parent,omitempty"`
}

type primeOutput struct {
	Matters []primeMatter  `json:"matters"`
	Docket  []string       `json:"docket"`
	Counts  map[string]int `json:"counts"`
}

var primeContext bool

var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Compact dump for LLM context injection",
	Long: `Token-efficient summary. Excludes done and dropped matters. Bodies omitted.

Use --context to wrap output with workflow instructions for Claude Code hooks.
In --context mode, exits silently if no .mull/ directory exists.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// In context mode, exit silently if not a mull project.
		if primeContext {
			if _, err := os.Stat(".mull"); os.IsNotExist(err) {
				os.Exit(0)
			}
		}

		all, err := store.ListMatters(nil)
		if err != nil {
			return err
		}

		out := primeOutput{
			Matters: []primeMatter{},
			Docket:  []string{},
			Counts:  make(map[string]int),
		}

		for _, m := range all {
			if m.IsTerminal() {
				continue
			}

			out.Counts[m.Status]++
			out.Matters = append(out.Matters, primeMatter{
				ID:      m.ID,
				Title:   m.Title,
				Status:  m.Status,
				Tags:    m.Tags,
				Epic:    m.Epic,
				Relates: m.Relates,
				Blocks:  m.Blocks,
				Needs:   m.Needs,
				Parent:  m.Parent,
			})
		}

		entries, err := store.LoadDocket()
		if err != nil {
			return err
		}
		for _, e := range entries {
			out.Docket = append(out.Docket, e.ID)
		}

		if primeContext {
			return outputContext(out)
		}
		return json.NewEncoder(os.Stdout).Encode(out)
	},
}

func outputContext(out primeOutput) error {
	jsonBytes, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}

	fmt.Print(`# Mull — Matter Tracking

## Landscape

` + "```json\n" + string(jsonBytes) + "\n```" + `

## Workflow

- ` + "`mull show <id>`" + ` + ` + "`mull graph <id>`" + ` to load full context
- ` + "`mull add \"<title>\" --status raw --epic <name>`" + ` to capture new ideas
- ` + "`mull append <id> \"<text>\"`" + ` for details as they emerge
- ` + "`mull set <id> <key> <value>`" + ` for metadata
- ` + "`mull link <id> <type> <id>`" + ` for relationships (relates, blocks, needs, parent)
- ` + "`mull done <id>`" + ` to close a matter (sets done + removes from docket)
- ` + "`mull docket`" + ` to see the prioritized work queue
- ` + "`mull docket --invert`" + ` to see matters NOT on the docket
- ` + "`mull graph [id]`" + ` to see dependency relationships
- ` + "`mull search <query>`" + ` to find matters by keyword
- ` + "`mull list --epic <name>`" + ` to filter by epic
- ` + "`mull epics`" + ` to list all epics with counts

## Statuses

Valid statuses: raw, refined, planned, done, dropped. No others accepted.

## Closing vs Deleting

- ` + "`mull done <id>`" + ` — marks as done, matter stays for reference. **This is almost always what you want.**
- ` + "`mull drop <id>`" + ` — decided against, matter stays for reference
- ` + "`mull rm <id>`" + ` — **permanent delete**, only for junk/mistakes

## Principles

- **Capture as you go** — don't wait until the end
- **Match user's energy** — a tickler is not a spec, don't over-process
- **Don't push toward planning** — only when user signals execution intent
`)
	return nil
}

func init() {
	primeCmd.Flags().BoolVar(&primeContext, "context", false, "Wrap output with workflow instructions (for hooks)")
	rootCmd.AddCommand(primeCmd)
}
