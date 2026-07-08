package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

// keyDeath handles the stark death screen: any key routes to the dock
// respawn summary, where the loss is explained. No other action is
// available here — the cockpit terminal is dead.
func (g *Game) keyDeath(k string) []tea.Cmd {
	if k == "" {
		return nil
	}
	g.scr = scrSummary
	return nil
}

// renderDeath draws the dark red full-screen failure state. No verbose
// recap appears here — that's the next screen's job.
func (g *Game) renderDeath() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff2a3a")).
		Background(lipgloss.Color("#1a0006")).
		Bold(true)
	msg := style.Render("CONNECTION LOST")
	body := lipgloss.Place(g.width, g.height-2, lipgloss.Center, lipgloss.Center, msg)
	hint := theme.DimStyle.Render("press any key")
	return body + "\n" + hint
}
