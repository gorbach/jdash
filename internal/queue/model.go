package queue

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorbach/jenkins-gotui/internal/jenkins"
	"github.com/gorbach/jenkins-gotui/internal/ui"
)

// Model represents the build queue panel
type Model struct {
	width         int
	height        int
	queuedItems   []jenkins.QueueItem
	runningBuilds []jenkins.RunningBuild
	spinner       spinner.Model
	client        *jenkins.Client
	polling       bool
	lastPoll      time.Time
	err           error
}

// New creates a new queue panel model
func New(client *jenkins.Client) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow

	return Model{
		client:  client,
		spinner: s,
		polling: true,
	}
}

// Init initializes the model and starts polling
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.pollQueueCmd(),
		m.tickCmd(),
	)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		// Update elapsed times every second
		if m.polling {
			return m, m.tickCmd()
		}
		return m, nil

	case pollQueueMsg:
		// Trigger a queue poll
		return m, m.pollQueueCmd()

	case RefreshRequestedMsg:
		return m, m.pollQueueCmd()

	case queueUpdateMsg:
		// Queue data fetched successfully
		m.queuedItems = msg.queuedItems
		m.runningBuilds = msg.runningBuilds
		m.lastPoll = time.Now()
		m.err = nil

		// Schedule next poll in 3 seconds
		if m.polling {
			return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return pollQueueMsg{}
			})
		}
		return m, nil

	case queueErrorMsg:
		// Error fetching queue
		m.err = msg.err

		// Retry in 5 seconds on error
		if m.polling {
			return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
				return pollQueueMsg{}
			})
		}
		return m, nil
	}

	return m, nil
}

// View renders the queue panel
func (m Model) View() string {
	var b strings.Builder

	// Title with total count (running + queued)
	totalCount := len(m.runningBuilds) + len(m.queuedItems)
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14")). // Cyan
		Render(fmt.Sprintf("Build Queue (%d)", totalCount))

	b.WriteString(title)
	b.WriteString("\n\n")

	// Show error if present
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // Red
		b.WriteString(errStyle.Render(fmt.Sprintf("Error: %s", m.err.Error())))
		b.WriteString("\n\n")
	}

	// Show items or empty state
	if totalCount == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
		b.WriteString(emptyStyle.Render("[Empty queue]"))
	} else {
		// First show running builds
		for _, build := range m.runningBuilds {
			b.WriteString(m.renderRunningBuild(build))
			b.WriteString("\n")
		}

		// Then show queued items
		for _, item := range m.queuedItems {
			b.WriteString(m.renderQueueItem(item))
			b.WriteString("\n")
		}
	}

	// Add polling indicator at bottom if there's space
	if m.height > 10 {
		b.WriteString("\n")
		lastPollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		if !m.lastPoll.IsZero() {
			elapsed := time.Since(m.lastPoll).Round(time.Second)
			b.WriteString(lastPollStyle.Render(fmt.Sprintf("Last poll: %s ago", elapsed)))
		}
	}

	return b.String()
}

// renderRunningBuild renders a currently executing build
func (m Model) renderRunningBuild(build jenkins.RunningBuild) string {
	var b strings.Builder

	// Show animated spinner for running builds
	b.WriteString(m.spinner.View())
	b.WriteString(" ")

	// Build number
	buildNum := fmt.Sprintf("#%d", build.BuildNumber)
	buildStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow
	b.WriteString(buildStyle.Render(buildNum))
	b.WriteString(" ")

	// Job name
	nameStyle := lipgloss.NewStyle().Bold(true)
	b.WriteString(nameStyle.Render(build.JobName))
	b.WriteString("  ")

	// Elapsed time
	elapsed := build.GetElapsedTime()
	elapsedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	b.WriteString(elapsedStyle.Render(formatDuration(elapsed)))

	return b.String()
}

// renderQueueItem renders a single queued item (not yet executing)
func (m Model) renderQueueItem(item jenkins.QueueItem) string {
	var b strings.Builder

	// Queued but not building yet - show pending icon
	b.WriteString(ui.IconPending)
	b.WriteString(" ")

	// Job name
	jobName := item.GetJobName()
	if jobName == "" {
		jobName = item.Task.URL // Fallback to URL
	}
	nameStyle := lipgloss.NewStyle().Bold(true)
	b.WriteString(nameStyle.Render(jobName))
	b.WriteString("  ")

	// Time in queue
	elapsed := item.GetInQueueDuration()
	elapsedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	b.WriteString(elapsedStyle.Render(formatDuration(elapsed)))

	// Show reason if blocked or stuck
	if item.Blocked || item.Stuck {
		b.WriteString(" ")
		reasonStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Italic(true)
		if item.Stuck {
			b.WriteString(reasonStyle.Render("[STUCK]"))
		} else if item.Blocked {
			b.WriteString(reasonStyle.Render("[BLOCKED]"))
		}
	}

	return b.String()
}

// tickCmd returns a command that sends a tick message every second
func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// pollQueueCmd returns a command that fetches both queued and running builds
func (m Model) pollQueueCmd() tea.Cmd {
	return func() tea.Msg {
		// Fetch queued items (waiting to start)
		queuedItems, err := m.client.GetBuildQueue()
		if err != nil {
			return queueErrorMsg{err: err}
		}

		// Fetch running builds (currently executing)
		runningBuilds, err := m.client.GetRunningBuilds()
		if err != nil {
			return queueErrorMsg{err: err}
		}

		return queueUpdateMsg{
			queuedItems:   queuedItems,
			runningBuilds: runningBuilds,
		}
	}
}

// formatDuration formats a duration in a human-readable format
// For durations under 1 minute: "45s"
// For durations over 1 minute: "2m 34s"
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60

	if seconds == 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	return fmt.Sprintf("%dm %ds", minutes, seconds)
}
