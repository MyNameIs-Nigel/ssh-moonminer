package tui

import (
	"strings"
	"testing"

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

var responsiveViewports = []testViewport{
	{79, 23},
	{80, 24},
	{100, 32},
	{120, 40},
	{144, 48},
	{160, 60},
	{200, 60},
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
	x, y, ok := textPosition(out, text)
	if !ok {
		t.Errorf("cannot check %s hitbox: rendered text %q is missing", wantID, text)
		return
	}
	b, ok := g.hits.At(x, y)
	if !ok || b.ID != wantID {
		t.Errorf("%s hitbox does not cover rendered %q at (%d,%d): got %#v", wantID, text, x, y, b)
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
