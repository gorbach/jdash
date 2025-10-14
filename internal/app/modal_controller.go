package app

import tea "github.com/charmbracelet/bubbletea"

type modalController struct {
	kind  modalType
	model tea.Model
}

func (mc modalController) Active() bool {
	return mc.kind != modalNone && mc.model != nil
}

func (mc modalController) Clear() modalController {
	mc.kind = modalNone
	mc.model = nil
	return mc
}

func (mc modalController) Set(kind modalType, model tea.Model) modalController {
	mc.kind = kind
	mc.model = model
	return mc
}

func (mc modalController) View() string {
	if !mc.Active() {
		return ""
	}
	return mc.model.View()
}

func (mc modalController) Update(msg tea.Msg) (modalController, tea.Cmd, bool) {
	if !mc.Active() {
		return mc, nil, false
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "q":
			return mc, tea.Quit, true
		}
	}

	var cmd tea.Cmd
	mc.model, cmd = mc.model.Update(msg)
	return mc, cmd, true
}

func (mc modalController) Dispatch(msg tea.Msg) (modalController, tea.Cmd) {
	if !mc.Active() {
		return mc, nil
	}
	var cmd tea.Cmd
	mc.model, cmd = mc.model.Update(msg)
	return mc, cmd
}
