package tui

import (
	"fmt"
	"strings"
	"testing"

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
	if len(lines) != g.height-chromeH {
		t.Fatalf("screen body has %d rows, want %d", len(lines), g.height-chromeH)
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
	g.View()

	leftW := g.contentWidth() / 2
	rightX := g.contentX() + leftW + 1

	if b, ok := g.hits.At(g.contentX()+1, panelBodyY(1)); !ok || b.ID != "world:0" {
		t.Fatalf("world:0 hitbox missing at (%d,%d), got %#v", g.contentX()+1, panelBodyY(1), b)
	}
	if b, ok := g.hits.At(rightX, panelBodyY(1)); !ok || b.ID != "svc:refuel" {
		t.Fatalf("svc:refuel hitbox missing at (%d,%d), got %#v", rightX, panelBodyY(1), b)
	}
	if b, ok := g.hits.At(rightX, panelBodyY(3)); !ok || b.ID != "svc:repair" {
		t.Fatalf("svc:repair hitbox missing at (%d,%d), got %#v", rightX, panelBodyY(3), b)
	}
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

	// BAIL sits below the 10-row framed asteroid field.
	bailY := bodyLineY(16)
	if b, ok := g.hits.At(1, bailY); !ok || b.ID != "btn:bail" {
		t.Fatalf("btn:bail hitbox missing at (1,%d), got %#v", bailY, b)
	}

	// The asteroid click target covers the field, but BAIL itself remains a
	// narrow control line rather than a full-width target.
	if b, ok := g.hits.At(60, bailY); ok && b.ID == "btn:bail" {
		t.Fatalf("btn:bail hitbox unexpectedly wide, found at (60,%d)", bailY)
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
	g.View()

	// A lit pressure point makes the framed asteroid itself the skill target.
	gaugeY := bodyLineY(6)
	if b, ok := g.hits.At(1, gaugeY); !ok || b.ID != "btn:skillcheck" {
		t.Fatalf("btn:skillcheck hitbox missing at (1,%d), got %#v", gaugeY, b)
	}
	bailY := bodyLineY(16)
	if b, ok := g.hits.At(1, bailY); !ok || b.ID != "btn:bail" {
		t.Fatalf("btn:bail hitbox missing at (1,%d), got %#v", bailY, b)
	}
}
