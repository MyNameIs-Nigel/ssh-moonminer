package game

import (
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestJumpCommitSurvivesDisconnectAndSnapshotsAreIsolated(t *testing.T) {
	m := testManager(t)
	res := mustAttach(t, m, 10)
	sess := res.Session
	sess.actor.do(func() { s := sess.actor.state; s.JumpClass = 0; s.Credits = 100000 })
	if _, err := sess.AcquireShip(11, "cicada"); err != nil {
		t.Fatal(err)
	}
	snap, err := sess.Jump(12, "eridani")
	if err != nil {
		t.Fatal(err)
	}
	fuel, hull := snap.State.Fuel, snap.State.Hull
	if snap.State.Ships["cicada"].SystemID != "sol" {
		t.Fatal("jump moved parked ship")
	}
	snap.State.Frontier["eridani"].RunsByDestination["eris_veil"] = 900
	sess.Detach()
	again := mustAttach(t, m, 13)
	reloaded, err := again.Session.SnapshotNow()
	if err != nil {
		t.Fatal(err)
	}
	s := reloaded.State
	if s.SystemID != "eridani" || sim.ActiveShip(&s).SystemID != "eridani" || s.JumpCount != 1 || s.Fuel != fuel || s.Hull != hull {
		t.Fatal("disconnect changed committed transit")
	}
	if s.Frontier["eridani"].RunsByDestination["eris_veil"] != 0 {
		t.Fatal("snapshot aliases actor frontier")
	}
}
