package details

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
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

type inFlightAction struct {
	kind   actionKind
	ticket uint64
	label  string
}

type actionFeedback struct {
	ticket  uint64
	message string
	isError bool
}

type confirmationState struct {
	kind   actionKind
	prompt string
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

	actionSpinner spinner.Model
	inFlight      *inFlightAction
	feedback      *actionFeedback
	confirmation  *confirmationState
	actionTicket  uint64
}

// New creates a new details panel model.
func New(client *jenkins.Client) Model {
	vp := viewport.New(0, 0)
	actSpinner := spinner.New()
	actSpinner.Spinner = spinner.Dot
	actSpinner.Style = ui.HighlightStyle
	model := Model{
		client:        client,
		viewport:      vp,
		actionSpinner: actSpinner,
	}
	model.refreshContent()
	return model
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return m.viewport.Init()
}

// Update handles messages for the details panel.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewportSize()
		// fallthrough to refresh content

	case jobs.JobSelectedMsg:
		m.handleJobSelected(msg.Job, &cmds)

	case jobs.JobSelectionClearedMsg:
		m.handleJobCleared()

	case jobDetailsResultMsg:
		if msg.ticket != m.requestID {
			// Outdated response, ignore.
			return m, nil
		}

		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.recentBuilds = nil
			if m.inFlight != nil && m.inFlight.ticket == msg.ticket {
				cmds = append(cmds, m.setFeedbackWithTicket(msg.ticket, fmt.Sprintf("✗ %v", msg.err), true))
				m.inFlight = nil
			}
			break
		}

		m.err = nil
		if msg.details != nil {
			jobCopy := msg.details.Job
			m.selectedJob = &jobCopy
			m.recentBuilds = append([]jenkins.Build(nil), msg.details.Builds...)
		}

		if m.inFlight != nil && m.inFlight.ticket == msg.ticket {
			message := defaultSuccessMessage(m.selectedJob, m.inFlight.kind)
			cmds = append(cmds, m.setFeedbackWithTicket(msg.ticket, message, false))
			m.inFlight = nil
		}

	case actionResultMsg:
		if m.inFlight == nil || m.inFlight.ticket != msg.ticket {
			return m, nil
		}
		feedbackMsg := msg.message
		if feedbackMsg == "" {
			if msg.err != nil {
				feedbackMsg = fmt.Sprintf("✗ %v", msg.err)
			} else {
				feedbackMsg = defaultSuccessMessage(m.selectedJob, msg.kind)
			}
		}
		cmds = append(cmds, m.setFeedbackWithTicket(msg.ticket, feedbackMsg, msg.err != nil))
		m.inFlight = nil

	case actionMessageClearedMsg:
		if m.feedback != nil && m.feedback.ticket == msg.ticket {
			m.feedback = nil
		}

	case tea.KeyMsg:
		var cmd tea.Cmd
		m, cmd = m.handleKeyMsg(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case spinner.TickMsg:
		if m.inFlight != nil {
			var spinCmd tea.Cmd
			m.actionSpinner, spinCmd = m.actionSpinner.Update(msg)
			if spinCmd != nil {
				cmds = append(cmds, spinCmd)
			}
		}
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	if vpCmd != nil {
		cmds = append(cmds, vpCmd)
	}

	m.refreshContent()
	return m, tea.Batch(cmds...)
}

// View renders the details panel.
func (m Model) View() string {
	return m.viewport.View()
}

func (m *Model) handleJobSelected(job jenkins.Job, cmds *[]tea.Cmd) {
	m.resetActionState()
	jobCopy := job
	m.selectedJob = &jobCopy
	m.recentBuilds = nil
	m.loading = true
	m.err = nil
	m.viewport.GotoTop()
	if cmd, _ := m.startJobDetailsRequest(jobCopy); cmd != nil && cmds != nil {
		*cmds = append(*cmds, cmd)
	}
}

func (m *Model) handleJobCleared() {
	m.loading = false
	m.err = nil
	m.selectedJob = nil
	m.recentBuilds = nil
	m.resetActionState()
	m.viewport.GotoTop()
}

func (m *Model) resetActionState() {
	m.inFlight = nil
	m.feedback = nil
	m.confirmation = nil
}

func (m *Model) startJobDetailsRequest(job jenkins.Job) (tea.Cmd, uint64) {
	m.requestID++
	ticket := m.requestID
	return m.fetchJobDetailsCmd(job, ticket), ticket
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

func (m *Model) refreshContent() {
	m.viewport.SetContent(strings.TrimRight(m.composeContent(), "\n"))
}

func (m *Model) composeContent() string {
	switch {
	case m.loading:
		return m.renderLoadingContent()
	case m.err != nil:
		return m.renderErrorContent()
	case m.selectedJob == nil:
		return m.renderPlaceholderContent()
	default:
		return m.renderDetailsContent()
	}
}

func (m *Model) renderPlaceholderContent() string {
	var b strings.Builder
	b.WriteString(ui.TitleStyle.Render("Job Details"))
	b.WriteString("\n\n")
	b.WriteString(ui.SubtleStyle.Render("Select a job to view details"))
	b.WriteString("\n")
	b.WriteString(ui.SubtleStyle.Render("Actions become available once a build job is selected."))
	return b.String()
}

func (m *Model) renderLoadingContent() string {
	var b strings.Builder
	b.WriteString(ui.TitleStyle.Render("Job Details"))
	b.WriteString("\n\n")
	label := "Loading job details..."
	if m.selectedJob != nil {
		label = fmt.Sprintf("Loading details for %s...", m.selectedJob.Name)
	}
	b.WriteString(ui.SubtleStyle.Render(label))
	return b.String()
}

func (m *Model) renderErrorContent() string {
	var b strings.Builder
	b.WriteString(ui.TitleStyle.Render("Job Details"))
	if m.selectedJob != nil {
		b.WriteString("\n")
		b.WriteString(ui.HighlightStyle.Render(fmt.Sprintf("Job: %s", m.selectedJob.Name)))
	}
	b.WriteString("\n\n")
	b.WriteString(ui.ErrorStyle.Render("Failed to load job details"))
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ui.SubtleStyle.Render(m.err.Error()))
	}
	b.WriteString("\n\n")
	b.WriteString(ui.SubtleStyle.Render("Press 'r' to refresh or reselect the job"))
	return b.String()
}

func (m *Model) renderDetailsContent() string {
	job := m.selectedJob
	if job == nil {
		return m.renderPlaceholderContent()
	}

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
	m.appendRecentBuilds(&b)

	b.WriteString("\n")
	b.WriteString(ui.HighlightStyle.Render("─ Actions ─"))
	b.WriteString("\n")
	m.appendActions(&b)

	m.appendActionStatus(&b)
	return b.String()
}

func (m *Model) appendRecentBuilds(b *strings.Builder) {
	if len(m.recentBuilds) == 0 {
		b.WriteString(ui.SubtleStyle.Render("No build history available"))
		b.WriteString("\n")
		return
	}

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

func (m *Model) appendActions(b *strings.Builder) {
	job := m.selectedJob
	labels := buildActionLabels(job)
	if len(labels) == 0 {
		b.WriteString(ui.SubtleStyle.Render("No actions available"))
		b.WriteString("\n")
		return
	}

	b.WriteString(ui.SubtleStyle.Render(strings.Join(labels, "    ")))
	b.WriteString("\n")
}

func (m *Model) appendActionStatus(b *strings.Builder) {
	var wrote bool

	if m.confirmation != nil {
		if !wrote {
			b.WriteString("\n")
			wrote = true
		}
		b.WriteString(ui.ErrorStyle.Render(m.confirmation.prompt))
		b.WriteString("\n")
	}

	if m.inFlight != nil {
		if !wrote {
			b.WriteString("\n")
			wrote = true
		}
		indicator := m.actionSpinner.View()
		if indicator == "" {
			indicator = "…"
		}
		b.WriteString(fmt.Sprintf("%s %s\n", indicator, m.inFlight.label))
	}

	if m.feedback != nil {
		if !wrote {
			b.WriteString("\n")
			wrote = true
		}
		style := ui.SubtleStyle
		if m.feedback.isError {
			style = ui.ErrorStyle
		} else {
			style = ui.SuccessStyle
		}
		b.WriteString(style.Render(m.feedback.message))
		b.WriteString("\n")
	}
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

func (m *Model) nextActionTicket() uint64 {
	m.actionTicket++
	return m.actionTicket
}

func (m *Model) setFeedbackWithTicket(ticket uint64, message string, isError bool) tea.Cmd {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	if ticket == 0 {
		ticket = m.nextActionTicket()
	}
	m.feedback = &actionFeedback{
		ticket:  ticket,
		message: message,
		isError: isError,
	}
	return clearActionMessageCmd(ticket)
}

func (m *Model) setFeedback(message string, isError bool) tea.Cmd {
	return m.setFeedbackWithTicket(0, message, isError)
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.confirmation != nil {
		return m.handleConfirmationKey(msg)
	}

	if m.loading || m.selectedJob == nil {
		return m, nil
	}

	switch msg.String() {
	case "b":
		return m.startTriggerBuildAction()
	case "a":
		return m.startAbortPrompt()
	case "r":
		return m.startRefreshAction()
	case "l":
		return m.requestAction(actionKindViewLogs)
	case "p":
		return m.requestAction(actionKindViewParameters)
	case "H":
		return m.requestAction(actionKindViewHistory)
	case "c":
		return m.requestAction(actionKindViewConfig)
	default:
		return m, nil
	}
}

func (m Model) handleConfirmationKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.confirmation == nil {
		return m, nil
	}

	switch msg.String() {
	case "y", "Y", "enter":
		kind := m.confirmation.kind
		m.confirmation = nil
		if kind == actionKindAbortBuild {
			return m.startAbortExecution()
		}
		return m, nil
	case "n", "N", "esc":
		m.confirmation = nil
		return m, m.setFeedback("Abort cancelled", false)
	default:
		return m, nil
	}
}

func (m Model) startTriggerBuildAction() (Model, tea.Cmd) {
	if m.client == nil || m.inFlight != nil {
		return m, nil
	}
	job := m.selectedJob
	if job == nil || job.IsFolder() {
		return m, nil
	}

	ticket := m.nextActionTicket()
	m.inFlight = &inFlightAction{
		kind:   actionKindTriggerBuild,
		ticket: ticket,
		label:  fmt.Sprintf("Triggering build for %s...", job.Name),
	}
	m.feedback = nil

	cmd := triggerBuildCmd(m.client, job.Name, job.FullName, ticket)
	return m, tea.Batch(cmd, m.actionSpinner.Tick)
}

func (m Model) startAbortPrompt() (Model, tea.Cmd) {
	if m.inFlight != nil || !isBuildRunning(m.selectedJob) {
		return m, nil
	}
	job := m.selectedJob
	m.confirmation = &confirmationState{
		kind:   actionKindAbortBuild,
		prompt: fmt.Sprintf("Abort running build #%d for %s? (y/N)", job.LastBuild.Number, job.Name),
	}
	return m, nil
}

func (m Model) startAbortExecution() (Model, tea.Cmd) {
	if m.client == nil || m.inFlight != nil || !isBuildRunning(m.selectedJob) {
		return m, nil
	}
	job := m.selectedJob
	ticket := m.nextActionTicket()
	m.inFlight = &inFlightAction{
		kind:   actionKindAbortBuild,
		ticket: ticket,
		label:  fmt.Sprintf("Aborting build #%d...", job.LastBuild.Number),
	}
	m.feedback = nil
	cmd := abortBuildCmd(m.client, job.Name, job.FullName, job.LastBuild.Number, ticket)
	return m, tea.Batch(cmd, m.actionSpinner.Tick)
}

func (m Model) startRefreshAction() (Model, tea.Cmd) {
	if m.inFlight != nil || m.selectedJob == nil {
		return m, nil
	}
	jobCopy := *m.selectedJob
	m.loading = true
	m.err = nil
	cmd, ticket := m.startJobDetailsRequest(jobCopy)
	m.inFlight = &inFlightAction{
		kind:   actionKindRefresh,
		ticket: ticket,
		label:  fmt.Sprintf("Refreshing %s...", jobCopy.Name),
	}
	m.feedback = nil
	return m, tea.Batch(cmd, m.actionSpinner.Tick)
}

func (m Model) requestAction(kind actionKind) (Model, tea.Cmd) {
	job := m.selectedJob
	if job == nil {
		return m, nil
	}

	jobCopy := *job
	var buildPtr *jenkins.Build
	if jobCopy.LastBuild != nil {
		buildCopy := *jobCopy.LastBuild
		buildPtr = &buildCopy
	}

	cmd := actionRequestCmd(kind, jobCopy, buildPtr)
	feedbackCmd := m.setFeedback(requestMessage(kind, &jobCopy), false)

	return m, tea.Batch(cmd, feedbackCmd)
}

func defaultSuccessMessage(job *jenkins.Job, kind actionKind) string {
	name := jobDisplayName(job)
	switch kind {
	case actionKindTriggerBuild:
		return fmt.Sprintf("✓ Build triggered for %s", name)
	case actionKindAbortBuild:
		return fmt.Sprintf("✓ Abort signal sent to %s", name)
	case actionKindRefresh:
		return fmt.Sprintf("✓ Refreshed %s", name)
	default:
		return "✓ Action completed"
	}
}

func requestMessage(kind actionKind, job *jenkins.Job) string {
	name := jobDisplayName(job)
	switch kind {
	case actionKindViewLogs:
		return fmt.Sprintf("→ Opening console logs for %s", name)
	case actionKindViewParameters:
		return fmt.Sprintf("→ Opening parameters for %s", name)
	case actionKindViewHistory:
		return fmt.Sprintf("→ Opening build history for %s", name)
	case actionKindViewConfig:
		return fmt.Sprintf("→ Opening configuration for %s", name)
	default:
		return "→ Action requested"
	}
}

func jobDisplayName(job *jenkins.Job) string {
	if job == nil || job.Name == "" {
		return "job"
	}
	return job.Name
}

func buildActionLabels(job *jenkins.Job) []string {
	if job == nil {
		return nil
	}

	if job.IsFolder() {
		return []string{
			"H - History",
			"r - Refresh",
		}
	}

	labels := []string{
		"b - Build now",
		"l - View logs",
		"H - History",
		"r - Refresh",
		"p - Parameters",
		"c - Config",
	}
	if isBuildRunning(job) {
		labels = append(labels, "a - Abort build")
	}
	return labels
}

func isBuildRunning(job *jenkins.Job) bool {
	if job == nil || job.LastBuild == nil {
		return false
	}
	return job.LastBuild.Building
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
