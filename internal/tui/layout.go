package tui

import (
	"fmt"

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

// overlayHitLine registers a hitbox for one line inside a centered floating
// overlay panel (compositeView centers every overlay the same way — see
// overlay.go) of size panelW×panelH, at body row lineIdx. Shared by every
// overlay with a selectable row list (tweaks, slot picker, remove confirm)
// so the centering/clamping math lives in exactly one place.
func (g *Game) overlayHitLine(panelW, panelH, lineIdx int, label, idPrefix string, data any) {
	y0 := (g.height - panelH) / 2
	x0 := (g.width - panelW) / 2
	if y0 < 0 {
		y0 = 0
	}
	if x0 < 0 {
		x0 = 0
	}
	y := y0 + 1 + lineIdx
	x := x0 + 1
	w := lipgloss.Width(label)
	if w < 1 {
		w = 1
	}
	g.addHit(x, y, w, 1, fmt.Sprintf("%s:%v", idPrefix, data), data)
}
