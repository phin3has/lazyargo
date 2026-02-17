package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CommandItem struct {
	Title string
	ID    ViewID
}

type CommandPalette struct {
	items []CommandItem
	idx   int

	width  int
	height int
}

func NewCommandPalette(items []CommandItem) CommandPalette {
	return CommandPalette{items: items, idx: 0}
}

// Update returns (updatedPalette, chosenViewID, close).
func (p CommandPalette) Update(msg tea.Msg) (CommandPalette, *ViewID, bool) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		return p, nil, false
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return p, nil, true
		case "up", "k":
			if p.idx > 0 {
				p.idx--
			}
			return p, nil, false
		case "down", "j":
			if p.idx < len(p.items)-1 {
				p.idx++
			}
			return p, nil, false
		case "enter":
			if len(p.items) == 0 {
				return p, nil, true
			}
			id := p.items[p.idx].ID
			return p, &id, true
		}
	}
	return p, nil, false
}

func (p CommandPalette) View() string {
	if len(p.items) == 0 {
		return ""
	}

	title := lipgloss.NewStyle().Bold(true).Render("Command palette")
	help := lipgloss.NewStyle().Faint(true).Render("↑/↓ (or j/k) · Enter select · Esc close")

	selected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	rows := make([]string, 0, len(p.items))
	for i, it := range p.items {
		line := it.Title
		if i == p.idx {
			line = selected.Render("→ " + line)
		} else {
			line = "  " + line
		}
		rows = append(rows, line)
	}

	body := strings.Join(rows, "\n")

	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	return box.Render(title + "\n" + help + "\n\n" + body)
}
