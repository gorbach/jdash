package statusbar

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// messageKind allows us to render temporary feedback with basic styling.
type messageKind int

const (
	messageNone messageKind = iota
	messageInfo
	messageSuccess
	messageError
)

const (
	messageDuration         = 3 * time.Second
	statusHeartbeatInterval = time.Second
)

type messageExpiredMsg struct {
	ticket uint64
}

type heartbeatMsg struct{}

// RefreshStartedMsg tells the status bar that a global refresh has kicked off.
type RefreshStartedMsg struct{}

// RefreshFinishedMsg tells the status bar that a refresh completed (successfully or not).
type RefreshFinishedMsg struct {
	JobCount int
	Err      error
}

// Model represents the status bar state and rendering logic.
type Model struct {
	serverURL string

	jobCount int

	message       string
	messageStyle  messageKind
	messageTicket uint64

	width   int
	loading bool
}

// New creates a new status bar model.
func New(serverURL string) Model {
	return Model{
		serverURL: serverURL,
		loading:   true,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Tick(statusHeartbeatInterval, func(time.Time) tea.Msg {
		return heartbeatMsg{}
	})
}

// Update handles messages following TEA patterns.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case RefreshStartedMsg:
		m.loading = true
		m.message = ""
		m.messageStyle = messageNone
		// Keep the bar in a loading state until completion.
		return m, nil

	case RefreshFinishedMsg:
		m.loading = false
		if msg.JobCount >= 0 {
			m.jobCount = msg.JobCount
		}
		if msg.Err != nil {
			return m.setMessage(messageError, fmt.Sprintf("Refresh failed: %v", msg.Err))
		}
		return m.setMessage(messageSuccess, "✓ Refreshed")

	case messageExpiredMsg:
		if msg.ticket == m.messageTicket {
			m.message = ""
			m.messageStyle = messageNone
		}
		return m, nil

	case heartbeatMsg:
		return m, tea.Tick(statusHeartbeatInterval, func(time.Time) tea.Msg {
			return heartbeatMsg{}
		})
	}

	return m, nil
}

func (m Model) setMessage(kind messageKind, text string) (Model, tea.Cmd) {
	m.messageTicket++
	m.message = text
	m.messageStyle = kind

	ticket := m.messageTicket
	cmd := tea.Tick(messageDuration, func(time.Time) tea.Msg {
		return messageExpiredMsg{ticket: ticket}
	})

	return m, cmd
}

// View renders the status bar.
func (m Model) View() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("12")).
		Width(m.width).
		Padding(0, 1)

	parts := []string{
		"jenkins-tui",
		fmt.Sprintf("Connected: %s", formatServerURL(m.serverURL)),
	}

	if m.loading {
		parts = append(parts, "Refreshing…")
	} else {
		parts = append(parts, fmt.Sprintf("%d jobs", m.jobCount))
	}

	parts = append(parts, "? for help")

	if m.message != "" {
		parts = append(parts, renderMessage(m.message, m.messageStyle))
	}

	content := strings.Join(parts, " | ")
	return style.Render(content)
}

func formatServerURL(url string) string {
	if url == "" {
		return "—"
	}
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	return strings.TrimSuffix(url, "/")
}

func renderMessage(text string, kind messageKind) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}

	style := lipgloss.NewStyle().Bold(true)

	switch kind {
	case messageError:
		style = style.Foreground(lipgloss.Color("1"))
	case messageSuccess:
		style = style.Foreground(lipgloss.Color("10"))
	case messageInfo:
		style = style.Foreground(lipgloss.Color("11"))
	default:
		style = style.Foreground(lipgloss.Color("7"))
	}

	return style.Render(text)
}
