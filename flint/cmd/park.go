package cmd

import (
	"fmt"
	"os"
	"strings"

	"flint/internal/git"
	"flint/internal/obsidian"

	"github.com/spf13/cobra"
)

var parkCmd = &cobra.Command{
	Use:   "park [notes]",
	Short: "Save context before stepping away",
	Long: `Two-step flow for LLM callers:
  1. flint park --context  — print git state to stdout (no write)
  2. flint park "notes"    — write git context + notes to Obsidian daily note

With no args and no --context flag, defaults to printing context.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}

		ctx, err := git.Context(dir)
		if err != nil {
			return fmt.Errorf("git context failed: %w", err)
		}

		// No args: print context and exit
		if len(args) == 0 {
			fmt.Print(obsidian.FormatContext(ctx))
			return nil
		}

		// With args: write park entry
		notes := strings.Join(args, " ")
		if err := client.Park(project, ctx, notes); err != nil {
			return fmt.Errorf("park failed: %w", err)
		}
		fmt.Printf("parked #%s (%s)\n", project, ctx.Branch)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(parkCmd)
}
