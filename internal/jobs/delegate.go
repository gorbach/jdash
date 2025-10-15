package jobs

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorbach/jdash/internal/ui"
	"github.com/gorbach/jdash/internal/utils"
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

	// Indentation based on level (skip for search results to align matches)
	indent := ""
	if !node.SearchResult {
		indent = getIndentation(node.Level)
	}

	// Expansion icon (only relevant in tree mode)
	var expandIcon string
	if !node.SearchResult {
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
	if node.SearchResult {
		name = node.FullName
	}
	if len(node.MatchIndexes) > 0 {
		name = renderHighlightedText(name, node.MatchIndexes)
	}

	// Metadata (status label, duration and timestamp for non-folders)
	var metadata string
	if node.Job != nil && !node.IsFolder {
		jobStatus := node.Job.GetStatus()
		statusStyle := ui.GetStatusStyle(jobStatus)
		statusLabel := statusStyle.Render(fmt.Sprintf("[%s]", jobStatus))

		if node.Job.LastBuild != nil {
			duration := utils.FormatDuration(node.Job.LastBuild.GetDuration())
			timestamp := utils.FormatRelativeTime(node.Job.LastBuild.GetTimestamp())
			metadata = fmt.Sprintf("  %s  %s  %s", statusLabel,
				ui.SubtleStyle.Render(duration),
				ui.SubtleStyle.Render(timestamp))
		} else {
			metadata = fmt.Sprintf("  %s  %s", statusLabel, ui.SubtleStyle.Render("never built"))
		}
	}

	// Combine parts
	var builder strings.Builder
	builder.WriteString(indent)
	builder.WriteString(expandIcon)
	builder.WriteString(status)
	builder.WriteString(" ")
	builder.WriteString(name)
	builder.WriteString(metadata)
	line := builder.String()

	// Apply selection style if this item is selected
	if index == m.Index() {
		line = ui.SelectedStyle.Render(line)
	}

	fmt.Fprint(w, line)
}

func renderHighlightedText(text string, indexes []int) string {
	if len(indexes) == 0 {
		return text
	}

	runes := []rune(text)
	highlight := make(map[int]struct{}, len(indexes))
	for _, idx := range indexes {
		highlight[idx] = struct{}{}
	}

	var b strings.Builder
	for i, r := range runes {
		ch := string(r)
		if _, ok := highlight[i]; ok {
			b.WriteString(ui.SearchHighlightStyle.Render(ch))
		} else {
			b.WriteString(ch)
		}
	}

	return b.String()
}
