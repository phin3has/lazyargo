package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PlaceholderView is a minimal top-level view used to exercise navigation
// before the real views are implemented.
//
// MET-65 uses this to validate the multi-view router without changing existing
// Application behavior.
type PlaceholderView struct {
	id    ViewID
	title string
}

func NewPlaceholderView(id ViewID, title string) PlaceholderView {
	return PlaceholderView{id: id, title: title}
}

func (v PlaceholderView) ID() ViewID { return v.id }

func (v PlaceholderView) Init() tea.Cmd { return nil }

func (v PlaceholderView) Update(msg tea.Msg) (View, tea.Cmd) {
	return v, nil
}

func (v PlaceholderView) View() string {
	header := lipgloss.NewStyle().Bold(true).Render(v.title)
	body := fmt.Sprintf("%s view coming soon.", v.title)

	content := header + "\n\n" + body + "\n"
	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

func (v PlaceholderView) SetSize(width, height int) {
	// No-op for placeholder views.
}
