package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var dropCmd = &cobra.Command{
	Use:   "drop <id>",
	Short: "Drop a matter by setting its status to dropped",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := store.UpdateMatter(args[0], "status", "dropped")
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(m)
	},
}

func init() {
	rootCmd.AddCommand(dropCmd)
}
