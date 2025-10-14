package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorbach/jenkins-gotui/internal/details"
	"github.com/gorbach/jenkins-gotui/internal/jenkins"
	"github.com/gorbach/jenkins-gotui/internal/jobs"
	"github.com/gorbach/jenkins-gotui/internal/parameters"
	"github.com/gorbach/jenkins-gotui/internal/queue"
	"github.com/gorbach/jenkins-gotui/internal/statusbar"
)

// PanelID represents which panel is active
type PanelID int

const (
	PanelJobs PanelID = iota
	PanelQueue
	PanelDetails
)

type modalType int

const (
	modalNone modalType = iota
	modalParameters
)

var (
	dimContentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// Model is the root application model
type Model struct {
	activePanel  PanelID
	jobsPanel    jobs.Model
	queuePanel   queue.Model
	detailsPanel details.Model
	statusBar    statusbar.Model
	width        int
	height       int
	serverURL    string
	client       *jenkins.Client
	modal        tea.Model
	modalKind    modalType
}

// New creates a new application model
func New(serverURL string, client *jenkins.Client) Model {
	return Model{
		activePanel:  PanelJobs,
		jobsPanel:    jobs.New(client),
		queuePanel:   queue.New(client),
		detailsPanel: details.New(client),
		statusBar:    statusbar.New(serverURL),
		serverURL:    serverURL,
		client:       client,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.jobsPanel.Init(),
		m.queuePanel.Init(),
		m.detailsPanel.Init(),
		m.statusBar.Init(),
	)
}

// Update handles messages following TEA message routing patterns:
// 1. Handle global keys in root model (quit, panel switching)
// 2. Broadcast WindowSizeMsg to all children with panel-specific dimensions
// 3. Route KeyMsg to active panel only
// 4. Broadcast other messages to all children (custom messages, ticks, etc.)
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		var resizeCmd tea.Cmd
		m, resizeCmd = m.handleWindowResize(msg)
		if resizeCmd != nil {
			cmds = append(cmds, resizeCmd)
		}
		if m.modalActive() {
			var modalCmd tea.Cmd
			m, modalCmd = m.forwardModal(msg)
			if modalCmd != nil {
				cmds = append(cmds, modalCmd)
			}
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		if m.modalActive() {
			if msg.String() == "ctrl+c" || msg.String() == "q" {
				return m, tea.Quit
			}
			var modalCmd tea.Cmd
			m, modalCmd = m.forwardModal(msg)
			if modalCmd != nil {
				cmds = append(cmds, modalCmd)
			}
			return m, tea.Batch(cmds...)
		}

		if handled, newModel, cmd := m.handleGlobalKeys(msg); handled {
			return newModel, cmd
		}
		return m.routeKeyToActivePanel(msg)

	case details.ActionRequestMsg:
		var cmd tea.Cmd
		m, cmd = m.handleActionRequest(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case parameters.SubmittedMsg:
		var cmd tea.Cmd
		m, cmd = m.handleParameterSubmit(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case parameters.CancelledMsg:
		var cmd tea.Cmd
		m, cmd = m.handleParameterCancel(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	if m.modalActive() {
		var modalCmd tea.Cmd
		m, modalCmd = m.forwardModal(msg)
		if modalCmd != nil {
			cmds = append(cmds, modalCmd)
		}
	}

	var broadcastCmd tea.Cmd
	m, broadcastCmd = m.broadcastToAllPanels(msg)
	if broadcastCmd != nil {
		cmds = append(cmds, broadcastCmd)
	}

	return m, tea.Batch(cmds...)
}

// handleWindowResize updates dimensions and broadcasts to all panels
func (m Model) handleWindowResize(msg tea.WindowSizeMsg) (Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height

	// Calculate dimensions once
	dims := m.calculatePanelDimensions()

	var cmds []tea.Cmd
	var cmd tea.Cmd

	// Send panel-specific dimensions to each child
	m.jobsPanel, cmd = m.jobsPanel.Update(tea.WindowSizeMsg{
		Width:  dims.jobsWidth,
		Height: dims.jobsHeight,
	})
	cmds = append(cmds, cmd)

	m.queuePanel, cmd = m.queuePanel.Update(tea.WindowSizeMsg{
		Width:  dims.queueWidth,
		Height: dims.queueHeight,
	})
	cmds = append(cmds, cmd)

	m.detailsPanel, cmd = m.detailsPanel.Update(tea.WindowSizeMsg{
		Width:  dims.detailsWidth,
		Height: dims.detailsHeight,
	})
	cmds = append(cmds, cmd)

	m.statusBar, cmd = m.statusBar.Update(tea.WindowSizeMsg{
		Width: msg.Width,
	})
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// panelDimensions holds calculated dimensions for all panels
type panelDimensions struct {
	jobsWidth, jobsHeight       int
	queueWidth, queueHeight     int
	detailsWidth, detailsHeight int
}

// calculatePanelDimensions computes dimensions for all panels based on terminal size
func (m Model) calculatePanelDimensions() panelDimensions {
	statusBarHeight := 1
	topPanelHeight := (m.height - statusBarHeight) * 2 / 3
	bottomPanelHeight := (m.height - statusBarHeight) - topPanelHeight
	leftPanelWidth := m.width / 2
	rightPanelWidth := m.width - leftPanelWidth

	// Account for borders (2px per side) and padding
	return panelDimensions{
		jobsWidth:     leftPanelWidth - 4,
		jobsHeight:    topPanelHeight - 4,
		queueWidth:    rightPanelWidth - 4,
		queueHeight:   topPanelHeight - 4,
		detailsWidth:  m.width - 4,
		detailsHeight: bottomPanelHeight - 4,
	}
}

// handleGlobalKeys processes global navigation keys (quit, panel switching)
// Returns (handled, model, cmd) - handled indicates if the key was processed
func (m Model) handleGlobalKeys(msg tea.KeyMsg) (bool, Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return true, m, tea.Quit

	case "tab":
		m.activePanel = (m.activePanel + 1) % 3
		return true, m, nil

	case "shift+tab":
		m.activePanel = (m.activePanel - 1 + 3) % 3
		return true, m, nil

	case "1":
		m.activePanel = PanelJobs
		return true, m, nil

	case "2":
		m.activePanel = PanelQueue
		return true, m, nil

	case "3":
		m.activePanel = PanelDetails
		return true, m, nil

	case "?":
		// TODO: Show help overlay
		return true, m, nil
	}

	return false, m, nil // Not handled, continue routing
}

// routeKeyToActivePanel forwards keyboard input to the currently focused panel
func (m Model) routeKeyToActivePanel(msg tea.KeyMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.activePanel {
	case PanelJobs:
		m.jobsPanel, cmd = m.jobsPanel.Update(msg)
	case PanelQueue:
		m.queuePanel, cmd = m.queuePanel.Update(msg)
	case PanelDetails:
		m.detailsPanel, cmd = m.detailsPanel.Update(msg)
	}

	return m, cmd
}

// broadcastToAllPanels sends messages that all panels need (custom messages, spinner ticks, etc.)
func (m Model) broadcastToAllPanels(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	// All panels receive custom messages (loading states, data fetched, etc.)
	m.jobsPanel, cmd = m.jobsPanel.Update(msg)
	cmds = append(cmds, cmd)

	m.queuePanel, cmd = m.queuePanel.Update(msg)
	cmds = append(cmds, cmd)

	m.detailsPanel, cmd = m.detailsPanel.Update(msg)
	cmds = append(cmds, cmd)

	m.statusBar, cmd = m.statusBar.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Calculate dimensions
	statusBarHeight := 1
	topPanelHeight := (m.height - statusBarHeight) * 2 / 3
	bottomPanelHeight := (m.height - statusBarHeight) - topPanelHeight
	leftPanelWidth := m.width / 2
	rightPanelWidth := m.width - leftPanelWidth

	// Render top panels (jobs + queue)
	jobsPanel := m.renderPanel(PanelJobs, m.jobsPanel.View(), leftPanelWidth, topPanelHeight)
	queuePanel := m.renderPanel(PanelQueue, m.queuePanel.View(), rightPanelWidth, topPanelHeight)
	topPanels := lipgloss.JoinHorizontal(lipgloss.Top, jobsPanel, queuePanel)

	// Render bottom panel (details)
	bottomPanel := m.renderPanel(PanelDetails, m.detailsPanel.View(), m.width, bottomPanelHeight)

	// Render status bar
	statusBarView := m.statusBar.View()

	baseContent := lipgloss.JoinVertical(
		lipgloss.Left,
		topPanels,
		bottomPanel,
		statusBarView,
	)

	if !m.modalActive() {
		return baseContent
	}

	dimmed := dimContentStyle.Render(baseContent)
	baseView := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(dimmed)

	modalView := ""
	if m.modal != nil {
		modalView = m.modal.View()
	}
	if modalView == "" {
		return baseView
	}

	if m.width > 0 && m.height > 0 {
		modalView = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalView)
	}

	return overlayStrings(baseView, modalView)
}

// renderPanel renders a panel with borders and focus highlighting
func (m Model) renderPanel(id PanelID, content string, width, height int) string {
	borderColor := lipgloss.Color("8") // Dim gray
	if m.activePanel == id {
		borderColor = lipgloss.Color("10") // Bright green for active
	}

	style := lipgloss.NewStyle().
		Width(width-2).   // Account for border
		Height(height-2). // Account for border
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	return style.Render(content)
}

func (m Model) modalActive() bool {
	return m.modalKind != modalNone && m.modal != nil
}

func (m Model) forwardModal(msg tea.Msg) (Model, tea.Cmd) {
	if !m.modalActive() {
		return m, nil
	}
	updated, cmd := m.modal.Update(msg)
	m.modal = updated
	return m, cmd
}

func (m Model) handleActionRequest(msg details.ActionRequestMsg) (Model, tea.Cmd) {
	switch msg.Kind {
	case details.ActionKindViewParameters:
		return m.openParametersModal(msg)
	default:
		return m.broadcastToAllPanels(msg)
	}
}

func (m Model) handleParameterSubmit(msg parameters.SubmittedMsg) (Model, tea.Cmd) {
	m = m.clearModal()
	submission := details.ParameterSubmissionMsg{
		JobFullName: msg.JobFullName,
		JobName:     msg.JobName,
		Values:      cloneParameterValues(msg.Values),
	}
	return m.broadcastToAllPanels(submission)
}

func (m Model) handleParameterCancel(msg parameters.CancelledMsg) (Model, tea.Cmd) {
	m = m.clearModal()
	return m.broadcastToAllPanels(details.ParameterCancelledMsg{JobFullName: msg.JobFullName})
}

func (m Model) openParametersModal(req details.ActionRequestMsg) (Model, tea.Cmd) {
	if len(req.ParameterDefinitions) == 0 {
		return m, nil
	}

	m = m.clearModal()
	modal := parameters.New(req.Job.Name, req.Job.FullName, req.ParameterDefinitions)
	initCmd := modal.Init()
	m.modal = modal
	m.modalKind = modalParameters

	var cmds []tea.Cmd
	if initCmd != nil {
		cmds = append(cmds, initCmd)
	}
	if m.width > 0 && m.height > 0 {
		var sizeCmd tea.Cmd
		m, sizeCmd = m.forwardModal(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		if sizeCmd != nil {
			cmds = append(cmds, sizeCmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) clearModal() Model {
	m.modal = nil
	m.modalKind = modalNone
	return m
}

func cloneParameterValues(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
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
