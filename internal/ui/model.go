package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazyargo/internal/argocd"
	"lazyargo/internal/config"
)

type Model struct {
	cfg    config.Config
	client argocd.Client

	styles styles
	keys   keyMap
	help   help.Model

	width  int
	height int

	apps     []argocd.Application
	selected int

	statusLine string
	err        error
}

func NewModel(cfg config.Config, client argocd.Client) Model {
	h := help.New()
	h.ShowAll = false

	m := Model{
		cfg:    cfg,
		client: client,
		styles: newStyles(),
		keys:   newKeyMap(),
		help:   h,
	}
	return m
}

func (m Model) Init() tea.Cmd {
	// Initial data load.
	return m.refreshCmd()
}

type appsMsg struct {
	apps []argocd.Application
	err  error
}

func (m Model) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		apps, err := m.client.ListApplications(context.Background())
		return appsMsg{apps: apps, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case appsMsg:
		m.err = msg.err
		if msg.err == nil {
			m.apps = msg.apps
			if m.selected >= len(m.apps) {
				m.selected = max(0, len(m.apps)-1)
			}
			m.statusLine = fmt.Sprintf("loaded %d apps", len(m.apps))
		} else {
			m.statusLine = "failed to load apps"
		}
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		case key.Matches(msg, m.keys.Refresh):
			m.statusLine = "refreshing…"
			return m, m.refreshCmd()
		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.apps)-1 {
				m.selected++
			}
			return m, nil
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	header := m.styles.Header.Width(m.width).Render("lazyArgo")

	helpView := m.help.View(m.keys)
	helpBar := m.styles.HelpBar.Width(m.width).Render(helpView)

	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(helpBar)
	if bodyHeight < 0 {
		bodyHeight = 0
	}

	sidebarWidth := m.cfg.UI.SidebarWidth
	if sidebarWidth < 20 {
		sidebarWidth = 20
	}
	mainWidth := m.width - sidebarWidth
	if mainWidth < 20 {
		mainWidth = 20
		sidebarWidth = max(20, m.width-mainWidth)
	}

	sidebar := m.renderSidebar(sidebarWidth, bodyHeight)
	main := m.renderMain(mainWidth, bodyHeight)

	row := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)

	return lipgloss.JoinVertical(lipgloss.Top, header, row, helpBar)
}

func (m Model) renderSidebar(w, h int) string {
	title := m.styles.SidebarTitle.Render("Applications")
	lines := []string{title, strings.Repeat("─", max(0, w-2))}

	if m.err != nil {
		lines = append(lines, m.styles.Error.Render(m.err.Error()))
	}

	for i, a := range m.apps {
		name := a.Name
		if i == m.selected {
			lines = append(lines, m.styles.SidebarSelected.Render("▶ "+name))
		} else {
			lines = append(lines, m.styles.SidebarItem.Render("  "+name))
		}
	}

	content := strings.Join(lines, "\n")
	return m.styles.Sidebar.Width(w).Height(h).Render(content)
}

func (m Model) renderMain(w, h int) string {
	var content string
	if len(m.apps) == 0 {
		content = "No applications. Press 'r' to refresh."
		if m.statusLine != "" {
			content += "\n\n" + m.statusLine
		}
		return m.styles.Main.Width(w).Height(h).Render(content)
	}

	app := m.apps[m.selected]
	content = fmt.Sprintf(
		"Name:      %s\nNamespace: %s\nProject:   %s\nHealth:    %s\nSync:      %s\nRepo:      %s\nPath:      %s\nRevision:  %s\nCluster:   %s\n\n%s",
		app.Name,
		app.Namespace,
		app.Project,
		app.Health,
		app.Sync,
		blankIfEmpty(app.RepoURL, "—"),
		blankIfEmpty(app.Path, "—"),
		blankIfEmpty(app.Revision, "—"),
		blankIfEmpty(app.Cluster, "—"),
		m.statusLine,
	)

	return m.styles.Main.Width(w).Height(h).Render(content)
}

func blankIfEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
