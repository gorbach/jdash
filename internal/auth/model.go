package auth

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorbach/jdash/internal/jenkins"
)

// FocusField represents which field is currently focused
type FocusField int

const (
	FocusURL FocusField = iota
	FocusUsername
	FocusToken
	FocusTestButton
	FocusOkButton
)

// Model represents the authentication screen state
type Model struct {
	urlInput      textinput.Model
	usernameInput textinput.Model
	tokenInput    textinput.Model
	focusedField  FocusField
	testing       bool
	testSuccess   bool
	spinner       spinner.Model
	error         string
	width         int
	height        int
	onSuccess     func()
}

// testResultMsg is sent when connection test completes
type testResultMsg struct {
	success bool
	err     error
}

// saveCompleteMsg is sent when config save completes
type saveCompleteMsg struct {
	err error
}

// New creates a new authentication model
func New() Model {
	// URL input
	urlInput := textinput.New()
	urlInput.Placeholder = "https://jenkins.example.com"
	urlInput.Focus()
	urlInput.CharLimit = 256
	urlInput.Width = 50

	// Username input
	usernameInput := textinput.New()
	usernameInput.Placeholder = "your-username"
	usernameInput.CharLimit = 100
	usernameInput.Width = 50

	// Token input
	tokenInput := textinput.New()
	tokenInput.Placeholder = "your-api-token"
	tokenInput.CharLimit = 256
	tokenInput.Width = 50
	tokenInput.EchoMode = textinput.EchoPassword
	tokenInput.EchoCharacter = '•'

	// Spinner
	s := spinner.New()
	s.Spinner = spinner.Dot

	return Model{
		urlInput:      urlInput,
		usernameInput: usernameInput,
		tokenInput:    tokenInput,
		focusedField:  FocusURL,
		spinner:       s,
	}
}

// SetOnSuccess sets the callback for successful authentication
func (m *Model) SetOnSuccess(fn func()) {
	m.onSuccess = fn
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "tab", "shift+tab":
			m.error = "" // Clear error on navigation
			if msg.String() == "tab" {
				m.focusNext()
			} else {
				m.focusPrev()
			}
			return m, nil

		case "enter":
			if m.focusedField == FocusTestButton {
				return m, m.testConnection()
			} else if m.focusedField == FocusOkButton && m.testSuccess {
				return m, m.saveConfig()
			}
		}

	case testResultMsg:
		m.testing = false
		m.testSuccess = msg.success
		if msg.err != nil {
			m.error = msg.err.Error()
		} else {
			m.error = ""
			m.focusedField = FocusOkButton
		}
		return m, nil

	case saveCompleteMsg:
		if msg.err != nil {
			m.error = fmt.Sprintf("Failed to save config: %v", msg.err)
			return m, nil
		}
		// Success - call callback
		if m.onSuccess != nil {
			m.onSuccess()
		}
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Update the focused input
	cmd := m.updateInputs(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// updateInputs updates the text inputs based on focus
func (m *Model) updateInputs(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	switch m.focusedField {
	case FocusURL:
		m.urlInput, cmd = m.urlInput.Update(msg)
	case FocusUsername:
		m.usernameInput, cmd = m.usernameInput.Update(msg)
	case FocusToken:
		m.tokenInput, cmd = m.tokenInput.Update(msg)
	}

	return cmd
}

// focusNext moves focus to the next field
func (m *Model) focusNext() {
	fields := m.getFieldOrder()
	current := -1
	for i, f := range fields {
		if f == m.focusedField {
			current = i
			break
		}
	}
	if current >= 0 {
		next := (current + 1) % len(fields)
		m.setFocus(fields[next])
	}
}

// focusPrev moves focus to the previous field
func (m *Model) focusPrev() {
	fields := m.getFieldOrder()
	current := -1
	for i, f := range fields {
		if f == m.focusedField {
			current = i
			break
		}
	}
	if current >= 0 {
		prev := (current - 1 + len(fields)) % len(fields)
		m.setFocus(fields[prev])
	}
}

// getFieldOrder returns the tab order of fields
func (m *Model) getFieldOrder() []FocusField {
	if m.testSuccess {
		return []FocusField{FocusURL, FocusUsername, FocusToken, FocusOkButton}
	}
	return []FocusField{FocusURL, FocusUsername, FocusToken, FocusTestButton}
}

// setFocus sets focus to a specific field
func (m *Model) setFocus(field FocusField) {
	m.focusedField = field

	// Update input focus states
	m.urlInput.Blur()
	m.usernameInput.Blur()
	m.tokenInput.Blur()

	switch field {
	case FocusURL:
		m.urlInput.Focus()
	case FocusUsername:
		m.usernameInput.Focus()
	case FocusToken:
		m.tokenInput.Focus()
	}
}

// testConnection tests the Jenkins connection
func (m *Model) testConnection() tea.Cmd {
	url := strings.TrimSpace(m.urlInput.Value())
	username := strings.TrimSpace(m.usernameInput.Value())
	token := strings.TrimSpace(m.tokenInput.Value())

	// Validate inputs
	if url == "" || username == "" || token == "" {
		m.error = "Please fill in all fields"
		return nil
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		m.error = "URL must start with http:// or https://"
		return nil
	}

	m.testing = true
	m.error = ""

	return func() tea.Msg {
		client := jenkins.NewClient(jenkins.Credentials{
			URL:      url,
			Username: username,
			Token:    token,
		})

		err := client.TestConnection()
		return testResultMsg{
			success: err == nil,
			err:     err,
		}
	}
}

// saveConfig saves the configuration
func (m *Model) saveConfig() tea.Cmd {
	url := strings.TrimSpace(m.urlInput.Value())
	username := strings.TrimSpace(m.usernameInput.Value())
	token := strings.TrimSpace(m.tokenInput.Value())

	return func() tea.Msg {
		err := SaveServerConfig(ServerConfig{
			URL:      url,
			Username: username,
			Token:    token,
		})
		return saveCompleteMsg{err: err}
	}
}

// View renders the authentication screen
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	return renderAuthModal(m)
}

// Style definitions
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			MarginBottom(1)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	labelFocusedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("14")).
				Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true)

	buttonStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color("15"))

	buttonFocusedStyle = lipgloss.NewStyle().
				Padding(0, 2).
				Background(lipgloss.Color("14")).
				Foreground(lipgloss.Color("0")).
				Bold(true)

	buttonSuccessStyle = lipgloss.NewStyle().
				Padding(0, 2).
				Background(lipgloss.Color("10")).
				Foreground(lipgloss.Color("0")).
				Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			MarginTop(1)

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("12")).
			Padding(1, 2).
			Width(70)
)

// renderAuthModal renders the authentication modal
func renderAuthModal(m Model) string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Jenkins Authentication"))
	b.WriteString("\n\n")

	// URL field
	urlLabel := "Server URL"
	if m.focusedField == FocusURL {
		b.WriteString(labelFocusedStyle.Render(urlLabel))
	} else {
		b.WriteString(labelStyle.Render(urlLabel))
	}
	b.WriteString(labelStyle.Render(" (e.g., https://jenkins.example.com)"))
	b.WriteString("\n")
	b.WriteString(m.urlInput.View())
	b.WriteString("\n\n")

	// Username field
	usernameLabel := "Username"
	if m.focusedField == FocusUsername {
		b.WriteString(labelFocusedStyle.Render(usernameLabel))
	} else {
		b.WriteString(labelStyle.Render(usernameLabel))
	}
	b.WriteString("\n")
	b.WriteString(m.usernameInput.View())
	b.WriteString("\n\n")

	// Token field
	tokenLabel := "API Token"
	if m.focusedField == FocusToken {
		b.WriteString(labelFocusedStyle.Render(tokenLabel))
	} else {
		b.WriteString(labelStyle.Render(tokenLabel))
	}
	b.WriteString("\n")
	b.WriteString(m.tokenInput.View())
	b.WriteString("\n\n")

	// Status messages
	if m.testing {
		b.WriteString(m.spinner.View())
		b.WriteString(" Testing connection...")
	} else if m.error != "" {
		b.WriteString(errorStyle.Render("✗ " + m.error))
	} else if m.testSuccess {
		b.WriteString(successStyle.Render("✓ Connection successful!"))
	}
	b.WriteString("\n\n")

	// Buttons
	buttonRow := ""
	if !m.testSuccess {
		if m.focusedField == FocusTestButton {
			buttonRow = buttonFocusedStyle.Render("[ Test Connection ]")
		} else {
			buttonRow = buttonStyle.Render("[ Test Connection ]")
		}
	} else {
		if m.focusedField == FocusOkButton {
			buttonRow = buttonSuccessStyle.Render("[ OK ]")
		} else {
			buttonStyle := lipgloss.NewStyle().
				Padding(0, 2).
				Background(lipgloss.Color("10")).
				Foreground(lipgloss.Color("15"))
			buttonRow = buttonStyle.Render("[ OK ]")
		}
	}
	b.WriteString(lipgloss.NewStyle().Width(70).Align(lipgloss.Center).Render(buttonRow))
	b.WriteString("\n")

	// Help text
	b.WriteString(helpStyle.Render("Tab: Navigate | Enter: Select | Esc: Quit"))

	// Wrap in modal
	content := modalStyle.Render(b.String())

	// Center on screen
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}
