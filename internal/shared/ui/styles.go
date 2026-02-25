package ui

import "github.com/charmbracelet/lipgloss"

var (
	HighlightColor = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	SubtleColor    = lipgloss.AdaptiveColor{Light: "#D9D9D9", Dark: "#383838"}
	AccentColor    = lipgloss.AdaptiveColor{Light: "#00BBFF", Dark: "#00BBFF"}
	SuccessColor   = lipgloss.AdaptiveColor{Light: "#22C55E", Dark: "#22C55E"}
	ErrorColor     = lipgloss.AdaptiveColor{Light: "#EF4444", Dark: "#EF4444"}
	WarningColor   = lipgloss.AdaptiveColor{Light: "#F59E0B", Dark: "#F59E0B"}

	PaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(SubtleColor).
			Padding(0, 1)

	ActivePaneStyle = PaneStyle.Copy().
			BorderForeground(HighlightColor)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true).
			MarginBottom(1)

	ItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	SelectedItemStyle = ItemStyle.Copy().
				Foreground(HighlightColor).
				Bold(true)

	DirStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	FileStyle = lipgloss.NewStyle().Foreground(SubtleColor)

	HelpStyle = lipgloss.NewStyle().
			Foreground(SubtleColor).
			MarginTop(1)

	DialogStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(HighlightColor).
			Padding(1, 2).
			Align(lipgloss.Center)

	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(AccentColor)

	TabStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, true, false, true).
			BorderForeground(SubtleColor).
			Padding(0, 2)

	ActiveTabStyle = TabStyle.Copy().
			BorderForeground(HighlightColor).
			Foreground(HighlightColor).
			Bold(true)

	TabWindowStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, true, true, true).
			BorderForeground(HighlightColor).
			Padding(0, 1)

	SpinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

	StatusSuccessStyle = lipgloss.NewStyle().Foreground(SuccessColor)
	StatusErrorStyle   = lipgloss.NewStyle().Foreground(ErrorColor)
	StatusWarningStyle = lipgloss.NewStyle().Foreground(WarningColor)

	InputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	InputFocusStyle = InputStyle.Copy().
			Border(lipgloss.NormalBorder()).
			BorderForeground(AccentColor)

	ButtonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Background(HighlightColor).
			Padding(0, 2).
			Margin(0, 1)

	ButtonFocusedStyle = ButtonStyle.Copy().
				Underline(true)

	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(AccentColor).
				Border(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(SubtleColor)

	TableRowStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, true).
			BorderForeground(SubtleColor)

	TableRowSelectedStyle = TableRowStyle.Copy().
				Foreground(HighlightColor).
				Background(lipgloss.Color("235"))
)

func ProgressBar(width int, percent float64) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := int(float64(width) * percent / 100)
	empty := width - filled

	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

var strings = struct {
	Repeat func(string, int) string
}{
	Repeat: func(s string, count int) string {
		result := ""
		for i := 0; i < count; i++ {
			result += s
		}
		return result
	},
}
