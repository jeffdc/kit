package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Create a new matter",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]
		meta := make(map[string]any)

		tags, _ := cmd.Flags().GetStringSlice("tag")
		if len(tags) > 0 {
			meta["tags"] = tags
		}
		if s, _ := cmd.Flags().GetString("status"); s != "" {
			meta["status"] = s
		}
		if e, _ := cmd.Flags().GetString("effort"); e != "" {
			meta["effort"] = e
		}
		if ep, _ := cmd.Flags().GetString("epic"); ep != "" {
			meta["epic"] = ep
		}

		m, err := store.CreateMatter(title, meta)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(m)
	},
}

func init() {
	addCmd.Flags().StringSlice("tag", nil, "add a tag (repeatable)")
	addCmd.Flags().String("status", "", "set initial status")
	addCmd.Flags().String("effort", "", "set effort estimate")
	addCmd.Flags().String("epic", "", "assign to an epic")
	rootCmd.AddCommand(addCmd)
}
