package tui

import (
	"strings"

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
// the last HUD frame, then the deep-red CONNECTION LOST card inside the
// shared frame shell. Reduced motion skips straight to the final card.
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
	layout := g.frameLayout()
	body := style.Render(g.deathFrameText)
	return lipgloss.Place(layout.Width, layout.Height, lipgloss.Left, lipgloss.Top, body,
		lipgloss.WithWhitespaceStyle(style))
}

// renderDeathFinal draws the settled dark-red failure card inside the frame.
// Chrome and keybar rows are reserved but carry no normal HUD data.
func (g *Game) renderDeathFinal() string {
	layout := g.frameLayout()
	bgStyle := lipgloss.NewStyle().Background(lipgloss.Color(deathBg))
	textStyle := bgStyle.Foreground(lipgloss.Color(deathFg)).Bold(true)
	dimStyle := bgStyle.Foreground(lipgloss.Color(deathDim))

	chrome := strings.Join([]string{
		bgStyle.Render(strings.Repeat(" ", layout.Width)),
		bgStyle.Render(strings.Repeat(" ", layout.Width)),
		bgStyle.Render(strings.Repeat(" ", layout.Width)),
	}, "\n")

	line := textStyle.Render("CONNECTION LOST")
	body := lipgloss.Place(layout.Width, layout.BodyH, lipgloss.Center, lipgloss.Center, line,
		lipgloss.WithWhitespaceStyle(bgStyle))

	hint := dimStyle.Render("press any key")
	keybar := lipgloss.Place(layout.Width, keybarH, lipgloss.Center, lipgloss.Bottom, hint,
		lipgloss.WithWhitespaceStyle(bgStyle))

	return chrome + "\n" + body + "\n" + keybar
}
