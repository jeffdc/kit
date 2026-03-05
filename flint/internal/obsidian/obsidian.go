package obsidian

import (
	"fmt"
	"strings"
)

// Runner executes obsidian CLI commands.
type Runner interface {
	Run(args ...string) (string, error)
}

// Client wraps the obsidian CLI.
type Client struct {
	runner Runner
}

func New(runner Runner) *Client {
	return &Client{runner: runner}
}

// Append adds a tagged entry to today's daily note.
func (c *Client) Append(project, text string) error {
	content := fmt.Sprintf("#%s %s", project, text)
	_, err := c.runner.Run("daily:append", "content="+content)
	return err
}

// Search finds notes containing the project tag.
func (c *Client) Search(project string) ([]string, error) {
	query := fmt.Sprintf("query=#%s", project)
	output, err := c.runner.Run("search", query)
	if err != nil {
		return nil, err
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return nil, nil
	}
	return strings.Split(output, "\n"), nil
}
