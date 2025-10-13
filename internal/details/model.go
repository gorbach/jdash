package details

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorbach/jenkins-gotui/internal/jenkins"
	"github.com/gorbach/jenkins-gotui/internal/jobs"
	"github.com/gorbach/jenkins-gotui/internal/ui"
	"github.com/gorbach/jenkins-gotui/internal/utils"
)

const maxRecentBuilds = 10

type jobDetailsResultMsg struct {
	ticket      uint64
	jobFullName string
	details     *jenkins.JobDetails
	err         error
}

// Model represents the job details panel.
type Model struct {
	client *jenkins.Client

	viewport viewport.Model
	width    int
	height   int

	selectedJob  *jenkins.Job
	recentBuilds []jenkins.Build

	loading   bool
	err       error
	requestID uint64
}

// New creates a new details panel model.
func New(client *jenkins.Client) Model {
	vp := viewport.New(0, 0)
	model := Model{
		client:   client,
		viewport: vp,
	}
	model.setPlaceholderContent()
	return model
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return m.viewport.Init()
}

// Update handles messages for the details panel.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewportSize()
		return m, nil

	case jobs.JobSelectedMsg:
		jobCopy := msg.Job
		m.selectedJob = &jobCopy
		m.loading = true
		m.err = nil
		m.recentBuilds = nil
		m.requestID++
		m.setLoadingContent()
		return m, m.fetchJobDetailsCmd(jobCopy, m.requestID)

	case jobs.JobSelectionClearedMsg:
		m.loading = false
		m.err = nil
		m.selectedJob = nil
		m.recentBuilds = nil
		m.setPlaceholderContent()
		return m, nil

	case jobDetailsResultMsg:
		if msg.ticket != m.requestID {
			// Outdated response, ignore.
			return m, nil
		}

		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.setErrorContent(msg.err)
			return m, nil
		}

		m.err = nil
		if msg.details != nil {
			jobCopy := msg.details.Job
			m.selectedJob = &jobCopy
			m.recentBuilds = append([]jenkins.Build(nil), msg.details.Builds...)
		}
		m.setDetailsContent()
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the details panel.
func (m Model) View() string {
	return m.viewport.View()
}

func (m *Model) fetchJobDetailsCmd(job jenkins.Job, ticket uint64) tea.Cmd {
	client := m.client
	fullName := job.FullName

	return func() tea.Msg {
		if client == nil {
			return jobDetailsResultMsg{
				ticket:      ticket,
				jobFullName: fullName,
				err:         fmt.Errorf("Jenkins client not configured"),
			}
		}

		details, err := client.GetJobDetails(fullName, maxRecentBuilds)
		if err != nil {
			return jobDetailsResultMsg{
				ticket:      ticket,
				jobFullName: fullName,
				err:         err,
			}
		}

		return jobDetailsResultMsg{
			ticket:      ticket,
			jobFullName: fullName,
			details:     details,
		}
	}
}

func (m *Model) setPlaceholderContent() {
	var b strings.Builder
	b.WriteString(ui.TitleStyle.Render("Job Details"))
	b.WriteString("\n\n")
	b.WriteString(ui.SubtleStyle.Render("Select a job to view details"))
	b.WriteString("\n\n")
	b.WriteString(ui.SubtleStyle.Render("b - Build now    l - View logs    h - History"))
	m.setContent(b.String())
}

func (m *Model) setLoadingContent() {
	var b strings.Builder
	b.WriteString(ui.TitleStyle.Render("Job Details"))
	b.WriteString("\n\n")
	label := "Loading job details..."
	if m.selectedJob != nil {
		label = fmt.Sprintf("Loading details for %s...", m.selectedJob.Name)
	}
	b.WriteString(ui.SubtleStyle.Render(label))
	m.setContent(b.String())
}

func (m *Model) setErrorContent(err error) {
	var b strings.Builder
	b.WriteString(ui.TitleStyle.Render("Job Details"))
	if m.selectedJob != nil {
		b.WriteString("\n")
		b.WriteString(ui.HighlightStyle.Render(fmt.Sprintf("Job: %s", m.selectedJob.Name)))
	}
	b.WriteString("\n\n")
	b.WriteString(ui.ErrorStyle.Render("Failed to load job details"))
	if err != nil {
		b.WriteString("\n")
		b.WriteString(ui.SubtleStyle.Render(err.Error()))
	}
	b.WriteString("\n\n")
	b.WriteString(ui.SubtleStyle.Render("Press 'r' to refresh or reselect the job"))
	m.setContent(b.String())
}

func (m *Model) setDetailsContent() {
	if m.selectedJob == nil {
		m.setPlaceholderContent()
		return
	}

	job := m.selectedJob
	var b strings.Builder

	b.WriteString(ui.TitleStyle.Render(fmt.Sprintf("Job: %s", job.Name)))
	b.WriteString("\n")

	statusText := ui.GetStatusText(job.GetStatus())
	durationText := ui.SubtleStyle.Render("Duration: —")
	if job.LastBuild != nil {
		durationText = ui.SubtleStyle.Render("Duration: " + formatDurationFromBuild(job.LastBuild))
	}
	b.WriteString(fmt.Sprintf("Status: %s    %s\n", statusText, durationText))

	if job.LastBuild != nil {
		lastBuild := job.LastBuild
		lastBuildLine := fmt.Sprintf("Last Build: #%d    Triggered: %s",
			lastBuild.Number,
			ui.SubtleStyle.Render(formatRelativeTimeFromBuild(lastBuild)),
		)
		triggeredBy := lastBuild.GetTriggeredBy()
		if triggeredBy == "" {
			triggeredBy = "—"
		}
		branch := lastBuild.GetBranch()
		if branch == "" {
			branch = "—"
		}
		actorsLine := fmt.Sprintf("By: %s    Branch: %s",
			ui.SubtleStyle.Render(triggeredBy),
			ui.SubtleStyle.Render(branch),
		)
		b.WriteString(lastBuildLine)
		b.WriteString("\n")
		b.WriteString(actorsLine)
		b.WriteString("\n")
	} else {
		b.WriteString("Last Build: —    Triggered: —\n")
		b.WriteString("By: —    Branch: —\n")
	}

	b.WriteString("\n")
	b.WriteString(ui.HighlightStyle.Render("─ Recent Builds ─"))
	b.WriteString("\n")

	if len(m.recentBuilds) == 0 {
		b.WriteString(ui.SubtleStyle.Render("No build history available"))
		b.WriteString("\n")
	} else {
		for i := range m.recentBuilds {
			build := &m.recentBuilds[i]
			status := build.GetStatus()
			statusStyled := ui.GetStatusStyle(status).Render(
				fmt.Sprintf("%s %s", ui.GetStatusIcon(status), status),
			)
			duration := ui.SubtleStyle.Render(formatDurationFromBuild(build))
			when := ui.SubtleStyle.Render(formatRelativeTimeFromBuild(build))

			line := fmt.Sprintf("#%-5d %s  %s  %s",
				build.Number,
				statusStyled,
				duration,
				when,
			)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(ui.HighlightStyle.Render("─ Actions ─"))
	b.WriteString("\n")
	b.WriteString(ui.SubtleStyle.Render("b - Build now    l - View logs    h - History"))
	b.WriteString("\n")

	m.setContent(b.String())
}

func (m *Model) setContent(content string) {
	m.viewport.SetContent(strings.TrimRight(content, "\n"))
	m.viewport.GotoTop()
}

func (m *Model) updateViewportSize() {
	if m.width < 0 {
		m.width = 0
	}
	if m.height < 0 {
		m.height = 0
	}
	m.viewport.Width = m.width
	m.viewport.Height = m.height
}

func formatDurationFromBuild(build *jenkins.Build) string {
	if build == nil {
		return "—"
	}
	if build.Building {
		return "running"
	}
	if build.Duration <= 0 {
		return "—"
	}
	return utils.FormatDuration(build.GetDuration())
}

func formatRelativeTimeFromBuild(build *jenkins.Build) string {
	if build == nil {
		return "unknown"
	}
	if build.Building && build.Timestamp == 0 {
		return "in progress"
	}
	if build.Timestamp == 0 {
		return "unknown"
	}
	return utils.FormatRelativeTime(build.GetTimestamp())
}
