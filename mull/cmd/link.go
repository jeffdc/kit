package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:   "link <id> <type> <id> [id...]",
	Short: "Create a relationship between matters",
	Long:  `Type is one of: relates, blocks, needs, parent.`,
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		id1 := args[0]
		relType := args[1]
		targets := args[2:]

		results := make([]map[string]string, 0, len(targets))
		for _, id2 := range targets {
			if err := store.LinkMatters(id1, relType, id2); err != nil {
				return err
			}
			results = append(results, map[string]string{
				"from": id1,
				"type": relType,
				"to":   id2,
			})
		}

		// Backward compatible: single target returns single object
		if len(results) == 1 {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{
				"linked": results[0],
			})
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"linked": results,
		})
	},
}

func init() {
	rootCmd.AddCommand(linkCmd)
}
