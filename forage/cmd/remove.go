package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Delete a book from your library",
	Long: `Permanently delete a book from your library. Cannot be undone.
Use "forage drop" instead to mark as dropped but keep the record.

Example:
  forage remove a3f2

Output: {"removed": "a3f2"}`,
	Args: cobra.ExactArgs(1),
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
