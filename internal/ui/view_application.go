package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// ApplicationView adapts the existing ui.Model (Application screen) to the new
// View interface.
//
// This keeps behavior unchanged while allowing RootModel to route between
// multiple views in later milestones.
type ApplicationView struct {
	m Model
}

func NewApplicationView(m Model) ApplicationView {
	return ApplicationView{m: m}
}

func (v ApplicationView) ID() ViewID { return ViewApplication }

func (v ApplicationView) Init() tea.Cmd { return v.m.Init() }

func (v ApplicationView) Update(msg tea.Msg) (View, tea.Cmd) {
	updated, cmd := v.m.Update(msg)
	// The underlying model should always remain a ui.Model.
	if um, ok := updated.(Model); ok {
		v.m = um
		return v, cmd
	}
	// Fallback: keep current state if something unexpected happened.
	return v, cmd
}

func (v ApplicationView) View() string { return v.m.View() }

func (v ApplicationView) SetSize(width, height int) {
	// Reuse the existing size handling without reaching into unexported fields.
	_, _ = v.m.Update(tea.WindowSizeMsg{Width: width, Height: height})
}
