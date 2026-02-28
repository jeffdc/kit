package cmd

import (
	"encoding/json"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var readCmd = &cobra.Command{
	Use:   "read <id>",
	Short: "Mark a book as read",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		book, err := store.UpdateBook(id, "status", "read")
		if err != nil {
			return err
		}

		book, err = store.UpdateBook(id, "date_read", time.Now().Format("2006-01-02"))
		if err != nil {
			return err
		}

		return json.NewEncoder(os.Stdout).Encode(confirm(book))
	},
}

func init() {
	rootCmd.AddCommand(readCmd)
}
