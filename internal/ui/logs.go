package ui

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazyargo/internal/argocd"
)

type logsModel struct {
	styles styles
	client argocd.Client

	appName string
	podName string

	container string
	follow    bool
	wrap      bool

	width  int
	height int
	vp     viewport.Model

	lines []string
	err   error

	searchMode bool
	searchIn   textinput.Model
	searchQ    string

	streamCancel context.CancelFunc
	streamCh     chan tea.Msg
	streamOn     bool
}

type logLineMsg struct{ line string }

type logErrMsg struct{ err error }

type logDoneMsg struct{}

func newLogsModel(st styles, c argocd.Client, appName, podName string) logsModel {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = false

	ti := textinput.New()
	ti.Placeholder = "search"
	ti.Prompt = "/ "
	ti.CharLimit = 128
	ti.Width = 40

	return logsModel{
		styles:   st,
		client:   c,
		appName:  appName,
		podName:  podName,
		follow:   true,
		wrap:     false,
		vp:       vp,
		searchIn: ti,
		lines:    nil,
	}
}

func (m logsModel) initCmd() tea.Cmd {
	return tea.Batch(m.startStreamCmd(), m.waitStreamMsgCmd())
}

func (m *logsModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.vp.Width = max(1, w)
	m.vp.Height = max(1, h-2)
	m.vp.SetContent(m.renderBody())
}

func (m logsModel) Update(msg tea.Msg) (logsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil
	case logLineMsg:
		m.lines = append(m.lines, msg.line)
		m.vp.SetContent(m.renderBody())
		if m.follow {
			m.vp.GotoBottom()
		}
		return m, m.waitStreamMsgCmd()
	case logErrMsg:
		m.err = msg.err
		m.vp.SetContent(m.renderBody())
		return m, nil
	case logDoneMsg:
		m.streamOn = false
		return m, nil
	case tea.KeyMsg:
		if m.searchMode {
			switch msg.String() {
			case "enter":
				m.searchQ = strings.TrimSpace(m.searchIn.Value())
				m.searchMode = false
				m.searchIn.Blur()
				m.vp.SetContent(m.renderBody())
				m.jumpToMatch(true)
				return m, nil
			case "esc":
				m.searchMode = false
				m.searchIn.SetValue("")
				m.searchIn.Blur()
				m.vp.SetContent(m.renderBody())
				return m, nil
			}
			var cmd tea.Cmd
			m.searchIn, cmd = m.searchIn.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "f":
			m.follow = !m.follow
			m.vp.SetContent(m.renderBody())
			// Restart stream if turning follow on.
			if m.follow && !m.streamOn {
				return m, tea.Batch(m.startStreamCmd(), m.waitStreamMsgCmd())
			}
			return m, nil
		case "w":
			m.wrap = !m.wrap
			m.vp.SetContent(m.renderBody())
			return m, nil
		case "/":
			m.searchMode = true
			m.searchIn.SetValue("")
			m.searchIn.Focus()
			m.vp.SetContent(m.renderBody())
			return m, nil
		case "n":
			m.jumpToMatch(false)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m logsModel) View() string {
	head := fmt.Sprintf("Logs: %s/%s  [container:%s]  [follow:%v]  [wrap:%v]  f=follow  w=wrap  /=search  n=next  esc=close",
		m.appName, m.podName, blankIfEmpty(m.container, "default"), m.follow, m.wrap)
	headStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
	return lipgloss.JoinVertical(lipgloss.Top, headStyle.Width(m.width).Render(head), m.vp.View())
}

func (m logsModel) renderBody() string {
	if m.err != nil {
		return "Error:\n\n" + m.err.Error()
	}

	head := ""
	if m.searchMode {
		head = "Search: " + m.searchIn.View() + "\n\n"
	} else if m.searchQ != "" {
		head = "Search: " + m.searchQ + " (n=next, /=new)\n\n"
	}

	if len(m.lines) == 0 {
		return head + "(no log lines yet)"
	}

	if !m.wrap {
		return head + strings.Join(m.lines, "\n")
	}

	// naive wrap: insert newlines at width.
	wrapped := make([]string, 0, len(m.lines))
	maxW := max(20, m.width-2)
	for _, l := range m.lines {
		for len(l) > maxW {
			wrapped = append(wrapped, l[:maxW])
			l = l[maxW:]
		}
		wrapped = append(wrapped, l)
	}
	return head + strings.Join(wrapped, "\n")
}

func (m *logsModel) jumpToMatch(fromTop bool) {
	q := strings.ToLower(strings.TrimSpace(m.searchQ))
	if q == "" {
		return
	}
	start := 0
	if !fromTop {
		start = m.vp.YOffset + 1
	}
	for i := start; i < len(m.lines); i++ {
		if strings.Contains(strings.ToLower(m.lines[i]), q) {
			m.vp.SetYOffset(i)
			return
		}
	}
}

func (m *logsModel) startStreamCmd() tea.Cmd {
	// Stop any existing stream.
	if m.streamCancel != nil {
		m.streamCancel()
		m.streamCancel = nil
	}
	m.streamCh = make(chan tea.Msg, 100)
	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel
	m.streamOn = true

	app := m.appName
	pod := m.podName
	container := m.container
	follow := m.follow
	c := m.client
	ch := m.streamCh

	return func() tea.Msg {
		go func() {
			rc, err := c.PodLogs(ctx, app, pod, container, follow)
			if err != nil {
				ch <- logErrMsg{err: err}
				close(ch)
				return
			}
			defer rc.Close()

			s := bufio.NewScanner(rc)
			for s.Scan() {
				select {
				case <-ctx.Done():
					close(ch)
					return
				default:
				}
				ch <- logLineMsg{line: s.Text()}
			}
			if err := s.Err(); err != nil {
				ch <- logErrMsg{err: err}
			} else {
				ch <- logDoneMsg{}
			}
			close(ch)
		}()
		return nil
	}
}

func (m logsModel) waitStreamMsgCmd() tea.Cmd {
	ch := m.streamCh
	return func() tea.Msg {
		if ch == nil {
			return nil
		}
		msg, ok := <-ch
		if !ok {
			return logDoneMsg{}
		}
		return msg
	}
}
