package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

func (g *Game) requestQuit() []tea.Cmd {
	if g.id.IsGuest() {
		g.overlay = ovGuestQuit
		return nil
	}
	return []tea.Cmd{tea.Quit}
}
func (g *Game) updateGuestQuit(k string) []tea.Cmd {
	switch k {
	case "enter", "y":
		return []tea.Cmd{tea.Quit}
	case "esc", "q", "n":
		g.overlay = ovNone
	}
	return nil
}
func (g *Game) renderGuestQuit() string {
	w, h := g.layoutOverlaySize(62, 9)
	lines := reflowOverlayLines([]string{"Progress will be discarded when you leave this guest game.", "Reconnect with an SSH key to save future progress.", "", "[ENTER] LEAVE", "[ESC] STAY"}, w-4)
	for i, line := range lines {
		if strings.Contains(line, "[ENTER]") {
			g.overlayHitLine(w, h, i, line, "guest", "leave")
		}
		if strings.Contains(line, "[ESC]") {
			g.overlayHitLine(w, h, i, line, "guest", "stay")
		}
	}
	return theme.Panel("LEAVE GUEST GAME?", w, h, strings.Join(lines, "\n"), theme.Accent(theme.HueGold))
}
func (g *Game) guestFrame(frame string) string {
	lines := strings.Split(frame, "\n")
	lines[0] = theme.Gold.Render(clipFrameLine("GUEST · NOT SAVED — leaving the game resets progress", g.contentWidth()))
	return strings.Join(lines, "\n")
}
