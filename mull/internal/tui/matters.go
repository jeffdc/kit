package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderMatters(a *App) string {
	var b strings.Builder

	// Tab bar
	b.WriteString(renderTabBar(0))
	b.WriteString("\n")

	// Filter indicator
	b.WriteString(renderFilterLine(a))
	b.WriteString("\n\n")

	matters := a.filteredMatters()

	if len(matters) == 0 {
		b.WriteString("  No matters found. Run `mull add` to create one.\n")
		b.WriteString("\n")
		b.WriteString(renderStatusBar(a, 0))
		return b.String()
	}

	// Column widths
	idW := 6
	statusW := 9
	epicW := 14
	effortW := 8
	padding := idW + statusW + epicW + effortW + 8 // separators
	titleW := a.width - padding
	if titleW < 10 {
		titleW = 10
	}

	// Header
	hdr := fmt.Sprintf("  %-*s %-*s %-*s %-*s %s",
		idW, "ID",
		titleW, "Title",
		statusW, "Status",
		epicW, "Epic",
		"Effort",
	)
	b.WriteString(headerStyle.Render(hdr))
	b.WriteString("\n")

	// Rows - show as many as fit
	maxRows := a.height - 6
	if maxRows < 1 {
		maxRows = len(matters)
	}

	// Scroll offset so cursor is always visible
	offset := 0
	if a.cursor >= maxRows {
		offset = a.cursor - maxRows + 1
	}

	for i := offset; i < len(matters) && i-offset < maxRows; i++ {
		m := matters[i]
		title := m.Title
		if len(title) > titleW {
			title = title[:titleW-1] + "…"
		}

		row := fmt.Sprintf("  %-*s %-*s %-*s %-*s %-*s",
			idW, m.ID,
			titleW, title,
			statusW, m.Status,
			epicW, truncate(m.Epic, epicW),
			effortW, m.Effort,
		)

		if i == a.cursor {
			b.WriteString(selectedRow.Width(a.width).Render(row))
		} else {
			// Color the status portion
			styledStatus := statusStyle(m.Status).Render(fmt.Sprintf("%-*s", statusW, m.Status))
			row = fmt.Sprintf("  %-*s %-*s %s %-*s %-*s",
				idW, m.ID,
				titleW, title,
				styledStatus,
				epicW, truncate(m.Epic, epicW),
				effortW, m.Effort,
			)
			b.WriteString(row)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(renderStatusBar(a, len(matters)))

	return b.String()
}

func renderTabBar(active int) string {
	tabs := []string{"1 Matters", "2 Docket"}
	var parts []string
	for i, t := range tabs {
		if i == active {
			parts = append(parts, activeTab.Render(t))
		} else {
			parts = append(parts, inactiveTab.Render(t))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func renderFilterLine(a *App) string {
	var parts []string

	switch a.filter {
	case filterOpen:
		parts = append(parts, "open")
	case filterClosed:
		parts = append(parts, "closed")
	case filterAll:
		parts = append(parts, "all")
	case filterStatus:
		parts = append(parts, a.statusFilter)
	}

	if a.epicFilter != "" {
		parts = append(parts, "epic="+a.epicFilter)
	}
	if a.searchQuery != "" {
		parts = append(parts, fmt.Sprintf("search=%q", a.searchQuery))
	}

	if a.searching {
		return "  " + a.searchInput.View()
	}

	return filterStyle.Render("  Filter: " + strings.Join(parts, ", "))
}

func renderStatusBar(a *App, count int) string {
	left := statusBarStyle.Render(fmt.Sprintf("  %d matters", count))
	if a.flash != "" {
		left += "  " + flashStyle.Render(a.flash)
	}
	right := statusBarStyle.Render("? help  q quit")
	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func truncate(s string, w int) string {
	if len(s) <= w {
		return s
	}
	if w <= 1 {
		return s[:w]
	}
	return s[:w-1] + "…"
}
