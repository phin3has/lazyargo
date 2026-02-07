package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
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

	createModal      bool
	createStep       createStep
	createNameInput  textinput.Model
	createPathInput  textinput.Model
	createNSInput    textinput.Model
	createRevInput   textinput.Model
	createList       list.Model
	createProjects   []string
	createRepos      []string
	createClusters   []string
	createProject    string
	createRepo       string
	createCluster    string
	createSyncPolicy string
	createErr        error
	createCreating   bool

	editModal      bool
	editStep       createStep
	editApp        string
	editRepoInput  textinput.Model
	editPathInput  textinput.Model
	editRevInput   textinput.Model
	editClusterIn  textinput.Model
	editNSInput    textinput.Model
	editSyncPolicy string
	editErr        error
	editSaving     bool

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

	focusResources bool
	resourceSel    int

	resourceDetails *resourceDetailsModel
	eventsView      *eventsModel
	logsView        *logsModel
	diffView        *diffModel
	historyView     *historyModel

	detail     *argocd.Application
	detailErr  error
	statusLine string
	err        error
}

type sortMode int

type createStep int

const (
	sortByName sortMode = iota
	sortByHealth
	sortBySync
)

const (
	createStepName createStep = iota
	createStepProject
	createStepRepo
	createStepPath
	createStepRevision
	createStepCluster
	createStepNamespace
	createStepSyncPolicy
	createStepConfirm
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

	nameIn := textinput.New()
	nameIn.Placeholder = "app name"
	nameIn.Prompt = "name> "
	nameIn.CharLimit = 128
	nameIn.Width = 32

	repoPath := textinput.New()
	repoPath.Placeholder = "path/chart"
	repoPath.Prompt = "path> "
	repoPath.CharLimit = 256
	repoPath.Width = 48

	nsIn := textinput.New()
	nsIn.Placeholder = "namespace"
	nsIn.Prompt = "ns> "
	nsIn.CharLimit = 128
	nsIn.Width = 32

	revIn := textinput.New()
	revIn.Placeholder = "revision (default: main)"
	revIn.Prompt = "rev> "
	revIn.CharLimit = 128
	revIn.Width = 32

	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.SetShowHelp(true)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()

	edRepo := textinput.New()
	edRepo.Placeholder = "repo URL"
	edRepo.Prompt = "repo> "
	edRepo.CharLimit = 256
	edRepo.Width = 48

	edPath := textinput.New()
	edPath.Placeholder = "path/chart"
	edPath.Prompt = "path> "
	edPath.CharLimit = 256
	edPath.Width = 48

	edRev := textinput.New()
	edRev.Placeholder = "revision"
	edRev.Prompt = "rev> "
	edRev.CharLimit = 128
	edRev.Width = 32

	edCluster := textinput.New()
	edCluster.Placeholder = "cluster server"
	edCluster.Prompt = "cluster> "
	edCluster.CharLimit = 256
	edCluster.Width = 48

	edNS := textinput.New()
	edNS.Placeholder = "namespace"
	edNS.Prompt = "ns> "
	edNS.CharLimit = 128
	edNS.Width = 32

	serverLabel := cfg.ArgoCD.Server
	if _, ok := client.(*argocd.MockClient); ok {
		serverLabel = "mock"
	}

	m := Model{
		cfg:             cfg,
		client:          client,
		styles:          newStyles(),
		keys:            newKeyMap(),
		help:            h,
		filterInput:     ti,
		deleteInput:     del,
		createNameInput: nameIn,
		createPathInput: repoPath,
		createNSInput:   nsIn,
		createRevInput:  revIn,
		createList:      l,
		editRepoInput:   edRepo,
		editPathInput:   edPath,
		editRevInput:    edRev,
		editClusterIn:   edCluster,
		editNSInput:     edNS,
		sortMode:        sortByName,
		serverLabel:     serverLabel,
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

type projectsMsg struct {
	items []string
	err   error
}

type reposMsg struct {
	items []string
	err   error
}

type clustersMsg struct {
	items []string
	err   error
}

type createMsg struct {
	appName string
	err     error
}

type updateMsg struct {
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

func (m Model) loadProjectsCmd() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.ListProjects(context.Background())
		return projectsMsg{items: items, err: err}
	}
}

func (m Model) loadReposCmd() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.ListRepositories(context.Background())
		return reposMsg{items: items, err: err}
	}
}

func (m Model) loadClustersCmd() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.ListClusters(context.Background())
		return clustersMsg{items: items, err: err}
	}
}

func (m Model) createAppCmd(app argocd.Application) tea.Cmd {
	return func() tea.Msg {
		err := m.client.CreateApplication(context.Background(), app)
		return createMsg{appName: app.Name, err: err}
	}
}

func (m Model) updateAppCmd(app argocd.Application) tea.Cmd {
	return func() tea.Msg {
		err := m.client.UpdateApplication(context.Background(), app)
		return updateMsg{appName: app.Name, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureSidebarSelectionVisible()
		if m.resourceDetails != nil {
			rd := *m.resourceDetails
			rd.setSize(msg.Width-2, msg.Height-2)
			m.resourceDetails = &rd
		}
		if m.eventsView != nil {
			ev := *m.eventsView
			ev.setSize(msg.Width-2, msg.Height-2)
			m.eventsView = &ev
		}
		if m.logsView != nil {
			lv := *m.logsView
			lv.setSize(msg.Width-2, msg.Height-2)
			m.logsView = &lv
		}
		if m.diffView != nil {
			dv := *m.diffView
			dv.setSize(msg.Width-2, msg.Height-2)
			m.diffView = &dv
		}
		if m.historyView != nil {
			hv := *m.historyView
			hv.setSize(msg.Width-2, msg.Height-2)
			m.historyView = &hv
		}
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
			// Clamp resource selection.
			if m.resourceSel >= len(msg.app.Resources) {
				m.resourceSel = max(0, len(msg.app.Resources)-1)
			}
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
	case projectsMsg:
		m.createErr = msg.err
		if msg.err == nil {
			m.createProjects = msg.items
		}
		return m, nil
	case reposMsg:
		m.createErr = msg.err
		if msg.err == nil {
			m.createRepos = msg.items
		}
		return m, nil
	case clustersMsg:
		m.createErr = msg.err
		if msg.err == nil {
			m.createClusters = msg.items
		}
		return m, nil
	case createMsg:
		m.createCreating = false
		if msg.err != nil {
			m.createErr = msg.err
			m.statusLine = "create failed"
			return m, nil
		}
		m = m.resetCreateWizard()
		m.statusLine = "application created"
		return m, tea.Batch(m.refreshCmd())
	case updateMsg:
		m.editSaving = false
		if msg.err != nil {
			m.editErr = msg.err
			m.statusLine = "update failed"
			return m, nil
		}
		m = m.resetEditWizard()
		m.statusLine = "application updated"
		return m, tea.Batch(m.refreshCmd())
	case tea.KeyMsg:
		if m.resourceDetails != nil {
			// Close handled here.
			switch msg.String() {
			case "esc", "q":
				m.resourceDetails = nil
				m.statusLine = "closed resource view"
				return m, nil
			}
			var cmd tea.Cmd
			rd := *m.resourceDetails
			rd, cmd = rd.Update(msg)
			m.resourceDetails = &rd
			return m, cmd
		}
		if m.eventsView != nil {
			switch msg.String() {
			case "esc", "q":
				m.eventsView = nil
				m.statusLine = "closed events"
				return m, nil
			}
			var cmd tea.Cmd
			ev := *m.eventsView
			ev, cmd = ev.Update(msg)
			m.eventsView = &ev
			return m, cmd
		}
		if m.logsView != nil {
			switch msg.String() {
			case "esc", "q":
				m.logsView = nil
				m.statusLine = "closed logs"
				return m, nil
			}
			var cmd tea.Cmd
			lv := *m.logsView
			lv, cmd = lv.Update(msg)
			m.logsView = &lv
			return m, cmd
		}
		if m.diffView != nil {
			switch msg.String() {
			case "esc", "q":
				m.diffView = nil
				m.statusLine = "closed diff"
				return m, nil
			}
			var cmd tea.Cmd
			dv := *m.diffView
			dv, cmd = dv.Update(msg)
			m.diffView = &dv
			return m, cmd
		}
		if m.historyView != nil {
			switch msg.String() {
			case "esc", "q":
				m.historyView = nil
				m.statusLine = "closed history"
				return m, nil
			case "enter":
				// MET-61 will add revision detail. For now just acknowledge.
				m.statusLine = "revision details not implemented yet"
			}
			var cmd tea.Cmd
			hv := *m.historyView
			hv, cmd = hv.Update(msg)
			m.historyView = &hv
			return m, cmd
		}

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

		if m.editModal {
			return m.updateEditWizard(msg)
		}
		if m.createModal {
			return m.updateCreateWizard(msg)
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
		case msg.String() == "tab":
			m.focusResources = !m.focusResources
			if m.focusResources {
				m.statusLine = "focus: resources (tab to switch)"
			} else {
				m.statusLine = "focus: applications (tab to switch)"
			}
			return m, nil
		case (msg.String() == "enter" || msg.String() == "v") && m.focusResources:
			if m.detail == nil || len(m.detail.Resources) == 0 {
				return m, nil
			}
			r := m.detail.Resources[clamp(m.resourceSel, 0, len(m.detail.Resources)-1)]
			ref := argocd.ResourceRef{Group: r.Group, Kind: r.Kind, Name: r.Name, Namespace: r.Namespace, Version: r.Version}
			rd := newResourceDetailsModel(m.styles, m.client, m.detail.Name, ref)
			rd.setSize(m.width-4, m.height-4)
			m.resourceDetails = &rd
			m.statusLine = "loading resource…"
			return m, rd.initCmd()
		case msg.String() == "E":
			if len(m.apps) == 0 {
				return m, nil
			}
			name := m.apps[m.selected].Name
			ev := newEventsModel(m.styles, m.client, name)
			ev.setSize(m.width-4, m.height-4)
			m.eventsView = &ev
			m.statusLine = "loading events…"
			return m, ev.initCmd()
		case msg.String() == "l" && m.focusResources:
			if m.detail == nil || len(m.detail.Resources) == 0 {
				return m, nil
			}
			r := m.detail.Resources[clamp(m.resourceSel, 0, len(m.detail.Resources)-1)]
			if !strings.EqualFold(r.Kind, "pod") {
				m.statusLine = "select a Pod to view logs"
				return m, nil
			}
			lv := newLogsModel(m.styles, m.client, m.detail.Name, r.Name)
			lv.setSize(m.width-4, m.height-4)
			m.logsView = &lv
			m.statusLine = "loading logs…"
			return m, lv.initCmd()
		case key.Matches(msg, m.keys.History):
			if len(m.apps) == 0 {
				return m, nil
			}
			// Prefer loaded details.
			app := m.apps[m.selected]
			if m.detail != nil && m.detail.Name == app.Name {
				app = *m.detail
			}
			hv := newHistoryModel(m.styles, app)
			hv.setSize(m.width-4, m.height-4)
			m.historyView = &hv
			m.statusLine = "history"
			return m, nil
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Diff):
			if len(m.apps) == 0 {
				return m, nil
			}
			name := m.apps[m.selected].Name
			var filter *argocd.ResourceRef
			if m.focusResources && m.detail != nil && len(m.detail.Resources) > 0 {
				r := m.detail.Resources[clamp(m.resourceSel, 0, len(m.detail.Resources)-1)]
				ref := argocd.ResourceRef{Group: r.Group, Kind: r.Kind, Name: r.Name, Namespace: r.Namespace, Version: r.Version}
				filter = &ref
			}
			dv := newDiffModel(m.styles, m.client, name, filter)
			dv.setSize(m.width-4, m.height-4)
			m.diffView = &dv
			m.statusLine = "loading diff…"
			return m, dv.initCmd()
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
		case key.Matches(msg, m.keys.CreateApp):
			m.createModal = true
			m.createStep = createStepName
			m.createErr = nil
			m.createCreating = false
			m.createProject = ""
			m.createRepo = ""
			m.createCluster = ""
			m.createSyncPolicy = "manual"
			m.createNameInput.SetValue("")
			m.createPathInput.SetValue("")
			m.createNSInput.SetValue("")
			m.createRevInput.SetValue("main")
			m.createNameInput.Focus()
			m.createList.SetItems(nil)
			m.statusLine = "create app"
			return m, tea.Batch(m.loadProjectsCmd(), m.loadReposCmd(), m.loadClustersCmd())
		case key.Matches(msg, m.keys.EditApp):
			if len(m.apps) == 0 {
				return m, nil
			}
			name := m.apps[m.selected].Name
			app := m.apps[m.selected]
			if m.detail != nil && m.detail.Name == name {
				app = *m.detail
			}
			m.editModal = true
			m.editStep = createStepRepo
			m.editApp = name
			m.editErr = nil
			m.editSaving = false
			m.editRepoInput.SetValue(app.RepoURL)
			m.editPathInput.SetValue(app.Path)
			m.editRevInput.SetValue(blankIfEmpty(app.Revision, "main"))
			m.editClusterIn.SetValue(app.Cluster)
			m.editNSInput.SetValue(app.Namespace)
			if app.SyncPolicy != "" {
				m.editSyncPolicy = strings.ToLower(app.SyncPolicy)
			} else {
				m.editSyncPolicy = "manual"
			}
			m.editRepoInput.Focus()
			m.statusLine = "edit app"
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
			if m.focusResources {
				if m.resourceSel > 0 {
					m.resourceSel--
				}
				return m, nil
			}
			if m.selected > 0 {
				m.selected--
				m.ensureSidebarSelectionVisible()
				m.detail = nil
				m.detailErr = nil
				m.resourceSel = 0
				return m, m.loadDetailCmd(m.apps[m.selected].Name, false)
			}
			return m, nil
		case key.Matches(msg, m.keys.Down):
			if m.focusResources {
				if m.detail != nil && m.resourceSel < len(m.detail.Resources)-1 {
					m.resourceSel++
				}
				return m, nil
			}
			if m.selected < len(m.apps)-1 {
				m.selected++
				m.ensureSidebarSelectionVisible()
				m.detail = nil
				m.detailErr = nil
				m.resourceSel = 0
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
	if m.resourceDetails != nil {
		// Render resource detail overlay inside main panel.
		return m.styles.Main.Width(w).Height(h).Render(m.resourceDetails.View())
	}
	if m.eventsView != nil {
		return m.styles.Main.Width(w).Height(h).Render(m.eventsView.View())
	}
	if m.logsView != nil {
		return m.styles.Main.Width(w).Height(h).Render(m.logsView.View())
	}
	if m.diffView != nil {
		return m.styles.Main.Width(w).Height(h).Render(m.diffView.View())
	}
	if m.historyView != nil {
		return m.styles.Main.Width(w).Height(h).Render(m.historyView.View())
	}
	if m.editModal {
		return m.styles.Main.Width(w).Height(h).Render(m.renderEditWizard())
	}
	if m.createModal {
		return m.styles.Main.Width(w).Height(h).Render(m.renderCreateWizard())
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
		renderResources(app.Resources, m.resourceSel, m.focusResources, m.styles),
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

func (m Model) resetCreateWizard() Model {
	m.createModal = false
	m.createStep = createStepName
	m.createErr = nil
	m.createCreating = false
	m.createProject = ""
	m.createRepo = ""
	m.createCluster = ""
	m.createSyncPolicy = "manual"
	m.createNameInput.Blur()
	m.createPathInput.Blur()
	m.createNSInput.Blur()
	m.createRevInput.Blur()
	m.createList.SetItems(nil)
	return m
}

type stringItem string

func (s stringItem) Title() string       { return string(s) }
func (s stringItem) Description() string { return "" }
func (s stringItem) FilterValue() string { return string(s) }

func (m Model) setCreateList(title string, items []string) Model {
	li := make([]list.Item, 0, len(items))
	for _, it := range items {
		li = append(li, stringItem(it))
	}
	m.createList.Title = title
	m.createList.SetItems(li)
	m.createList.Select(0)
	return m
}

func (m Model) updateCreateWizard(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m = m.resetCreateWizard()
		m.statusLine = "create cancelled"
		return m, nil
	case "left":
		if m.createStep > createStepName {
			m.createStep--
			m.createErr = nil
		}
		return m, nil
	}

	if m.createErr != nil && k.String() != "enter" {
		// Allow navigating even with an error.
	}

	switch m.createStep {
	case createStepName:
		if k.String() == "enter" {
			m.createProject = ""
			m.createStep = createStepProject
			m.createNameInput.Blur()
			m = m.setCreateList("Project", m.createProjects)
			return m, nil
		}
		var cmd tea.Cmd
		m.createNameInput, cmd = m.createNameInput.Update(k)
		return m, cmd
	case createStepProject, createStepRepo, createStepCluster, createStepSyncPolicy:
		if k.String() == "enter" {
			if it, ok := m.createList.SelectedItem().(stringItem); ok {
				sel := string(it)
				switch m.createStep {
				case createStepProject:
					m.createProject = sel
					m.createStep = createStepRepo
					m = m.setCreateList("Repository", m.createRepos)
				case createStepRepo:
					m.createRepo = sel
					m.createStep = createStepPath
					m.createPathInput.Focus()
				case createStepCluster:
					m.createCluster = sel
					m.createStep = createStepNamespace
					m.createNSInput.Focus()
				case createStepSyncPolicy:
					m.createSyncPolicy = strings.ToLower(sel)
					m.createStep = createStepConfirm
				}
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.createList, cmd = m.createList.Update(k)
		return m, cmd
	case createStepPath:
		if k.String() == "enter" {
			m.createPathInput.Blur()
			m.createStep = createStepCluster
			m = m.setCreateList("Cluster", m.createClusters)
			return m, nil
		}
		var cmd tea.Cmd
		m.createPathInput, cmd = m.createPathInput.Update(k)
		return m, cmd
	case createStepNamespace:
		if k.String() == "enter" {
			m.createNSInput.Blur()
			m.createStep = createStepSyncPolicy
			m = m.setCreateList("Sync policy", []string{"manual", "auto"})
			return m, nil
		}
		var cmd tea.Cmd
		m.createNSInput, cmd = m.createNSInput.Update(k)
		return m, cmd
	case createStepConfirm:
		switch k.String() {
		case "y":
			if m.createCreating {
				return m, nil
			}
			m.createCreating = true
			app := argocd.Application{
				Name:           strings.TrimSpace(m.createNameInput.Value()),
				Project:        strings.TrimSpace(m.createProject),
				RepoURL:        strings.TrimSpace(m.createRepo),
				Path:           strings.TrimSpace(m.createPathInput.Value()),
				Revision:       strings.TrimSpace(blankIfEmpty(m.createRevInput.Value(), "main")),
				Cluster:        strings.TrimSpace(m.createCluster),
				Namespace:      strings.TrimSpace(m.createNSInput.Value()),
				SyncPolicy:     m.createSyncPolicy,
				Health:         "",
				Sync:           "",
				Resources:      nil,
				OperationState: nil,
			}
			m.statusLine = "creating…"
			return m, m.createAppCmd(app)
		case "n":
			m = m.resetCreateWizard()
			m.statusLine = "create cancelled"
			return m, nil
		}
	}
	return m, nil
}

func (m Model) renderCreateWizard() string {
	head := []string{"Create application", ""}
	if m.createErr != nil {
		head = append(head, "Error: "+m.createErr.Error(), "")
	}
	if m.createCreating {
		head = append(head, "Creating…", "")
	}

	switch m.createStep {
	case createStepName:
		return strings.Join(append(head, "Step 1/7: Name", m.createNameInput.View(), "", "Enter=next  Esc=cancel"), "\n")
	case createStepProject:
		return strings.Join(append(head, "Step 2/7: Project", m.createList.View(), "", "Enter=select  ←=back  Esc=cancel"), "\n")
	case createStepRepo:
		return strings.Join(append(head, "Step 3/7: Repository", m.createList.View(), "", "Enter=select  ←=back  Esc=cancel"), "\n")
	case createStepPath:
		return strings.Join(append(head, "Step 4/7: Path/Chart", m.createPathInput.View(), "", "Enter=next  ←=back  Esc=cancel"), "\n")
	case createStepCluster:
		return strings.Join(append(head, "Step 5/7: Destination cluster", m.createList.View(), "", "Enter=select  ←=back  Esc=cancel"), "\n")
	case createStepNamespace:
		return strings.Join(append(head, "Step 6/7: Namespace", m.createNSInput.View(), "", "Enter=next  ←=back  Esc=cancel"), "\n")
	case createStepSyncPolicy:
		return strings.Join(append(head, "Step 7/7: Sync policy", m.createList.View(), "", "Enter=select  ←=back  Esc=cancel"), "\n")
	case createStepConfirm:
		sum := []string{
			"Confirm:",
			"  name:      " + strings.TrimSpace(m.createNameInput.Value()),
			"  project:   " + m.createProject,
			"  repo:      " + m.createRepo,
			"  path:      " + m.createPathInput.Value(),
			"  cluster:   " + m.createCluster,
			"  namespace: " + m.createNSInput.Value(),
			"  sync:      " + m.createSyncPolicy,
			"",
			"y=create  n=cancel  ←=back",
		}
		return strings.Join(append(head, sum...), "\n")
	default:
		return strings.Join(append(head, "Unknown step"), "\n")
	}
}

func (m Model) resetEditWizard() Model {
	m.editModal = false
	m.editStep = createStepRepo
	m.editApp = ""
	m.editErr = nil
	m.editSaving = false
	m.editRepoInput.Blur()
	m.editPathInput.Blur()
	m.editRevInput.Blur()
	m.editClusterIn.Blur()
	m.editNSInput.Blur()
	m.editSyncPolicy = "manual"
	return m
}

func (m Model) updateEditWizard(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m = m.resetEditWizard()
		m.statusLine = "edit cancelled"
		return m, nil
	case "left":
		if m.editStep > createStepRepo {
			m.editStep--
			m.editErr = nil
		}
		return m, nil
	}

	// Ensure only the active input is focused.
	focus := func(step createStep) {
		m.editRepoInput.Blur()
		m.editPathInput.Blur()
		m.editRevInput.Blur()
		m.editClusterIn.Blur()
		m.editNSInput.Blur()
		switch step {
		case createStepRepo:
			m.editRepoInput.Focus()
		case createStepPath:
			m.editPathInput.Focus()
		case createStepRevision:
			m.editRevInput.Focus()
		case createStepCluster:
			m.editClusterIn.Focus()
		case createStepNamespace:
			m.editNSInput.Focus()
		}
	}

	switch m.editStep {
	case createStepRepo:
		if k.String() == "enter" {
			m.editStep = createStepPath
			focus(m.editStep)
			return m, nil
		}
		var cmd tea.Cmd
		m.editRepoInput, cmd = m.editRepoInput.Update(k)
		return m, cmd
	case createStepPath:
		if k.String() == "enter" {
			m.editStep = createStepRevision
			focus(m.editStep)
			return m, nil
		}
		var cmd tea.Cmd
		m.editPathInput, cmd = m.editPathInput.Update(k)
		return m, cmd
	case createStepRevision:
		if k.String() == "enter" {
			m.editStep = createStepCluster
			focus(m.editStep)
			return m, nil
		}
		var cmd tea.Cmd
		m.editRevInput, cmd = m.editRevInput.Update(k)
		return m, cmd
	case createStepCluster:
		if k.String() == "enter" {
			m.editStep = createStepNamespace
			focus(m.editStep)
			return m, nil
		}
		var cmd tea.Cmd
		m.editClusterIn, cmd = m.editClusterIn.Update(k)
		return m, cmd
	case createStepNamespace:
		if k.String() == "enter" {
			m.editStep = createStepSyncPolicy
			return m, nil
		}
		var cmd tea.Cmd
		m.editNSInput, cmd = m.editNSInput.Update(k)
		return m, cmd
	case createStepSyncPolicy:
		switch k.String() {
		case "a":
			m.editSyncPolicy = "auto"
		case "m":
			m.editSyncPolicy = "manual"
		case "enter":
			m.editStep = createStepConfirm
		}
		return m, nil
	case createStepConfirm:
		switch k.String() {
		case "y":
			if m.editSaving {
				return m, nil
			}
			m.editSaving = true
			m.statusLine = "saving…"
			app := argocd.Application{
				Name:           m.editApp,
				Project:        "",
				RepoURL:        strings.TrimSpace(m.editRepoInput.Value()),
				Path:           strings.TrimSpace(m.editPathInput.Value()),
				Revision:       strings.TrimSpace(blankIfEmpty(m.editRevInput.Value(), "main")),
				Cluster:        strings.TrimSpace(m.editClusterIn.Value()),
				Namespace:      strings.TrimSpace(m.editNSInput.Value()),
				SyncPolicy:     m.editSyncPolicy,
				Resources:      nil,
				OperationState: nil,
			}
			return m, m.updateAppCmd(app)
		case "n":
			m = m.resetEditWizard()
			m.statusLine = "edit cancelled"
			return m, nil
		}
	}
	return m, nil
}

func (m Model) renderEditWizard() string {
	head := []string{fmt.Sprintf("Edit application: %s", m.editApp), ""}
	if m.editErr != nil {
		head = append(head, "Error: "+m.editErr.Error(), "")
	}
	if m.editSaving {
		head = append(head, "Saving…", "")
	}

	switch m.editStep {
	case createStepRepo:
		return strings.Join(append(head, "Repo URL", m.editRepoInput.View(), "", "Enter=next  ←=back  Esc=cancel"), "\n")
	case createStepPath:
		return strings.Join(append(head, "Path", m.editPathInput.View(), "", "Enter=next  ←=back  Esc=cancel"), "\n")
	case createStepRevision:
		return strings.Join(append(head, "Revision", m.editRevInput.View(), "", "Enter=next  ←=back  Esc=cancel"), "\n")
	case createStepCluster:
		return strings.Join(append(head, "Destination cluster", m.editClusterIn.View(), "", "Enter=next  ←=back  Esc=cancel"), "\n")
	case createStepNamespace:
		return strings.Join(append(head, "Namespace", m.editNSInput.View(), "", "Enter=next  ←=back  Esc=cancel"), "\n")
	case createStepSyncPolicy:
		return strings.Join(append(head,
			"Sync policy (press 'a' for auto, 'm' for manual)",
			"Current: "+m.editSyncPolicy,
			"",
			"Enter=next  ←=back  Esc=cancel",
		), "\n")
	case createStepConfirm:
		sum := []string{
			"Confirm update:",
			"  repo:      " + strings.TrimSpace(m.editRepoInput.Value()),
			"  path:      " + strings.TrimSpace(m.editPathInput.Value()),
			"  rev:       " + strings.TrimSpace(blankIfEmpty(m.editRevInput.Value(), "main")),
			"  cluster:   " + strings.TrimSpace(m.editClusterIn.Value()),
			"  namespace: " + strings.TrimSpace(m.editNSInput.Value()),
			"  sync:      " + m.editSyncPolicy,
			"",
			"y=save  n=cancel  ←=back",
		}
		return strings.Join(append(head, sum...), "\n")
	default:
		return strings.Join(append(head, "Unknown step"), "\n")
	}
}

func renderResources(rs []argocd.Resource, selected int, focus bool, st styles) string {
	if len(rs) == 0 {
		return "  (none yet)"
	}
	lines := make([]string, 0, len(rs)+1)
	lines = append(lines, "  (tab to focus resources, enter/v to view)")
	for i, r := range rs {
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
		prefix := "  "
		style := st.SidebarItem
		if i == selected {
			prefix = "▶ "
			if focus {
				style = st.SidebarSelected
			} else {
				style = st.SidebarTitle
			}
		}
		lines = append(lines, style.Render(fmt.Sprintf("%s%s/%s (%s) [%s/%s]", prefix, kind, r.Name, ns, health, status)))
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
