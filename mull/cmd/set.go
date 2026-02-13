package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <id> <key> <value>",
	Short: "Set a metadata field on a matter",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := store.UpdateMatter(args[0], args[1], args[2])
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(m)
	},
}

func init() {
	rootCmd.AddCommand(setCmd)
}
