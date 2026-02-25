package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent   = lipgloss.Color("#7C6F64") // warm brown (gruvbox-ish)
	colorGreen    = lipgloss.Color("#98971A")
	colorYellow   = lipgloss.Color("#D79921")
	colorBlue     = lipgloss.Color("#458588")
	colorRed      = lipgloss.Color("#CC241D")
	colorSubtle   = lipgloss.Color("#665C54")
	colorSelected = lipgloss.Color("#3C3836")
	colorBG       = lipgloss.Color("#1D2021")
	colorFG       = lipgloss.Color("#EBDBB2")
	colorDim      = lipgloss.Color("#504945")

	styleTitle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	styleSubtitle = lipgloss.NewStyle().
			Foreground(colorSubtle)

	styleDivider = lipgloss.NewStyle().
			Foreground(colorDim)

	styleSelectedItem = lipgloss.NewStyle().
				Foreground(colorYellow).
				Bold(true)

	styleNormalItem = lipgloss.NewStyle().
			Foreground(colorFG)

	styleDimItem = lipgloss.NewStyle().
			Foreground(colorSubtle)

	styleTag = lipgloss.NewStyle().
			Foreground(colorBlue)

	styleError = lipgloss.NewStyle().
			Foreground(colorRed)

	styleSuccess = lipgloss.NewStyle().
			Foreground(colorGreen)

	styleHint = lipgloss.NewStyle().
			Foreground(colorSubtle)

	styleAILabel = lipgloss.NewStyle().
			Foreground(colorBlue).
			Bold(true)

	styleInputBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorAccent).
				Padding(0, 1)

	styleInputActive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorYellow).
				Padding(0, 1)

	stylePanelBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBlue).
				Padding(1, 2)

	styleConfirm = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)
)
