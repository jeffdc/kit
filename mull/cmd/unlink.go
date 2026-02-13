package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var unlinkCmd = &cobra.Command{
	Use:   "unlink <id> <type> <id>",
	Short: "Remove a relationship between two matters",
	Long:  `Type is one of: relates, blocks, needs, parent.`,
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		id1 := args[0]
		relType := args[1]
		id2 := args[2]

		if err := store.UnlinkMatters(id1, relType, id2); err != nil {
			return err
		}

		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"unlinked": map[string]string{
				"from": id1,
				"type": relType,
				"to":   id2,
			},
		})
	},
}

func init() {
	rootCmd.AddCommand(unlinkCmd)
}
