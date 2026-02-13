package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Permanently delete a matter",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		if err := store.DeleteMatter(id); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"deleted": id})
	},
}

func init() {
	rootCmd.AddCommand(rmCmd)
}
