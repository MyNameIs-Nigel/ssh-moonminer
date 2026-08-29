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
	footerRow := -1
	for i, row := range lines {
		if strings.Contains(row, "Close to port.") && strings.Contains(row, "Bigger rocks pay") {
			footerRow = i
			break
		}
	}
	if footerRow < 0 {
		t.Fatalf("chart footers were not aligned on the same rendered row:\n%s", strings.Join(lines, "\n"))
	}
	for i := footerRow + 1; i < len(lines); i++ {
		row := lines[i]
		if strings.Contains(row, "│") && strings.Trim(row, " │") != "" {
			t.Fatalf("chart footer row %d was not bottom-aligned inside its panes; later body row %d contains %q", footerRow, i, row)
		}
	}
}

func TestChartMarksOnlyCurrentSystemWithSolidDiamond(t *testing.T) {
	g := newChartGame(t, 80, 24)
	out := ansi.Strip(g.renderChart())
	if !strings.Contains(out, "◆ SOL") || !strings.Contains(out, "◇ ERIDANI DRIFT") {
		t.Fatalf("chart did not distinguish the current system:\n%s", out)
	}

	g.snap.State.SystemID = "eridani"
	out = ansi.Strip(g.renderChart())
	if !strings.Contains(out, "◇ SOL") || !strings.Contains(out, "◆ ERIDANI DRIFT") {
		t.Fatalf("chart did not move the current-system marker:\n%s", out)
	}
}

func TestPirateRadarShowsJammerCountdownBeforeETA(t *testing.T) {
	g := newChartGame(t, 80, 24)
	run := &sim.ActiveRun{JammerRemaining: 7.6, PirateETAMin: 4, PirateETAMax: 8}
	out := ansi.Strip(g.renderPirateRadar(run))
	if !strings.Contains(out, "JAMMED 8s") || strings.Contains(out, "PIRATE ETA") {
		t.Fatalf("jammed radar should hide ETA behind its countdown:\n%s", out)
	}
	run.JammerRemaining = 0
	out = ansi.Strip(g.renderPirateRadar(run))
	if !strings.Contains(out, "PIRATE ETA ~4-8s") {
		t.Fatalf("radar should reveal ETA after jammer expiry:\n%s", out)
	}
}

func TestWideCockpitCentersAndOffsetsHitboxes(t *testing.T) {
	g := newChartGame(t, 180, 24)
	g.View()
	if g.contentWidth() != maxCockpitW || g.contentX() != 18 {
		t.Fatalf("wide content layout = %d at x=%d, want %d at x=18", g.contentWidth(), g.contentX(), maxCockpitW)
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
	// Y is the pointer row, deliberately set low/high to prove it has no
	// bearing on direction. The wheel button is the source of truth.
	g.updateWheel(tea.MouseWheelMsg{Button: tea.MouseWheelUp, Y: 23})
	if g.worldSel != 0 {
		t.Fatalf("wheel up selected %d, want 0", g.worldSel)
	}
	g.updateWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown, Y: 1})
	if g.worldSel != 1 {
		t.Fatalf("wheel down selected %d, want 1", g.worldSel)
	}
	g.updateWheel(tea.MouseWheelMsg{})
	if g.worldSel != 1 {
		t.Fatalf("zero wheel delta selected %d, want 1", g.worldSel)
	}
}
