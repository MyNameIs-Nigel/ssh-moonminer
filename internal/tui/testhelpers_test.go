package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
)

type testViewport struct {
	w int
	h int
}

// responsiveBoundaryWidths and responsiveBoundaryHeights are the documented
// frame-contract edges from docs/tui/06-responsive-menu-overhaul.md.
var (
	responsiveBoundaryWidths  = []int{79, 80, 81, 82, 119, 143, 144, 145, 200}
	responsiveBoundaryHeights = []int{23, 24, 25, 30, 47, 48, 49, 60}
)

// responsiveViewports is a pairwise covering set: every width and height
// boundary appears at least once without the full Cartesian product.
var responsiveViewports = []testViewport{
	{79, 23},  // guard both axes
	{80, 24},  // minimum supported
	{81, 24},  // chart 41/40 remainder
	{82, 30},  // width remainder + mid height
	{119, 30}, // mid width
	{143, 48}, // width below cap
	{144, 48}, // cap both (typical)
	{145, 48}, // width above cap
	{200, 60}, // horizontal + vertical gutters
	{80, 25},  // height above minimum
	{80, 47},  // height below cap
	{80, 49},  // height above cap
	{80, 60},  // vertical gutter only
	{144, 23}, // guard at wide terminal
	{144, 25},
	{144, 47},
	{144, 49},
	{144, 60},
}

// responsiveVariantViewports exercises every render variant without repeating
// the full boundary matrix.
var responsiveVariantViewports = []testViewport{
	{80, 24},
	{81, 24},
	{144, 48},
	{200, 60},
	{80, 60},
	{144, 49},
}

func supportedViewports(vps []testViewport) []testViewport {
	out := make([]testViewport, 0, len(vps))
	for _, vp := range vps {
		if vp.w >= minWidth && vp.h >= minHeight {
			out = append(out, vp)
		}
	}
	return out
}

func (v testViewport) String() string {
	return strings.Join([]string{itoa(v.w), "x", itoa(v.h)}, "")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func newResponsiveTestGame(t testing.TB, width, height int) *Game {
	t.Helper()
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	return &Game{
		content:   c,
		width:     width,
		height:    height,
		scr:       scrChart,
		hits:      hitbox.New(),
		snap:      sim.Snapshot{State: *sim.New(c, 42, 1000)},
		id:        identity.SessionIdentity{Slot: "layout-test"},
		tickCount: 1,
	}
}

func departResponsiveTestGame(t testing.TB, g *Game) *sim.Asteroid {
	t.Helper()
	st := &g.snap.State
	if err := sim.Depart(st, g.content, 1); err != nil {
		t.Fatal(err)
	}
	if len(st.Belt) == 0 {
		t.Fatal("test departure generated an empty belt")
	}
	for i := range st.Belt {
		st.Belt[i].Scanned = true
	}
	return &st.Belt[0]
}

func startResponsiveTestRun(t testing.TB, g *Game) (*sim.State, *sim.ActiveRun, *sim.Asteroid) {
	t.Helper()
	ast := departResponsiveTestGame(t, g)
	if err := sim.Lock(&g.snap.State, g.content, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	g.scr = scrMining
	return &g.snap.State, g.snap.State.Run, ast
}

func withResponsiveMiningRun(t testing.TB, g *Game, fn func(*sim.State, *sim.ActiveRun, *sim.Asteroid)) *Game {
	t.Helper()
	st, run, ast := startResponsiveTestRun(t, g)
	if fn != nil {
		fn(st, run, ast)
	}
	return g
}

func strippedLines(s string) []string {
	return strings.Split(ansi.Strip(s), "\n")
}

func assertPhysicalBounds(t testing.TB, out string, width, height int) {
	t.Helper()
	lines := strings.Split(out, "\n")
	if len(lines) > height {
		t.Errorf("rendered %d physical rows into a %d-row viewport", len(lines), height)
	}
	for y, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Errorf("row %d rendered %d columns into a %d-column viewport: %q", y, got, width, ansi.Strip(line))
		}
	}
}

func assertSemanticAnchors(t testing.TB, out string, anchors ...string) {
	t.Helper()
	plain := ansi.Strip(out)
	for _, anchor := range anchors {
		if !strings.Contains(plain, anchor) {
			t.Errorf("render dropped required semantic anchor %q", anchor)
		}
	}
}

func textPosition(out, text string) (x, y int, ok bool) {
	for y, line := range strippedLines(out) {
		if byteX := strings.Index(line, text); byteX >= 0 {
			return lipgloss.Width(line[:byteX]), y, true
		}
	}
	return 0, 0, false
}

func assertHitboxAtText(t testing.TB, g *Game, out, text, wantID string) {
	t.Helper()
	tx, ty, ok := textPosition(out, text)
	if !ok {
		t.Errorf("cannot check %s hitbox: rendered text %q is missing", wantID, text)
		return
	}
	fx, fy, ok := g.frameMouse(tx, ty)
	if !ok {
		t.Errorf("%s hitbox lookup rejected terminal position (%d,%d) for rendered %q", wantID, tx, ty, text)
		return
	}
	b, ok := g.hits.At(fx, fy)
	if !ok || b.ID != wantID {
		t.Errorf("%s hitbox does not cover rendered %q at frame (%d,%d): got %#v", wantID, text, fx, fy, b)
	}
}

type contentBounds struct {
	left   int
	top    int
	right  int
	bottom int
	ok     bool
}

func nonBlankBounds(out string) contentBounds {
	b := contentBounds{left: int(^uint(0) >> 1), top: int(^uint(0) >> 1), right: -1, bottom: -1}
	for y, line := range strippedLines(out) {
		x := 0
		for _, r := range line {
			cellW := lipgloss.Width(string(r))
			if r == ' ' {
				x += cellW
				continue
			}
			b.ok = true
			if x < b.left {
				b.left = x
			}
			if cellRight := x + max(1, cellW) - 1; cellRight > b.right {
				b.right = cellRight
			}
			if y < b.top {
				b.top = y
			}
			if y > b.bottom {
				b.bottom = y
			}
			x += cellW
		}
	}
	return b
}

func panelStarts(line string) []int {
	var starts []int
	x := 0
	for _, r := range ansi.Strip(line) {
		if r == '┌' {
			starts = append(starts, x)
		}
		x += lipgloss.Width(string(r))
	}
	return starts
}

func assertFrameGeometry(t testing.TB, g *Game, vp testViewport) {
	t.Helper()
	layout := g.frameLayout()
	wantW := min(vp.w, maxFrameW)
	wantH := min(vp.h, maxFrameH)
	wantX := (vp.w - wantW) / 2
	wantY := (vp.h - wantH) / 2
	if layout.Width != wantW || layout.Height != wantH || layout.X != wantX || layout.Y != wantY {
		t.Errorf("frame layout = %+v, want %dx%d at (%d,%d)", layout, wantW, wantH, wantX, wantY)
	}
	if g.contentWidth() != wantW || g.contentHeight() != wantH || g.contentX() != wantX || g.contentY() != wantY {
		t.Errorf("content accessors = %dx%d at (%d,%d), want %dx%d at (%d,%d)",
			g.contentWidth(), g.contentHeight(), g.contentX(), g.contentY(), wantW, wantH, wantX, wantY)
	}
	wantBody := wantH - chromeH - keybarH
	if layout.BodyH != wantBody || g.bodyHeight() != wantBody {
		t.Errorf("body height = layout %d game %d, want %d", layout.BodyH, g.bodyHeight(), wantBody)
	}
}

func assertContentInsideFrame(t testing.TB, out string, g *Game) {
	t.Helper()
	bounds := nonBlankBounds(out)
	if !bounds.ok {
		t.Fatal("rendered no visible content")
	}
	left := g.contentX()
	top := g.contentY()
	right := left + g.contentWidth() - 1
	bottom := top + g.contentHeight() - 1
	if bounds.left < left || bounds.right > right || bounds.top < top || bounds.bottom > bottom {
		t.Errorf("visible content [%d,%d]x[%d,%d] escaped frame [%d,%d]x[%d,%d]",
			bounds.left, bounds.right, bounds.top, bounds.bottom, left, right, top, bottom)
	}
}

func assertOverlayMarkerInsideFrame(t testing.TB, g *Game, out, marker string) {
	t.Helper()
	tx, ty, ok := textPosition(out, marker)
	if !ok {
		t.Fatalf("overlay marker %q not found", marker)
	}
	fx, fy, ok := g.frameMouse(tx, ty)
	if !ok {
		t.Fatalf("overlay marker %q at terminal (%d,%d) is outside the frame", marker, tx, ty)
	}
	if fx < 0 || fy < 0 || fx >= g.contentWidth() || fy >= g.contentHeight() {
		t.Fatalf("overlay marker %q maps to frame (%d,%d) outside %dx%d", marker, fx, fy, g.contentWidth(), g.contentHeight())
	}
}

func assertExactFlex(t testing.TB, available int, specs []FlexSpec, want []int) {
	t.Helper()
	got, err := AllocateFlex(available, specs)
	if err != nil {
		t.Fatalf("AllocateFlex(%d): %v", available, err)
	}
	if len(got) != len(want) {
		t.Fatalf("AllocateFlex(%d) = %v, want %v", available, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AllocateFlex(%d) = %v, want exact %v", available, got, want)
		}
	}
}

func gutterClickPositions(g *Game) (left, right, top, bottom tea.MouseClickMsg) {
	layout := g.frameLayout()
	left = tea.MouseClickMsg{X: layout.X - 1, Y: layout.Y + chromeH}
	right = tea.MouseClickMsg{X: layout.X + layout.Width, Y: layout.Y + chromeH}
	top = tea.MouseClickMsg{X: layout.X + layout.Width/2, Y: layout.Y - 1}
	bottom = tea.MouseClickMsg{X: layout.X + layout.Width/2, Y: layout.Y + layout.Height}
	return left, right, top, bottom
}
