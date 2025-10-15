package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorbach/jdash/internal/jenkins"
	"github.com/gorbach/jdash/internal/jobs"
	"github.com/gorbach/jdash/internal/queue"
	"github.com/gorbach/jdash/internal/statusbar"
)

// PanelID represents which panel is active.
type PanelID int

const (
	PanelJobs PanelID = iota
	PanelQueue
	PanelBottom
)

type modalType int

const (
	modalNone modalType = iota
	modalParameters
)

type bottomView int

const (
	bottomViewDetails bottomView = iota
	bottomViewConsole
)

var dimContentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

const (
	helpViewportMinWidth  = 30
	helpViewportMaxWidth  = 60
	helpViewportMinHeight = 12
)

const helpContent = `Key Bindings

Global
  q        quit application
  r        refresh all data
  ?        toggle this help
  Tab      next panel
  1-3      jump to panel

Jobs List (Panel 1)
  Up/k     move up
  Down/j   move down
  Left/h   collapse node
  Right/l  expand node
  Space    toggle expand
  Enter    view details
  g/G      top/bottom
  /        search
  b        build now

Build Info (Panel 3)
  b        build now / configure
  l        view logs
  p        parameters (if available)
  c        view config
  r        refresh details
  H        build history
  a        abort running build

[Press ? or Esc to close]
`

// Model is the root Bubble Tea model for the application.
type Model struct {
	activePanel PanelID

	width  int
	height int

	serverURL string
	client    *jenkins.Client

	jobsPanel  jobs.Model
	queuePanel queue.Model
	bottom     bottomPane
	statusBar  statusbar.Model

	help  helpOverlay
	modal modalController
	async consoleTargetTracker
}

// New creates a new application model.
func New(serverURL string, client *jenkins.Client) Model {
	help := newHelpOverlay(helpContent)
	bottom := newBottomPane(client)

	return Model{
		activePanel: PanelJobs,
		serverURL:   serverURL,
		client:      client,
		jobsPanel:   jobs.New(client),
		queuePanel:  queue.New(client),
		bottom:      bottom,
		statusBar:   statusbar.New(serverURL),
		help:        help,
	}
}

// Init initialises all child models and viewports.
func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	cmds = append(cmds,
		m.jobsPanel.Init(),
		m.queuePanel.Init(),
		m.statusBar.Init(),
		m.help.InitCmd(),
	)

	for _, cmd := range m.bottom.InitCmds() {
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}
