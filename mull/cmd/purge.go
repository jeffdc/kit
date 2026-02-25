package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var purgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Delete done matters older than a threshold",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		olderThan, _ := cmd.Flags().GetString("older-than")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		includeDropped, _ := cmd.Flags().GetBool("include-dropped")

		days, err := parseDuration(olderThan)
		if err != nil {
			return err
		}
		cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

		all, err := store.ListMatters(nil)
		if err != nil {
			return err
		}

		type purgeEntry struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Status  string `json:"status"`
			Updated string `json:"updated"`
		}

		var targets []purgeEntry
		for _, m := range all {
			if m.Status == "done" || (includeDropped && m.Status == "dropped") {
				if m.Updated <= cutoff {
					targets = append(targets, purgeEntry{
						ID:      m.ID,
						Title:   m.Title,
						Status:  m.Status,
						Updated: m.Updated,
					})
				}
			}
		}

		if dryRun {
			result := map[string]any{
				"dry_run": true,
				"count":   len(targets),
				"matters": targets,
			}
			if targets == nil {
				result["matters"] = []any{}
			}
			return json.NewEncoder(os.Stdout).Encode(result)
		}

		var deleted []purgeEntry
		var errors []map[string]string
		for _, t := range targets {
			if err := store.DeleteMatter(t.ID); err != nil {
				errors = append(errors, map[string]string{"id": t.ID, "error": err.Error()})
				continue
			}
			_ = store.DocketRemove(t.ID)
			_ = store.RemoveAllReferences(t.ID)
			deleted = append(deleted, t)
		}

		result := map[string]any{
			"deleted": deleted,
			"count":   len(deleted),
		}
		if deleted == nil {
			result["deleted"] = []any{}
		}
		if len(errors) > 0 {
			result["errors"] = errors
		}
		return json.NewEncoder(os.Stdout).Encode(result)
	},
}

// parseDuration parses a duration string like "30d" into days.
func parseDuration(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 30, nil
	}
	s = strings.TrimSuffix(s, "d")
	days, err := strconv.Atoi(s)
	if err != nil || days < 0 {
		return 0, fmt.Errorf("invalid duration %q, expected positive integer with optional 'd' suffix (e.g. 30d)", s)
	}
	return days, nil
}

func init() {
	purgeCmd.Flags().String("older-than", "30d", "delete matters older than this (e.g. 30d, 60d)")
	purgeCmd.Flags().Bool("dry-run", false, "show what would be deleted without deleting")
	purgeCmd.Flags().Bool("include-dropped", false, "also purge dropped matters (default: done only)")
	rootCmd.AddCommand(purgeCmd)
}
