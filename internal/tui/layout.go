package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
)

const (
	chromeH      = 3
	panelBorderH = 1
)

// panelBodyY returns the terminal row for a line index inside a bordered panel body.
func panelBodyY(lineIdx int) int {
	return chromeH + panelBorderH + lineIdx
}

// bodyLineY returns the terminal row for a line in the screen body (no panel border).
func bodyLineY(lineIdx int) int {
	return chromeH + lineIdx
}

func (g *Game) addHit(x, y, w, h int, id string, data any) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	g.hits.Add(hitbox.Box{X: x, Y: y, W: w, H: h, ID: id, Data: data})
}

func (g *Game) hitPanelLine(lineIdx, x int, label string, id string, data any) {
	w := lipgloss.Width(label)
	if w < 1 {
		w = 1
	}
	g.addHit(x, panelBodyY(lineIdx), w, 1, id, data)
}

func (g *Game) hitBodyLine(lineIdx, x int, label string, id string, data any) {
	w := lipgloss.Width(label)
	if w < 1 {
		w = 1
	}
	g.addHit(x, bodyLineY(lineIdx), w, 1, id, data)
}
