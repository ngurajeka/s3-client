package ui

import (
	"github.com/charmbracelet/lipgloss"
)

type Dialog struct {
	Title   string
	Message string
	Width   int
	Height  int
}

func NewDialog(title, message string) *Dialog {
	return &Dialog{
		Title:   title,
		Message: message,
		Width:   60,
		Height:  10,
	}
}

func (d *Dialog) SetSize(width, height int) {
	d.Width = width
	d.Height = height
}

func (d *Dialog) View() string {
	title := HeaderStyle.Render(d.Title)
	content := lipgloss.NewStyle().Render(d.Message)

	lines := []string{
		title,
		"",
		content,
	}

	dialog := lipgloss.JoinVertical(lipgloss.Center, lines...)

	return DialogStyle.Width(d.Width).Height(d.Height).Render(dialog)
}

type ConfirmDialog struct {
	Title    string
	Message  string
	Prompt   string
	Width    int
	Height   int
	Selected int
}

func NewConfirmDialog(title, message, prompt string) *ConfirmDialog {
	return &ConfirmDialog{
		Title:    title,
		Message:  message,
		Prompt:   prompt,
		Width:    50,
		Height:   8,
		Selected: 0,
	}
}

func (d *ConfirmDialog) Toggle() {
	d.Selected = 1 - d.Selected
}

func (d *ConfirmDialog) View() string {
	title := HeaderStyle.Render(d.Title)
	content := lipgloss.NewStyle().Render(d.Message)

	yesStyle := ButtonStyle
	noStyle := ButtonStyle
	if d.Selected == 0 {
		yesStyle = ButtonFocusedStyle
	} else {
		noStyle = ButtonFocusedStyle
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center,
		yesStyle.Render("[ Yes ]"),
		noStyle.Render("[ No ]"),
	)

	prompt := lipgloss.NewStyle().Foreground(SubtleColor).Render(d.Prompt)

	lines := []string{
		title,
		"",
		content,
		"",
		prompt,
		buttons,
	}

	dialog := lipgloss.JoinVertical(lipgloss.Center, lines...)

	return DialogStyle.Width(d.Width).Height(d.Height).Render(dialog)
}

type InputDialog struct {
	Title   string
	Message string
	Prompt  string
	Value   string
	Width   int
	Height  int
}

func NewInputDialog(title, message, prompt string) *InputDialog {
	return &InputDialog{
		Title:   title,
		Message: message,
		Prompt:  prompt,
		Value:   "",
		Width:   50,
		Height:  10,
	}
}

func (d *InputDialog) SetValue(value string) {
	d.Value = value
}

func (d *InputDialog) View() string {
	title := HeaderStyle.Render(d.Title)
	content := lipgloss.NewStyle().Render(d.Message)

	inputLabel := lipgloss.NewStyle().Foreground(AccentColor).Render(d.Prompt)
	inputField := InputStyle.Width(d.Width - 4).Render(d.Value)

	lines := []string{
		title,
		"",
		content,
		"",
		inputLabel,
		inputField,
	}

	dialog := lipgloss.JoinVertical(lipgloss.Center, lines...)

	return DialogStyle.Width(d.Width).Height(d.Height).Render(dialog)
}

type ProgressDialog struct {
	Title   string
	Message string
	Percent float64
	Width   int
	Height  int
}

func NewProgressDialog(title, message string) *ProgressDialog {
	return &ProgressDialog{
		Title:   title,
		Message: message,
		Percent: 0,
		Width:   50,
		Height:  8,
	}
}

func (d *ProgressDialog) SetPercent(percent float64) {
	d.Percent = percent
}

func (d *ProgressDialog) View() string {
	title := HeaderStyle.Render(d.Title)
	content := lipgloss.NewStyle().Render(d.Message)

	barWidth := d.Width - 10
	bar := ProgressBar(barWidth, d.Percent)
	percentText := lipgloss.NewStyle().Render(d.PercentText())

	progressLine := lipgloss.NewStyle().
		Width(d.Width - 4).
		Render("[" + bar + "] " + percentText)

	lines := []string{
		title,
		"",
		content,
		"",
		progressLine,
	}

	dialog := lipgloss.JoinVertical(lipgloss.Center, lines...)

	return DialogStyle.Width(d.Width).Height(d.Height).Render(dialog)
}

func (d *ProgressDialog) PercentText() string {
	return "100%"
}

func PlaceOverlay(baseWidth, baseHeight int, overlay string) string {
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := lipgloss.Height(overlay)

	topPadding := (baseHeight - overlayHeight) / 2
	leftPadding := (baseWidth - overlayWidth) / 2

	if topPadding < 0 {
		topPadding = 0
	}
	if leftPadding < 0 {
		leftPadding = 0
	}

	return lipgloss.Place(baseWidth, baseHeight,
		lipgloss.Center, lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(SubtleColor),
	)
}
