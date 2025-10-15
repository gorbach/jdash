package parameters

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorbach/jdash/internal/jenkins"
	"github.com/gorbach/jdash/internal/ui"
)

// Model represents the modal used to collect parameter values before triggering a build.
type Model struct {
	jobName     string
	jobFullName string

	definitions []jenkins.ParameterDefinition
	inputs      []textinput.Model
	focusIndex  int

	width  int
	height int

	errMessage string
}

// SubmittedMsg is emitted when the user confirms the trigger with parameter values.
type SubmittedMsg struct {
	JobName     string
	JobFullName string
	Values      map[string]string
}

// CancelledMsg is emitted when the user cancels the parameter modal.
type CancelledMsg struct {
	JobFullName string
}

// New creates a parameter modal model seeded with parameter definitions.
func New(jobName, jobFullName string, defs []jenkins.ParameterDefinition) *Model {
	model := &Model{
		jobName:     jobName,
		jobFullName: jobFullName,
		definitions: append([]jenkins.ParameterDefinition(nil), defs...),
		inputs:      make([]textinput.Model, len(defs)),
	}

	for i := range model.definitions {
		def := &model.definitions[i]
		ti := textinput.New()
		ti.Prompt = ""
		ti.PromptStyle = ui.HighlightStyle
		ti.CharLimit = 256
		ti.SetCursorMode(textinput.CursorBlink)
		ti.CursorStyle = ui.HighlightStyle
		ti.TextStyle = lipgloss.NewStyle()
		ti.Placeholder = def.DefaultValueString()
		ti.SetValue(def.DefaultValueString())
		ti.Width = preferredInputWidth(def)
		ti.Blur()
		model.inputs[i] = ti
	}

	return model
}

// Init focuses the first field (if present).
func (m *Model) Init() tea.Cmd {
	if len(m.inputs) == 0 {
		return nil
	}
	cmd := m.inputs[m.focusIndex].Focus()
	return cmd
}

// Update handles TEA messages for the modal.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, cancelCmd(m.jobFullName)
		case "tab":
			return m, m.shiftFocus(1)
		case "shift+tab":
			return m, m.shiftFocus(-1)
		case "enter":
			return m, submitCmd(m.jobName, m.jobFullName, m.collectValues())
		}
	}

	if len(m.inputs) == 0 {
		return m, nil
	}

	var cmd tea.Cmd
	m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	return m, cmd
}

// View renders the modal overlay.
func (m *Model) View() string {
	var content strings.Builder

	title := ui.TitleStyle.Render(fmt.Sprintf("Trigger Build: %s", m.jobName))
	content.WriteString(title)
	content.WriteString("\n\n")

	if len(m.definitions) == 0 {
		content.WriteString(ui.SubtleStyle.Render("This job has no configurable parameters."))
		content.WriteString("\n")
	} else {
		for i := range m.definitions {
			def := &m.definitions[i]
			label := ui.HighlightStyle.Render(def.Name)
			content.WriteString(label)
			content.WriteString("\n")

			if desc := strings.TrimSpace(def.Description); desc != "" {
				content.WriteString(ui.SubtleStyle.Render(desc))
				content.WriteString("\n")
			}

			if len(def.Choices) > 0 {
				choices := ui.SubtleStyle.Render("Choices: " + strings.Join(def.Choices, ", "))
				content.WriteString(choices)
				content.WriteString("\n")
			}

			content.WriteString(m.inputs[i].View())
			content.WriteString("\n\n")
		}
	}

	content.WriteString(ui.SubtleStyle.Render("[Tab] Next  [Shift+Tab] Previous  [Enter] Trigger  [Esc] Cancel"))
	if strings.TrimSpace(m.errMessage) != "" {
		content.WriteString("\n")
		content.WriteString(ui.ErrorStyle.Render(m.errMessage))
	}

	modalWidth := clampInt(48, 80, m.longestInputWidth()+8)
	panel := lipgloss.NewStyle().
		Width(modalWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("10")).
		Padding(1, 2).
		Render(strings.TrimRight(content.String(), "\n"))

	if m.width == 0 || m.height == 0 {
		return panel
	}

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		panel,
	)
}

func (m *Model) shiftFocus(delta int) tea.Cmd {
	if len(m.inputs) == 0 {
		return nil
	}
	next := (m.focusIndex + delta + len(m.inputs)) % len(m.inputs)
	if next == m.focusIndex {
		return nil
	}

	var cmds []tea.Cmd
	current := &m.inputs[m.focusIndex]
	current.Blur()
	m.focusIndex = next
	nextInput := &m.inputs[m.focusIndex]
	if focusCmd := nextInput.Focus(); focusCmd != nil {
		cmds = append(cmds, focusCmd)
	}
	return tea.Batch(cmds...)
}

func (m *Model) collectValues() map[string]string {
	if len(m.definitions) == 0 {
		return map[string]string{}
	}
	values := make(map[string]string, len(m.definitions))
	for i := range m.definitions {
		def := &m.definitions[i]
		var inputValue string
		if i < len(m.inputs) {
			inputValue = m.inputs[i].Value()
		}
		values[def.Name] = normalizeParameterValue(*def, inputValue)
	}
	return values
}

func (m *Model) longestInputWidth() int {
	width := 32
	for i := range m.definitions {
		if w := preferredInputWidth(&m.definitions[i]); w > width {
			width = w
		}
	}
	return width
}

func preferredInputWidth(def *jenkins.ParameterDefinition) int {
	base := len(def.DefaultValueString())
	if len(def.Choices) > 0 {
		for _, choice := range def.Choices {
			if l := len(choice); l > base {
				base = l
			}
		}
	}
	if base < 24 {
		base = 24
	}
	if base > 48 {
		base = 48
	}
	return base
}

func normalizeParameterValue(def jenkins.ParameterDefinition, raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = strings.TrimSpace(def.DefaultValueString())
	}

	switch strings.ToLower(def.GetType()) {
	case "hudson.model.booleanparameterdefinition", "booleanparameterdefinition":
		if value == "" {
			return "false"
		}
		switch strings.ToLower(value) {
		case "true", "1", "yes", "y":
			return "true"
		default:
			return "false"
		}
	default:
		return value
	}
}

func submitCmd(jobName, jobFullName string, values map[string]string) tea.Cmd {
	copied := make(map[string]string, len(values))
	for k, v := range values {
		copied[k] = v
	}
	return func() tea.Msg {
		return SubmittedMsg{
			JobName:     jobName,
			JobFullName: jobFullName,
			Values:      copied,
		}
	}
}

func cancelCmd(jobFullName string) tea.Cmd {
	return func() tea.Msg {
		return CancelledMsg{JobFullName: jobFullName}
	}
}

func clampInt(min, max, value int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
