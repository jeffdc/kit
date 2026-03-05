package cmd

import (
	"fmt"
	"os"

	"flint/internal/git"
	"flint/internal/obsidian"

	"github.com/spf13/cobra"
)

var (
	client  *obsidian.Client
	project string
)

var rootCmd = &cobra.Command{
	Use:   "flint [thought]",
	Short: "Quick capture to Obsidian, tagged by project",
	Long:  "Capture thoughts to your Obsidian daily note, automatically tagged with the current git project.",
	Args:  cobra.ArbitraryArgs,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		client = obsidian.New(&obsidian.CLIRunner{})
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		project, err = git.RepoName(dir)
		if err != nil {
			// Outside a git repo — use "inbox"
			project = "inbox"
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return showRecent()
		}
		return capture(args)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
