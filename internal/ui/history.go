package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazyargo/internal/argocd"
)

type historyModel struct {
	styles styles

	app argocd.Application

	width  int
	height int
	vp     viewport.Model

	selected int
}

func newHistoryModel(st styles, app argocd.Application) historyModel {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = false
	m := historyModel{styles: st, app: app, vp: vp}
	m.vp.SetContent(m.renderBody())
	return m
}

func (m *historyModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.vp.Width = max(1, w)
	m.vp.Height = max(1, h-2)
	m.vp.SetContent(m.renderBody())
}

func (m historyModel) Update(msg tea.Msg) (historyModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.selected > 0 {
				m.selected--
				m.vp.SetContent(m.renderBody())
				m.ensureVisible()
			}
			return m, nil
		case "down", "j":
			if m.selected < len(m.app.History)-1 {
				m.selected++
				m.vp.SetContent(m.renderBody())
				m.ensureVisible()
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m historyModel) View() string {
	head := fmt.Sprintf("History: %s  enter=details  esc=close", m.app.Name)
	headStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
	return lipgloss.JoinVertical(lipgloss.Top, headStyle.Width(m.width).Render(head), m.vp.View())
}

func (m historyModel) renderBody() string {
	if len(m.app.History) == 0 {
		lines := []string{"(no history in application status)"}
		if m.app.OperationState != nil {
			lines = append(lines, "", "Current operation:", "  phase: "+m.app.OperationState.Phase, "  message: "+m.app.OperationState.Message)
		}
		return strings.Join(lines, "\n")
	}

	lines := make([]string, 0, len(m.app.History)+5)
	if m.app.OperationState != nil {
		lines = append(lines, m.styles.StatusWarn.Render("Operation in progress: "+m.app.OperationState.Phase+" — "+m.app.OperationState.Message), "")
	}

	for i, h := range m.app.History {
		prefix := "  "
		st := m.styles.StatusValue
		if i == m.selected {
			prefix = "▶ "
			st = m.styles.SidebarSelected
		}
		when := blankIfEmpty(h.DeployedAt, "—")
		status := blankIfEmpty(h.Status, "—")
		msg := strings.TrimSpace(h.Message)
		if msg == "" {
			msg = "—"
		}
		rev := blankIfEmpty(h.Revision, "—")
		who := blankIfEmpty(h.Source, "—")
		lines = append(lines, st.Render(fmt.Sprintf("%s%s  %s  %s", prefix, when, status, rev)))
		lines = append(lines, "    "+msg)
		lines = append(lines, "    by: "+who, "")
	}
	return strings.Join(lines, "\n")
}

func (m *historyModel) ensureVisible() {
	// crude: keep the selected line near top.
	if m.selected < 0 {
		m.selected = 0
	}
	m.vp.SetYOffset(max(0, m.selected*4-2))
}
