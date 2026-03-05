package obsidian

import (
	"fmt"
	"flint/internal/git"
	"strings"
)

// FormatPark formats a context dump as a markdown block.
func FormatPark(project string, ctx *git.RepoContext, notes string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#%s #park\n", project)
	fmt.Fprintf(&b, "**Branch:** %s\n", ctx.Branch)

	if len(ctx.DirtyFiles) > 0 {
		b.WriteString("**Dirty files:**\n")
		for _, f := range ctx.DirtyFiles {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
	}

	if len(ctx.RecentCommits) > 0 {
		b.WriteString("**Recent commits:**\n")
		for _, c := range ctx.RecentCommits {
			fmt.Fprintf(&b, "- %s\n", c)
		}
	}

	if notes != "" {
		fmt.Fprintf(&b, "**Notes:** %s\n", notes)
	}

	return b.String()
}

// Park writes a context dump to the daily note.
func (c *Client) Park(project string, ctx *git.RepoContext, notes string) error {
	content := FormatPark(project, ctx, notes)
	_, err := c.runner.Run("daily:append", "content="+content)
	return err
}
