package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mynameis-nigel/ssh-moonminer/internal/game"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestDelayedSnapshotCannotUndoDock(t *testing.T) {
	g := newResponsiveTestGame(t, 80, 24)
	g.sess = &game.Session{}
	old := g.snap
	old.State = *g.snap.State.Clone()
	old.State.WorldIdx = 1
	old.State.Run = &sim.ActiveRun{}
	old.Revision = 1
	g.snap.Revision = 2
	g.Update(snapMsg(old))
	if !g.snap.State.IsDocked() || g.scr != scrChart {
		t.Fatal("delayed mining snapshot undid dock")
	}
}

func TestQuitWorksBelowMinimumViewport(t *testing.T) {
	g := newResponsiveTestGame(t, 40, 10)
	_, cmd := g.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+C ignored on resize screen")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("Ctrl+C did not quit")
	}
}
