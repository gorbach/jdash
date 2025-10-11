package details

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the job details panel
type Model struct {
	width  int
	height int
}

// New creates a new details panel model
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

// View renders the details panel
func (m Model) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("13")).
		Render("Job Details / Console Output")

	placeholder := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true).
		Render("\n\n[Select a job to view details]")

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Render("\n\nb - Build | l - Logs | H - History")

	return title + placeholder + help
}
