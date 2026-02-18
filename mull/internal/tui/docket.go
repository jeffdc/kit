package tui

import (
	"fmt"
	"strings"
)

func renderDocket(a *App) string {
	var b strings.Builder

	// Tab bar
	b.WriteString(renderTabBar(1))
	b.WriteString("\n\n")

	matters := a.docketMatters()

	if len(matters) == 0 {
		b.WriteString("  Docket is empty. Press d on a matter to add it.\n")
		b.WriteString("\n")
		b.WriteString(renderDocketStatusBar(a, 0))
		return b.String()
	}

	// Find docket notes for display
	noteMap := make(map[string]string)
	for _, e := range a.docket {
		if e.Note != "" {
			noteMap[e.ID] = e.Note
		}
	}

	// Column widths
	numW := 3
	idW := 6
	statusW := 9
	epicW := 14
	noteW := 16
	padding := numW + idW + statusW + epicW + noteW + 10
	titleW := a.width - padding
	if titleW < 10 {
		titleW = 10
	}

	// Header
	hdr := fmt.Sprintf("  %-*s %-*s %-*s %-*s %-*s %s",
		numW, "#",
		idW, "ID",
		titleW, "Title",
		statusW, "Status",
		epicW, "Epic",
		"Note",
	)
	b.WriteString(headerStyle.Render(hdr))
	b.WriteString("\n")

	// Rows
	maxRows := a.height - 5
	if maxRows < 1 {
		maxRows = len(matters)
	}

	offset := 0
	if a.docketCursor >= maxRows {
		offset = a.docketCursor - maxRows + 1
	}

	for i := offset; i < len(matters) && i-offset < maxRows; i++ {
		m := matters[i]
		title := m.Title
		if len(title) > titleW {
			title = title[:titleW-1] + "…"
		}
		note := truncate(noteMap[m.ID], noteW)

		row := fmt.Sprintf("  %-*d %-*s %-*s %-*s %-*s %s",
			numW, i+1,
			idW, m.ID,
			titleW, title,
			statusW, m.Status,
			epicW, truncate(m.Epic, epicW),
			note,
		)

		if i == a.docketCursor {
			b.WriteString(selectedRow.Width(a.width).Render(row))
		} else {
			styledStatus := statusStyle(m.Status).Render(fmt.Sprintf("%-*s", statusW, m.Status))
			row = fmt.Sprintf("  %-*d %-*s %-*s %s %-*s %s",
				numW, i+1,
				idW, m.ID,
				titleW, title,
				styledStatus,
				epicW, truncate(m.Epic, epicW),
				note,
			)
			b.WriteString(row)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(renderDocketStatusBar(a, len(matters)))

	return b.String()
}

func renderDocketStatusBar(a *App, count int) string {
	left := statusBarStyle.Render(fmt.Sprintf("  %d items on docket", count))
	if a.flash != "" {
		left += "  " + flashStyle.Render(a.flash)
	}
	right := statusBarStyle.Render("? help  q quit")
	gap := a.width - len(left) - len(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}
