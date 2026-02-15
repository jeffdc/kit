package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <id> [id...] <key> <value>",
	Short: "Set a metadata field on one or more matters",
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Last two args are key/value, everything before is an ID.
		key := args[len(args)-2]
		value := args[len(args)-1]
		ids := args[:len(args)-2]

		if len(ids) == 1 {
			m, err := store.UpdateMatter(ids[0], key, value)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(m)
		}

		// Batch mode: collect results and errors.
		type result struct {
			ID    string `json:"id"`
			Error string `json:"error,omitempty"`
		}
		var results []any
		var errs []string
		for _, id := range ids {
			m, err := store.UpdateMatter(id, key, value)
			if err != nil {
				results = append(results, result{ID: id, Error: err.Error()})
				errs = append(errs, fmt.Sprintf("%s: %s", id, err.Error()))
			} else {
				results = append(results, m)
			}
		}
		if err := json.NewEncoder(os.Stdout).Encode(results); err != nil {
			return err
		}
		if len(errs) > 0 {
			return fmt.Errorf("errors on %d of %d matters", len(errs), len(ids))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setCmd)
}
