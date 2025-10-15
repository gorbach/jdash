package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorbach/jdash/internal/console"
	"github.com/gorbach/jdash/internal/details"
	"github.com/gorbach/jdash/internal/jobs"
	"github.com/gorbach/jdash/internal/parameters"
	"github.com/gorbach/jdash/internal/queue"
	"github.com/gorbach/jdash/internal/statusbar"
)

type panelDimensions struct {
	jobsWidth, jobsHeight     int
	queueWidth, queueHeight   int
	bottomWidth, bottomHeight int
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd     tea.Cmd
		handled bool
	)

	var cmds []tea.Cmd

	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		var resizeCmd tea.Cmd
		m, resizeCmd = m.handleWindowResize(sizeMsg)
		if resizeCmd != nil {
			cmds = append(cmds, resizeCmd)
		}

		if m.modal.Active() {
			var modalCmd tea.Cmd
			m.modal, modalCmd = m.modal.Dispatch(sizeMsg)
			if modalCmd != nil {
				cmds = append(cmds, modalCmd)
			}
		}

		return m, tea.Batch(cmds...)
	}

	m.help, cmd, handled = m.help.Handle(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	if handled {
		return m, tea.Batch(cmds...)
	}

	m.modal, cmd, handled = m.modal.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	if handled {
		switch msg.(type) {
		case parameters.SubmittedMsg, parameters.CancelledMsg:
			handled = false
		}
	}
	if handled {
		return m, tea.Batch(cmds...)
	}

	switch typed := msg.(type) {
	case tea.KeyMsg:
		if handled, updated, keyCmd := m.handleGlobalKeys(typed); handled {
			if keyCmd != nil {
				cmds = append(cmds, keyCmd)
			}
			return updated, tea.Batch(cmds...)
		}

		var keyCmd tea.Cmd
		m, keyCmd = m.routeKeyToActivePanel(typed)
		if keyCmd != nil {
			cmds = append(cmds, keyCmd)
		}
		return m, tea.Batch(cmds...)

	case details.ActionRequestMsg:
		var actionCmd tea.Cmd
		m, actionCmd = m.handleActionRequest(typed)
		if actionCmd != nil {
			cmds = append(cmds, actionCmd)
		}
		return m, tea.Batch(cmds...)

	case parameters.SubmittedMsg:
		var submitCmd tea.Cmd
		m, submitCmd = m.handleParameterSubmit(typed)
		if submitCmd != nil {
			cmds = append(cmds, submitCmd)
		}
		return m, tea.Batch(cmds...)

	case parameters.CancelledMsg:
		var cancelCmd tea.Cmd
		m, cancelCmd = m.handleParameterCancel(typed)
		if cancelCmd != nil {
			cmds = append(cmds, cancelCmd)
		}
		return m, tea.Batch(cmds...)

	case console.ExitRequestedMsg:
		var exitCmd tea.Cmd
		m, exitCmd = m.handleConsoleExit()
		if exitCmd != nil {
			cmds = append(cmds, exitCmd)
		}
		return m, tea.Batch(cmds...)

	case consoleTargetResolvedMsg:
		var resolveCmd tea.Cmd
		m, resolveCmd = m.handleConsoleTargetResolved(typed)
		if resolveCmd != nil {
			cmds = append(cmds, resolveCmd)
		}
		return m, tea.Batch(cmds...)
	}

	var broadcastCmd tea.Cmd
	m, broadcastCmd = m.broadcastToAllPanels(msg)
	if broadcastCmd != nil {
		cmds = append(cmds, broadcastCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleWindowResize(msg tea.WindowSizeMsg) (Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m = m.updateHelpViewportSize()

	dims := m.calculatePanelDimensions()

	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.jobsPanel, cmd = m.jobsPanel.Update(tea.WindowSizeMsg{
		Width:  dims.jobsWidth,
		Height: dims.jobsHeight,
	})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.queuePanel, cmd = m.queuePanel.Update(tea.WindowSizeMsg{
		Width:  dims.queueWidth,
		Height: dims.queueHeight,
	})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	var bottomCmds []tea.Cmd
	m.bottom, bottomCmds = m.bottom.Resize(dims.bottomWidth, dims.bottomHeight)
	cmds = append(cmds, bottomCmds...)

	m.statusBar, cmd = m.statusBar.Update(tea.WindowSizeMsg{
		Width: msg.Width,
	})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) calculatePanelDimensions() panelDimensions {
	statusBarHeight := 1
	topPanelHeight := (m.height - statusBarHeight) * 2 / 3
	bottomPanelHeight := (m.height - statusBarHeight) - topPanelHeight
	leftPanelWidth := m.width / 2
	rightPanelWidth := m.width - leftPanelWidth

	return panelDimensions{
		jobsWidth:    leftPanelWidth - 4,
		jobsHeight:   topPanelHeight - 4,
		queueWidth:   rightPanelWidth - 4,
		queueHeight:  topPanelHeight - 4,
		bottomWidth:  m.width - 4,
		bottomHeight: bottomPanelHeight - 4,
	}
}

func (m Model) handleGlobalKeys(msg tea.KeyMsg) (bool, Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return true, m, tea.Quit

	case "tab":
		m.activePanel = (m.activePanel + 1) % 3
		return true, m, nil

	case "shift+tab":
		m.activePanel = (m.activePanel - 1 + 3) % 3
		return true, m, nil

	case "1":
		m.activePanel = PanelJobs
		return true, m, nil

	case "2":
		m.activePanel = PanelQueue
		return true, m, nil

	case "3":
		m.activePanel = PanelBottom
		return true, m, nil

	case "r":
		refreshModel, refreshCmd := m.startGlobalRefresh()
		return true, refreshModel, refreshCmd
	}
	return false, m, nil
}

func (m Model) startGlobalRefresh() (Model, tea.Cmd) {
	var cmds []tea.Cmd

	var cmd tea.Cmd
	m.statusBar, cmd = m.statusBar.Update(statusbar.RefreshStartedMsg{})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.jobsPanel, cmd = m.jobsPanel.Update(jobs.RefreshRequestedMsg{})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.queuePanel, cmd = m.queuePanel.Update(queue.RefreshRequestedMsg{})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	if m.bottom.IsConsoleActive() {
		m.bottom, cmd = m.bottom.UpdateConsole(console.RefreshRequestedMsg{})
	} else {
		m.bottom, cmd = m.bottom.UpdateDetails(details.RefreshRequestedMsg{})
	}
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) routeKeyToActivePanel(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.activePanel {
	case PanelJobs:
		var cmd tea.Cmd
		m.jobsPanel, cmd = m.jobsPanel.Update(msg)
		return m, cmd

	case PanelQueue:
		var cmd tea.Cmd
		m.queuePanel, cmd = m.queuePanel.Update(msg)
		return m, cmd

	case PanelBottom:
		var cmd tea.Cmd
		m.bottom, cmd = m.bottom.UpdateActive(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) broadcastToAllPanels(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.jobsPanel, cmd = m.jobsPanel.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.queuePanel, cmd = m.queuePanel.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	var bottomCmds []tea.Cmd
	m.bottom, bottomCmds = m.bottom.Broadcast(msg)
	cmds = append(cmds, bottomCmds...)

	m.statusBar, cmd = m.statusBar.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	switch t := msg.(type) {
	case jobs.JobsFetchedMsg:
		m.statusBar, cmd = m.statusBar.Update(statusbar.RefreshFinishedMsg{
			JobCount: len(t.Jobs),
		})
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case jobs.JobsErrorMsg:
		m.statusBar, cmd = m.statusBar.Update(statusbar.RefreshFinishedMsg{
			JobCount: -1,
			Err:      t.Err,
		})
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleActionRequest(msg details.ActionRequestMsg) (Model, tea.Cmd) {
	switch msg.Kind {
	case details.ActionKindViewParameters:
		return m.openParametersModal(msg)
	case details.ActionKindViewLogs:
		return m.openConsoleView(msg)
	default:
		return m.broadcastToAllPanels(msg)
	}
}

func (m Model) handleParameterSubmit(msg parameters.SubmittedMsg) (Model, tea.Cmd) {
	m.modal = m.modal.Clear()
	submission := details.ParameterSubmissionMsg{
		JobFullName: msg.JobFullName,
		JobName:     msg.JobName,
		Values:      cloneParameterValues(msg.Values),
	}
	return m.broadcastToAllPanels(submission)
}

func (m Model) handleParameterCancel(msg parameters.CancelledMsg) (Model, tea.Cmd) {
	m.modal = m.modal.Clear()
	return m.broadcastToAllPanels(details.ParameterCancelledMsg{JobFullName: msg.JobFullName})
}

func (m Model) openParametersModal(req details.ActionRequestMsg) (Model, tea.Cmd) {
	if len(req.ParameterDefinitions) == 0 {
		return m, nil
	}

	m.modal = m.modal.Clear()
	modal := parameters.New(req.Job.Name, req.Job.FullName, req.ParameterDefinitions)

	var cmds []tea.Cmd
	if initCmd := modal.Init(); initCmd != nil {
		cmds = append(cmds, initCmd)
	}

	m.modal = m.modal.Set(modalParameters, modal)

	if m.width > 0 && m.height > 0 {
		var sizeCmd tea.Cmd
		m.modal, sizeCmd = m.modal.Dispatch(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		if sizeCmd != nil {
			cmds = append(cmds, sizeCmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) openConsoleView(req details.ActionRequestMsg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	jobName := req.Job.Name
	if jobName == "" {
		jobName = req.Job.FullName
	}

	buildNumber := 0
	if req.Build != nil && req.Build.Number > 0 {
		buildNumber = req.Build.Number
	} else if req.Job.LastBuild != nil && req.Job.LastBuild.Number > 0 {
		buildNumber = req.Job.LastBuild.Number
	}

	buildURL := ""
	if req.Build != nil && req.Build.URL != "" {
		buildURL = req.Build.URL
	} else if req.Job.LastBuild != nil && req.Job.LastBuild.URL != "" {
		buildURL = req.Job.LastBuild.URL
	}

	if buildURL == "" && req.Job.URL != "" && buildNumber > 0 {
		trimmed := strings.TrimSuffix(req.Job.URL, "/")
		buildURL = fmt.Sprintf("%s/%d/", trimmed, buildNumber)
	}

	m.bottom = m.bottom.ShowConsole()

	openMsg := console.OpenRequestMsg{
		JobName:     jobName,
		JobFullName: req.Job.FullName,
		BuildNumber: buildNumber,
		BuildURL:    buildURL,
	}

	var consoleCmd tea.Cmd
	m.bottom, consoleCmd = m.bottom.UpdateConsole(openMsg)
	if consoleCmd != nil {
		cmds = append(cmds, consoleCmd)
	}

	if m.width > 0 && m.height > 0 {
		dims := m.calculatePanelDimensions()
		var sizeCmds []tea.Cmd
		m.bottom, sizeCmds = m.bottom.Resize(dims.bottomWidth, dims.bottomHeight)
		cmds = append(cmds, sizeCmds...)
	}

	m.async = m.async.WithTarget(req.Job.FullName, jobName, buildURL, buildNumber)
	m.activePanel = PanelBottom

	if resolveCmd := resolveConsoleTargetCmd(m.client, req.Job.FullName); resolveCmd != nil {
		cmds = append(cmds, resolveCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleConsoleExit() (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.bottom, cmd = m.bottom.ShowDetails()
	m.activePanel = PanelBottom
	return m, cmd
}

func (m Model) handleConsoleTargetResolved(msg consoleTargetResolvedMsg) (Model, tea.Cmd) {
	if m.async.JobFullName() == "" {
		return m, nil
	}

	var open *console.OpenRequestMsg
	m.async, open = m.async.ApplyResolution(msg)
	if open == nil {
		return m, nil
	}

	var cmd tea.Cmd
	m.bottom, cmd = m.bottom.UpdateConsole(*open)
	return m, cmd
}
