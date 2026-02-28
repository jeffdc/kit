package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderDetail(a *App) string {
	b := a.detailBook
	if b == nil {
		return "No book selected"
	}

	var s strings.Builder

	s.WriteString(detailHeading.Render(b.Title))
	s.WriteString("\n\n")

	writeMetaRow(&s, "Author", b.Author, "Status", statusStyle(b.Status).Render(b.Status))
	writeMetaRow(&s, "Added", b.DateAdded, "Read", b.DateRead)

	if len(b.Tags) > 0 {
		writeMetaRow(&s, "Tags", strings.Join(b.Tags, ", "), "", "")
	}
	if b.Rating > 0 {
		stars := strings.Repeat("★", b.Rating) + strings.Repeat("☆", 5-b.Rating)
		writeMetaRow(&s, "Rating", stars, "", "")
	}

	s.WriteString("\n")

	if b.Body != "" {
		s.WriteString(a.viewport.View())
	}

	s.WriteString("\n")
	s.WriteString(renderDetailStatusBar(a))

	return s.String()
}

func writeMetaRow(s *strings.Builder, label1, value1, label2, value2 string) {
	left := detailLabel.Render(label1+":") + " " + detailValue.Render(value1)
	if label2 != "" && value2 != "" {
		right := detailLabel.Render(label2+":") + " " + detailValue.Render(value2)
		s.WriteString(fmt.Sprintf("  %-36s %s\n", left, right))
	} else {
		s.WriteString(fmt.Sprintf("  %s\n", left))
	}
}

func renderDetailStatusBar(a *App) string {
	left := statusBarStyle.Render(fmt.Sprintf("  %s", a.detailBook.ID))
	if a.flash != "" {
		left += "  " + flashStyle.Render(a.flash)
	}
	right := statusBarStyle.Render("esc back  ? help  q quit")
	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}
