package console

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorbach/jenkins-gotui/internal/jenkins"
	"github.com/gorbach/jenkins-gotui/internal/ui"
	"github.com/gorbach/jenkins-gotui/internal/utils"
)

const (
	defaultPollInterval = 2 * time.Second
	// maxIdlePollIterations caps how long we keep polling when Jenkins has not
	// produced any console output yet (e.g. job still queued). With the default
	// poll interval this covers roughly five minutes.
	maxIdlePollIterations = 150
	minViewportHeight     = 3
	minViewportWidth      = 10
	searchPrompt          = "/ "
)

// OpenRequestMsg instructs the console model to start streaming logs for the given build.
type OpenRequestMsg struct {
	JobName     string
	JobFullName string
	BuildNumber int
	BuildURL    string
}

// DeactivateMsg signals that the console view is no longer visible and should pause background work.
type DeactivateMsg struct{}

// ExitRequestedMsg is emitted when the user presses Esc to leave the console view.
type ExitRequestedMsg struct{}

type pollLogsMsg struct {
	session uint64
}

type logsChunkMsg struct {
	session    uint64
	content    string
	nextOffset int64
	more       bool
	err        error
}

// RefreshRequestedMsg asks the console view to fetch the latest logs.
type RefreshRequestedMsg struct{}

// Model implements a viewport-based console log viewer with live streaming, search, and auto-scroll.
type Model struct {
	client *jenkins.Client

	viewport viewport.Model
	width    int
	height   int

	jobName     string
	jobFullName string
	buildNumber int
	hasTarget   bool

	autoScroll    bool
	shouldPoll    bool
	pollInterval  time.Duration
	fetchInFlight bool
	session       uint64
	nextOffset    int64
	buildURL      string

	content       []byte
	hasContent    bool
	idlePolls     int
	lastUpdated   time.Time
	err           error
	concealActive bool

	searchInput   textinput.Model
	searchActive  bool
	searchMessage string

	statusMessage string
}

// New creates a new console model.
func New(client *jenkins.Client) Model {
	vp := viewport.New(0, 0)

	ti := textinput.New()
	ti.Prompt = searchPrompt
	ti.Placeholder = "Search logs"
	ti.CharLimit = 256

	return Model{
		client:       client,
		viewport:     vp,
		autoScroll:   true,
		pollInterval: defaultPollInterval,
		searchInput:  ti,
	}
}

// Init initializes the console model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update processes incoming messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m = m.handleWindowSize(msg)

	case tea.KeyMsg:
		var cmd tea.Cmd
		m, cmd = m.handleKeyMsg(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case OpenRequestMsg:
		var cmd tea.Cmd
		m, cmd = m.handleOpenRequest(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case DeactivateMsg:
		m = m.handleDeactivate()

	case RefreshRequestedMsg:
		if m.hasTarget {
			m.err = nil
			m.statusMessage = "Refreshing logs..."
			var fetchCmd tea.Cmd
			m, fetchCmd = m.startFetch()
			if fetchCmd != nil {
				cmds = append(cmds, fetchCmd)
			}
		}

	case pollLogsMsg:
		if msg.session == m.session && m.shouldPoll && !m.fetchInFlight {
			var cmd tea.Cmd
			m, cmd = m.startFetch()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case logsChunkMsg:
		if msg.session == m.session {
			var cmd tea.Cmd
			m, cmd = m.handleLogsChunk(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	if m.searchActive {
		if _, isKey := msg.(tea.KeyMsg); !isKey {
			var inputCmd tea.Cmd
			m.searchInput, inputCmd = m.searchInput.Update(msg)
			if inputCmd != nil {
				cmds = append(cmds, inputCmd)
			}
		}
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	if vpCmd != nil {
		cmds = append(cmds, vpCmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the console view.
func (m Model) View() string {
	title := ui.TitleStyle.Render(fmt.Sprintf("Console: %s #%d", m.jobName, m.buildNumber))

	if !m.hasTarget {
		notice := ui.SubtleStyle.Render("No build selected. Trigger a build to view console logs.")
		return lipgloss.JoinVertical(lipgloss.Left, title, notice)
	}

	var sections []string
	sections = append(sections, title)

	if m.err != nil {
		sections = append(sections, ui.ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	sections = append(sections, m.viewport.View())

	if m.searchActive {
		searchLine := lipgloss.NewStyle().
			Foreground(ui.ColorHighlight).
			Render(fmt.Sprintf("Search %s", m.searchInput.View()))
		sections = append(sections, searchLine)
	} else if m.searchMessage != "" {
		sections = append(sections, ui.SubtleStyle.Render(m.searchMessage))
	}

	statusLine := m.renderStatusLine()
	if statusLine != "" {
		sections = append(sections, statusLine)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderStatusLine() string {
	auto := "OFF"
	autoStyle := ui.SubtleStyle
	if m.autoScroll {
		auto = "ON"
		autoStyle = ui.HighlightStyle
	}

	stream := ui.SubtleStyle.Render("[Idle]")
	if m.shouldPoll || m.fetchInFlight {
		stream = ui.HighlightStyle.Render("[Streaming]")
	}

	updated := ""
	if !m.lastUpdated.IsZero() {
		updated = ui.SubtleStyle.Render(fmt.Sprintf("Last update %s", m.lastUpdated.Format("15:04:05")))
	}

	parts := []string{
		autoStyle.Render(fmt.Sprintf("[Auto-scroll: %s]", auto)),
		ui.SubtleStyle.Render("[s: Toggle]"),
		ui.SubtleStyle.Render("[Esc: Back]"),
		ui.SubtleStyle.Render("[/: Search]"),
		stream,
	}
	if updated != "" {
		parts = append(parts, updated)
	}
	if m.statusMessage != "" {
		parts = append(parts, ui.SubtleStyle.Render(m.statusMessage))
	}

	return strings.Join(parts, "  ")
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height

	contentWidth := clamp(msg.Width-2, minViewportWidth)
	contentHeight := clamp(msg.Height-4, minViewportHeight)

	m.viewport.Width = contentWidth
	m.viewport.Height = contentHeight
	m.searchInput.Width = clamp(msg.Width-6, 20)

	return m
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.searchActive {
		return m.handleSearchKey(msg)
	}

	switch msg.String() {
	case "esc":
		return m, emitExitRequested()
	case "s":
		m.autoScroll = !m.autoScroll
		if m.autoScroll {
			m.viewport.GotoBottom()
		}
		return m, nil
	case "/":
		m.searchActive = true
		m.searchMessage = ""
		m.searchInput.Focus()
		m.searchInput.SetValue("")
		return m, nil
	case "j":
		m.autoScroll = false
		m.viewport.LineDown(1)
		return m, nil
	case "k":
		m.autoScroll = false
		m.viewport.LineUp(1)
		return m, nil
	case "r":
		m.err = nil
		m.statusMessage = "Refreshing logs..."
		var cmd tea.Cmd
		m, cmd = m.startFetch()
		return m, cmd
	}

	switch msg.Type {
	case tea.KeyDown, tea.KeyUp, tea.KeyPgDown, tea.KeyPgUp, tea.KeyLeft, tea.KeyRight, tea.KeyHome, tea.KeyEnd:
		m.autoScroll = false
	}

	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.searchActive = false
		m.searchMessage = ""
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		return m, nil
	case tea.KeyEnter:
		query := strings.TrimSpace(m.searchInput.Value())
		if query == "" {
			m.searchMessage = "Enter text to search"
			return m, nil
		}
		return m.performSearch(query)
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

// SearchActive reports whether the console search interface is active.
func (m Model) SearchActive() bool {
	return m.searchActive
}

func (m Model) handleOpenRequest(msg OpenRequestMsg) (Model, tea.Cmd) {
	m.session++
	m.jobName = msg.JobName
	m.jobFullName = msg.JobFullName
	m.buildNumber = msg.BuildNumber
	m.buildURL = strings.TrimSpace(msg.BuildURL)
	m.hasTarget = (msg.BuildNumber > 0 && msg.JobFullName != "") || m.buildURL != ""
	m.autoScroll = true
	m.shouldPoll = false
	m.fetchInFlight = false
	m.nextOffset = 0
	m.err = nil
	m.statusMessage = ""
	m.searchActive = false
	m.searchMessage = ""
	m.searchInput.Blur()
	m.searchInput.SetValue("")
	m.hasContent = false
	m.idlePolls = 0
	m.concealActive = false
	m.content = m.content[:0]
	m.viewport.SetContent("")
	m.viewport.GotoTop()

	if !m.hasTarget {
		m.statusMessage = "Build has no console output yet."
		return m, nil
	}

	var cmd tea.Cmd
	m.shouldPoll = true
	m, cmd = m.startFetch()
	return m, cmd
}

func (m Model) handleDeactivate() Model {
	m.shouldPoll = false
	m.searchActive = false
	m.searchInput.Blur()
	return m
}

func (m Model) startFetch() (Model, tea.Cmd) {
	if m.client == nil || !m.hasTarget || m.fetchInFlight {
		return m, nil
	}

	client := m.client
	fullName := m.jobFullName
	number := m.buildNumber
	offset := m.nextOffset
	buildURL := m.buildURL
	session := m.session

	m.fetchInFlight = true

	return m, func() tea.Msg {
		chunk, next, more, err := client.GetProgressiveLog(buildURL, fullName, number, offset)
		return logsChunkMsg{
			session:    session,
			content:    chunk,
			nextOffset: next,
			more:       more,
			err:        err,
		}
	}
}

func (m Model) handleLogsChunk(msg logsChunkMsg) (Model, tea.Cmd) {
	m.fetchInFlight = false

	if msg.err != nil {
		m.err = msg.err
		m.idlePolls = 0
		m.shouldPoll = false
		m.statusMessage = "Failed to fetch logs. Press r to retry."
		return m, nil
	}

	prevOffset := m.nextOffset

	hasProgress := false

	sanitized, conceal := utils.StripANSISecrets(msg.content, m.concealActive)
	m.concealActive = conceal
	chunkLen := len(sanitized)

	if chunkLen > 0 {
		preview := sanitized
		if len(preview) > 120 {
			preview = sanitized[:120] + "…"
		}
		m.content = append(m.content, []byte(sanitized)...)
		m.viewport.SetContent(string(m.content))
		m.hasContent = true
		hasProgress = true
	}

	if msg.nextOffset > prevOffset {
		hasProgress = true
	}

	if hasProgress {
		m.idlePolls = 0
	} else {
		m.idlePolls++
	}

	m.nextOffset = msg.nextOffset
	m.shouldPoll = msg.more
	m.err = nil
	if hasProgress {
		m.statusMessage = ""
	}
	m.lastUpdated = time.Now()

	if m.autoScroll {
		m.viewport.GotoBottom()
	}

	if !m.shouldPoll && m.idlePolls < maxIdlePollIterations {
		m.shouldPoll = true
		if !hasProgress && m.statusMessage == "" {
			m.statusMessage = "Waiting for console output..."
		}
	}

	if !m.shouldPoll && m.idlePolls >= maxIdlePollIterations {
		if m.statusMessage == "" {
			if m.hasContent {
				m.statusMessage = "Console output idle. Press r to refresh."
			} else {
				m.statusMessage = "No console output yet. Press r to retry."
			}
		}
	}

	if m.shouldPoll {
		return m, m.scheduleNextPoll()
	}

	return m, nil
}

func (m Model) scheduleNextPoll() tea.Cmd {
	session := m.session
	interval := m.pollInterval
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return pollLogsMsg{session: session}
	})
}

func (m Model) performSearch(query string) (Model, tea.Cmd) {
	text := string(m.content)
	if len(text) == 0 {
		m.searchMessage = "Log is empty"
		return m, nil
	}

	currentLine := m.viewport.YOffset
	startIdx := byteOffsetForLine(text, currentLine)

	idx := strings.Index(text[startIdx:], query)
	if idx == -1 && startIdx > 0 {
		idx = strings.Index(text, query)
		if idx == -1 {
			m.searchMessage = fmt.Sprintf("No match for %q", query)
			return m, nil
		}
	} else if idx >= 0 {
		idx += startIdx
	}

	if idx < 0 {
		m.searchMessage = fmt.Sprintf("No match for %q", query)
		return m, nil
	}

	line := strings.Count(text[:idx], "\n")
	m.viewport.SetYOffset(line)
	m.autoScroll = false
	m.searchActive = false
	m.searchInput.Blur()
	m.searchMessage = fmt.Sprintf("Match at line %d", line+1)

	return m, nil
}

func emitExitRequested() tea.Cmd {
	return func() tea.Msg {
		return ExitRequestedMsg{}
	}
}

func byteOffsetForLine(text string, line int) int {
	if line <= 0 {
		return 0
	}

	index := 0
	count := 0
	for index < len(text) && count < line {
		next := strings.IndexByte(text[index:], '\n')
		if next < 0 {
			return len(text)
		}
		index += next + 1
		count++
	}
	return index
}

func clamp(value, minValue int) int {
	if value < minValue {
		return minValue
	}
	return value
}
