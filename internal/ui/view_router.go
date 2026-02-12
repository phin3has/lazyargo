package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"lazyargo/internal/argocd"
	"lazyargo/internal/config"
)

// ViewID is a stable identifier for a top-level view (tab).
//
// This is intentionally simple for now; MET-65 can expand this into
// user-facing tabs, a command palette, and additional views.
type ViewID string

const (
	ViewApplication ViewID = "application"
)

// View is a lightweight interface for top-level UI screens.
//
// It is designed to be implemented by adapters around existing Bubble Tea
// models, allowing us to incrementally refactor without changing behavior.
type View interface {
	ID() ViewID
	Init() tea.Cmd
	Update(msg tea.Msg) (View, tea.Cmd)
	View() string
	SetSize(width, height int)
}

// RootModel is the top-level Bubble Tea model.
//
// For MET-64 it routes all messages to the existing Application view.
// Future work (MET-65+) can add a visible tab bar, view switching, and a
// command palette without disturbing per-view logic.
type RootModel struct {
	width  int
	height int

	active View
}

func NewRootModel(cfg config.Config, client argocd.Client) RootModel {
	app := NewApplicationView(NewModel(cfg, client))
	return RootModel{active: app}
}

func (m RootModel) Init() tea.Cmd {
	if m.active == nil {
		return nil
	}
	return m.active.Init()
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.active != nil {
			m.active.SetSize(msg.Width, msg.Height)
		}
		// The active view will already have received a WindowSizeMsg via SetSize.
		// Avoid double-delivery here.
		return m, nil
	}

	if m.active == nil {
		return m, nil
	}
	v, cmd := m.active.Update(msg)
	m.active = v
	return m, cmd
}

func (m RootModel) View() string {
	if m.active == nil {
		return ""
	}
	return m.active.View()
}
