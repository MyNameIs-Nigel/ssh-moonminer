package tui

import (
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

	if b, ok := g.hits.At(1, panelBodyY(0)); !ok || b.ID != "world:0" {
		t.Fatalf("world:0 hitbox missing at (1,%d), got %#v", panelBodyY(0), b)
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
	st.Run = &sim.ActiveRun{
		Drill:  50,
		Pirate: 10,
		Yield:  100,
	}
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

	// Overdrive and bail buttons are the last two content lines before keybar.
	overdriveY := bodyLineY(8)
	bailY := bodyLineY(9)
	if b, ok := g.hits.At(1, overdriveY); !ok || b.ID != "btn:overdrive" {
		t.Fatalf("btn:overdrive hitbox missing at (1,%d), got %#v", overdriveY, b)
	}
	if b, ok := g.hits.At(1, bailY); !ok || b.ID != "btn:bail" {
		t.Fatalf("btn:bail hitbox missing at (1,%d), got %#v", bailY, b)
	}
}
