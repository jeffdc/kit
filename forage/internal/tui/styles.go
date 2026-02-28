package tui

import "github.com/charmbracelet/lipgloss"

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#CCCCCC"}).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#555555"})

	selectedRow = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "#DDDDFF", Dark: "#333366"}).
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})

	statusColors = map[string]lipgloss.Style{
		"wishlist": lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#2255AA", Dark: "#5588DD"}),
		"reading":  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#AA8800", Dark: "#DDAA00"}),
		"read":     lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#228833", Dark: "#55BB66"}),
		"dropped":  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#AA3333", Dark: "#DD5555"}),
	}

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#999999"})

	flashStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#228833", Dark: "#55BB66"}).
			Bold(true)

	filterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}).
			Italic(true)

	detailHeading = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})

	detailLabel = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#999999"}).
			Width(10)

	detailValue = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#CCCCCC"})

	helpBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#555555"}).
		Padding(1, 2)

	dimOverlay = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#555555"})
)

func statusStyle(status string) lipgloss.Style {
	if s, ok := statusColors[status]; ok {
		return s
	}
	return lipgloss.NewStyle()
}
