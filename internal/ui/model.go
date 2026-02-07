package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
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

	appsAll       []argocd.Application
	apps          []argocd.Application
	selected      int
	sidebarOffset int

	filterInput  textinput.Model
	filterActive bool
	driftOnly    bool

	deleteModal   bool
	deleteApp     string
	deleteCascade bool
	deleteInput   textinput.Model

	sortMode sortMode

	serverLabel string
	lastRefresh time.Time

	syncModal          bool
	syncTargets        []string
	syncPreview        map[string][]argocd.Resource // drifted resources snapshot
	syncDryRunComplete bool
	syncDryRunResults  []syncResult

	rollbackModal    bool
	rollbackApp      string
	rollbackLoading  bool
	rollbackErr      error
	rollbackRevs     []argocd.Revision
	rollbackSelected int
	rollbackConfirm  bool

	terminateModal   bool
	terminateApp     string
	terminateLoading bool
	terminateErr     error
	terminateConfirm bool

	detail     *argocd.Application
	detailErr  error
	statusLine string
	err        error
}

type sortMode int

const (
	sortByName sortMode = iota
	sortByHealth
	sortBySync
)

func (s sortMode) String() string {
	switch s {
	case sortByHealth:
		return "health"
	case sortBySync:
		return "sync"
	default:
		return "name"
	}
}

func NewModel(cfg config.Config, client argocd.Client) Model {
	h := help.New()
	h.ShowAll = false

	ti := textinput.New()
	ti.Placeholder = "filter apps…"
	ti.Prompt = "/ "
	ti.CharLimit = 128
	ti.Width = 24

	del := textinput.New()
	del.Placeholder = "type app name to confirm"
	del.Prompt = "> "
	del.CharLimit = 256
	del.Width = 32

	serverLabel := cfg.ArgoCD.Server
	if _, ok := client.(*argocd.MockClient); ok {
		serverLabel = "mock"
	}

	m := Model{
		cfg:         cfg,
		client:      client,
		styles:      newStyles(),
		keys:        newKeyMap(),
		help:        h,
		filterInput: ti,
		deleteInput: del,
		sortMode:    sortByName,
		serverLabel: serverLabel,
	}
	return m
}

func (m Model) Init() tea.Cmd {
	// Initial data load.
	return tea.Batch(m.refreshCmd())
}

type appsMsg struct {
	apps []argocd.Application
	err  error
}

type detailMsg struct {
	app argocd.Application
	err error
}

type syncResult struct {
	name string
	err  error
}

type syncBatchMsg struct {
	dryRun  bool
	results []syncResult
}

type revisionsMsg struct {
	appName   string
	revisions []argocd.Revision
	err       error
}

type rollbackMsg struct {
	appName string
	err     error
}

type terminateMsg struct {
	appName string
	err     error
}

type deleteMsg struct {
	appName string
	err     error
}

func (m Model) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		apps, err := m.client.ListApplications(context.Background())
		return appsMsg{apps: apps, err: err}
	}
}

func (m Model) loadDetailCmd(name string, hard bool) tea.Cmd {
	return func() tea.Msg {
		app, err := m.client.RefreshApplication(context.Background(), name, hard)
		return detailMsg{app: app, err: err}
	}
}

func (m Model) syncBatchCmd(targets []string, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		results := make([]syncResult, 0, len(targets))
		for _, name := range targets {
			err := m.client.SyncApplication(context.Background(), name, dryRun)
			results = append(results, syncResult{name: name, err: err})
		}
		return syncBatchMsg{dryRun: dryRun, results: results}
	}
}

func (m Model) loadRevisionsCmd(appName string) tea.Cmd {
	return func() tea.Msg {
		revs, err := m.client.ListRevisions(context.Background(), appName)
		return revisionsMsg{appName: appName, revisions: revs, err: err}
	}
}

func (m Model) rollbackCmd(appName string, id int64) tea.Cmd {
	return func() tea.Msg {
		err := m.client.RollbackApplication(context.Background(), appName, id)
		return rollbackMsg{appName: appName, err: err}
	}
}

func (m Model) terminateCmd(appName string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.TerminateOperation(context.Background(), appName)
		return terminateMsg{appName: appName, err: err}
	}
}

func (m Model) deleteCmd(appName string, cascade bool) tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeleteApplication(context.Background(), appName, cascade)
		return deleteMsg{appName: appName, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureSidebarSelectionVisible()
		return m, nil
	case appsMsg:
		m.err = msg.err
		m.detail = nil
		m.detailErr = nil
		if msg.err == nil {
			m.appsAll = msg.apps
			m.lastRefresh = time.Now().UTC()
			m.applyFilter(false)
			m.ensureSidebarSelectionVisible()
			m.statusLine = fmt.Sprintf("loaded %d apps", len(m.appsAll))
			if len(m.apps) > 0 {
				// Auto-load details for the selected app.
				return m, m.loadDetailCmd(m.apps[m.selected].Name, false)
			}
		} else {
			m.statusLine = "failed to load apps"
		}
		return m, nil
	case detailMsg:
		m.detailErr = msg.err
		if msg.err == nil {
			m.detail = &msg.app
			m.statusLine = "loaded details"
		} else {
			m.detail = nil
			m.statusLine = "failed to load details"
		}
		return m, nil
	case syncBatchMsg:
		if msg.dryRun {
			m.syncDryRunComplete = true
			m.syncDryRunResults = msg.results
			m.statusLine = "dry-run complete (y=sync, n=cancel)"
			return m, nil
		}

		// Real sync finished: clear modal and refresh list.
		m.syncModal = false
		m.syncTargets = nil
		m.syncPreview = nil
		m.syncDryRunComplete = false
		m.syncDryRunResults = nil
		m.statusLine = "sync finished"
		return m, m.refreshCmd()
	case revisionsMsg:
		m.rollbackLoading = false
		m.rollbackErr = msg.err
		if msg.err == nil {
			m.rollbackRevs = msg.revisions
			m.rollbackSelected = 0
			m.rollbackConfirm = false
			m.statusLine = fmt.Sprintf("loaded %d revisions", len(msg.revisions))
		} else {
			m.rollbackRevs = nil
			m.statusLine = "failed to load revisions"
		}
		return m, nil
	case rollbackMsg:
		if msg.err != nil {
			m.rollbackErr = msg.err
			m.statusLine = "rollback failed"
			return m, nil
		}
		m.rollbackModal = false
		m.rollbackApp = ""
		m.rollbackLoading = false
		m.rollbackErr = nil
		m.rollbackRevs = nil
		m.rollbackConfirm = false
		m.statusLine = "rollback started"
		return m, tea.Batch(m.refreshCmd())
	case terminateMsg:
		m.terminateLoading = false
		m.terminateErr = msg.err
		if msg.err != nil {
			m.statusLine = "terminate failed"
			return m, nil
		}
		m.terminateModal = false
		m.terminateApp = ""
		m.terminateConfirm = false
		m.statusLine = "operation terminated"
		return m, tea.Batch(m.refreshCmd())
	case deleteMsg:
		if msg.err != nil {
			m.statusLine = "delete failed"
			m.err = msg.err
			return m, nil
		}
		m.deleteModal = false
		m.deleteApp = ""
		m.deleteCascade = false
		m.deleteInput.SetValue("")
		m.deleteInput.Blur()
		m.statusLine = "application deleted"
		return m, tea.Batch(m.refreshCmd())
	case tea.KeyMsg:
		if m.deleteModal {
			switch msg.String() {
			case "esc":
				m.deleteModal = false
				m.deleteApp = ""
				m.deleteCascade = false
				m.deleteInput.SetValue("")
				m.deleteInput.Blur()
				m.statusLine = "delete cancelled"
				return m, nil
			case "c":
				m.deleteCascade = !m.deleteCascade
				return m, nil
			case "enter":
				if strings.TrimSpace(m.deleteInput.Value()) != m.deleteApp {
					m.statusLine = "type the exact app name to confirm"
					return m, nil
				}
				m.statusLine = "deleting…"
				return m, m.deleteCmd(m.deleteApp, m.deleteCascade)
			}

			var cmd tea.Cmd
			m.deleteInput, cmd = m.deleteInput.Update(msg)
			return m, cmd
		}

		if m.syncModal {
			switch msg.String() {
			case "esc", "n":
				m.syncModal = false
				m.syncTargets = nil
				m.syncPreview = nil
				m.syncDryRunComplete = false
				m.syncDryRunResults = nil
				m.statusLine = "sync cancelled"
				return m, nil
			case "y":
				if !m.syncDryRunComplete {
					return m, nil
				}
				m.statusLine = "syncing…"
				return m, m.syncBatchCmd(m.syncTargets, false)
			}
			return m, nil
		}

		if m.terminateModal {
			switch msg.String() {
			case "esc", "n":
				m.terminateModal = false
				m.terminateApp = ""
				m.terminateLoading = false
				m.terminateErr = nil
				m.terminateConfirm = false
				m.statusLine = "terminate cancelled"
				return m, nil
			case "enter":
				if m.terminateLoading {
					return m, nil
				}
				m.terminateConfirm = true
				m.statusLine = "confirm terminate with y"
				return m, nil
			case "y":
				if !m.terminateConfirm || m.terminateLoading {
					return m, nil
				}
				m.terminateLoading = true
				m.statusLine = "terminating operation…"
				return m, m.terminateCmd(m.terminateApp)
			}
			return m, nil
		}

		if m.rollbackModal {
			switch msg.String() {
			case "esc", "n":
				m.rollbackModal = false
				m.rollbackApp = ""
				m.rollbackLoading = false
				m.rollbackErr = nil
				m.rollbackRevs = nil
				m.rollbackConfirm = false
				m.statusLine = "rollback cancelled"
				return m, nil
			case "up", "k":
				if m.rollbackSelected > 0 {
					m.rollbackSelected--
					m.rollbackConfirm = false
				}
				return m, nil
			case "down", "j":
				if m.rollbackSelected < len(m.rollbackRevs)-1 {
					m.rollbackSelected++
					m.rollbackConfirm = false
				}
				return m, nil
			case "enter":
				if len(m.rollbackRevs) == 0 || m.rollbackLoading {
					return m, nil
				}
				m.rollbackConfirm = true
				m.statusLine = "confirm rollback with y"
				return m, nil
			case "y":
				if !m.rollbackConfirm || len(m.rollbackRevs) == 0 || m.rollbackLoading {
					return m, nil
				}
				rev := m.rollbackRevs[m.rollbackSelected]
				m.rollbackLoading = true
				m.statusLine = fmt.Sprintf("rolling back to %d…", rev.ID)
				return m, m.rollbackCmd(m.rollbackApp, rev.ID)
			}
			return m, nil
		}

		// While filtering, most keys should go to the input first.
		if m.filterActive {
			// Escape clears + exits filter mode.
			if key.Matches(msg, m.keys.Clear) {
				m.filterInput.SetValue("")
				m.filterActive = false
				m.filterInput.Blur()
				m.applyFilter(true)
				m.ensureSidebarSelectionVisible()
				return m, nil
			}

			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.applyFilter(true)
			m.ensureSidebarSelectionVisible()
			return m, cmd
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		case key.Matches(msg, m.keys.Refresh):
			m.statusLine = "refreshing list…"
			return m, m.refreshCmd()
		case key.Matches(msg, m.keys.RefreshDetail):
			if len(m.apps) == 0 {
				return m, nil
			}
			m.statusLine = "refreshing details…"
			m.detail = nil
			m.detailErr = nil
			return m, m.loadDetailCmd(m.apps[m.selected].Name, false)
		case key.Matches(msg, m.keys.RefreshHard):
			if len(m.apps) == 0 {
				return m, nil
			}
			m.statusLine = "hard refreshing…"
			m.detail = nil
			m.detailErr = nil
			return m, m.loadDetailCmd(m.apps[m.selected].Name, true)
		case key.Matches(msg, m.keys.ToggleDrift):
			m.driftOnly = !m.driftOnly
			m.applyFilter(true)
			m.ensureSidebarSelectionVisible()
			if m.driftOnly {
				m.statusLine = "showing drift only"
			} else {
				m.statusLine = "showing all apps"
			}
			return m, nil
		case key.Matches(msg, m.keys.SyncBatch):
			targets := make([]string, 0)
			for _, a := range m.appsAll {
				if a.Sync != "Synced" {
					targets = append(targets, a.Name)
				}
			}
			if len(targets) == 0 {
				m.statusLine = "no drifted apps to sync"
				return m, nil
			}
			m.syncModal = true
			m.syncTargets = targets
			m.syncPreview = m.buildSyncPreview(targets)
			m.syncDryRunComplete = false
			m.syncDryRunResults = nil
			m.statusLine = "running dry-run…"
			return m, m.syncBatchCmd(targets, true)
		case key.Matches(msg, m.keys.SyncApp):
			if len(m.apps) == 0 {
				return m, nil
			}
			targets := []string{m.apps[m.selected].Name}
			m.syncModal = true
			m.syncTargets = targets
			m.syncPreview = m.buildSyncPreview(targets)
			m.syncDryRunComplete = false
			m.syncDryRunResults = nil
			m.statusLine = "running dry-run…"
			return m, m.syncBatchCmd(targets, true)
		case key.Matches(msg, m.keys.Rollback):
			if len(m.apps) == 0 {
				return m, nil
			}
			m.rollbackModal = true
			m.rollbackApp = m.apps[m.selected].Name
			m.rollbackLoading = true
			m.rollbackErr = nil
			m.rollbackRevs = nil
			m.rollbackSelected = 0
			m.rollbackConfirm = false
			m.statusLine = "loading revisions…"
			return m, m.loadRevisionsCmd(m.rollbackApp)
		case key.Matches(msg, m.keys.TerminateOp):
			if len(m.apps) == 0 {
				return m, nil
			}
			name := m.apps[m.selected].Name
			app := m.apps[m.selected]
			if m.detail != nil && m.detail.Name == name {
				app = *m.detail
			}
			if app.OperationState == nil {
				m.statusLine = "no operation in progress"
				return m, nil
			}
			m.terminateModal = true
			m.terminateApp = name
			m.terminateLoading = false
			m.terminateErr = nil
			m.terminateConfirm = false
			m.statusLine = "terminate operation?"
			return m, nil
		case key.Matches(msg, m.keys.DeleteApp):
			if len(m.apps) == 0 {
				return m, nil
			}
			m.deleteModal = true
			m.deleteApp = m.apps[m.selected].Name
			m.deleteCascade = false
			m.deleteInput.SetValue("")
			m.deleteInput.Focus()
			m.statusLine = "confirm delete"
			return m, nil
		case key.Matches(msg, m.keys.Filter):
			m.filterActive = true
			m.filterInput.Focus()
			return m, nil
		case key.Matches(msg, m.keys.Sort):
			m.sortMode = (m.sortMode + 1) % 3
			m.applyFilter(true)
			m.ensureSidebarSelectionVisible()
			m.statusLine = "sorted by " + m.sortMode.String()
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
				m.ensureSidebarSelectionVisible()
				m.detail = nil
				m.detailErr = nil
				return m, m.loadDetailCmd(m.apps[m.selected].Name, false)
			}
			return m, nil
		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.apps)-1 {
				m.selected++
				m.ensureSidebarSelectionVisible()
				m.detail = nil
				m.detailErr = nil
				return m, m.loadDetailCmd(m.apps[m.selected].Name, false)
			}
			return m, nil
		case key.Matches(msg, m.keys.Clear):
			// esc outside filter mode clears the filter but keeps focus unchanged.
			if m.filterInput.Value() != "" {
				m.filterInput.SetValue("")
				m.applyFilter(true)
				m.ensureSidebarSelectionVisible()
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

	headerTitle := "lazyArgo"
	if m.driftOnly {
		headerTitle += "  [drift]"
	}
	headerTitle += "  [sort:" + m.sortMode.String() + "]"
	if m.filterInput.Value() != "" || m.filterActive {
		headerTitle = headerTitle + "  " + m.filterInput.View()
	}
	header := m.styles.Header.Width(m.width).Render(headerTitle)

	footer := m.renderFooter(m.width)

	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
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

	return lipgloss.JoinVertical(lipgloss.Top, header, row, footer)
}

func (m Model) renderFooter(w int) string {
	drifted := 0
	for _, a := range m.appsAll {
		if a.Sync != "Synced" {
			drifted++
		}
	}

	ts := "never"
	if !m.lastRefresh.IsZero() {
		// Keep it compact.
		ts = m.lastRefresh.Format("15:04:05Z")
	}

	label := func(s string) string { return m.styles.StatusLabel.Render(s) }
	val := func(s string) string { return m.styles.StatusValue.Render(s) }

	driftStyle := m.styles.StatusValue
	if drifted > 0 {
		driftStyle = m.styles.StatusWarn
	}

	leftParts := []string{
		label("server:") + val(m.serverLabel),
		label("refresh:") + val(ts),
		label("apps:") + val(fmt.Sprintf("%d", len(m.appsAll))),
		label("drift:") + driftStyle.Render(fmt.Sprintf("%d", drifted)),
	}
	if strings.TrimSpace(m.statusLine) != "" {
		leftParts = append(leftParts, label("msg:")+val(m.statusLine))
	}
	left := strings.Join(leftParts, "  ")

	right := m.help.View(m.keys)

	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return m.styles.StatusBar.Width(w).Render(line)
}

func (m Model) renderSidebar(w, h int) string {
	titleText := "Applications"
	if len(m.appsAll) > 0 && len(m.apps) != len(m.appsAll) {
		titleText = fmt.Sprintf("Applications (%d/%d)", len(m.apps), len(m.appsAll))
	} else if len(m.appsAll) > 0 {
		titleText = fmt.Sprintf("Applications (%d)", len(m.appsAll))
	}
	title := m.styles.SidebarTitle.Render(titleText)
	lines := []string{title, strings.Repeat("─", max(0, w-2))}

	if m.err != nil {
		lines = append(lines, m.styles.Error.Render(m.err.Error()))
	}

	// Render only the visible window of apps.
	maxItems := h - len(lines)
	if maxItems < 0 {
		maxItems = 0
	}
	start := clamp(m.sidebarOffset, 0, max(0, len(m.apps)-1))
	end := min(len(m.apps), start+maxItems)

	for i := start; i < end; i++ {
		a := m.apps[i]
		name := a.Name
		if a.Sync != "" && a.Sync != "Synced" {
			name = "! " + name
		}
		if i == m.selected {
			lines = append(lines, m.styles.SidebarSelected.Render("▶ "+name))
		} else {
			lines = append(lines, m.styles.SidebarItem.Render("  "+name))
		}
	}

	// If there's room, show a small hint when list is truncated.
	if len(m.apps) > end && maxItems > 0 {
		lines[len(lines)-1] = lines[len(lines)-1] + m.styles.SidebarItem.Render("  …")
	}

	content := strings.Join(lines, "\n")
	return m.styles.Sidebar.Width(w).Height(h).Render(content)
}

func (m Model) renderMain(w, h int) string {
	var content string
	// If the initial list load failed, show a helpful error page.
	if m.err != nil {
		content = "Error loading applications:\n\n" + m.err.Error() + "\n\n" +
			"Common fixes:\n" +
			"  • Ensure ARGOCD_SERVER is reachable (default expects a local port-forward)\n" +
			"  • Ensure ARGOCD_AUTH_TOKEN is set\n" +
			"  • If using https://localhost:8080 and you see TLS errors, use --insecure or ARGOCD_INSECURE=true\n\n" +
			"Press 'r' to retry."
		return m.styles.Main.Width(w).Height(h).Render(content)
	}
	if m.deleteModal {
		lines := []string{fmt.Sprintf("Delete application: %s", m.deleteApp), ""}
		lines = append(lines, "This is destructive.")
		lines = append(lines, fmt.Sprintf("Cascade delete: %v (press 'c' to toggle)", m.deleteCascade))
		lines = append(lines, "", "Type the application name to confirm:", m.deleteInput.View(), "")
		lines = append(lines, "Enter=delete  Esc=cancel")
		content = strings.Join(lines, "\n")
		return m.styles.Main.Width(w).Height(h).Render(content)
	}
	if m.terminateModal {
		lines := []string{fmt.Sprintf("Terminate operation: %s", m.terminateApp), ""}
		if m.terminateErr != nil {
			lines = append(lines, "Error:", m.terminateErr.Error(), "")
		}
		if m.terminateLoading {
			lines = append(lines, "Terminating…")
		} else if m.terminateConfirm {
			lines = append(lines, "Confirm terminate? y=confirm, n/esc=cancel")
		} else {
			lines = append(lines, "Enter=select  y=confirm  n/esc=cancel")
		}
		content = strings.Join(lines, "\n")
		return m.styles.Main.Width(w).Height(h).Render(content)
	}
	if m.rollbackModal {
		lines := []string{fmt.Sprintf("Rollback: %s", m.rollbackApp), ""}
		if m.rollbackLoading {
			lines = append(lines, "Loading revisions…")
		}
		if m.rollbackErr != nil {
			lines = append(lines, "", "Error:", m.rollbackErr.Error())
		}
		if !m.rollbackLoading && len(m.rollbackRevs) == 0 && m.rollbackErr == nil {
			lines = append(lines, "No revisions found.")
		}
		if len(m.rollbackRevs) > 0 {
			lines = append(lines, "Select a revision:")
			for i, r := range m.rollbackRevs {
				prefix := "  "
				if i == m.rollbackSelected {
					prefix = "▶ "
				}
				sum := r.Revision
				if r.Message != "" {
					sum = r.Message
				}
				meta := strings.TrimSpace(strings.Join([]string{r.Author, r.Date}, " "))
				if meta != "" {
					meta = " (" + meta + ")"
				}
				lines = append(lines, fmt.Sprintf("%s#%d %s%s", prefix, r.ID, sum, meta))
			}
			lines = append(lines, "")
			if m.rollbackConfirm {
				rev := m.rollbackRevs[m.rollbackSelected]
				lines = append(lines, fmt.Sprintf("Confirm rollback to #%d? y=confirm, n/esc=cancel", rev.ID))
			} else {
				lines = append(lines, "Enter=select  y=confirm  n/esc=cancel")
			}
		}
		content = strings.Join(lines, "\n")
		return m.styles.Main.Width(w).Height(h).Render(content)
	}
	if m.syncModal {
		lines := []string{"Sync (dry-run preview)", ""}
		lines = append(lines, fmt.Sprintf("Targets: %d", len(m.syncTargets)))
		for _, name := range m.syncTargets {
			lines = append(lines, "  - "+name)
			if rs := m.syncPreview[name]; len(rs) > 0 {
				lines = append(lines, "    Resources to reconcile:")
				for _, r := range rs {
					kind := r.Kind
					if r.Group != "" {
						kind = r.Group + "/" + r.Kind
					}
					ns := r.Namespace
					if ns == "" {
						ns = "—"
					}
					st := r.Status
					if st == "" {
						st = "—"
					}
					lines = append(lines, fmt.Sprintf("      - %s/%s (%s) [%s]", kind, r.Name, ns, st))
				}
			}
		}
		lines = append(lines, "")
		if !m.syncDryRunComplete {
			lines = append(lines, "Running dry-run…")
		} else {
			lines = append(lines, "Dry-run results:")
			for _, r := range m.syncDryRunResults {
				if r.err != nil {
					lines = append(lines, fmt.Sprintf("  ✗ %s: %v", r.name, r.err))
				} else {
					n := len(m.syncPreview[r.name])
					suffix := ""
					if n > 0 {
						suffix = fmt.Sprintf(" (%d resources)", n)
					}
					lines = append(lines, fmt.Sprintf("  ✓ %s%s", r.name, suffix))
				}
			}
			lines = append(lines, "", "Press y to run sync, n/esc to cancel.")
		}
		content = strings.Join(lines, "\n")
		return m.styles.Main.Width(w).Height(h).Render(content)
	}
	if len(m.apps) == 0 {
		content = "No applications. Press 'r' to refresh."
		if m.statusLine != "" {
			content += "\n\n" + m.statusLine
		}
		return m.styles.Main.Width(w).Height(h).Render(content)
	}

	base := m.apps[m.selected]
	app := base
	if m.detail != nil && m.detail.Name == base.Name {
		app = *m.detail
	}

	detailBlock := ""
	if m.detailErr != nil {
		detailBlock = "\n\nError loading details:\n\n" + m.detailErr.Error() + "\n\nPress 'r' to retry."
	}

	content = fmt.Sprintf(
		"Name:      %s\nNamespace: %s\nProject:   %s\nHealth:    %s\nSync:      %s\nRepo:      %s\nPath:      %s\nRevision:  %s\nCluster:   %s\n\nResources:\n%s\n\n%s%s",
		app.Name,
		app.Namespace,
		app.Project,
		app.Health,
		app.Sync,
		blankIfEmpty(app.RepoURL, "—"),
		blankIfEmpty(app.Path, "—"),
		blankIfEmpty(app.Revision, "—"),
		blankIfEmpty(app.Cluster, "—"),
		renderResources(app.Resources),
		m.statusLine,
		detailBlock,
	)

	return m.styles.Main.Width(w).Height(h).Render(content)
}

func (m *Model) applyFilter(keepSelectionByName bool) {
	prevName := ""
	if keepSelectionByName && len(m.apps) > 0 && m.selected >= 0 && m.selected < len(m.apps) {
		prevName = m.apps[m.selected].Name
	}

	q := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	filtered := make([]argocd.Application, 0, len(m.appsAll))
	for _, a := range m.appsAll {
		if q != "" && !strings.Contains(strings.ToLower(a.Name), q) {
			continue
		}
		if m.driftOnly && a.Sync == "Synced" {
			continue
		}
		filtered = append(filtered, a)
	}
	m.apps = filtered
	m.sortApps()

	if len(m.apps) == 0 {
		m.selected = 0
		m.detail = nil
		m.detailErr = nil
		return
	}

	// Try to keep selection stable by app name.
	if prevName != "" {
		for i := range m.apps {
			if m.apps[i].Name == prevName {
				m.selected = i
				return
			}
		}
	}

	if m.selected >= len(m.apps) {
		m.selected = max(0, len(m.apps)-1)
	}
}

func (m *Model) sortApps() {
	if len(m.apps) < 2 {
		return
	}

	healthRank := func(s string) int {
		s = strings.TrimSpace(strings.ToLower(s))
		switch s {
		case "degraded":
			return 0
		case "missing":
			return 1
		case "suspended":
			return 2
		case "progressing":
			return 3
		case "healthy":
			return 4
		case "":
			return 98
		default:
			return 50
		}
	}

	syncRank := func(s string) int {
		s = strings.TrimSpace(strings.ToLower(s))
		switch s {
		case "outofsync", "out-of-sync", "out_of_sync":
			return 0
		case "unknown":
			return 1
		case "synced":
			return 2
		case "":
			return 98
		default:
			return 50
		}
	}

	sort.SliceStable(m.apps, func(i, j int) bool {
		a, b := m.apps[i], m.apps[j]
		switch m.sortMode {
		case sortByHealth:
			ri, rj := healthRank(a.Health), healthRank(b.Health)
			if ri != rj {
				return ri < rj
			}
		case sortBySync:
			ri, rj := syncRank(a.Sync), syncRank(b.Sync)
			if ri != rj {
				return ri < rj
			}
		default:
			// sortByName
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}

func (m *Model) ensureSidebarSelectionVisible() {
	if len(m.apps) == 0 {
		m.sidebarOffset = 0
		return
	}
	if m.height == 0 {
		return
	}

	// Approximate visible rows: header (1) + help (1) + sidebar title+rule (2).
	bodyHeight := m.height - 2
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	visible := bodyHeight - 2
	if visible < 1 {
		visible = 1
	}

	if m.selected < m.sidebarOffset {
		m.sidebarOffset = m.selected
	}
	if m.selected >= m.sidebarOffset+visible {
		m.sidebarOffset = m.selected - visible + 1
	}

	maxOffset := max(0, len(m.apps)-visible)
	m.sidebarOffset = clamp(m.sidebarOffset, 0, maxOffset)
}

func blankIfEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func (m *Model) buildSyncPreview(targets []string) map[string][]argocd.Resource {
	preview := make(map[string][]argocd.Resource, len(targets))
	for _, name := range targets {
		// Prefer loaded details for the selected app.
		var rs []argocd.Resource
		if m.detail != nil && m.detail.Name == name {
			rs = m.detail.Resources
		} else {
			for _, a := range m.appsAll {
				if a.Name == name {
					rs = a.Resources
					break
				}
			}
		}
		if len(rs) == 0 {
			continue
		}
		out := make([]argocd.Resource, 0)
		for _, r := range rs {
			if strings.TrimSpace(r.Status) != "" && r.Status != "Synced" {
				out = append(out, r)
			}
		}
		if len(out) > 0 {
			preview[name] = out
		}
	}
	return preview
}

func renderResources(rs []argocd.Resource) string {
	if len(rs) == 0 {
		return "  (none yet)"
	}
	lines := make([]string, 0, len(rs))
	for _, r := range rs {
		// Keep it compact for now.
		kind := r.Kind
		if r.Group != "" {
			kind = r.Group + "/" + r.Kind
		}
		health := r.Health
		if health == "" {
			health = "—"
		}
		status := r.Status
		if status == "" {
			status = "—"
		}
		ns := r.Namespace
		if ns == "" {
			ns = "—"
		}
		lines = append(lines, fmt.Sprintf("  %s/%s (%s) [%s/%s]", kind, r.Name, ns, health, status))
	}
	return strings.Join(lines, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
