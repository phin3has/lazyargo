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

type eventsModel struct {
	styles styles
	client argocd.Client
	app    string

	width  int
	height int
	vp     viewport.Model

	loading bool
	err     error
	events  []argocd.Event
}

type eventsLoadedMsg struct {
	events []argocd.Event
	err    error
}

func newEventsModel(st styles, c argocd.Client, appName string) eventsModel {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = false
	return eventsModel{styles: st, client: c, app: appName, vp: vp, loading: true}
}

func (m eventsModel) initCmd() tea.Cmd {
	return func() tea.Msg {
		ev, err := m.client.ListEvents(context.Background(), m.app)
		return eventsLoadedMsg{events: ev, err: err}
	}
}

func (m *eventsModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.vp.Width = max(1, w)
	m.vp.Height = max(1, h-2)
	m.vp.SetContent(m.renderBody())
}

func (m eventsModel) Update(msg tea.Msg) (eventsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case eventsLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.events = msg.events
		m.vp.SetContent(m.renderBody())
		return m, nil
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		// parent handles esc/q
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m eventsModel) View() string {
	head := fmt.Sprintf("Events: %s  esc=close", m.app)
	headStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
	return lipgloss.JoinVertical(lipgloss.Top, headStyle.Width(m.width).Render(head), m.vp.View())
}

func (m eventsModel) renderBody() string {
	if m.loading {
		return "Loading…"
	}
	if m.err != nil {
		return "Error:\n\n" + m.err.Error()
	}
	if len(m.events) == 0 {
		return "(no events)"
	}
	lines := make([]string, 0, len(m.events))
	for _, e := range m.events {
		ts := strings.TrimSpace(e.Timestamp)
		if ts == "" {
			ts = "—"
		}
		typ := strings.TrimSpace(e.Type)
		if typ == "" {
			typ = "—"
		}
		reason := strings.TrimSpace(e.Reason)
		msg := strings.TrimSpace(e.Message)
		obj := strings.TrimSpace(e.InvolvedObject)
		line := fmt.Sprintf("%s  %-7s %-18s %s", ts, typ, reason, msg)
		if obj != "" {
			line += " (" + obj + ")"
		}

		style := m.styles.StatusValue
		if strings.EqualFold(typ, "warning") {
			style = m.styles.StatusWarn
		}
		lines = append(lines, style.Render(line))
	}
	return strings.Join(lines, "\n")
}
