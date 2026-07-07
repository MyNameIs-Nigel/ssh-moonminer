package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type errScreen struct {
	width, height int
}

// NewErrScreen returns a fallback error screen.
func NewErrScreen() Model { return errScreen{} }

func (errScreen) Init() tea.Cmd { return nil }

func (e errScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.WindowSizeMsg:
	case tea.KeyPressMsg:
		return e, tea.Quit
	}
	return e, nil
}

func (e errScreen) View() tea.View {
	msg := "Could not open Moon Miner.\nPress any key to disconnect."
	v := tea.NewView(lipgloss.Place(max(e.width, 1), max(e.height, 1), lipgloss.Center, lipgloss.Center, msg))
	v.AltScreen = true
	v.WindowTitle = "MOON MINER"
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
