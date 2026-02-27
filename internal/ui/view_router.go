package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/phin3has/lazyargo/internal/argocd"
	"github.com/phin3has/lazyargo/internal/config"
)

// ViewID is a stable identifier for a top-level view (tab).
type ViewID string

const (
	ViewApplication ViewID = "application"
	ViewProjects    ViewID = "projects"
	ViewRepos       ViewID = "repos"
	ViewClusters    ViewID = "clusters"
	ViewSettings    ViewID = "settings"
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
// MET-64 introduced the initial view router scaffold.
// MET-65 expands it into a minimal navigation framework with a command palette
// and placeholder views (without disturbing the existing Application view).
type RootModel struct {
	width  int
	height int

	views  map[ViewID]View
	order  []ViewID
	active ViewID

	palette     CommandPalette
	paletteOpen bool
}

func NewRootModel(cfg config.Config, client argocd.Client) RootModel {
	app := NewApplicationView(NewModel(cfg, client))

	views := map[ViewID]View{
		ViewApplication: app,
		ViewProjects:    NewPlaceholderView(ViewProjects, "Projects"),
		ViewRepos:       NewPlaceholderView(ViewRepos, "Repositories"),
		ViewClusters:    NewPlaceholderView(ViewClusters, "Clusters"),
		ViewSettings:    NewPlaceholderView(ViewSettings, "Settings"),
	}

	order := []ViewID{ViewApplication, ViewProjects, ViewRepos, ViewClusters, ViewSettings}

	items := []CommandItem{
		{Title: "Application", ID: ViewApplication},
		{Title: "Projects", ID: ViewProjects},
		{Title: "Repositories", ID: ViewRepos},
		{Title: "Clusters", ID: ViewClusters},
		{Title: "Settings", ID: ViewSettings},
	}

	return RootModel{
		views:   views,
		order:   order,
		active:  ViewApplication,
		palette: NewCommandPalette(items),
	}
}

func (m RootModel) Init() tea.Cmd {
	if v := m.views[m.active]; v != nil {
		return v.Init()
	}
	return nil
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Update active view size.
		if v := m.views[m.active]; v != nil {
			v.SetSize(msg.Width, msg.Height)
			m.views[m.active] = v
		}
		// Keep palette sized too.
		m.palette, _, _ = m.palette.Update(msg)
		return m, nil
	case tea.KeyMsg:
		// Global toggle for command palette. Use ctrl+p to avoid stealing the
		// existing application-level `tab` behavior.
		if msg.String() == "ctrl+p" && !m.paletteOpen {
			m.paletteOpen = true
			return m, nil
		}
		if m.paletteOpen {
			p, chosen, close := m.palette.Update(msg)
			m.palette = p
			if chosen != nil {
				m = m.switchTo(*chosen)
			}
			if close {
				m.paletteOpen = false
			}
			return m, nil
		}
	}

	// Route to active view.
	v := m.views[m.active]
	if v == nil {
		return m, nil
	}
	updated, cmd := v.Update(msg)
	m.views[m.active] = updated
	return m, cmd
}

func (m RootModel) View() string {
	v := m.views[m.active]
	if v == nil {
		return ""
	}

	base := v.View()
	if !m.paletteOpen {
		return base
	}

	// Simple overlay: render the palette after the active view.
	return base + "\n\n" + m.palette.View()
}

func (m RootModel) switchTo(id ViewID) RootModel {
	if _, ok := m.views[id]; !ok {
		return m
	}
	m.active = id
	if m.width > 0 && m.height > 0 {
		v := m.views[m.active]
		v.SetSize(m.width, m.height)
		m.views[m.active] = v
	}
	return m
}
