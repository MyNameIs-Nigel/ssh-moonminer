package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
)

func newChartGame(t *testing.T, width, height int) *Game {
	t.Helper()
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	return &Game{
		content: c,
		width:   width,
		height:  height,
		scr:     scrChart,
		hits:    hitbox.New(),
		snap:    sim.Snapshot{State: *sim.New(c, 42, 1000)},
		id:      identity.SessionIdentity{Slot: "test"},
	}
}

func TestMarqueeChartTextRollsLongText(t *testing.T) {
	const text = "Slow-cut black ice among razor debris. Every contact attacks."
	first := marqueeChartText(text, 20, 0)
	next := marqueeChartText(text, 20, 1)
	if first == next || len([]rune(first)) != 20 || len([]rune(next)) != 20 {
		t.Fatalf("marquee did not advance in a fixed viewport: %q -> %q", first, next)
	}
}

func TestWrapChartTextPreservesWords(t *testing.T) {
	lines := wrapChartText("Bigger rocks pay more but attract pirates.", 14)
	want := []string{"Bigger rocks", "pay more but", "attract", "pirates."}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Fatalf("wrap = %#v, want %#v", lines, want)
	}
}

func TestChartFootersShareTheBottomRow(t *testing.T) {
	g := newChartGame(t, 80, 24)
	g.worldSel = 1 // Vesta's description fits, making the footer easy to read.
	lines := strings.Split(ansi.Strip(g.renderChart()), "\n")
	footerRow := g.height - 7 // panel top + all body rows except the final border
	if footerRow >= len(lines) {
		t.Fatalf("footer row %d missing from chart:\n%s", footerRow, strings.Join(lines, "\n"))
	}
	row := lines[footerRow]
	if !strings.Contains(row, "Close to port.") || !strings.Contains(row, "Bigger rocks pay") {
		t.Fatalf("chart footer was not bottom-aligned in both columns (row %d): %q", footerRow, row)
	}
}

func TestWideCockpitCentersAndOffsetsHitboxes(t *testing.T) {
	g := newChartGame(t, 120, 24)
	g.View()
	if g.contentWidth() != maxCockpitW || g.contentX() != 12 {
		t.Fatalf("wide content layout = %d at x=%d, want %d at x=12", g.contentWidth(), g.contentX(), maxCockpitW)
	}
	if _, ok := g.hits.At(1, panelBodyY(1)); ok {
		t.Fatal("content hitbox should not remain at the terminal edge on a wide screen")
	}
	if b, ok := g.hits.At(g.contentX()+1, panelBodyY(1)); !ok || b.ID != "world:0" {
		t.Fatalf("offset world hitbox missing: %#v", b)
	}
}

func TestKeybarShowsSteadyActiveShipMarker(t *testing.T) {
	g := newChartGame(t, 80, 24)
	g.tickCount = 0
	first := ansi.Strip(g.renderKeybar("Q QUIT"))
	g.tickCount = 1
	second := ansi.Strip(g.renderKeybar("Q QUIT"))
	if first != second || !strings.HasSuffix(first, "■") {
		t.Fatalf("miner keybar should end in a steady square: %q -> %q", first, second)
	}
	g.snap.State.ActiveShipID = "warden"
	if got := ansi.Strip(g.renderKeybar("Q QUIT")); !strings.HasSuffix(got, "▲") {
		t.Fatalf("fighter keybar should end in triangle: %q", got)
	}
	g.snap.State.ActiveShipID = "mule"
	if got := ansi.Strip(g.renderKeybar("Q QUIT")); !strings.HasSuffix(got, "█") {
		t.Fatalf("freighter keybar should end in block: %q", got)
	}
}

func TestPermitPromptOpensWhenTryingLockedDestination(t *testing.T) {
	g := newChartGame(t, 80, 24)
	g.worldSel = 0 // Ceres is a navigation-permit gate for a fresh pilot.
	if !g.openPermitPrompt() || g.overlay != ovPermit {
		t.Fatal("locked route should open a permit confirmation")
	}
	out := ansi.Strip(g.renderPermitOverlay())
	for _, want := range []string{"PERMIT GATE", "ROUTE PERMIT REQUIRED", "NAVIGATION PERMIT", "BUY PERMIT"} {
		if !strings.Contains(out, want) {
			t.Fatalf("permit prompt missing %q:\n%s", want, out)
		}
	}
}

func TestWheelUpMovesSelectionUp(t *testing.T) {
	g := newChartGame(t, 80, 24)
	g.worldSel = 1
	g.updateWheel(tea.MouseWheelMsg{Y: 1})
	if g.worldSel != 0 {
		t.Fatalf("wheel up selected %d, want 0", g.worldSel)
	}
	g.updateWheel(tea.MouseWheelMsg{Y: -1})
	if g.worldSel != 1 {
		t.Fatalf("wheel down selected %d, want 1", g.worldSel)
	}
	g.updateWheel(tea.MouseWheelMsg{})
	if g.worldSel != 1 {
		t.Fatalf("zero wheel delta selected %d, want 1", g.worldSel)
	}
}
