package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

const (
	deathBg  = "#1a0006"
	deathFg  = "#ff2a3a"
	deathDim = "#8a1420"
)

// renderDeath draws the ship-loss sequence: a brief CRT-dying flicker over
// the last HUD frame, then the deep-red, full-screen CONNECTION LOST card.
// Reduced motion skips straight to the final card.
func (g *Game) renderDeath() string {
	if g.deathFrame < g.deathFlickerFrames {
		return g.renderDeathFlicker()
	}
	return g.renderDeathFinal()
}

// renderDeathFlicker cycles the last real mining-screen frame (stripped of
// its normal color) through a few monochrome/inverted states to sell a dying
// terminal before the hard cut to CONNECTION LOST.
func (g *Game) renderDeathFlicker() string {
	var style lipgloss.Style
	switch g.deathFrame % 3 {
	case 0:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#e8eef5")).Background(lipgloss.Color("#000000"))
	case 1:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#3a3a3a")).Background(lipgloss.Color("#000000"))
	default:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#0a0a0a")).Background(lipgloss.Color("#d8d8d8"))
	}
	body := style.Render(g.deathFrameText)
	return lipgloss.Place(g.width, g.height, lipgloss.Left, lipgloss.Top, body,
		lipgloss.WithWhitespaceStyle(style))
}

// renderDeathFinal draws the settled dark-red full-screen failure card. No
// verbose recap appears here — that's the next screen's job.
func (g *Game) renderDeathFinal() string {
	bgStyle := lipgloss.NewStyle().Background(lipgloss.Color(deathBg))
	textStyle := bgStyle.Foreground(lipgloss.Color(deathFg)).Bold(true)

	line := textStyle.Render("CONNECTION LOST")
	body := lipgloss.Place(g.width, g.height-2, lipgloss.Center, lipgloss.Center, line,
		lipgloss.WithWhitespaceStyle(bgStyle))
	hint := bgStyle.Foreground(lipgloss.Color(deathDim)).Render("press any key")
	hintLine := lipgloss.Place(g.width, 1, lipgloss.Center, lipgloss.Top, hint,
		lipgloss.WithWhitespaceStyle(bgStyle))
	return body + "\n" + hintLine
}
