package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a matter by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := store.GetMatter(args[0])
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(m)
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
