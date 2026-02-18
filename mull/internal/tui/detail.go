package tui

import (
	"fmt"
	"strings"

	"mull/internal/model"
)

func renderDetail(a *App) string {
	m := a.detailMatter
	if m == nil {
		return "No matter selected"
	}

	var b strings.Builder

	// Title
	b.WriteString(detailHeading.Render(m.Title))
	b.WriteString("\n\n")

	// Metadata
	writeMetaRow(&b, "Status", statusStyle(m.Status).Render(m.Status), "Epic", m.Epic)
	writeMetaRow(&b, "Effort", m.Effort, "Created", m.Created)

	if len(m.Tags) > 0 {
		writeMetaRow(&b, "Tags", strings.Join(m.Tags, ", "), "", "")
	}
	if len(m.Relates) > 0 || len(m.Blocks) > 0 {
		writeMetaRow(&b, "Relates", strings.Join(m.Relates, ", "), "Blocks", strings.Join(m.Blocks, ", "))
	}
	if len(m.Needs) > 0 || m.Parent != "" {
		writeMetaRow(&b, "Needs", strings.Join(m.Needs, ", "), "Parent", m.Parent)
	}
	if len(m.Docs) > 0 {
		writeMetaRow(&b, "Docs", strings.Join(m.Docs, ", "), "", "")
	}

	b.WriteString("\n")

	// Body in viewport
	if m.Body != "" {
		b.WriteString(a.viewport.View())
	}

	b.WriteString("\n")
	b.WriteString(renderDetailStatusBar(a))

	return b.String()
}

func writeMetaRow(b *strings.Builder, label1, value1, label2, value2 string) {
	left := detailLabel.Render(label1+":") + " " + detailValue.Render(value1)
	if label2 != "" && value2 != "" {
		right := detailLabel.Render(label2+":") + " " + detailValue.Render(value2)
		b.WriteString(fmt.Sprintf("  %-36s %s\n", left, right))
	} else {
		b.WriteString(fmt.Sprintf("  %s\n", left))
	}
}

func renderDetailStatusBar(a *App) string {
	m := a.detailMatter
	left := statusBarStyle.Render(fmt.Sprintf("  %s", matterRef(m)))
	if a.flash != "" {
		left += "  " + flashStyle.Render(a.flash)
	}
	right := statusBarStyle.Render("esc back  ? help  q quit")
	gap := a.width - len(left) - len(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func matterRef(m *model.Matter) string {
	if m == nil {
		return ""
	}
	return m.ID
}
