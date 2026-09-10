package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
	"github.com/mynameis-nigel/ssh-moonminer/internal/version"
)

func TestHelpOverlayShowsVersion(t *testing.T) {
	g := &Game{overlay: ovHelp, scr: scrChart}
	out := g.renderHelpOverlay()
	want := "MOON MINER v" + version.Version + " (" + version.Channel + ")"
	if !strings.Contains(out, want) {
		t.Fatalf("help overlay: expected %q, got:\n%s", want, out)
	}
}

func TestPanelBodyY(t *testing.T) {
	if got := panelBodyY(0); got != 4 {
		t.Fatalf("panelBodyY(0) = %d, want 4", got)
	}
	if got := panelBodyY(1); got != 5 {
		t.Fatalf("panelBodyY(1) = %d, want 5", got)
	}
}

func TestBottomKeybarUsesTheScreenBottom(t *testing.T) {
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	g := &Game{content: c, width: 80, height: 30, snap: sim.Snapshot{State: *sim.New(c, 1, 1000)}}
	out := g.renderBottomKeybar("BELT CONTENT", "Q DOCK")
	lines := strings.Split(out, "\n")
	wantRows := g.bodyHeight() + keybarH
	if len(lines) != wantRows {
		t.Fatalf("screen body has %d rows, want %d (body %d + keybar %d)", len(lines), wantRows, g.bodyHeight(), keybarH)
	}
	if !strings.Contains(lines[len(lines)-1], "DOCK") {
		t.Fatalf("keybar is not on the final screen row: %q", lines[len(lines)-1])
	}
}

func TestChartHitboxAlignment(t *testing.T) {
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	st := sim.New(c, 42, 1000)
	g := &Game{
		content:   c,
		width:     80,
		height:    24,
		scr:       scrChart,
		hits:      hitbox.New(),
		snap:      sim.Snapshot{State: *st},
		id:        identity.SessionIdentity{Slot: "test"},
		worldSel:  0,
		tickCount: 1,
	}
	out := g.View().Content
	assertHitboxAtText(t, g, out, c.Worlds[0].Name, "world:0")
	assertHitboxAtText(t, g, out, "REFUEL", "svc:refuel")
	assertHitboxAtText(t, g, out, "[R] REPAIR", "svc:repair")
}

func TestMiningHitboxAlignment(t *testing.T) {
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	st := sim.New(c, 42, 1000)
	_ = sim.Depart(st, c, 1)
	st.Belt[0].Scanned = true
	st.Fuel = 500
	if err := sim.Lock(st, c, st.Belt[0].ID, 1000); err != nil {
		t.Fatal(err)
	}
	st.Run.MinedUnits = 50
	st.Run.CargoValue = 100
	st.CargoUnits = 77
	g := &Game{
		content:   c,
		width:     80,
		height:    24,
		scr:       scrMining,
		hits:      hitbox.New(),
		snap:      sim.Snapshot{State: *st},
		id:        identity.SessionIdentity{Slot: "test"},
		tickCount: 1,
	}
	view := g.View()
	wantCargo := fmt.Sprintf("CARGO %.0f/%.0f", st.CargoUnits+sim.RunHeldUnits(st.Run), sim.CargoCapacityUnits(st, c))
	if !strings.Contains(view.Content, wantCargo) {
		t.Fatalf("mining cargo display missing %q:\n%s", wantCargo, view.Content)
	}

	assertHitboxAtText(t, g, view.Content, "BAIL", "btn:bail")

	// The asteroid click target covers the field, but BAIL itself remains a
	// narrow control line rather than a full-width target.
	_, bailY, ok := textPosition(view.Content, "BAIL")
	if !ok {
		t.Fatal("rendered BAIL control is missing")
	}
	frameY := bailY - g.contentY()
	if b, ok := g.hits.At(g.contentWidth()-1, frameY); ok && b.ID == "btn:bail" {
		t.Fatalf("btn:bail hitbox unexpectedly spans the cockpit at frame y=%d", frameY)
	}
}

func TestMiningSkillCheckHitboxAlignment(t *testing.T) {
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	st := sim.New(c, 42, 1000)
	_ = sim.Depart(st, c, 1)
	st.Belt[0].Scanned = true
	st.Fuel = 500
	if err := sim.Lock(st, c, st.Belt[0].ID, 1000); err != nil {
		t.Fatal(err)
	}
	st.Run.MinedUnits = 50
	st.Run.CargoValue = 100
	st.Run.SkillCheck = &sim.SkillCheck{Window: 3.0}
	g := &Game{
		content:   c,
		width:     80,
		height:    24,
		scr:       scrMining,
		hits:      hitbox.New(),
		snap:      sim.Snapshot{State: *st},
		id:        identity.SessionIdentity{Slot: "test"},
		tickCount: 1,
	}
	out := g.View().Content

	// The countdown is back in the left status column, while the viewport also
	// accepts a click directly on the lit pressure point.
	assertHitboxAtText(t, g, out, "PRESSURE POINT ACTIVE", "btn:skillcheck")
	assertHitboxAtText(t, g, out, "BAIL", "btn:bail")
}

func TestAllocateFlexChartTieOrderAt81(t *testing.T) {
	widths, err := AllocateFlex(80, []FlexSpec{{Min: 40, Weight: 1}, {Min: 40, Weight: 1}})
	if err != nil || widths[0] != 40 || widths[1] != 40 {
		t.Fatalf("80 columns = %v err=%v, want 40/40", widths, err)
	}
	widths, err = AllocateFlex(81, []FlexSpec{{Min: 40, Weight: 1}, {Min: 40, Weight: 1}})
	if err != nil || widths[0] != 41 || widths[1] != 40 {
		t.Fatalf("81 columns = %v err=%v, want 41/40 (declaration-order tie)", widths, err)
	}
}

func TestAllocateFlexPreservesSum(t *testing.T) {
	specs := []FlexSpec{{Min: 24, Weight: 3}, {Min: 32, Weight: 4}, {Min: 24, Weight: 3}}
	for _, total := range []int{80, 100, 144} {
		widths, err := AllocateFlex(total, specs)
		if err != nil {
			t.Fatalf("AllocateFlex(%d): %v", total, err)
		}
		sum := 0
		for _, w := range widths {
			sum += w
		}
		if sum != total {
			t.Fatalf("widths %v sum to %d, want %d", widths, sum, total)
		}
	}
}

func TestAllocateFlexOneRegion(t *testing.T) {
	widths, err := AllocateFlex(42, []FlexSpec{{Min: 10, Weight: 1}})
	if err != nil || len(widths) != 1 || widths[0] != 42 {
		t.Fatalf("one region = %#v err=%v, want [42]", widths, err)
	}
}

func TestAllocateFlexZeroRemaining(t *testing.T) {
	widths, err := AllocateFlex(80, []FlexSpec{{Min: 40, Weight: 1}, {Min: 40, Weight: 1}})
	if err != nil || widths[0] != 40 || widths[1] != 40 {
		t.Fatalf("zero remaining = %#v err=%v", widths, err)
	}
}

func TestAllocateFlexInvalidInputs(t *testing.T) {
	cases := []struct {
		name      string
		available int
		specs     []FlexSpec
	}{
		{name: "empty", available: 10, specs: nil},
		{name: "non-positive weight", available: 10, specs: []FlexSpec{{Min: 1, Weight: 0}}},
		{name: "negative minimum", available: 10, specs: []FlexSpec{{Min: -1, Weight: 1}}},
		{name: "insufficient space", available: 10, specs: []FlexSpec{{Min: 6, Weight: 1}, {Min: 6, Weight: 1}}},
		{name: "integer overflow", available: 1 << 62, specs: []FlexSpec{{Min: 0, Weight: 1 << 20}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := AllocateFlex(tc.available, tc.specs); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestAllocateFlexLargeRemainingPreservesSum(t *testing.T) {
	specs := []FlexSpec{{Min: 40, Weight: 1}, {Min: 40, Weight: 1}}
	const total = 1_000_000
	widths, err := AllocateFlex(total, specs)
	if err != nil {
		t.Fatal(err)
	}
	sum := widths[0] + widths[1]
	if sum != total || widths[0] < 40 || widths[1] < 40 {
		t.Fatalf("large allocation = %v sum=%d, want sum %d with mins honored", widths, sum, total)
	}
	want := total / 2
	if widths[0] != want || widths[1] != want {
		t.Fatalf("equal-weight large allocation = %v, want %d/%d", widths, want, want)
	}
}

func TestSafeMulDetectsOverflow(t *testing.T) {
	if _, ok := safeMul(1<<62, 4); ok {
		t.Fatal("expected overflow for large product")
	}
	if got, ok := safeMul(1_000_000, 7); !ok || got != 7_000_000 {
		t.Fatalf("safeMul(1e6,7) = (%d,%v), want (7000000,true)", got, ok)
	}
}

func TestMustAllocateFlexPanicsOnInvalidInput(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("mustAllocateFlex did not panic on invalid input")
		}
	}()
	mustAllocateFlex(10, nil)
}

func TestClipFrameLinePreservesBalancedANSI(t *testing.T) {
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Render("ABCDEFGHIJ")
	clipped := clipFrameLine(styled, 5)
	if lipgloss.Width(clipped) != 5 {
		t.Fatalf("clipped width = %d, want 5", lipgloss.Width(clipped))
	}
	if plain := ansi.Strip(clipped); plain != "ABCDE" {
		t.Fatalf("clipped plain = %q, want ABCDE", plain)
	}
	if !strings.HasPrefix(clipped, "\x1b") {
		t.Fatalf("clipped line should keep opening style sequence: %q", clipped)
	}
}

func TestPanelViewportWithFooterKeepsSelection(t *testing.T) {
	top := []string{"a", "b", "c", "d", "e", "f"}
	bottom := []string{"footer"}
	lines, scroll := panelViewportWithFooter(top, bottom, 4, 4)
	if scroll != 3 {
		t.Fatalf("scroll = %d, want 3", scroll)
	}
	if len(lines) != 4 || lines[len(lines)-1] != "footer" || lines[1] != "e" {
		t.Fatalf("viewport = %#v", lines)
	}
}

func TestLayoutPinnedColumnPinsBottomRows(t *testing.T) {
	top := []string{"status-1", "status-2", "status-3", "status-4", "status-5"}
	bottom := []string{"[B] BAIL"}
	lines, scroll := layoutPinnedColumn(top, bottom, 20, 4)
	if scroll != 2 || len(lines) != 4 {
		t.Fatalf("layout = %#v scroll=%d", lines, scroll)
	}
	if lines[len(lines)-1] != "[B] BAIL" {
		t.Fatalf("action not pinned: %#v", lines)
	}
}

func TestBottomKeybarUsesEffectiveFrameOnTallTerminal(t *testing.T) {
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	g := &Game{content: c, width: 80, height: 60, snap: sim.Snapshot{State: *sim.New(c, 1, 1000)}}
	out := g.renderBottomKeybar("BELT CONTENT", "Q DOCK")
	lines := strings.Split(out, "\n")
	wantRows := g.bodyHeight() + keybarH
	if len(lines) != wantRows {
		t.Fatalf("tall terminal body has %d rows, want %d (frame body %d + keybar %d)", len(lines), wantRows, g.bodyHeight(), keybarH)
	}
}
