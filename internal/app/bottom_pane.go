package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorbach/jenkins-gotui/internal/console"
	"github.com/gorbach/jenkins-gotui/internal/details"
	"github.com/gorbach/jenkins-gotui/internal/jenkins"
)

type bottomPane struct {
	active  bottomView
	details details.Model
	console console.Model
}

func newBottomPane(client *jenkins.Client) bottomPane {
	return bottomPane{
		active:  bottomViewDetails,
		details: details.New(client),
		console: console.New(client),
	}
}

func (b bottomPane) InitCmds() []tea.Cmd {
	return []tea.Cmd{
		b.details.Init(),
		b.console.Init(),
	}
}

func (b bottomPane) Active() bottomView {
	return b.active
}

func (b bottomPane) IsConsoleActive() bool {
	return b.active == bottomViewConsole
}

func (b bottomPane) View() string {
	switch b.active {
	case bottomViewConsole:
		return b.console.View()
	default:
		return b.details.View()
	}
}

func (b bottomPane) UpdateActive(msg tea.Msg) (bottomPane, tea.Cmd) {
	switch b.active {
	case bottomViewConsole:
		return b.updateConsole(msg)
	default:
		return b.updateDetails(msg)
	}
}

func (b bottomPane) UpdateDetails(msg tea.Msg) (bottomPane, tea.Cmd) {
	return b.updateDetails(msg)
}

func (b bottomPane) UpdateConsole(msg tea.Msg) (bottomPane, tea.Cmd) {
	return b.updateConsole(msg)
}

func (b bottomPane) updateDetails(msg tea.Msg) (bottomPane, tea.Cmd) {
	var cmd tea.Cmd
	b.details, cmd = b.details.Update(msg)
	return b, cmd
}

func (b bottomPane) updateConsole(msg tea.Msg) (bottomPane, tea.Cmd) {
	var cmd tea.Cmd
	b.console, cmd = b.console.Update(msg)
	return b, cmd
}

func (b bottomPane) Broadcast(msg tea.Msg) (bottomPane, []tea.Cmd) {
	var cmds []tea.Cmd

	var cmd tea.Cmd
	b.details, cmd = b.details.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	b.console, cmd = b.console.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return b, cmds
}

func (b bottomPane) Resize(width, height int) (bottomPane, []tea.Cmd) {
	var cmds []tea.Cmd
	sizeMsg := tea.WindowSizeMsg{Width: width, Height: height}

	var cmd tea.Cmd
	b.details, cmd = b.details.Update(sizeMsg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	b.console, cmd = b.console.Update(sizeMsg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return b, cmds
}

func (b bottomPane) ShowConsole() bottomPane {
	b.active = bottomViewConsole
	return b
}

func (b bottomPane) ShowDetails() (bottomPane, tea.Cmd) {
	if b.active == bottomViewDetails {
		return b, nil
	}
	var cmd tea.Cmd
	b.console, cmd = b.console.Update(console.DeactivateMsg{})
	b.active = bottomViewDetails
	return b, cmd
}
