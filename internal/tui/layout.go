package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
)

const (
	chromeH      = 3
	panelBorderH = 1
	maxCockpitW  = 96
)

// contentWidth keeps the cockpit readable on wide terminals. Above this
// width every game screen uses the same centered frame rather than stretching
// panels just because the terminal happens to be large.
func (g *Game) contentWidth() int {
	if g.width > maxCockpitW {
		return maxCockpitW
	}
	return g.width
}

func (g *Game) contentX() int {
	return max(0, (g.width-g.contentWidth())/2)
}

// positionCockpit centers the shared fixed-width frame. Rendering functions
// deliberately stay unaware of the outer terminal gutter, while hitbox
// helpers below account for the same offset.
func (g *Game) positionCockpit(body string) string {
	left := strings.Repeat(" ", g.contentX())
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		line = ansi.Truncate(line, g.contentWidth(), "")
		lines[i] = left + line
	}
	return strings.Join(lines, "\n")
}

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
	g.addHit(x+g.contentX(), panelBodyY(lineIdx), w, 1, id, data)
}

func (g *Game) hitBodyLine(lineIdx, x int, label string, id string, data any) {
	w := lipgloss.Width(label)
	if w < 1 {
		w = 1
	}
	g.addHit(x+g.contentX(), bodyLineY(lineIdx), w, 1, id, data)
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
