package tui

import (
	"strings"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func shieldStatusGame(t *testing.T) (*Game, *sim.State) {
	t.Helper()
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	st.Credits = 100000
	if err := sim.InstallSlotDevice(st, c, "skiff", sim.SlotUtility, 0, sim.ItemShield, 0); err != nil {
		t.Fatal(err)
	}
	return newShipyardGame(t, st), st
}

func TestShieldStatusAppearsInGlobalHUDButNotPortServices(t *testing.T) {
	g, _ := shieldStatusGame(t)
	g.scr = scrChart

	if hud := g.renderChrome(); !strings.Contains(hud, "SHD 30/30") {
		t.Fatalf("global HUD should show shield charge, got:\n%s", hud)
	}
	if dock := g.renderChart(); strings.Contains(dock, "SHIELD") || strings.Contains(dock, "CHARGED") {
		t.Fatalf("port services should omit redundant shield status, got:\n%s", dock)
	}
}

func TestBeltShieldStatusShowsRechargeProgress(t *testing.T) {
	g, st := shieldStatusGame(t)
	if err := sim.Depart(st, g.content, 1); err != nil {
		t.Fatal(err)
	}
	st.Ships[st.ActiveShipID].ShieldHP = 10
	g.snap.State = *st
	g.scr = scrBelt

	out := g.renderBelt()
	if !strings.Contains(out, "SHIELD") || !strings.Contains(out, "RECHARGING") {
		t.Fatalf("belt should show shield recovery status, got:\n%s", out)
	}
}
