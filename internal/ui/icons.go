package ui

import "github.com/gorbach/jdash/internal/jenkins"

// Status icons as specified in docs2.md
const (
	IconSuccess  = "✓"
	IconFailed   = "✗"
	IconBuilding = "⟳"
	IconPending  = "○"
	IconDisabled = "-"
	IconUnstable = "⚠"
	IconAborted  = "◯"
	IconFolder   = "📁"

	// Tree expansion icons
	IconExpanded  = "▼"
	IconCollapsed = "▶"
)

// GetStatusIcon returns the appropriate icon for a given status
func GetStatusIcon(status string) string {
	switch status {
	case jenkins.StatusSuccess:
		return IconSuccess
	case jenkins.StatusFailed:
		return IconFailed
	case jenkins.StatusBuilding:
		return IconBuilding
	case jenkins.StatusUnstable:
		return IconUnstable
	case jenkins.StatusDisabled:
		return IconDisabled
	case jenkins.StatusAborted:
		return IconAborted
	case jenkins.StatusPending, jenkins.StatusNotBuilt, jenkins.StatusNeverBuilt:
		return IconPending
	case jenkins.StatusFolder:
		return IconFolder
	default:
		return IconPending
	}
}

// GetStatusText returns a formatted status text with icon and color
func GetStatusText(status string) string {
	icon := GetStatusIcon(status)
	style := GetStatusStyle(status)
	return style.Render(icon + " " + status)
}
