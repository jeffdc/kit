package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var dropCmd = &cobra.Command{
	Use:   "drop <id>",
	Short: "Mark a book as dropped",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		book, err := store.UpdateBook(args[0], "status", "dropped")
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(confirm(book))
	},
}

func init() {
	rootCmd.AddCommand(dropCmd)
}
