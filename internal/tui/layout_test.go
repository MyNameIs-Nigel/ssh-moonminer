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

	leftW := g.width/2 - 1
	rightX := leftW + 1

	if b, ok := g.hits.At(1, panelBodyY(1)); !ok || b.ID != "world:0" {
		t.Fatalf("world:0 hitbox missing at (1,%d), got %#v", panelBodyY(1), b)
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

	// BAIL is the last content line before the keybar during mining
	// (no skill check active in this snapshot).
	bailY := bodyLineY(8)
	if b, ok := g.hits.At(1, bailY); !ok || b.ID != "btn:bail" {
		t.Fatalf("btn:bail hitbox missing at (1,%d), got %#v", bailY, b)
	}

	// Radar-widget columns (right of the stat bars) must not steal any of
	// the left column's hitboxes — re-check at a column comfortably inside
	// the radar box and confirm it isn't btn:bail.
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

	// Skill-check gauge sits where the RADAR row used to be; BAIL moves
	// down two more lines (gauge line + its trailing blank) beneath it.
	gaugeY := bodyLineY(8)
	if b, ok := g.hits.At(1, gaugeY); !ok || b.ID != "btn:skillcheck" {
		t.Fatalf("btn:skillcheck hitbox missing at (1,%d), got %#v", gaugeY, b)
	}
	bailY := bodyLineY(10)
	if b, ok := g.hits.At(1, bailY); !ok || b.ID != "btn:bail" {
		t.Fatalf("btn:bail hitbox missing at (1,%d), got %#v", bailY, b)
	}
}
