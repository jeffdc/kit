package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List matters with optional filters",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filters := make(map[string]string)

		if s, _ := cmd.Flags().GetString("status"); s != "" {
			filters["status"] = s
		}
		if t, _ := cmd.Flags().GetString("tag"); t != "" {
			filters["tag"] = t
		}
		if e, _ := cmd.Flags().GetString("effort"); e != "" {
			filters["effort"] = e
		}

		matters, err := store.ListMatters(filters)
		if err != nil {
			return err
		}
		if matters == nil {
			return json.NewEncoder(os.Stdout).Encode([]any{})
		}
		return json.NewEncoder(os.Stdout).Encode(matters)
	},
}

func init() {
	listCmd.Flags().String("status", "", "filter by status")
	listCmd.Flags().String("tag", "", "filter by tag")
	listCmd.Flags().String("effort", "", "filter by effort")
	rootCmd.AddCommand(listCmd)
}
