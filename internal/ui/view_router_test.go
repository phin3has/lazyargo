package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phin3has/lazyargo/internal/config"
)

func TestRootModel_RoutesToActiveView(t *testing.T) {
	root := NewRootModel(config.Default(), &fakeClient{})

	// Window size should be handled at the root and forwarded via SetSize.
	updated, cmd := root.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if cmd != nil {
		t.Fatalf("expected nil cmd for WindowSizeMsg, got %v", cmd)
	}
	if _, ok := updated.(RootModel); !ok {
		t.Fatalf("expected RootModel update result, got %T", updated)
	}

	// A quit key should be routed to the active view.
	_, cmd = root.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("expected a quit cmd")
	}
}
