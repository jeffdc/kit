package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Create a new matter",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]
		meta := make(map[string]any)

		tags, _ := cmd.Flags().GetStringSlice("tag")
		if len(tags) > 0 {
			meta["tags"] = tags
		}
		if s, _ := cmd.Flags().GetString("status"); s != "" {
			meta["status"] = s
		}
		if e, _ := cmd.Flags().GetString("effort"); e != "" {
			meta["effort"] = e
		}
		if ep, _ := cmd.Flags().GetString("epic"); ep != "" {
			meta["epic"] = ep
		}

		m, err := store.CreateMatter(title, meta)
		if err != nil {
			return err
		}

		// Body
		if body, _ := cmd.Flags().GetString("body"); body != "" {
			m, err = store.AppendBody(m.ID, body)
			if err != nil {
				return fmt.Errorf("matter %s created but body failed: %w", m.ID, err)
			}
		}

		// Links
		relatesIDs, _ := cmd.Flags().GetStringSlice("relates")
		for _, targetID := range relatesIDs {
			if err := store.LinkMatters(m.ID, "relates", targetID); err != nil {
				return fmt.Errorf("matter %s created but link failed: %w", m.ID, err)
			}
		}

		blocksIDs, _ := cmd.Flags().GetStringSlice("blocks")
		for _, targetID := range blocksIDs {
			if err := store.LinkMatters(m.ID, "blocks", targetID); err != nil {
				return fmt.Errorf("matter %s created but link failed: %w", m.ID, err)
			}
		}

		needsIDs, _ := cmd.Flags().GetStringSlice("needs")
		for _, targetID := range needsIDs {
			if err := store.LinkMatters(m.ID, "needs", targetID); err != nil {
				return fmt.Errorf("matter %s created but link failed: %w", m.ID, err)
			}
		}

		parentID, _ := cmd.Flags().GetString("parent")
		if parentID != "" {
			if err := store.LinkMatters(m.ID, "parent", parentID); err != nil {
				return fmt.Errorf("matter %s created but link failed: %w", m.ID, err)
			}
		}

		// Re-read if any links were created
		if len(relatesIDs) > 0 || len(blocksIDs) > 0 || len(needsIDs) > 0 || parentID != "" {
			m, err = store.GetMatter(m.ID)
			if err != nil {
				return err
			}
		}

		// Docket
		if docket, _ := cmd.Flags().GetBool("docket"); docket {
			if err := store.DocketAdd(m.ID, "", ""); err != nil {
				return fmt.Errorf("matter %s created but docket add failed: %w", m.ID, err)
			}
		}

		return json.NewEncoder(os.Stdout).Encode(m)
	},
}

func init() {
	addCmd.Flags().StringSlice("tag", nil, "add a tag (repeatable)")
	addCmd.Flags().String("status", "", "set initial status")
	addCmd.Flags().String("effort", "", "set effort estimate")
	addCmd.Flags().String("epic", "", "assign to an epic")
	addCmd.Flags().String("body", "", "set the matter body")
	addCmd.Flags().StringSlice("relates", nil, "link as relates to these matter IDs (repeatable)")
	addCmd.Flags().StringSlice("blocks", nil, "link as blocks these matter IDs (repeatable)")
	addCmd.Flags().StringSlice("needs", nil, "link as needs these matter IDs (repeatable)")
	addCmd.Flags().String("parent", "", "set parent matter ID")
	addCmd.Flags().Bool("docket", false, "add the new matter to the docket")
	rootCmd.AddCommand(addCmd)
}
