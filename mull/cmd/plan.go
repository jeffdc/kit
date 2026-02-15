package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan <id>",
	Short: "Mark a matter as planned",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := store.UpdateMatter(args[0], "status", "planned")
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(m)
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
}
