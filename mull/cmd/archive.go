package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

type archiveEntry struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Move done and dropped matters into .mull/archive",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		all, err := store.ListMatters(nil)
		if err != nil {
			return err
		}

		var targets []archiveEntry
		for _, m := range all {
			if !m.IsTerminal() {
				continue
			}
			targets = append(targets, archiveEntry{
				ID:     m.ID,
				Title:  m.Title,
				Status: m.Status,
			})
		}

		result := map[string]any{
			"archived": targets,
			"count":    len(targets),
		}
		if targets == nil {
			result["archived"] = []any{}
			result["message"] = "no matters archived"
			return json.NewEncoder(os.Stdout).Encode(result)
		}

		var archived []archiveEntry
		var errors []map[string]string
		for _, target := range targets {
			if _, err := store.ArchiveMatter(target.ID); err != nil {
				errors = append(errors, map[string]string{"id": target.ID, "error": err.Error()})
				continue
			}
			archived = append(archived, target)
		}

		for _, target := range archived {
			_ = store.DocketRemove(target.ID)
			_ = store.RemoveAllReferences(target.ID)
		}

		result["archived"] = archived
		result["count"] = len(archived)
		if archived == nil {
			result["archived"] = []any{}
		}
		if len(errors) > 0 {
			result["errors"] = errors
		}

		return json.NewEncoder(os.Stdout).Encode(result)
	},
}

func init() {
	rootCmd.AddCommand(archiveCmd)
}
