package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
)

type helpOverlay struct {
	visible  bool
	viewport viewport.Model
}

func newHelpOverlay(content string) helpOverlay {
	vp := viewport.New(0, 0)
	vp.SetContent(content)
	return helpOverlay{viewport: vp}
}

func (h helpOverlay) InitCmd() tea.Cmd {
	return h.viewport.Init()
}

func (h helpOverlay) Active() bool {
	return h.visible
}

func (h helpOverlay) SetSize(width, height int) helpOverlay {
	if width > 0 {
		h.viewport.Width = width
	}
	if height > 0 {
		h.viewport.Height = height
	}
	return h
}

func (h helpOverlay) Handle(msg tea.Msg) (helpOverlay, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		if key == "?" {
			if h.visible {
				h.visible = false
			} else {
				h.visible = true
				h.viewport.GotoTop()
			}
			return h, nil, true
		}

		if !h.visible {
			return h, nil, false
		}

		switch key {
		case "esc":
			h.visible = false
			return h, nil, true
		case "ctrl+c", "q":
			return h, tea.Quit, true
		default:
			var cmd tea.Cmd
			h.viewport, cmd = h.viewport.Update(msg)
			return h, cmd, true
		}

	case tea.MouseMsg:
		if !h.visible {
			return h, nil, false
		}
		var cmd tea.Cmd
		h.viewport, cmd = h.viewport.Update(msg)
		return h, cmd, true
	}

	return h, nil, false
}
