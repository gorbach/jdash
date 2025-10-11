package jobs

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorbach/jenkins-gotui/internal/ui"
	"github.com/gorbach/jenkins-gotui/internal/utils"
)

// jobDelegate implements list.ItemDelegate for rendering JobTree nodes
type jobDelegate struct{}

func newJobDelegate() jobDelegate {
	return jobDelegate{}
}

// Height returns the height of each item (1 line per job)
func (d jobDelegate) Height() int {
	return 1
}

// Spacing returns the spacing between items
func (d jobDelegate) Spacing() int {
	return 0
}

// Update handles delegate-specific updates
func (d jobDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

// Render renders a single job tree node
func (d jobDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	node, ok := item.(JobTree)
	if !ok {
		return
	}

	// Indentation based on level
	indent := getIndentation(node.Level)

	// Expansion icon
	var expandIcon string
	if node.IsFolder {
		if node.Expanded {
			expandIcon = ui.IconExpanded
		} else {
			expandIcon = ui.IconCollapsed
		}
		expandIcon += " "
	} else {
		expandIcon = "  "
	}

	// Status icon and styling
	var status string
	if node.Job != nil {
		jobStatus := node.Job.GetStatus()
		icon := ui.GetStatusIcon(jobStatus)
		statusStyle := ui.GetStatusStyle(jobStatus)

		if node.IsFolder {
			status = ui.SubtleStyle.Render(icon)
		} else {
			status = statusStyle.Render(icon)
		}
	} else {
		status = ui.SubtleStyle.Render(ui.IconFolder)
	}

	// Job name
	name := node.Name

	// Metadata (duration and timestamp for non-folders)
	var metadata string
	if node.Job != nil && !node.IsFolder {
		if node.Job.LastBuild != nil {
			duration := utils.FormatDuration(node.Job.LastBuild.GetDuration())
			timestamp := utils.FormatRelativeTime(node.Job.LastBuild.GetTimestamp())
			metadata = ui.SubtleStyle.Render(fmt.Sprintf("  %s  %s", duration, timestamp))
		} else {
			metadata = ui.SubtleStyle.Render("  never built")
		}
	}

	// Combine parts
	line := indent + expandIcon + status + " " + name + metadata

	// Apply selection style if this item is selected
	if index == m.Index() {
		line = ui.SelectedStyle.Render(line)
	}

	fmt.Fprint(w, line)
}
