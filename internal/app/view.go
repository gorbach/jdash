package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	statusBarHeight := 1
	topPanelHeight := (m.height - statusBarHeight) * 2 / 3
	bottomPanelHeight := (m.height - statusBarHeight) - topPanelHeight
	leftPanelWidth := m.width / 2
	rightPanelWidth := m.width - leftPanelWidth

	jobsPanel := m.renderPanel(PanelJobs, m.jobsPanel.View(), leftPanelWidth, topPanelHeight)
	queuePanel := m.renderPanel(PanelQueue, m.queuePanel.View(), rightPanelWidth, topPanelHeight)
	topPanels := lipgloss.JoinHorizontal(lipgloss.Top, jobsPanel, queuePanel)

	bottomPanel := m.renderPanel(PanelBottom, m.bottom.View(), m.width, bottomPanelHeight)
	statusBarView := m.statusBar.View()

	baseContent := lipgloss.JoinVertical(
		lipgloss.Left,
		topPanels,
		bottomPanel,
		statusBarView,
	)

	if m.help.Active() {
		baseContent = m.renderHelpOverlay(baseContent)
	}

	if !m.modal.Active() {
		return baseContent
	}

	dimmed := dimContentStyle.Render(baseContent)
	baseView := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(dimmed)

	modalView := m.modal.View()
	if modalView == "" {
		return baseView
	}

	if m.width > 0 && m.height > 0 {
		modalView = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalView)
	}

	return overlayStrings(baseView, modalView)
}

func (m Model) renderPanel(id PanelID, content string, width, height int) string {
	borderColor := lipgloss.Color("8")
	if m.activePanel == id {
		borderColor = lipgloss.Color("10")
	}

	style := lipgloss.NewStyle().
		Width(width - 2).
		Height(height - 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	return style.Render(content)
}

func (m Model) renderHelpOverlay(baseContent string) string {
	if m.width <= 0 || m.height <= 0 {
		return baseContent
	}

	dimmed := dimContentStyle.Render(baseContent)
	baseView := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(dimmed)

	helpView := m.helpBoxView()
	if helpView == "" {
		return baseView
	}

	helpView = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpView)
	return overlayStrings(baseView, helpView)
}

func (m Model) helpBoxView() string {
	if m.help.viewport.Width <= 0 || m.help.viewport.Height <= 0 {
		return ""
	}

	body := lipgloss.NewStyle().
		Width(m.help.viewport.Width).
		Padding(1, 2).
		Render(m.help.viewport.View())

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("12")).
		Background(lipgloss.Color("235")).
		Width(m.help.viewport.Width + 4)

	return boxStyle.Render(body)
}

func (m Model) updateHelpViewportSize() Model {
	if m.width <= 0 || m.height <= 0 {
		return m
	}

	availableWidth := maxInt(m.width-4, 1)
	minWidth := minInt(helpViewportMinWidth, availableWidth)
	maxWidth := minInt(helpViewportMaxWidth, availableWidth)
	candidateWidth := m.width - 10
	width := clampInt(candidateWidth, minWidth, maxWidth)
	if width < 1 {
		width = availableWidth
	}

	availableHeight := maxInt(m.height-4, 1)
	height := clampInt(m.height/2, helpViewportMinHeight, availableHeight)

	overlay := m.help
	overlay.viewport.Width = width
	overlay.viewport.Height = height
	m.help = overlay

	return m
}

func overlayStrings(base, overlay string) string {
	if overlay == "" {
		return base
	}
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")
	maxLines := len(baseLines)
	if len(overlayLines) > maxLines {
		maxLines = len(overlayLines)
	}
	var builder strings.Builder
	for i := 0; i < maxLines; i++ {
		var baseLine, overlayLine string
		if i < len(baseLines) {
			baseLine = baseLines[i]
		}
		if i < len(overlayLines) {
			overlayLine = overlayLines[i]
		}
		if overlayLine == "" {
			builder.WriteString(baseLine)
		} else if baseLine == "" {
			builder.WriteString(overlayLine)
		} else {
			baseRunes := []rune(baseLine)
			overlayRunes := []rune(overlayLine)
			width := len(baseRunes)
			if len(overlayRunes) > width {
				width = len(overlayRunes)
			}
			merged := make([]rune, width)
			for j := 0; j < width; j++ {
				switch {
				case j < len(overlayRunes):
					merged[j] = overlayRunes[j]
				case j < len(baseRunes):
					merged[j] = baseRunes[j]
				default:
					merged[j] = ' '
				}
			}
			builder.WriteString(string(merged))
		}
		if i < maxLines-1 {
			builder.WriteRune('\n')
		}
	}
	return builder.String()
}
