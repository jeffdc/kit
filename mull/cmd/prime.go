package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

type primeMatter struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Status  string   `json:"status"`
	Tags    []string `json:"tags,omitempty"`
	Relates []string `json:"relates,omitempty"`
	Blocks  []string `json:"blocks,omitempty"`
	Needs   []string `json:"needs,omitempty"`
	Parent  string   `json:"parent,omitempty"`
}

type primeOutput struct {
	Matters []primeMatter      `json:"matters"`
	Docket  []string           `json:"docket"`
	Counts  map[string]int     `json:"counts"`
}

var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Compact dump for LLM context injection",
	Long:  `Token-efficient summary. Excludes done and dropped matters. Bodies omitted.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
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
			if m.Status == "done" || m.Status == "dropped" {
				continue
			}

			out.Counts[m.Status]++
			out.Matters = append(out.Matters, primeMatter{
				ID:      m.ID,
				Title:   m.Title,
				Status:  m.Status,
				Tags:    m.Tags,
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

		return json.NewEncoder(os.Stdout).Encode(out)
	},
}

func init() {
	rootCmd.AddCommand(primeCmd)
}
