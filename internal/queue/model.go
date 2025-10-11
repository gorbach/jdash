package queue

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the build queue panel
type Model struct {
	width  int
	height int
}

// New creates a new queue panel model
func New() Model {
	return Model{}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// View renders the queue panel
func (m Model) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("11")).
		Render("Build Queue (0)")

	placeholder := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true).
		Render("\n\n[Queue polling will be added in Story 7]")

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Render("\n\n[Empty queue]")

	return title + placeholder + help
}
