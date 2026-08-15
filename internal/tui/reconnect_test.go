package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
)

func reconnectTestGame(t *testing.T, mutate func(*sim.State)) *Game {
	t.Helper()
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	st := sim.New(c, 1, 1000)
	if mutate != nil {
		mutate(st)
	}
	return &Game{
		content:   c,
		width:     80,
		height:    24,
		scr:       initialScreen(st),
		snap:      sim.Snapshot{State: *st},
		id:        identity.SessionIdentity{Slot: "test"},
		hits:      hitbox.New(),
		tickCount: 1,
		now:       2000,
	}
}

// TestInitialScreenFollowsSavedLocation is the core of docs/framework/05: the
// opening screen is a function of the restored save, not a constant.
func TestInitialScreenFollowsSavedLocation(t *testing.T) {
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}

	docked := sim.New(c, 1, 1000)
	if got := initialScreen(docked); got != scrChart {
		t.Fatalf("docked pilot opened on screen %v, want scrChart", got)
	}

	inBelt := sim.New(c, 1, 1000)
	if err := sim.Depart(inBelt, c, 1); err != nil {
		t.Fatal(err)
	}
	if got := initialScreen(inBelt); got != scrBelt {
		t.Fatalf("belt-side pilot opened on screen %v, want scrBelt", got)
	}

	// An emptied belt is a normal reachable state — the pilot mined every
	// contact out. Routing them to the chart is the same softlock, so the
	// rule must not be qualified by len(Belt) > 0.
	emptied := sim.New(c, 1, 1000)
	if err := sim.Depart(emptied, c, 1); err != nil {
		t.Fatal(err)
	}
	emptied.Belt = nil
	if got := initialScreen(emptied); got != scrBelt {
		t.Fatalf("emptied-belt pilot opened on screen %v, want scrBelt", got)
	}
}

// TestChartWhileInBeltOffersReturn covers the belt-and-suspenders rule: if a
// pilot reaches the chart while still at a belt, Enter must take them back
// rather than departing (which the sim now refuses anyway).
func TestChartWhileInBeltOffersReturn(t *testing.T) {
	g := reconnectTestGame(t, func(st *sim.State) {
		st.WorldIdx = 1
		st.SystemID = "sol"
	})
	g.scr = scrChart
	g.rockSel = 4

	if cmds := g.keyChart("enter"); cmds != nil {
		t.Fatalf("RETURN TO BELT issued commands: %v", cmds)
	}
	if g.scr != scrBelt {
		t.Fatalf("Enter on the chart while in belt left screen %v, want scrBelt", g.scr)
	}
	if g.rockSel != 0 {
		t.Fatalf("rockSel not reset on return: %d", g.rockSel)
	}
}

// TestChartWhileInBeltRefusesPortServices pins that the port keys never reach
// the sim (there is no session in this fixture — a call would panic) and tell
// the pilot how to recover instead.
func TestChartWhileInBeltRefusesPortServices(t *testing.T) {
	for _, k := range []string{"f", "r", "c", "i", "s"} {
		g := reconnectTestGame(t, func(st *sim.State) {
			st.WorldIdx = 1
			st.SystemID = "sol"
		})
		g.scr = scrChart
		if cmds := g.keyChart(k); cmds != nil {
			t.Fatalf("key %q issued commands while in belt: %v", k, cmds)
		}
		if g.scr != scrChart {
			t.Fatalf("key %q changed screen to %v", k, g.scr)
		}
		if g.flash.text == "" {
			t.Fatalf("key %q gave the pilot no explanation", k)
		}
	}
}

// TestChartRendersServicesOfflineWhileInBelt checks the pilot can see why the
// port is refusing, and how to get out.
func TestChartRendersServicesOfflineWhileInBelt(t *testing.T) {
	g := reconnectTestGame(t, func(st *sim.State) {
		st.WorldIdx = 1
		st.SystemID = "sol"
	})
	g.scr = scrChart
	out := ansi.Strip(g.renderChart())
	if !strings.Contains(out, "RETURN TO BELT") {
		t.Fatalf("chart offers no way back to the belt:\n%s", out)
	}
	if !strings.Contains(out, "IN BELT") {
		t.Fatalf("chart does not say why services are refusing:\n%s", out)
	}

	dockedGame := reconnectTestGame(t, nil)
	dockedGame.scr = scrChart
	dockedOut := ansi.Strip(dockedGame.renderChart())
	if strings.Contains(dockedOut, "RETURN TO BELT") {
		t.Fatalf("docked chart wrongly offers RETURN TO BELT:\n%s", dockedOut)
	}
}

// TestSignalLostOverlayShowsTheAutopilotOutcome checks the reconnect notice
// renders from a persisted RunRecord — the Session that produced the original
// RunOutcome died with the connection.
func TestSignalLostOverlayShowsTheAutopilotOutcome(t *testing.T) {
	rec := sim.RunRecord{
		When: 1500, World: "VESTA", Asteroid: "AST-01",
		Outcome:             string(sim.OutcomeBailed),
		CargoValueRecovered: 240,
		HullDelta:           -12,
		FuelDelta:           -8,
		Disconnected:        true,
	}
	g := reconnectTestGame(t, func(st *sim.State) {
		st.WorldIdx = 1
		st.RunLog = []sim.RunRecord{rec}
		st.DisconnectNotice = &rec
	})
	g.overlay = ovSignalLost
	g.signalLost = &rec

	out := ansi.Strip(g.renderOverlay())
	for _, want := range []string{"SIGNAL LOST", "BAILED", "AST-01", "240"} {
		if !strings.Contains(out, want) {
			t.Fatalf("reconnect notice missing %q:\n%s", want, out)
		}
	}

	// It must composite over whatever screen the pilot was restored onto.
	base := g.renderChrome() + "\n" + g.renderScreen()
	if !strings.Contains(ansi.Strip(g.compositeView(base)), "SIGNAL LOST") {
		t.Fatal("reconnect notice is not visible over the restored screen")
	}
}

// TestSignalLostOverlayRendersNothingWithoutARecord guards the composite path
// against a nil record: renderOverlay returning "" makes compositeView fall
// through to the base screen rather than painting an empty box.
func TestSignalLostOverlayRendersNothingWithoutARecord(t *testing.T) {
	g := reconnectTestGame(t, nil)
	g.overlay = ovSignalLost
	if got := g.renderOverlay(); got != "" {
		t.Fatalf("nil reconnect record rendered %q", got)
	}
}
