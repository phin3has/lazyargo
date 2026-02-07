package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazyargo/internal/argocd"
)

type revisionDetailsModel struct {
	styles styles
	client argocd.Client

	appName  string
	revision string

	width  int
	height int
	vp     viewport.Model

	loading bool
	err     error

	meta  argocd.RevisionMeta
	chart argocd.ChartMeta
}

type revisionDetailsLoadedMsg struct {
	meta  argocd.RevisionMeta
	chart argocd.ChartMeta
	err   error
}

func newRevisionDetailsModel(st styles, c argocd.Client, appName, revision string) revisionDetailsModel {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = false
	return revisionDetailsModel{styles: st, client: c, appName: appName, revision: revision, vp: vp, loading: true}
}

func (m revisionDetailsModel) initCmd() tea.Cmd {
	return func() tea.Msg {
		meta, err := m.client.RevisionMetadata(context.Background(), m.appName, m.revision)
		if err != nil {
			return revisionDetailsLoadedMsg{err: err}
		}
		chart, err := m.client.ChartDetails(context.Background(), m.appName, m.revision)
		if err != nil {
			return revisionDetailsLoadedMsg{meta: meta, err: err}
		}
		return revisionDetailsLoadedMsg{meta: meta, chart: chart, err: nil}
	}
}

func (m *revisionDetailsModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.vp.Width = max(1, w)
	m.vp.Height = max(1, h-2)
	m.vp.SetContent(m.renderBody())
}

func (m revisionDetailsModel) Update(msg tea.Msg) (revisionDetailsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case revisionDetailsLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.meta = msg.meta
		m.chart = msg.chart
		m.vp.SetContent(m.renderBody())
		return m, nil
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m revisionDetailsModel) View() string {
	head := fmt.Sprintf("Revision: %s  esc=close", m.revision)
	headStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
	return lipgloss.JoinVertical(lipgloss.Top, headStyle.Width(m.width).Render(head), m.vp.View())
}

func (m revisionDetailsModel) renderBody() string {
	if m.loading {
		return "Loading…"
	}
	if m.err != nil {
		return "Error:\n\n" + m.err.Error()
	}

	lines := []string{
		"Metadata:",
		"  author:  " + blankIfEmpty(strings.TrimSpace(m.meta.Author), "—"),
		"  date:    " + blankIfEmpty(strings.TrimSpace(m.meta.Date), "—"),
		"  tags:    " + blankIfEmpty(strings.Join(m.meta.Tags, ", "), "—"),
		"  message: " + blankIfEmpty(strings.TrimSpace(m.meta.Message), "—"),
		"",
		"Chart:",
		"  description: " + blankIfEmpty(strings.TrimSpace(m.chart.Description), "—"),
		"  maintainers: " + blankIfEmpty(strings.Join(m.chart.Maintainers, ", "), "—"),
		"  home:        " + blankIfEmpty(strings.TrimSpace(m.chart.Home), "—"),
	}
	return strings.Join(lines, "\n")
}
