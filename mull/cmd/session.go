package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Work session logs",
}

var sessionSaveCmd = &cobra.Command{
	Use:   "save [- | body]",
	Short: "Save a session log",
	Long: `Save a work session log. Body can be provided as a positional argument
or piped via stdin using "-".

Use --matter to associate the session with specific matters.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		matters, _ := cmd.Flags().GetStringSlice("matter")

		var body string
		if len(args) == 1 && args[0] != "-" {
			body = args[0]
		} else {
			info, err := os.Stdin.Stat()
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeCharDevice != 0 {
				return fmt.Errorf("no body argument and stdin is a terminal; pipe input or pass body as an argument")
			}
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			body = string(data)
		}

		sess, err := store.CreateSession(matters, body)
		if err != nil {
			return err
		}

		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"file":    sess.Filename,
			"date":    sess.Date.Format("2006-01-02T15:04"),
			"matters": sess.Matters,
		})
	},
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sessions, most recent first",
	RunE: func(cmd *cobra.Command, args []string) error {
		matterID, _ := cmd.Flags().GetString("matter")

		sessions, err := store.ListSessions(matterID)
		if err != nil {
			return err
		}

		type row struct {
			File    string   `json:"file"`
			Date    string   `json:"date"`
			Matters []string `json:"matters,omitempty"`
		}

		rows := make([]row, 0, len(sessions))
		for _, s := range sessions {
			rows = append(rows, row{
				File:    s.Filename,
				Date:    s.Date.Format("2006-01-02T15:04"),
				Matters: s.Matters,
			})
		}

		return json.NewEncoder(os.Stdout).Encode(rows)
	},
}

var sessionShowCmd = &cobra.Command{
	Use:   "show <filename>",
	Short: "Show a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sess, err := store.GetSession(args[0])
		if err != nil {
			return err
		}

		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"file":    sess.Filename,
			"date":    sess.Date.Format("2006-01-02T15:04"),
			"matters": sess.Matters,
			"body":    sess.Body,
		})
	},
}

var sessionContextCmd = &cobra.Command{
	Use:   "context",
	Short: "Show recent sessions for LLM context",
	RunE: func(cmd *cobra.Command, args []string) error {
		n, _ := cmd.Flags().GetInt("last")
		matterID, _ := cmd.Flags().GetString("matter")

		sessions, err := store.SessionContext(n, matterID)
		if err != nil {
			return err
		}

		type entry struct {
			File    string   `json:"file"`
			Date    string   `json:"date"`
			Matters []string `json:"matters,omitempty"`
			Body    string   `json:"body"`
		}

		entries := make([]entry, 0, len(sessions))
		for _, s := range sessions {
			entries = append(entries, entry{
				File:    s.Filename,
				Date:    s.Date.Format("2006-01-02T15:04"),
				Matters: s.Matters,
				Body:    s.Body,
			})
		}

		return json.NewEncoder(os.Stdout).Encode(entries)
	},
}

func init() {
	sessionSaveCmd.Flags().StringSlice("matter", nil, "matter IDs this session relates to")

	sessionListCmd.Flags().String("matter", "", "filter by matter ID")

	sessionContextCmd.Flags().Int("last", 3, "number of recent sessions to show")
	sessionContextCmd.Flags().String("matter", "", "filter by matter ID")

	sessionCmd.AddCommand(sessionSaveCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionShowCmd)
	sessionCmd.AddCommand(sessionContextCmd)
	rootCmd.AddCommand(sessionCmd)
}
