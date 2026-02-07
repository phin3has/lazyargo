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

type diffModel struct {
	styles styles
	client argocd.Client
	app    string

	filter *argocd.ResourceRef

	width  int
	height int
	vp     viewport.Model

	loading bool
	err     error
	diffs   []argocd.DiffResult

	showWhitespace bool
}

type diffLoadedMsg struct {
	diffs []argocd.DiffResult
	err   error
}

func newDiffModel(st styles, c argocd.Client, appName string, filter *argocd.ResourceRef) diffModel {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = false
	return diffModel{styles: st, client: c, app: appName, filter: filter, vp: vp, loading: true}
}

func (m diffModel) initCmd() tea.Cmd {
	return func() tea.Msg {
		d, err := m.client.ServerSideDiff(context.Background(), m.app)
		return diffLoadedMsg{diffs: d, err: err}
	}
}

func (m *diffModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.vp.Width = max(1, w)
	m.vp.Height = max(1, h-2)
	m.vp.SetContent(m.renderBody())
}

func (m diffModel) Update(msg tea.Msg) (diffModel, tea.Cmd) {
	switch msg := msg.(type) {
	case diffLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.diffs = msg.diffs
		m.vp.SetContent(m.renderBody())
		return m, nil
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "W":
			m.showWhitespace = !m.showWhitespace
			m.vp.SetContent(m.renderBody())
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

func (m diffModel) View() string {
	filter := ""
	if m.filter != nil {
		filter = fmt.Sprintf("  [resource:%s/%s]", m.filter.Kind, m.filter.Name)
	}
	head := fmt.Sprintf("Diff: %s%s  W=whitespace  esc=close", m.app, filter)
	headStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
	return lipgloss.JoinVertical(lipgloss.Top, headStyle.Width(m.width).Render(head), m.vp.View())
}

func (m diffModel) renderBody() string {
	if m.loading {
		return "Loading…"
	}
	if m.err != nil {
		return "Error:\n\n" + m.err.Error()
	}
	if len(m.diffs) == 0 {
		return "(no diffs)"
	}

	pick := func(d argocd.DiffResult) bool {
		if m.filter == nil {
			return true
		}
		f := *m.filter
		return d.Ref.Kind == f.Kind && d.Ref.Name == f.Name && d.Ref.Namespace == f.Namespace && d.Ref.Group == f.Group
	}

	parts := make([]string, 0)
	for _, d := range m.diffs {
		if !pick(d) {
			continue
		}
		title := d.Ref.Kind + "/" + d.Ref.Name
		if d.Ref.Namespace != "" {
			title += " (" + d.Ref.Namespace + ")"
		}
		if d.Ref.Group != "" {
			title = d.Ref.Group + "/" + title
		}
		if d.Modified {
			title = m.styles.StatusWarn.Render(title + " *")
		} else {
			title = m.styles.StatusValue.Render(title)
		}
		parts = append(parts, title)
		parts = append(parts, renderUnifiedDiff(d.Diff, m.showWhitespace, m.styles))
		parts = append(parts, "")
	}
	if len(parts) == 0 {
		return "(no diffs for selected resource)"
	}
	return strings.Join(parts, "\n")
}

func renderUnifiedDiff(diff string, showWhitespace bool, st styles) string {
	if strings.TrimSpace(diff) == "" {
		return "(empty diff)"
	}
	lines := strings.Split(strings.ReplaceAll(diff, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		orig := l
		if showWhitespace {
			l = strings.ReplaceAll(l, "\t", "→\t")
			l = strings.ReplaceAll(l, " ", "·")
			_ = orig
		}
		switch {
		case strings.HasPrefix(orig, "+") && !strings.HasPrefix(orig, "+++"):
			out = append(out, lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(l))
		case strings.HasPrefix(orig, "-") && !strings.HasPrefix(orig, "---"):
			out = append(out, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(l))
		default:
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
