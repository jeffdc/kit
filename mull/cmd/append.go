package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var appendCmd = &cobra.Command{
	Use:   "append <id> <text>",
	Short: "Append text to a matter's body",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := store.AppendBody(args[0], args[1])
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(m)
	},
}

func init() {
	rootCmd.AddCommand(appendCmd)
}
