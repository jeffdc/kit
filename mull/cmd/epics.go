package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

type epicSummary struct {
	Name   string         `json:"name"`
	Counts map[string]int `json:"counts"`
	Total  int            `json:"total"`
}

var epicsCmd = &cobra.Command{
	Use:   "epics",
	Short: "List all epics with matter counts",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		showAll, _ := cmd.Flags().GetBool("all")

		all, err := store.ListMatters(nil)
		if err != nil {
			return err
		}

		epics := make(map[string]map[string]int)
		for _, m := range all {
			if m.Epic == "" {
				continue
			}
			if !showAll && m.IsTerminal() {
				continue
			}
			if epics[m.Epic] == nil {
				epics[m.Epic] = make(map[string]int)
			}
			epics[m.Epic][m.Status]++
		}

		result := make([]epicSummary, 0, len(epics))
		for name, counts := range epics {
			total := 0
			for _, c := range counts {
				total += c
			}
			result = append(result, epicSummary{Name: name, Counts: counts, Total: total})
		}

		return json.NewEncoder(os.Stdout).Encode(result)
	},
}

func init() {
	epicsCmd.Flags().Bool("all", false, "include done and dropped matters in counts")
	rootCmd.AddCommand(epicsCmd)
}
