package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Delete a book from your library",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := store.DeleteBook(args[0]); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"removed": args[0]})
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
