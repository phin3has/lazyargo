package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up            key.Binding
	Down          key.Binding
	Refresh       key.Binding
	RefreshDetail key.Binding
	RefreshHard   key.Binding
	Diff          key.Binding
	History       key.Binding
	ToggleDrift   key.Binding
	SyncBatch     key.Binding
	SyncApp       key.Binding
	Rollback      key.Binding
	TerminateOp   key.Binding
	DeleteApp     key.Binding
	CreateApp     key.Binding
	EditApp       key.Binding
	Filter        key.Binding
	Sort          key.Binding
	Clear         key.Binding
	Help          key.Binding
	Quit          key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Refresh, k.RefreshDetail, k.RefreshHard, k.Diff, k.History, k.ToggleDrift, k.SyncBatch, k.SyncApp, k.Rollback, k.TerminateOp, k.DeleteApp, k.CreateApp, k.EditApp, k.Filter, k.Sort, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Refresh, k.RefreshDetail, k.RefreshHard, k.Diff, k.History},
		{k.ToggleDrift, k.SyncBatch, k.SyncApp, k.Rollback, k.TerminateOp, k.DeleteApp, k.CreateApp, k.EditApp, k.Filter, k.Sort, k.Clear, k.Diff, k.History},
		{k.Help, k.Quit},
	}
}

func newKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh list"),
		),
		RefreshDetail: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "refresh details"),
		),
		Diff: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "diff"),
		),
		History: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "history"),
		),
		RefreshHard: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "hard refresh"),
		),
		ToggleDrift: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "drift only"),
		),
		SyncBatch: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sync drifted"),
		),
		SyncApp: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "sync app"),
		),
		Rollback: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "rollback"),
		),
		TerminateOp: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "terminate op"),
		),
		DeleteApp: key.NewBinding(
			key.WithKeys("ctrl+d", "delete"),
			key.WithHelp("ctrl+d", "delete app"),
		),
		CreateApp: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "create app"),
		),
		EditApp: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit app"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Sort: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "sort"),
		),
		Clear: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear filter"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}
