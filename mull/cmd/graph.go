package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
	"mull/internal/model"
)

type graphNode struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type graphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

type graphOutput struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

var graphCmd = &cobra.Command{
	Use:   "graph [id]",
	Short: "Show dependency graph",
	Long:  `Without arguments, shows graph of all docket matters. With an ID, shows graph centered on that matter.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return graphSingle(args[0])
		}
		all, _ := cmd.Flags().GetBool("all")
		if all {
			return graphAll()
		}
		return graphDocket()
	},
}

func graphAll() error {
	matters, err := store.ListMatters(nil)
	if err != nil {
		return err
	}

	idSet := make(map[string]bool)
	for _, m := range matters {
		idSet[m.ID] = true
	}

	return buildGraph(idSet)
}

func graphDocket() error {
	entries, err := store.LoadDocket()
	if err != nil {
		return err
	}

	idSet := make(map[string]bool)
	for _, e := range entries {
		idSet[e.ID] = true
	}

	return buildGraph(idSet)
}

func graphSingle(id string) error {
	m, err := store.GetMatter(id)
	if err != nil {
		return err
	}

	idSet := map[string]bool{id: true}
	for _, r := range m.Blocks {
		idSet[r] = true
	}
	for _, r := range m.Needs {
		idSet[r] = true
	}
	for _, r := range m.Relates {
		idSet[r] = true
	}
	if m.Parent != "" {
		idSet[m.Parent] = true
	}

	return buildGraph(idSet)
}

func buildGraph(idSet map[string]bool) error {
	out := graphOutput{
		Nodes: []graphNode{},
		Edges: []graphEdge{},
	}

	seen := make(map[string]*model.Matter)
	for id := range idSet {
		m, err := store.GetMatter(id)
		if err != nil {
			continue
		}
		if m.IsTerminal() {
			continue
		}
		seen[id] = m
		out.Nodes = append(out.Nodes, graphNode{
			ID:     m.ID,
			Title:  m.Title,
			Status: m.Status,
		})
	}

	edgeSeen := make(map[string]bool)
	for id, m := range seen {
		for _, target := range m.Blocks {
			if _, ok := seen[target]; ok {
				key := id + "-blocks-" + target
				if !edgeSeen[key] {
					edgeSeen[key] = true
					out.Edges = append(out.Edges, graphEdge{From: id, To: target, Type: "blocks"})
				}
			}
		}
		for _, target := range m.Relates {
			if _, ok := seen[target]; ok {
				// Only add one direction for relates
				key1 := id + "-relates-" + target
				key2 := target + "-relates-" + id
				if !edgeSeen[key1] && !edgeSeen[key2] {
					edgeSeen[key1] = true
					out.Edges = append(out.Edges, graphEdge{From: id, To: target, Type: "relates"})
				}
			}
		}
	}

	return json.NewEncoder(os.Stdout).Encode(out)
}

func init() {
	graphCmd.Flags().BoolP("all", "a", false, "Show all matters, not just docket")
	rootCmd.AddCommand(graphCmd)
}
