package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var doneCmd = &cobra.Command{
	Use:   "done <id>",
	Short: "Mark a matter as done",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		m, err := store.UpdateMatter(id, "status", "done")
		if err != nil {
			return err
		}
		_ = store.DocketRemove(id) // ignore error if not in docket
		return json.NewEncoder(os.Stdout).Encode(m)
	},
}

func init() {
	rootCmd.AddCommand(doneCmd)
}
