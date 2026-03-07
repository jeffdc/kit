package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderList(a *App) string {
	var b strings.Builder

	// Filter line
	b.WriteString(renderFilterLine(a))
	b.WriteString("\n\n")

	books := a.filteredBooks()

	if len(books) == 0 {
		b.WriteString("  No books found.\n")
		b.WriteString("\n")
		b.WriteString(renderStatusBar(a, 0))
		return b.String()
	}

	// Column widths
	statusW := 10
	authorW := 20
	ratingW := 7
	padding := statusW + authorW + ratingW + 6
	titleW := a.width - padding
	if titleW < 10 {
		titleW = 10
	}

	// Header
	hdr := fmt.Sprintf("  %-*s %-*s %-*s %s",
		titleW, "Title",
		authorW, "Author",
		statusW, "Status",
		"Rating",
	)
	b.WriteString(headerStyle.Render(hdr))
	b.WriteString("\n")

	// Rows
	maxRows := a.height - 6
	if maxRows < 1 {
		maxRows = len(books)
	}

	offset := 0
	if a.cursor >= maxRows {
		offset = a.cursor - maxRows + 1
	}

	for i := offset; i < len(books) && i-offset < maxRows; i++ {
		bk := books[i]
		title := bk.Title
		if len(title) > titleW {
			title = title[:titleW-1] + "…"
		}
		author := truncate(bk.Author, authorW)
		rating := ""
		if bk.Rating > 0 {
			rating = strings.Repeat("★", bk.Rating)
		}

		if i == a.cursor {
			row := fmt.Sprintf("  %-*s %-*s %-*s %-*s",
				titleW, title,
				authorW, author,
				statusW, bk.Status,
				ratingW, rating,
			)
			b.WriteString(selectedRow.Width(a.width).Render(row))
		} else {
			styledStatus := statusStyle(bk.Status).Render(fmt.Sprintf("%-*s", statusW, bk.Status))
			row := fmt.Sprintf("  %-*s %-*s %s %-*s",
				titleW, title,
				authorW, author,
				styledStatus,
				ratingW, rating,
			)
			b.WriteString(row)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(renderStatusBar(a, len(books)))

	return b.String()
}

func renderFilterLine(a *App) string {
	var parts []string

	parts = append(parts, filterNames[a.filter])
	parts = append(parts, "sort="+sortLabels[a.sortMode])

	if a.searchQuery != "" {
		parts = append(parts, fmt.Sprintf("search=%q", a.searchQuery))
	}

	if a.searching {
		return "  " + a.searchInput.View()
	}

	return filterStyle.Render("  Filter: " + strings.Join(parts, ", "))
}

func renderStatusBar(a *App, count int) string {
	left := statusBarStyle.Render(fmt.Sprintf("  %d books", count))
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
