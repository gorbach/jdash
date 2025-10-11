package jobs

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorbach/jenkins-gotui/internal/jenkins"
	"github.com/gorbach/jenkins-gotui/internal/ui"
)

// Model represents the jobs list panel
type Model struct {
	client  *jenkins.Client
	tree    *JobTree
	allJobs []jenkins.Job
	list    list.Model
	loading bool
	spinner spinner.Model
	err     error
	width   int
	height  int
}

// New creates a new jobs panel model
func New(client *jenkins.Client) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = ui.BuildingStyle

	// Create empty list with custom delegate
	delegate := newJobDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Jobs"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = ui.TitleStyle

	return Model{
		client:  client,
		list:    l,
		loading: true,
		spinner: s,
	}
}

// Init initializes the model and starts fetching jobs
func (m Model) Init() tea.Cmd {
	if m.client == nil {
		return nil
	}
	return tea.Batch(
		m.spinner.Tick,
		fetchJobsCmd(m.client),
	)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update list size to fill available space
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil

	case JobsFetchedMsg:
		m.loading = false
		m.allJobs = msg.Jobs
		m.tree = buildTree(msg.Jobs)

		// Convert tree to list items
		m.refreshListItems()
		return m, nil

	case JobsErrorMsg:
		m.loading = false
		m.err = msg.Err
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		// Don't process keys while loading
		if m.loading {
			return m, nil
		}

		// Handle tree-specific keys first
		if m.tree != nil {
			visibleNodes := flattenVisibleNodes(m.tree)
			if len(visibleNodes) > 0 && m.list.Index() < len(visibleNodes) {
				currentNode := visibleNodes[m.list.Index()]

				switch msg.String() {
				case "h", "left":
					// Collapse node
					if currentNode.IsFolder && currentNode.Expanded {
						collapseNode(currentNode)
						m.refreshListItems()
						return m, nil
					}

				case "l", "right":
					// Expand node
					if currentNode.IsFolder && !currentNode.Expanded {
						expandNode(currentNode)
						m.refreshListItems()
						return m, nil
					}

				case " ":
					// Toggle expand/collapse
					if currentNode.IsFolder {
						toggleExpand(currentNode)
						m.refreshListItems()
						return m, nil
					}

				case "enter":
					// Select job and show details (TODO: implement in Story 6)
					return m, nil
				}
			}
		}
	}

	// Forward all other messages to the list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// refreshListItems updates the list with current visible tree nodes
func (m *Model) refreshListItems() {
	if m.tree == nil {
		m.list.SetItems([]list.Item{})
		return
	}

	visibleNodes := flattenVisibleNodes(m.tree)
	items := make([]list.Item, len(visibleNodes))
	for i, node := range visibleNodes {
		items[i] = *node
	}
	m.list.SetItems(items)
}

// View renders the jobs panel
func (m Model) View() string {
	if m.loading {
		return fmt.Sprintf("%s Loading jobs...", m.spinner.View())
	}

	if m.err != nil {
		title := ui.ErrorStyle.Render("Error loading jobs")
		errMsg := ui.SubtleStyle.Render(m.err.Error())
		help := ui.SubtleStyle.Render("\nPress 'r' to retry")
		return title + "\n\n" + errMsg + help
	}

	if m.tree == nil || len(m.allJobs) == 0 {
		title := ui.SubtleStyle.Render("No jobs found")
		help := ui.SubtleStyle.Render("\nPress 'r' to refresh")
		return title + help
	}

	// Update title with job count
	totalJobs := getTotalJobCount(m.tree)
	m.list.Title = fmt.Sprintf("Jobs (%d)", totalJobs)

	// Let bubbles/list handle all rendering
	return m.list.View()
}
