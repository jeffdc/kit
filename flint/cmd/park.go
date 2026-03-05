package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"flint/internal/git"

	"github.com/spf13/cobra"
)

var parkCmd = &cobra.Command{
	Use:   "park",
	Short: "Save context before stepping away",
	Long:  "Gathers git state (branch, dirty files, recent commits) and your notes, then writes a context dump to your Obsidian daily note.",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}

		ctx, err := git.Context(dir)
		if err != nil {
			return fmt.Errorf("git context failed: %w", err)
		}

		fmt.Println("What were you thinking? (empty line to finish)")
		var lines []string
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				break
			}
			lines = append(lines, line)
		}
		notes := strings.Join(lines, " ")

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
