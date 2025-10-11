package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorbach/jenkins-gotui/internal/details"
	"github.com/gorbach/jenkins-gotui/internal/jenkins"
	"github.com/gorbach/jenkins-gotui/internal/jobs"
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
}

// New creates a new application model
func New(serverURL string, client *jenkins.Client) Model {
	return Model{
		activePanel:  PanelJobs,
		jobsPanel:    jobs.New(client),
		queuePanel:   queue.New(),
		detailsPanel: details.New(),
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

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate panel dimensions
		statusBarHeight := 1
		topPanelHeight := (m.height - statusBarHeight) * 2 / 3
		bottomPanelHeight := (m.height - statusBarHeight) - topPanelHeight
		leftPanelWidth := m.width / 2
		rightPanelWidth := m.width - leftPanelWidth

		// Account for borders (2px per side)
		jobsPanelWidth := leftPanelWidth - 4
		jobsPanelHeight := topPanelHeight - 4
		queuePanelWidth := rightPanelWidth - 4
		queuePanelHeight := topPanelHeight - 4
		detailsPanelWidth := m.width - 4
		detailsPanelHeight := bottomPanelHeight - 4

		// Send panel-specific dimensions to each child
		var cmd tea.Cmd
		m.jobsPanel, cmd = m.jobsPanel.Update(tea.WindowSizeMsg{
			Width:  jobsPanelWidth,
			Height: jobsPanelHeight,
		})
		cmds = append(cmds, cmd)

		m.queuePanel, cmd = m.queuePanel.Update(tea.WindowSizeMsg{
			Width:  queuePanelWidth,
			Height: queuePanelHeight,
		})
		cmds = append(cmds, cmd)

		m.detailsPanel, cmd = m.detailsPanel.Update(tea.WindowSizeMsg{
			Width:  detailsPanelWidth,
			Height: detailsPanelHeight,
		})
		cmds = append(cmds, cmd)

		m.statusBar.SetWidth(msg.Width)

		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "tab":
			m.activePanel = (m.activePanel + 1) % 3
			return m, nil

		case "shift+tab":
			m.activePanel = (m.activePanel - 1 + 3) % 3
			return m, nil

		case "1":
			m.activePanel = PanelJobs
			return m, nil

		case "2":
			m.activePanel = PanelQueue
			return m, nil

		case "3":
			m.activePanel = PanelDetails
			return m, nil

		case "?":
			// TODO: Show help overlay
			return m, nil
		}
	}

	// Always update jobs panel to handle its messages (loading, etc.)
	// Skip WindowSizeMsg since we handle it explicitly above with panel-specific dimensions
	if _, isWindowSize := msg.(tea.WindowSizeMsg); !isWindowSize {
		var cmd tea.Cmd
		m.jobsPanel, cmd = m.jobsPanel.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Route messages to other panels (skip WindowSizeMsg - handled above)
	if _, isWindowSize := msg.(tea.WindowSizeMsg); !isWindowSize {
		var cmd tea.Cmd

		// Route key messages to active panel
		if _, ok := msg.(tea.KeyMsg); ok {
			switch m.activePanel {
			case PanelQueue:
				m.queuePanel, cmd = m.queuePanel.Update(msg)
				cmds = append(cmds, cmd)
			case PanelDetails:
				m.detailsPanel, cmd = m.detailsPanel.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

		// Always update status bar
		m.statusBar, cmd = m.statusBar.Update(msg)
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

	// Render bottom panel (details)
	bottomPanel := m.renderPanel(PanelDetails, m.detailsPanel.View(), m.width, bottomPanelHeight)

	// Render status bar
	statusBarView := m.statusBar.View()

	// Join all vertically
	return lipgloss.JoinVertical(
		lipgloss.Left,
		topPanels,
		bottomPanel,
		statusBarView,
	)
}

// renderPanel renders a panel with borders and focus highlighting
func (m Model) renderPanel(id PanelID, content string, width, height int) string {
	borderColor := lipgloss.Color("8") // Dim gray
	if m.activePanel == id {
		borderColor = lipgloss.Color("10") // Bright green for active
	}

	style := lipgloss.NewStyle().
		Width(width - 2).  // Account for border
		Height(height - 2). // Account for border
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	return style.Render(content)
}
