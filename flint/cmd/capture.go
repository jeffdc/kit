package cmd

import (
	"fmt"
	"strings"
)

func capture(args []string) error {
	thought := strings.Join(args, " ")
	if err := client.Append(project, thought); err != nil {
		return fmt.Errorf("capture failed: %w", err)
	}
	fmt.Printf("captured to #%s\n", project)
	return nil
}

func showRecent() error {
	results, err := client.Search(project)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}
	if len(results) == 0 {
		fmt.Printf("no entries for #%s\n", project)
		return nil
	}
	fmt.Printf("recent for #%s:\n", project)
	for _, r := range results {
		fmt.Printf("  %s\n", r)
	}
	return nil
}
