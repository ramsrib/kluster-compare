package tui

import "github.com/charmbracelet/lipgloss"

var (
	StyleApp = lipgloss.NewStyle().Padding(1, 2)

	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			MarginBottom(1)

	StyleSubHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	StyleTableHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")).
				Underline(true)

	StyleSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("12"))

	StyleEqual = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	StyleChanged = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3"))

	StyleOnlyLeft = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1"))

	StyleOnlyRight = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))

	StyleDiffAdded = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))

	StyleDiffRemoved = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1"))

	StyleDiffHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)

	StyleDiffContext = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	StyleError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)

	StyleHelp = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			MarginTop(1)

	StyleStatusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("8")).
			Padding(0, 1)
)
