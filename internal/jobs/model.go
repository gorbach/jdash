package jobs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorbach/jdash/internal/jenkins"
	"github.com/gorbach/jdash/internal/ui"
)

// Model represents the jobs list panel
type Model struct {
	client               jenkins.JenkinsClient
	tree                 *JobTree
	allJobs              []jenkins.Job
	list                 list.Model
	loading              bool
	spinner              spinner.Model
	err                  error
	width                int
	height               int
	searchMode           bool
	searchQuery          string
	searchInput          textinput.Model
	searchResults        []*JobTree
	searchCatalog        []*JobTree
	searchTicket         uint64
	totalSearchable      int
	preSearchSelection   string
	lastSelectedFullName string
}

// New creates a new jobs panel model
func New(client jenkins.JenkinsClient) Model {
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
	l.SetShowPagination(false)
	l.Styles.Title = ui.TitleStyle

	input := textinput.New()
	input.Placeholder = "Search jobs..."
	input.Prompt = "/ "
	input.CharLimit = 256
	input.PlaceholderStyle = ui.SubtleStyle
	input.PromptStyle = ui.HighlightStyle
	input.CursorEnd()
	input.Blur()

	return Model{
		client:      client,
		list:        l,
		loading:     true,
		spinner:     s,
		searchInput: input,
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
		m.updateListDimensions()
		return finalizeJobsModel(m, cmds)

	case JobsFetchedMsg:
		m.loading = false
		m.err = nil
		m.allJobs = msg.Jobs
		m.tree = buildTree(msg.Jobs)
		clearMatchHighlights(m.tree)
		m.searchCatalog = collectAllNodes(m.tree)
		m.totalSearchable = len(m.searchCatalog)
		m.refreshListItems()
		m.lastSelectedFullName = ""
		return finalizeJobsModel(m, cmds)

	case JobsErrorMsg:
		m.loading = false
		m.err = msg.Err
		m.tree = nil
		m.allJobs = nil
		m.list.SetItems([]list.Item{})
		return finalizeJobsModel(m, cmds)

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return finalizeJobsModel(m, cmds)

	case searchQueuedMsg:
		if msg.Ticket != m.searchTicket {
			return finalizeJobsModel(m, cmds)
		}
		m.applySearch(msg.Query)
		return finalizeJobsModel(m, cmds)

	case RefreshRequestedMsg:
		if m.client == nil {
			return finalizeJobsModel(m, cmds)
		}
		m.loading = true
		m.err = nil
		cmds = append(cmds, m.spinner.Tick)
		cmds = append(cmds, fetchJobsCmd(m.client))
		return finalizeJobsModel(m, cmds)

	case tea.KeyMsg:
		var cmd tea.Cmd
		m, cmd = m.handleKeyMsg(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return finalizeJobsModel(m, cmds)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	return finalizeJobsModel(m, cmds)
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}

	if handled, cmd := m.handleSearchKey(msg); handled {
		return m, cmd
	}

	var cmds []tea.Cmd

	if m.searchMode && m.searchInput.Focused() {
		previous := m.searchInput.Value()
		var inputCmd tea.Cmd
		m.searchInput, inputCmd = m.searchInput.Update(msg)
		if inputCmd != nil {
			cmds = append(cmds, inputCmd)
		}

		if value := m.searchInput.Value(); value != previous {
			if cmd := m.scheduleSearch(value); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

		// When search input is focused, prevent list from receiving any keyboard input
		return m, tea.Batch(cmds...)
	}

	nodes := m.currentNodes()
	if len(nodes) == 0 {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	m.ensureSelection(nodes)
	index := m.list.Index()
	if index < 0 || index >= len(nodes) {
		index = 0
	}
	currentNode := nodes[index]

	if m.isFiltering() {
		switch msg.String() {
		case "j", "down":
			m.moveCursor(1, nodes)
			return m, tea.Batch(cmds...)
		case "k", "up":
			m.moveCursor(-1, nodes)
			return m, tea.Batch(cmds...)
		case "g":
			m.selectIndex(0, nodes)
			return m, tea.Batch(cmds...)
		case "G":
			m.selectIndex(len(nodes)-1, nodes)
			return m, tea.Batch(cmds...)
		case "ctrl+d":
			m.pageMove(1, nodes)
			return m, tea.Batch(cmds...)
		case "ctrl+u":
			m.pageMove(-1, nodes)
			return m, tea.Batch(cmds...)
		case "enter":
			// Commit the selection and reveal it in the tree.
			expandPathToNode(currentNode)
			m.exitSearchMode(false)
			m.selectByFullName(currentNode.FullName)
			if !currentNode.IsFolder && currentNode.Job != nil {
				cmds = append(cmds, jobSelectedCmd(*currentNode.Job))
			}
			return m, tea.Batch(cmds...)
		}
	} else {
		switch msg.String() {
		case "h", "left":
			if currentNode.IsFolder {
				if currentNode.Expanded {
					collapseNode(currentNode)
					m.refreshListItems()
					m.selectByFullName(currentNode.FullName)
				} else if parent := currentNode.Parent; parent != nil && parent.Level >= 0 {
					m.selectNode(parent)
				}
			} else if parent := currentNode.Parent; parent != nil && parent.Level >= 0 {
				m.selectNode(parent)
			}
			return m, tea.Batch(cmds...)

		case "l", "right":
			if currentNode.IsFolder && !currentNode.Expanded {
				expandNode(currentNode)
				m.refreshListItems()
				m.selectByFullName(currentNode.FullName)
			}
			return m, tea.Batch(cmds...)

		case " ":
			if currentNode.IsFolder {
				toggleExpand(currentNode)
				m.refreshListItems()
				m.selectByFullName(currentNode.FullName)
			}
			return m, tea.Batch(cmds...)

		case "enter":
			if !currentNode.IsFolder && currentNode.Job != nil {
				cmds = append(cmds, jobSelectedCmd(*currentNode.Job))
			}
			return m, tea.Batch(cmds...)

		case "j", "down":
			m.moveCursor(1, nodes)
			return m, tea.Batch(cmds...)

		case "k", "up":
			m.moveCursor(-1, nodes)
			return m, tea.Batch(cmds...)

		case "g":
			m.selectIndex(0, nodes)
			return m, tea.Batch(cmds...)

		case "G":
			m.selectIndex(len(nodes)-1, nodes)
			return m, tea.Batch(cmds...)

		case "ctrl+d":
			m.pageMove(1, nodes)
			return m, tea.Batch(cmds...)

		case "ctrl+u":
			m.pageMove(-1, nodes)
			return m, tea.Batch(cmds...)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "/":
		if !m.searchMode {
			m.preSearchSelection = m.currentSelectionFullName()
			m.searchMode = true
			m.searchInput.SetValue(m.searchQuery)
			m.searchInput.CursorEnd()
			m.searchInput.Focus()
			m.updateListDimensions()
			return true, nil
		}
		if !m.searchInput.Focused() {
			m.searchInput.Focus()
			m.searchInput.CursorEnd()
			return true, nil
		}
		return false, nil

	case "esc":
		if m.searchMode {
			m.exitSearchMode(true)
			return true, nil
		}
		return false, nil
	}

	if !m.searchMode {
		return false, nil
	}

	switch msg.String() {
	case "enter":
		if m.searchInput.Focused() {
			m.searchInput.Blur()
			return true, nil
		}
	case "n":
		if !m.searchInput.Focused() && len(m.searchResults) > 0 {
			m.moveCursor(1, m.searchResults)
			return true, nil
		}
	case "N":
		if !m.searchInput.Focused() && len(m.searchResults) > 0 {
			m.moveCursor(-1, m.searchResults)
			return true, nil
		}
	}

	return false, nil
}

func (m *Model) exitSearchMode(restorePrevious bool) {
	m.searchTicket++
	m.searchMode = false
	m.searchInput.Blur()
	m.searchInput.SetValue("")
	m.applySearch("")
	m.updateListDimensions()
	if restorePrevious && m.preSearchSelection != "" {
		m.selectByFullName(m.preSearchSelection)
	}
	m.preSearchSelection = ""
}

func (m *Model) scheduleSearch(raw string) tea.Cmd {
	normalized := strings.TrimSpace(raw)
	m.searchTicket++
	ticket := m.searchTicket

	if normalized == "" {
		m.applySearch("")
		return nil
	}

	return debounceSearchCmd(normalized, ticket)
}

func (m *Model) applySearch(query string) {
	clearMatchHighlights(m.tree)
	m.searchQuery = strings.TrimSpace(query)
	m.searchResults = nil

	if m.searchQuery == "" {
		m.refreshListItems()
		return
	}

	matches := runFuzzySearch(m.searchQuery, m.searchCatalog)
	if len(matches) == 0 {
		m.refreshListItems()
		return
	}

	m.searchResults = make([]*JobTree, len(matches))
	for i, result := range matches {
		result.node.MatchIndexes = result.indexes
		result.node.SearchResult = true
		m.searchResults[i] = result.node
	}

	m.refreshListItems()
	m.list.Select(0)
}

func (m Model) isFiltering() bool {
	return m.searchQuery != ""
}

func (m Model) currentNodes() []*JobTree {
	if m.isFiltering() {
		return m.searchResults
	}
	return flattenVisibleNodes(m.tree)
}

func (m *Model) currentSelectionFullName() string {
	nodes := m.currentNodes()
	idx := m.list.Index()
	if idx >= 0 && idx < len(nodes) {
		return nodes[idx].FullName
	}
	return ""
}

func (m *Model) shouldShowSearchBar() bool {
	return m.searchMode
}

// InSearchMode reports whether the jobs list is currently in search mode.
func (m Model) InSearchMode() bool {
	return m.searchMode
}

func (m *Model) updateListDimensions() {
	height := m.height
	if m.shouldShowSearchBar() && height > 0 {
		height--
	}
	if height < 0 {
		height = 0
	}
	m.list.SetSize(m.width, height)
}

// refreshListItems updates the list with current visible tree nodes
func (m *Model) refreshListItems() {
	var nodes []*JobTree
	if m.isFiltering() {
		nodes = m.searchResults
	} else if m.tree != nil {
		nodes = flattenVisibleNodes(m.tree)
	}

	if len(nodes) == 0 {
		m.list.SetItems([]list.Item{})
		return
	}

	items := make([]list.Item, len(nodes))
	for i, node := range nodes {
		items[i] = *node
	}
	m.list.SetItems(items)

	m.ensureSelection(nodes)
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
	if m.isFiltering() || fullName == "" || m.tree == nil {
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
	if m.isFiltering() || target == nil || m.tree == nil {
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

func finalizeJobsModel(m Model, cmds []tea.Cmd) (Model, tea.Cmd) {
	if cmd := (&m).selectionChangedCmd(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) currentSelectionNode() *JobTree {
	nodes := m.currentNodes()
	idx := m.list.Index()
	if idx >= 0 && idx < len(nodes) {
		return nodes[idx]
	}
	return nil
}

func (m *Model) selectionChangedCmd() tea.Cmd {
	node := m.currentSelectionNode()
	if node == nil || node.Job == nil || node.IsFolder {
		if m.lastSelectedFullName == "" {
			return nil
		}
		m.lastSelectedFullName = ""
		return jobSelectionClearedCmd()
	}

	if node.FullName == m.lastSelectedFullName {
		return nil
	}

	m.lastSelectedFullName = node.FullName
	return jobSelectedCmd(*node.Job)
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

	content := m.list.View()
	if m.isFiltering() && len(m.searchResults) == 0 {
		content = ui.SubtleStyle.Render("No matches found")
	}

	if m.shouldShowSearchBar() {
		matchCount := m.totalSearchable
		if m.isFiltering() {
			matchCount = len(m.searchResults)
		}
		status := ui.SubtleStyle.Render(fmt.Sprintf("%d/%d matches", matchCount, m.totalSearchable))
		searchLine := fmt.Sprintf("%s  %s", m.searchInput.View(), status)
		content = strings.TrimRight(content, "\n")
		content = content + "\n" + searchLine
	}

	return content
}
