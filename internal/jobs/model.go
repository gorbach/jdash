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

		visibleNodes := flattenVisibleNodes(m.tree)
		if len(visibleNodes) == 0 {
			// Nothing to navigate
			return m, nil
		}

		// Ensure we have a valid selection before handling keys
		m.ensureSelection(visibleNodes)
		currentIdx := m.list.Index()
		if currentIdx < 0 || currentIdx >= len(visibleNodes) {
			currentIdx = 0
		}
		currentNode := visibleNodes[currentIdx]

		switch msg.String() {
		case "h", "left":
			if currentNode.IsFolder {
				if currentNode.Expanded {
					// Collapse expanded folder
					collapseNode(currentNode)
					m.refreshListItems()
					m.selectByFullName(currentNode.FullName)
				} else if parent := currentNode.Parent; parent != nil && parent.Level >= 0 {
					// Move to parent when already collapsed
					m.selectNode(parent)
				}
			} else if parent := currentNode.Parent; parent != nil && parent.Level >= 0 {
				// Move from job to its parent folder
				m.selectNode(parent)
			}
			return m, nil

		case "l", "right":
			// Expand node if collapsed
			if currentNode.IsFolder && !currentNode.Expanded {
				expandNode(currentNode)
				m.refreshListItems()
				m.selectByFullName(currentNode.FullName)
			}
			return m, nil

		case " ":
			// Toggle expand/collapse
			if currentNode.IsFolder {
				toggleExpand(currentNode)
				m.refreshListItems()
				m.selectByFullName(currentNode.FullName)
			}
			return m, nil

		case "enter":
			// Select job and show details (TODO: implement in Story 6)
			return m, nil

		case "j", "down":
			m.moveCursor(1, visibleNodes)
			return m, nil

		case "k", "up":
			m.moveCursor(-1, visibleNodes)
			return m, nil

		case "g":
			m.selectIndex(0, visibleNodes)
			return m, nil

		case "G":
			m.selectIndex(len(visibleNodes)-1, visibleNodes)
			return m, nil

		case "ctrl+d":
			m.pageMove(1, visibleNodes)
			return m, nil

		case "ctrl+u":
			m.pageMove(-1, visibleNodes)
			return m, nil
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
	if len(visibleNodes) == 0 {
		m.list.SetItems([]list.Item{})
		return
	}

	items := make([]list.Item, len(visibleNodes))
	for i, node := range visibleNodes {
		items[i] = *node
	}
	m.list.SetItems(items)

	m.ensureSelection(visibleNodes)
}

// ensureSelection resets the cursor if it's outside the visible nodes range.
func (m *Model) ensureSelection(visibleNodes []*JobTree) {
	if len(visibleNodes) == 0 {
		return
	}

	idx := m.list.Index()
	if idx < 0 || idx >= len(visibleNodes) {
		m.list.Select(0)
	}
}

// selectByFullName re-selects a node by its full name after the tree structure changes.
func (m *Model) selectByFullName(fullName string) {
	if fullName == "" || m.tree == nil {
		return
	}

	nodes := flattenVisibleNodes(m.tree)
	for idx, node := range nodes {
		if node.FullName == fullName {
			m.list.Select(idx)
			return
		}
	}

	if len(nodes) > 0 {
		m.list.Select(len(nodes) - 1)
	}
}

// selectNode selects the given node if it is currently visible.
func (m *Model) selectNode(target *JobTree) {
	if target == nil || m.tree == nil {
		return
	}

	nodes := flattenVisibleNodes(m.tree)
	for idx, node := range nodes {
		if node == target {
			m.list.Select(idx)
			return
		}
	}
}

// selectIndex moves the selection to an absolute index with wrap-around support.
func (m *Model) selectIndex(target int, nodes []*JobTree) {
	count := len(nodes)
	if count == 0 {
		return
	}

	if target < 0 {
		target = (target%count + count) % count
	}
	if target >= count {
		target = target % count
	}
	m.list.Select(target)
}

// moveCursor moves the cursor by delta positions with wrap-around.
func (m *Model) moveCursor(delta int, nodes []*JobTree) {
	if len(nodes) == 0 {
		return
	}

	idx := m.list.Index()
	if idx < 0 || idx >= len(nodes) {
		idx = 0
	}

	idx = (idx + delta) % len(nodes)
	if idx < 0 {
		idx += len(nodes)
	}
	m.list.Select(idx)
}

// pageMove performs half-page navigation similar to vim's ctrl+d / ctrl+u.
func (m *Model) pageMove(direction int, nodes []*JobTree) {
	if len(nodes) == 0 || direction == 0 {
		return
	}

	idx := m.list.Index()
	if idx < 0 || idx >= len(nodes) {
		idx = 0
	}

	step := len(m.list.VisibleItems()) / 2
	if step <= 0 {
		step = m.height / 2
	}
	if step <= 0 {
		step = 1
	}

	idx += direction * step
	if idx < 0 {
		idx = 0
	}
	if idx >= len(nodes) {
		idx = len(nodes) - 1
	}
	m.list.Select(idx)
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
