package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorbach/jenkins-gotui/internal/console"
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
	PanelBottom
)

type modalType int

const (
	modalNone modalType = iota
	modalParameters
)

type bottomView int

const (
	bottomViewDetails bottomView = iota
	bottomViewConsole
)

var (
	dimContentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

const (
	helpViewportMinWidth  = 30
	helpViewportMaxWidth  = 60
	helpViewportMinHeight = 12
)

const helpContent = `Key Bindings

Global
  q        quit application
  r        refresh all data
  ?        toggle this help
  Tab      next panel
  1-3      jump to panel

Jobs List (Panel 1)
  Up/k     move up
  Down/j   move down
  Left/h   collapse node
  Right/l  expand node
  Space    toggle expand
  Enter    view details
  g/G      top/bottom
  /        search
  b        build now

Build Info (Panel 3)
  b        build now / configure
  l        view logs
  p        parameters (if available)
  c        view config
  r        refresh details
  H        build history
  a        abort running build

[Press ? or Esc to close]
`

type consoleTargetResolvedMsg struct {
	JobFullName string
	BuildNumber int
	BuildURL    string
	Err         error
}

// Model is the root application model
type Model struct {
	activePanel  PanelID
	jobsPanel    jobs.Model
	queuePanel   queue.Model
	detailsPanel details.Model
	consolePanel console.Model
	statusBar    statusbar.Model
	width        int
	height       int
	serverURL    string
	client       *jenkins.Client
	helpVisible  bool
	helpViewport viewport.Model
	modal        tea.Model
	modalKind    modalType
	bottomView   bottomView
	// console target tracking for async build resolution
	consoleTargetFullName string
	consoleTargetJobName  string
	consoleTargetBuildURL string
	consoleTargetNumber   int
}

// New creates a new application model
func New(serverURL string, client *jenkins.Client) Model {
	helpVP := viewport.New(0, 0)
	helpVP.SetContent(helpContent)

	return Model{
		activePanel:  PanelJobs,
		bottomView:   bottomViewDetails,
		jobsPanel:    jobs.New(client),
		queuePanel:   queue.New(client),
		detailsPanel: details.New(client),
		consolePanel: console.New(client),
		statusBar:    statusbar.New(serverURL),
		helpViewport: helpVP,
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
		m.consolePanel.Init(),
		m.statusBar.Init(),
		m.helpViewport.Init(),
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

	case tea.MouseMsg:
		if m.helpVisible {
			var helpCmd tea.Cmd
			m.helpViewport, helpCmd = m.helpViewport.Update(msg)
			if helpCmd != nil {
				cmds = append(cmds, helpCmd)
			}
			return m, tea.Batch(cmds...)
		}

	case tea.KeyMsg:
		key := msg.String()

		// Avoid forwarding commands while help overlay is active.
		if key == "?" {
			if m.helpVisible {
				m.helpVisible = false
			} else {
				m.helpVisible = true
				m.helpViewport.GotoTop()
			}
			return m, nil
		}

		if m.helpVisible {
			switch key {
			case "esc":
				m.helpVisible = false
			case "ctrl+c", "q":
				return m, tea.Quit
			default:
				var helpCmd tea.Cmd
				m.helpViewport, helpCmd = m.helpViewport.Update(msg)
				if helpCmd != nil {
					cmds = append(cmds, helpCmd)
				}
			}
			return m, tea.Batch(cmds...)
		}

		if m.modalActive() {
			if key == "ctrl+c" || key == "q" {
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

	case console.ExitRequestedMsg:
		var cmd tea.Cmd
		m, cmd = m.handleConsoleExit()
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case consoleTargetResolvedMsg:
		var cmd tea.Cmd
		m, cmd = m.handleConsoleTargetResolved(msg)
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
	m = m.updateHelpViewportSize()

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
		Width:  dims.bottomWidth,
		Height: dims.bottomHeight,
	})
	cmds = append(cmds, cmd)

	m.consolePanel, cmd = m.consolePanel.Update(tea.WindowSizeMsg{
		Width:  dims.bottomWidth,
		Height: dims.bottomHeight,
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
	jobsWidth, jobsHeight     int
	queueWidth, queueHeight   int
	bottomWidth, bottomHeight int
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
		jobsWidth:    leftPanelWidth - 4,
		jobsHeight:   topPanelHeight - 4,
		queueWidth:   rightPanelWidth - 4,
		queueHeight:  topPanelHeight - 4,
		bottomWidth:  m.width - 4,
		bottomHeight: bottomPanelHeight - 4,
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
		m.activePanel = PanelBottom
		return true, m, nil

	case "r":
		refreshModel, refreshCmd := m.startGlobalRefresh()
		return true, refreshModel, refreshCmd
	}

	return false, m, nil // Not handled, continue routing
}

func (m Model) startGlobalRefresh() (Model, tea.Cmd) {
	var cmds []tea.Cmd

	var cmd tea.Cmd
	m.statusBar, cmd = m.statusBar.Update(statusbar.RefreshStartedMsg{})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.jobsPanel, cmd = m.jobsPanel.Update(jobs.RefreshRequestedMsg{})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.queuePanel, cmd = m.queuePanel.Update(queue.RefreshRequestedMsg{})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	switch m.bottomView {
	case bottomViewConsole:
		m.consolePanel, cmd = m.consolePanel.Update(console.RefreshRequestedMsg{})
	default:
		m.detailsPanel, cmd = m.detailsPanel.Update(details.RefreshRequestedMsg{})
	}
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// routeKeyToActivePanel forwards keyboard input to the currently focused panel
func (m Model) routeKeyToActivePanel(msg tea.KeyMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.activePanel {
	case PanelJobs:
		m.jobsPanel, cmd = m.jobsPanel.Update(msg)
	case PanelQueue:
		m.queuePanel, cmd = m.queuePanel.Update(msg)
	case PanelBottom:
		return m.updateBottomPanel(msg)
	}

	return m, cmd
}

func (m Model) updateBottomPanel(msg tea.Msg) (Model, tea.Cmd) {
	switch m.bottomView {
	case bottomViewConsole:
		var cmd tea.Cmd
		m.consolePanel, cmd = m.consolePanel.Update(msg)
		return m, cmd
	default:
		var cmd tea.Cmd
		m.detailsPanel, cmd = m.detailsPanel.Update(msg)
		return m, cmd
	}
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

	m.consolePanel, cmd = m.consolePanel.Update(msg)
	cmds = append(cmds, cmd)

	m.statusBar, cmd = m.statusBar.Update(msg)
	cmds = append(cmds, cmd)

	switch t := msg.(type) {
	case jobs.JobsFetchedMsg:
		m.statusBar, cmd = m.statusBar.Update(statusbar.RefreshFinishedMsg{
			JobCount: len(t.Jobs),
		})
		cmds = append(cmds, cmd)

	case jobs.JobsErrorMsg:
		m.statusBar, cmd = m.statusBar.Update(statusbar.RefreshFinishedMsg{
			JobCount: -1,
			Err:      t.Err,
		})
		cmds = append(cmds, cmd)
	}

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

	// Render bottom panel (details or console)
	bottomPanel := m.renderPanel(PanelBottom, m.bottomPanelView(), m.width, bottomPanelHeight)

	// Render status bar
	statusBarView := m.statusBar.View()

	baseContent := lipgloss.JoinVertical(
		lipgloss.Left,
		topPanels,
		bottomPanel,
		statusBarView,
	)

	if m.helpVisible {
		return m.renderHelpOverlay(baseContent)
	}

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

func (m Model) bottomPanelView() string {
	switch m.bottomView {
	case bottomViewConsole:
		return m.consolePanel.View()
	default:
		return m.detailsPanel.View()
	}
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
	case details.ActionKindViewLogs:
		return m.openConsoleView(msg)
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
	if m.helpViewport.Width <= 0 || m.helpViewport.Height <= 0 {
		return ""
	}

	body := lipgloss.NewStyle().
		Width(m.helpViewport.Width).
		Padding(1, 2).
		Render(m.helpViewport.View())

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("12")).
		Background(lipgloss.Color("235")).
		Width(m.helpViewport.Width + 4)

	return boxStyle.Render(body)
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

func (m Model) openConsoleView(req details.ActionRequestMsg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.bottomView != bottomViewConsole {
		m.bottomView = bottomViewConsole
	}

	jobName := req.Job.Name
	if jobName == "" {
		jobName = req.Job.FullName
	}
	m.consoleTargetJobName = jobName
	m.consoleTargetFullName = req.Job.FullName

	buildNumber := 0
	if req.Build != nil && req.Build.Number > 0 {
		buildNumber = req.Build.Number
	} else if req.Job.LastBuild != nil && req.Job.LastBuild.Number > 0 {
		buildNumber = req.Job.LastBuild.Number
	}

	buildURL := ""
	if req.Build != nil && req.Build.URL != "" {
		buildURL = req.Build.URL
	} else if req.Job.LastBuild != nil && req.Job.LastBuild.URL != "" {
		buildURL = req.Job.LastBuild.URL
	}

	if buildURL == "" && req.Job.URL != "" && buildNumber > 0 {
		trimmed := strings.TrimSuffix(req.Job.URL, "/")
		buildURL = fmt.Sprintf("%s/%d/", trimmed, buildNumber)
	}

	m.consoleTargetNumber = buildNumber
	m.consoleTargetBuildURL = buildURL

	openMsg := console.OpenRequestMsg{
		JobName:     jobName,
		JobFullName: req.Job.FullName,
		BuildNumber: buildNumber,
		BuildURL:    buildURL,
	}

	var consoleCmd tea.Cmd
	m.consolePanel, consoleCmd = m.consolePanel.Update(openMsg)
	if consoleCmd != nil {
		cmds = append(cmds, consoleCmd)
	}

	if m.width > 0 && m.height > 0 {
		dims := m.calculatePanelDimensions()
		sizeMsg := tea.WindowSizeMsg{
			Width:  dims.bottomWidth,
			Height: dims.bottomHeight,
		}
		var sizeCmd tea.Cmd
		m.consolePanel, sizeCmd = m.consolePanel.Update(sizeMsg)
		if sizeCmd != nil {
			cmds = append(cmds, sizeCmd)
		}
	}

	m.activePanel = PanelBottom
	if resolveCmd := resolveConsoleTargetCmd(m.client, req.Job.FullName); resolveCmd != nil {
		cmds = append(cmds, resolveCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) clearModal() Model {
	m.modal = nil
	m.modalKind = modalNone
	return m
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
		width = 1
	}

	availableHeight := maxInt(m.height-4, 3)
	minHeight := minInt(helpViewportMinHeight, availableHeight)
	candidateHeight := m.height - 6
	height := clampInt(candidateHeight, minHeight, availableHeight)
	if height < 3 {
		height = 3
	}

	m.helpViewport.Width = width
	m.helpViewport.Height = height

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

func clampInt(v, min, max int) int {
	if max < min {
		max = min
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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

func (m Model) showDetailsPanel() (Model, tea.Cmd) {
	if m.bottomView == bottomViewDetails {
		return m, nil
	}

	var cmds []tea.Cmd
	var consoleCmd tea.Cmd
	m.consolePanel, consoleCmd = m.consolePanel.Update(console.DeactivateMsg{})
	if consoleCmd != nil {
		cmds = append(cmds, consoleCmd)
	}

	m.bottomView = bottomViewDetails
	return m, tea.Batch(cmds...)
}

func (m Model) handleConsoleExit() (Model, tea.Cmd) {
	var cmd tea.Cmd
	m, cmd = m.showDetailsPanel()
	m.activePanel = PanelBottom
	return m, cmd
}

func (m Model) handleConsoleTargetResolved(msg consoleTargetResolvedMsg) (Model, tea.Cmd) {
	if msg.JobFullName == "" {
		return m, nil
	}
	if msg.Err != nil {
		return m, nil
	}
	if msg.JobFullName != m.consoleTargetFullName {
		return m, nil
	}

	number := msg.BuildNumber
	url := strings.TrimSpace(msg.BuildURL)

	if number <= 0 && url == "" {
		return m, nil
	}

	if number <= 0 {
		number = m.consoleTargetNumber
	}
	if url == "" {
		url = m.consoleTargetBuildURL
	}

	if number == m.consoleTargetNumber && url == m.consoleTargetBuildURL {
		return m, nil
	}

	m.consoleTargetNumber = number
	m.consoleTargetBuildURL = url

	openMsg := console.OpenRequestMsg{
		JobName:     m.consoleTargetJobName,
		JobFullName: msg.JobFullName,
		BuildNumber: number,
		BuildURL:    url,
	}

	var cmd tea.Cmd
	m.consolePanel, cmd = m.consolePanel.Update(openMsg)
	return m, cmd
}

func resolveConsoleTargetCmd(client *jenkins.Client, jobFullName string) tea.Cmd {
	if client == nil || jobFullName == "" {
		return nil
	}
	return func() tea.Msg {
		build, err := client.GetBuild(jobFullName, -1)
		if err != nil {
			return consoleTargetResolvedMsg{
				JobFullName: jobFullName,
				Err:         err,
			}
		}
		msg := consoleTargetResolvedMsg{
			JobFullName: jobFullName,
		}
		if build != nil {
			msg.BuildNumber = build.Number
			msg.BuildURL = build.URL
		}
		return msg
	}
}
