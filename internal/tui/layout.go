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
	keybarH      = 2
	panelBorderH = 1
	minFrameW    = 80
	maxFrameW    = 144
	minFrameH    = 24
	maxFrameH    = 48
)

// Rect is an axis-aligned region in frame coordinates.
type Rect struct {
	X, Y, W, H int
}

// FrameLayout is the bounded game surface placed inside the terminal.
type FrameLayout struct {
	TermW, TermH int
	X, Y         int
	Width        int
	Height       int
	BodyH        int
}

// FlexSpec declares one region's minimum extent and positive flex weight for
// AllocateFlex.
type FlexSpec struct {
	Min    int
	Weight int
}

// computeFrameLayout derives the centered frame for a supported terminal size.
// Callers must gate on minWidth/minHeight before using the result.
func computeFrameLayout(termW, termH int) FrameLayout {
	w := clampInt(termW, minFrameW, maxFrameW)
	h := clampInt(termH, minFrameH, maxFrameH)
	return FrameLayout{
		TermW: termW, TermH: termH,
		X:     (termW - w) / 2,
		Y:     (termH - h) / 2,
		Width: w, Height: h,
		BodyH: h - chromeH - keybarH,
	}
}

func (g *Game) frameLayout() FrameLayout {
	return computeFrameLayout(g.width, g.height)
}

// contentWidth is the effective frame width (80–144).
func (g *Game) contentWidth() int {
	if g.width < minWidth || g.height < minHeight {
		return g.width
	}
	return g.frameLayout().Width
}

// contentHeight is the effective frame height (24–48).
func (g *Game) contentHeight() int {
	if g.width < minWidth || g.height < minHeight {
		return g.height
	}
	return g.frameLayout().Height
}

// contentX is the terminal column of the frame's left edge.
func (g *Game) contentX() int {
	if g.width < minWidth || g.height < minHeight {
		return 0
	}
	return g.frameLayout().X
}

// contentY is the terminal row of the frame's top edge.
func (g *Game) contentY() int {
	if g.width < minWidth || g.height < minHeight {
		return 0
	}
	return g.frameLayout().Y
}

// bodyHeight is the flexible body row count between chrome and keybar.
func (g *Game) bodyHeight() int {
	if g.width < minWidth || g.height < minHeight {
		return max(0, g.height-chromeH-keybarH)
	}
	return g.frameLayout().BodyH
}

// AllocateFlex distributes available cells across regions using the
// largest-remainder rule with ties broken by declaration order.
func AllocateFlex(available int, specs []FlexSpec) ([]int, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("allocate flex: no regions")
	}
	minTotal := 0
	weightTotal := 0
	for _, s := range specs {
		if s.Weight <= 0 {
			return nil, fmt.Errorf("allocate flex: non-positive weight")
		}
		if s.Min < 0 {
			return nil, fmt.Errorf("allocate flex: negative minimum")
		}
		minTotal += s.Min
		weightTotal += s.Weight
	}
	if available < minTotal {
		return nil, fmt.Errorf("allocate flex: insufficient space")
	}
	widths := make([]int, len(specs))
	remainders := make([]int, len(specs))
	assigned := 0
	remaining := available - minTotal
	for i, s := range specs {
		widths[i] = s.Min
		prod, ok := safeMul(remaining, s.Weight)
		if !ok {
			return nil, fmt.Errorf("allocate flex: integer overflow")
		}
		extra := prod / weightTotal
		widths[i] += extra
		remainders[i] = prod % weightTotal
		assigned += extra
	}
	awarded := make([]bool, len(widths))
	for cells := remaining - assigned; cells > 0; cells-- {
		best := -1
		for i := range widths {
			if !awarded[i] && (best < 0 || remainders[i] > remainders[best]) {
				best = i
			}
		}
		if best < 0 {
			return nil, fmt.Errorf("allocate flex: remainder distribution stalled")
		}
		widths[best]++
		awarded[best] = true
	}
	return widths, nil
}

// safeMul multiplies two non-negative ints and reports whether the product
// fits in int (used by AllocateFlex before largest-remainder division).
func safeMul(a, b int) (int, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if a < 0 || b < 0 {
		return 0, false
	}
	c := a * b
	if c/a != b {
		return 0, false
	}
	return c, true
}

// mustAllocateFlex panics on invalid flex input. Screen helpers use this for
// layouts whose specs are fixed at compile time; AllocateFlex retains errors
// for callers that need to surface them.
func mustAllocateFlex(available int, specs []FlexSpec) []int {
	widths, err := AllocateFlex(available, specs)
	if err != nil {
		panic("mustAllocateFlex: " + err.Error())
	}
	return widths
}

// chartPaneWidths allocates the STAR CHART / PORT SERVICES split (40/40 min, 1:1).
func chartPaneWidths(frameW int) (left, right int) {
	widths := mustAllocateFlex(frameW, []FlexSpec{{Min: 40, Weight: 1}, {Min: 40, Weight: 1}})
	return widths[0], widths[1]
}

// shipyardPaneWidths allocates HANGAR / LOADOUT / STATUS (24/32/24 min, 3:4:3).
func shipyardPaneWidths(frameW int) (hangar, loadout, status int) {
	widths := mustAllocateFlex(frameW, []FlexSpec{
		{Min: 24, Weight: 3}, {Min: 32, Weight: 4}, {Min: 24, Weight: 3},
	})
	return widths[0], widths[1], widths[2]
}

// miningRunPaneWidths allocates STATUS-ACTIONS / TACTICAL VIEWPORT (46/34 min, 3:2).
func miningRunPaneWidths(frameW int) (left, right int) {
	widths := mustAllocateFlex(frameW, []FlexSpec{{Min: 46, Weight: 3}, {Min: 34, Weight: 2}})
	return widths[0], widths[1]
}

// clipFrameLine trims a styled line to frame display width with balanced ANSI.
func clipFrameLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(line) <= width {
		return line
	}
	return ansi.Truncate(line, width, "")
}

// normalizeFrameLines clips every line to width and pads or truncates to height.
func normalizeFrameLines(lines []string, width, height int) []string {
	if height < 0 {
		height = 0
	}
	out := make([]string, height)
	for i := 0; i < height; i++ {
		if i < len(lines) {
			out[i] = clipFrameLine(lines[i], width)
		}
	}
	return out
}

// placeFrameOnTerminal pads the clipped frame with terminal gutters.
func placeFrameOnTerminal(layout FrameLayout, frame string) string {
	lines := normalizeFrameLines(strings.Split(frame, "\n"), layout.Width, layout.Height)
	left := strings.Repeat(" ", layout.X)
	termLines := make([]string, layout.TermH)
	for i := range termLines {
		termLines[i] = ""
	}
	for i, line := range lines {
		row := layout.Y + i
		if row < 0 || row >= layout.TermH {
			continue
		}
		termLines[row] = left + line
	}
	return strings.Join(termLines, "\n")
}

// assembleFrameContent clips and pads raw frame output to the layout contract.
func (g *Game) assembleFrameContent(raw string) string {
	layout := g.frameLayout()
	return strings.Join(normalizeFrameLines(strings.Split(raw, "\n"), layout.Width, layout.Height), "\n")
}

// placeFrame centers the bounded frame inside the terminal.
func (g *Game) placeFrame(frame string) string {
	return placeFrameOnTerminal(g.frameLayout(), frame)
}

// positionCockpit horizontally centers frame lines. Deprecated for new code —
// prefer assembleFrameContent + placeFrame — but retained for death-flicker capture.
func (g *Game) positionCockpit(body string) string {
	layout := g.frameLayout()
	lines := normalizeFrameLines(strings.Split(body, "\n"), layout.Width, layout.Height)
	left := strings.Repeat(" ", layout.X)
	for i, line := range lines {
		lines[i] = left + line
	}
	return strings.Join(lines, "\n")
}

// renderBottomKeybar pads the body to the flexible frame body height and
// attaches the shared two-row keybar at the frame bottom.
func (g *Game) renderBottomKeybar(body, hint string) string {
	lines := strings.Split(body, "\n")
	bodyH := g.bodyHeight()
	for len(lines) < bodyH {
		lines = append(lines, "")
	}
	if len(lines) > bodyH {
		lines = lines[:bodyH]
	}
	return strings.Join(lines, "\n") + "\n" + g.renderKeybar(hint)
}

// panelBodyY returns the frame row for a line index inside a bordered panel body.
func panelBodyY(lineIdx int) int {
	return chromeH + panelBorderH + lineIdx
}

// bodyLineY returns the frame row for a line in the screen body (no panel border).
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

// frameMouse maps a terminal pointer position to frame coordinates. Positions
// in terminal gutters or outside the frame bounds are rejected.
func (g *Game) frameMouse(termX, termY int) (x, y int, ok bool) {
	if g.width < minWidth || g.height < minHeight {
		return 0, 0, false
	}
	layout := g.frameLayout()
	x = termX - layout.X
	y = termY - layout.Y
	if x < 0 || y < 0 || x >= layout.Width || y >= layout.Height {
		return 0, 0, false
	}
	return x, y, true
}

// hitAt translates terminal coordinates once and looks up the topmost box.
func (g *Game) hitAt(termX, termY int) (hitbox.Box, bool) {
	x, y, ok := g.frameMouse(termX, termY)
	if !ok {
		return hitbox.Box{}, false
	}
	return g.hits.At(x, y)
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

// layoutOverlaySize clamps an overlay's intrinsic outer size to the frame
// body band between chrome and keybar.
func (g *Game) layoutOverlaySize(intrinsicW, intrinsicH int) (w, h int) {
	layout := g.frameLayout()
	w = clampInt(intrinsicW, 4, layout.Width)
	h = clampInt(intrinsicH, 3, layout.BodyH)
	return w, h
}

// overlayPanelOrigin returns the frame-local top-left cell of a panel centered
// horizontally and vertically within the body band.
func overlayPanelOrigin(frameW, bodyH, panelW, panelH int) (x, y int) {
	x = (frameW - panelW) / 2
	y = chromeH + (bodyH-panelH)/2
	if x < 0 {
		x = 0
	}
	if y < chromeH {
		y = chromeH
	}
	return x, y
}

// scrollWindowLines returns visible rows from lines starting at scroll.
func scrollWindowLines(lines []string, scroll, visible int) []string {
	if visible < 1 {
		return nil
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > len(lines) {
		scroll = len(lines)
	}
	end := scroll + visible
	if end > len(lines) {
		end = len(lines)
	}
	out := append([]string(nil), lines[scroll:end]...)
	for len(out) < visible {
		out = append(out, "")
	}
	return out
}

// clampScroll keeps scroll within [0, max(0, total-visible)].
func clampScroll(scroll *int, total, visible int) {
	maxScroll := max(0, total-visible)
	if *scroll < 0 {
		*scroll = 0
	}
	if *scroll > maxScroll {
		*scroll = maxScroll
	}
}

// centeredScroll returns a scroll offset that keeps sel near the middle of a
// viewport when total exceeds visible.
func centeredScroll(sel, total, visible int) int {
	if total <= visible || visible < 1 {
		return 0
	}
	scroll := sel - visible/2
	if scroll < 0 {
		return 0
	}
	maxScroll := total - visible
	if scroll > maxScroll {
		return maxScroll
	}
	return scroll
}

// panelViewportWithFooter bottom-aligns footer rows inside rows, scrolling top
// so keepIdx stays visible when possible. Returns the composed lines and the
// scroll offset applied to top.
func panelViewportWithFooter(top, bottom []string, rows, keepIdx int) ([]string, int) {
	if rows < 1 {
		return nil, 0
	}
	if len(bottom) > rows {
		bottom = bottom[len(bottom)-rows:]
		return bottom, 0
	}
	avail := rows - len(bottom)
	if len(top) <= avail {
		return bottomAlignPanelLines(top, bottom, rows), 0
	}
	scroll := centeredScroll(keepIdx, len(top), avail)
	if scroll > len(top)-avail {
		scroll = len(top) - avail
	}
	if scroll < 0 {
		scroll = 0
	}
	visibleTop := top[scroll : scroll+avail]
	out := make([]string, 0, rows)
	out = append(out, visibleTop...)
	return append(out, bottom...), scroll
}

// reflowColumnLines word-wraps each line to width display cells.
func reflowColumnLines(lines []string, width int) []string {
	if width < 1 {
		return lines
	}
	var out []string
	for _, line := range lines {
		out = append(out, reflowOverlayLines([]string{line}, width)...)
	}
	return out
}

// truncateColumnLines clips every line to width.
func truncateColumnLines(lines []string, width int) []string {
	for i := range lines {
		lines[i] = clipFrameLine(lines[i], width)
	}
	return lines
}

// layoutPinnedColumn reflows top, pins bottom rows at the column foot, and
// scrolls top from the end so the newest status sits above mandatory actions.
func layoutPinnedColumn(top, bottom []string, width, height int) ([]string, int) {
	top = reflowColumnLines(top, width)
	bottom = truncateColumnLines(bottom, width)
	if height < 1 {
		return nil, 0
	}
	if len(bottom) > height {
		bottom = bottom[len(bottom)-height:]
		return bottom, 0
	}
	avail := height - len(bottom)
	if len(top) <= avail {
		out := append(append([]string(nil), top...), make([]string, avail-len(top))...)
		return append(out, bottom...), 0
	}
	scroll := len(top) - avail
	if scroll < 0 {
		scroll = 0
	}
	visibleTop := top[scroll : scroll+avail]
	return append(append([]string(nil), visibleTop...), bottom...), scroll
}

// overlayPanelViewport exposes a scroll window that keeps sel visible when
// lines exceed the overlay body height.
func overlayPanelViewport(lines []string, sel, panelBodyH int) []string {
	if panelBodyH < 1 {
		return nil
	}
	if len(lines) <= panelBodyH {
		return lines
	}
	scroll := centeredScroll(sel, len(lines), panelBodyH)
	return scrollWindowLines(lines, scroll, panelBodyH)
}

// reflowOverlayLines word-wraps each input line to innerW display cells while
// preserving a single leading style prefix on wrapped segments.
func reflowOverlayLines(lines []string, innerW int) []string {
	if innerW < 1 {
		return lines
	}
	var out []string
	for _, line := range lines {
		out = append(out, reflowOverlayLine(line, innerW)...)
	}
	return out
}

func reflowOverlayLine(line string, innerW int) []string {
	plain := ansi.Strip(line)
	if plain == "" {
		return []string{""}
	}
	if lipgloss.Width(plain) <= innerW {
		return []string{line}
	}
	prefix := overlayStylePrefix(line, plain)
	wrapped := wrapChartText(plain, innerW)
	out := make([]string, len(wrapped))
	for i, segment := range wrapped {
		out[i] = prefix + segment
		if prefix != "" {
			out[i] += "\x1b[0m"
		}
	}
	return out
}

func overlayStylePrefix(styled, plain string) string {
	if styled == "" || plain == "" {
		return ""
	}
	idx := strings.Index(styled, plain)
	if idx < 0 {
		return ""
	}
	return styled[:idx]
}

// overlayHitLine registers a hitbox for one line inside a centered floating
// overlay panel of size panelW×panelH, at body row lineIdx.
func (g *Game) overlayHitLine(panelW, panelH, lineIdx int, label, idPrefix string, data any) {
	layout := g.frameLayout()
	x0, y0 := overlayPanelOrigin(layout.Width, layout.BodyH, panelW, panelH)
	y := y0 + 1 + lineIdx
	x := x0 + 1
	w := lipgloss.Width(label)
	if w < 1 {
		w = 1
	}
	g.addHit(x, y, w, 1, fmt.Sprintf("%s:%v", idPrefix, data), data)
}
