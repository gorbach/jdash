package ui

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
	case "SUCCESS":
		return IconSuccess
	case "FAILED":
		return IconFailed
	case "BUILDING":
		return IconBuilding
	case "UNSTABLE":
		return IconUnstable
	case "DISABLED":
		return IconDisabled
	case "ABORTED":
		return IconAborted
	case "PENDING", "NOT_BUILT", "NEVER_BUILT":
		return IconPending
	case "FOLDER":
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
