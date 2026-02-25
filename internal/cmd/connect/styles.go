package connect

import "github.com/charmbracelet/lipgloss"

var (
	highlightColor = lipgloss.AdaptiveColor{Light: "#FF6B6B", Dark: "#FF6B6B"}
	subtleColor    = lipgloss.AdaptiveColor{Light: "#888888", Dark: "#666666"}
	accentColor    = lipgloss.AdaptiveColor{Light: "#4ECDC4", Dark: "#4ECDC4"}
	warningColor   = lipgloss.AdaptiveColor{Light: "#FFE66D", Dark: "#FFE66D"}

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	activePaneStyle = paneStyle.Copy().
			BorderForeground(highlightColor)

	headerStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true).
			MarginBottom(0)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	selectedItemStyle = itemStyle.Copy().
				Foreground(highlightColor).
				Bold(true).
				Reverse(true)

	dirStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	fileStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	helpStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			MarginTop(0)

	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(highlightColor).
			Padding(1, 2).
			Align(lipgloss.Center)

	progressBarStyle = lipgloss.NewStyle().
				Foreground(accentColor)

	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("142"))

	bottomPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.ThickBorder(), true, true, true, true).
				BorderForeground(lipgloss.Color("240")).
				Padding(0, 1)
)
