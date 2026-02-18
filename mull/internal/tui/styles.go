package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Tab bar
	activeTab = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
			Background(lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#555555"}).
			Padding(0, 2)

	inactiveTab = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#999999"}).
			Padding(0, 2)

	// Table
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#CCCCCC"}).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#555555"})

	selectedRow = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "#DDDDFF", Dark: "#333366"}).
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})

	// Status badges
	statusColors = map[string]lipgloss.Style{
		"raw":     lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#999999"}),
		"refined": lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#AA8800", Dark: "#DDAA00"}),
		"planned": lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#2255AA", Dark: "#5588DD"}),
		"done":    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#228833", Dark: "#55BB66"}),
		"dropped": lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#AA3333", Dark: "#DD5555"}),
	}

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#999999"})

	flashStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#228833", Dark: "#55BB66"}).
			Bold(true)

	// Filter indicator
	filterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}).
			Italic(true)

	// Detail view
	detailHeading = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})

	detailLabel = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#999999"}).
			Width(10)

	detailValue = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#CCCCCC"})

	// Help overlay
	helpBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#555555"}).
		Padding(1, 2)

	helpHeading = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#CCCCCC"}).
			MarginBottom(1)

	dimOverlay = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#555555"})
)

func statusStyle(status string) lipgloss.Style {
	if s, ok := statusColors[status]; ok {
		return s
	}
	return lipgloss.NewStyle()
}
