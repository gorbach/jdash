package statusbar

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the status bar
type Model struct {
	serverURL string
	jobCount  int
	message   string
	width     int
}

// New creates a new status bar model
func New(serverURL string) Model {
	return Model{
		serverURL: serverURL,
		jobCount:  0,
	}
}

// SetWidth sets the width of the status bar
func (m *Model) SetWidth(width int) {
	m.width = width
}

// SetJobCount updates the job count
func (m *Model) SetJobCount(count int) {
	m.jobCount = count
}

// SetMessage sets a temporary message
func (m *Model) SetMessage(msg string) {
	m.message = msg
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

// View renders the status bar
func (m Model) View() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("12")).
		Width(m.width).
		Padding(0, 1)

	left := fmt.Sprintf("jenkins-tui | %s", m.serverURL)

	center := m.message
	if center == "" {
		center = fmt.Sprintf("%d jobs", m.jobCount)
	}

	right := "Tab: Switch | ?: Help | q: Quit"

	// Simple layout: left, center space, right
	content := fmt.Sprintf("%s  %s  %s", left, center, right)

	return style.Render(content)
}
