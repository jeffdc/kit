package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search across matters",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := store.SearchMatters(args[0])
		if err != nil {
			return err
		}
		if results == nil {
			return json.NewEncoder(os.Stdout).Encode([]any{})
		}
		return json.NewEncoder(os.Stdout).Encode(results)
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
