package ui

import "github.com/charmbracelet/lipgloss"

// Color definitions based on spec
var (
	// Status colors
	ColorSuccess  = lipgloss.Color("10") // Green
	ColorFailed   = lipgloss.Color("9")  // Red
	ColorBuilding = lipgloss.Color("11") // Yellow
	ColorDisabled = lipgloss.Color("8")  // Gray
	ColorUnstable = lipgloss.Color("11") // Yellow
	ColorAborted  = lipgloss.Color("8")  // Gray
	ColorPending  = lipgloss.Color("8")  // Gray

	// UI colors
	ColorBorder       = lipgloss.Color("8")  // Dim gray
	ColorBorderActive = lipgloss.Color("10") // Bright green
	ColorTitle        = lipgloss.Color("12") // Bright blue
	ColorSubtle       = lipgloss.Color("8")  // Dim gray
	ColorHighlight    = lipgloss.Color("14") // Bright cyan
)

// Status styles
var (
	SuccessStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	FailedStyle  = lipgloss.NewStyle().Foreground(ColorFailed)
	BuildingStyle = lipgloss.NewStyle().Foreground(ColorBuilding)
	DisabledStyle = lipgloss.NewStyle().Foreground(ColorDisabled)
	UnstableStyle = lipgloss.NewStyle().Foreground(ColorUnstable)
	AbortedStyle  = lipgloss.NewStyle().Foreground(ColorAborted)
	PendingStyle  = lipgloss.NewStyle().Foreground(ColorPending)
)

// UI component styles
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorTitle)

	SubtleStyle = lipgloss.NewStyle().
			Foreground(ColorSubtle)

	HighlightStyle = lipgloss.NewStyle().
			Foreground(ColorHighlight).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorFailed).
			Bold(true)

	SelectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Bold(true)
)

// GetStatusStyle returns the appropriate style for a given status
func GetStatusStyle(status string) lipgloss.Style {
	switch status {
	case "SUCCESS":
		return SuccessStyle
	case "FAILED":
		return FailedStyle
	case "BUILDING":
		return BuildingStyle
	case "UNSTABLE":
		return UnstableStyle
	case "DISABLED":
		return DisabledStyle
	case "ABORTED":
		return AbortedStyle
	case "PENDING", "NOT_BUILT", "NEVER_BUILT":
		return PendingStyle
	default:
		return SubtleStyle
	}
}
