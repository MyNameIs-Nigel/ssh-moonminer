package tui

import (
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestIsSkillCheckKeyAcceptsPhysicalSpaceBar(t *testing.T) {
	for _, key := range []string{"e", "space", " "} {
		if !isSkillCheckKey(key) {
			t.Fatalf("%q should trigger a skill check", key)
		}
	}
	if isSkillCheckKey("enter") {
		t.Fatal("enter must not trigger a skill check")
	}
}

func TestWASDNavigationAliases(t *testing.T) {
	g := newResponsiveTestGame(t, 80, 24)
	g.worldSel = 1
	g.keyChart("w")
	if g.worldSel != 0 {
		t.Fatalf("W chart selection = %d, want 0", g.worldSel)
	}
	g.keyChart("s")
	if g.worldSel != 1 {
		t.Fatalf("S chart selection = %d, want 1", g.worldSel)
	}

	departResponsiveTestGame(t, g)
	g.rockSel = 1
	g.keyBelt("a")
	if g.rockSel != 0 {
		t.Fatalf("A belt selection = %d, want 0", g.rockSel)
	}
	g.keyBelt("d")
	if g.rockSel != 1 {
		t.Fatalf("D belt selection = %d, want 1", g.rockSel)
	}
}

func TestSummaryEAndEnterReturnToBelt(t *testing.T) {
	for _, key := range []string{"e", "enter"} {
		g := &Game{scr: scrSummary, lastOutcome: &sim.RunOutcome{Kind: sim.OutcomeBailed}}
		g.keySummary(key)
		if g.scr != scrBelt || g.lastOutcome != nil {
			t.Fatalf("%s did not return summary to belt", key)
		}
	}
}
