package ui

import "github.com/charmbracelet/lipgloss"

type styles struct {
	App             lipgloss.Style
	Header          lipgloss.Style
	Sidebar         lipgloss.Style
	SidebarTitle    lipgloss.Style
	SidebarItem     lipgloss.Style
	SidebarSelected lipgloss.Style
	Main            lipgloss.Style
	HelpBar         lipgloss.Style
	Error           lipgloss.Style
}

func newStyles() styles {
	border := lipgloss.RoundedBorder()

	return styles{
		App: lipgloss.NewStyle(),
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("62")).
			Padding(0, 1),
		Sidebar: lipgloss.NewStyle().
			Border(border).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1),
		SidebarTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")),
		SidebarItem: lipgloss.NewStyle(),
		SidebarSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")),
		Main: lipgloss.NewStyle().
			Border(border).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1),
		HelpBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1),
		Error: lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
	}
}
