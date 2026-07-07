package tui

import (
	"strings"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/version"
)

func TestHelpOverlayShowsVersionAndScreen(t *testing.T) {
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	g := &Game{
		content: c,
		scr:     scrBelt,
		overlay: ovHelp,
		snap:    sim.Snapshot{State: *sim.New(c, 1, 1000)},
	}
	out := g.renderHelpOverlay()
	wantVer := "MOON MINER v" + version.Version + " (" + version.Channel + ")"
	if !strings.Contains(out, wantVer) {
		t.Fatalf("help overlay missing version %q:\n%s", wantVer, out)
	}
	if !strings.Contains(out, "ASTEROID BELT") {
		t.Fatalf("help overlay missing screen name, got:\n%s", out)
	}
	if !strings.Contains(out, "S scan") && !strings.Contains(out, "S scan selected") {
		// belt help mentions scan
		if !strings.Contains(strings.ToLower(out), "scan") {
			t.Fatalf("belt help should mention scan, got:\n%s", out)
		}
	}
}

func TestHelpScrollDoesNotChangeWorldSelection(t *testing.T) {
	g := &Game{
		scr:      scrChart,
		overlay:  ovHelp,
		worldSel: 2,
	}
	g.updateHelpOverlay("down")
	g.updateHelpOverlay("down")
	if g.worldSel != 2 {
		t.Fatalf("help scroll changed worldSel: got %d want 2", g.worldSel)
	}
	if g.helpScroll == 0 {
		t.Fatal("expected helpScroll to advance")
	}
}

func TestHelpScrollMax(t *testing.T) {
	g := &Game{scr: scrChart, overlay: ovHelp}
	max := g.helpScrollMax()
	g.helpScroll = max + 5
	_ = g.renderHelpOverlay()
	if g.helpScroll != max {
		t.Fatalf("helpScroll clamp: got %d want %d", g.helpScroll, max)
	}
}
