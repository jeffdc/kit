package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <id> <key> <value>",
	Short: "Update a field on a book",
	Long:  "Valid keys: title, author, status, rating, tags, date_read.\nFor tags, use comma-separated values.",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		book, err := store.UpdateBook(args[0], args[1], args[2])
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(confirm(book))
	},
}

func init() {
	rootCmd.AddCommand(setCmd)
}
