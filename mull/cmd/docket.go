package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type docketRow struct {
	ID     string `json:"id"`
	Title  string `json:"title,omitempty"`
	Status string `json:"status,omitempty"`
	Epic   string `json:"epic,omitempty"`
	Note   string `json:"note,omitempty"`
}

var docketCmd = &cobra.Command{
	Use:   "docket",
	Short: "Show the prioritized work queue",
	RunE: func(cmd *cobra.Command, args []string) error {
		invert, _ := cmd.Flags().GetBool("invert")
		if invert {
			matters, err := store.ListMatters(nil)
			if err != nil {
				return err
			}
			// Exclude done/dropped by default unless --all
			showAll, _ := cmd.Flags().GetBool("all")
			if !showAll {
				matters = excludeTerminal(matters)
			}
			matters, err = excludeDocketed(matters)
			if err != nil {
				return err
			}
			if matters == nil {
				return json.NewEncoder(os.Stdout).Encode([]any{})
			}
			return json.NewEncoder(os.Stdout).Encode(matters)
		}

		entries, err := store.LoadDocket()
		if err != nil {
			return err
		}

		rows := make([]docketRow, 0, len(entries))
		for _, e := range entries {
			row := docketRow{ID: e.ID, Note: e.Note}
			m, err := store.GetMatter(e.ID)
			if err == nil {
				row.Title = m.Title
				row.Status = m.Status
				row.Epic = m.Epic
			}
			rows = append(rows, row)
		}

		return json.NewEncoder(os.Stdout).Encode(rows)
	},
}

var docketAddCmd = &cobra.Command{
	Use:   "add <id>",
	Short: "Add a matter to the docket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		afterID, _ := cmd.Flags().GetString("after")
		note, _ := cmd.Flags().GetString("note")

		if err := store.DocketAdd(id, afterID, note); err != nil {
			return err
		}

		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"status": "added",
			"id":     id,
		})
	},
}

var docketRmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Remove a matter from the docket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		if err := store.DocketRemove(id); err != nil {
			return err
		}

		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"status": "removed",
			"id":     id,
		})
	},
}

var docketMoveCmd = &cobra.Command{
	Use:   "move <id> --after <id>",
	Short: "Move a matter within the docket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		afterID, _ := cmd.Flags().GetString("after")

		if afterID == "" {
			return fmt.Errorf("--after flag is required")
		}

		if err := store.DocketMove(id, afterID); err != nil {
			return err
		}

		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"status": "moved",
			"id":     id,
		})
	},
}

func init() {
	docketAddCmd.Flags().String("after", "", "insert after this ID")
	docketAddCmd.Flags().String("note", "", "annotation for the docket entry")

	docketMoveCmd.Flags().String("after", "", "move to after this ID")

	docketCmd.Flags().Bool("invert", false, "show matters NOT on the docket")
	docketCmd.Flags().Bool("all", false, "include done and dropped matters")

	docketCmd.AddCommand(docketAddCmd)
	docketCmd.AddCommand(docketRmCmd)
	docketCmd.AddCommand(docketMoveCmd)
	rootCmd.AddCommand(docketCmd)
}
