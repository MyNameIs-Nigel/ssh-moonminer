package game

import (
	"context"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestTakenOverSessionCannotAccessSave(t *testing.T) {
	m := testManager(t)
	first := mustAttach(t, m, 1000).Session
	second := mustAttach(t, m, 2000).Session
	defer second.Detach()
	if _, err := first.DevSetFuel(2000, 0); err != ErrSessionClosed {
		t.Fatalf("stale intent: %v", err)
	}
	if _, err := first.SnapshotNow(); err != ErrSessionClosed {
		t.Fatalf("stale read: %v", err)
	}
	first.Detach()
	snap, err := second.SnapshotNow()
	if err != nil {
		t.Fatal(err)
	}
	if snap.State.Fuel != float64(m.content.Pilot.StartFuel) {
		t.Fatal("stale session changed fuel")
	}
	select {
	case _, ok := <-first.Snapshots():
		if ok {
			t.Fatal("unexpected snapshot")
		}
	default:
		t.Fatal("stale snapshot listener left blocked")
	}
}

func TestIdleAttachLastSeenPersists(t *testing.T) {
	m := testManager(t)
	mustAttach(t, m, 1000).Session.Detach()
	mustAttach(t, m, 2000).Session.Detach()
	rows, err := m.store.AllSaves(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("save count %d", len(rows))
	}
	s, err := sim.DecodeState(rows[0].State, m.content)
	if err != nil {
		t.Fatal(err)
	}
	if s.Stats.LastSeen != 2000 {
		t.Fatalf("LastSeen = %d, want 2000", s.Stats.LastSeen)
	}
}

func TestRefusePolicyPreservesOriginalSession(t *testing.T) {
	m := testManager(t)
	m.policy = PolicyRefuse
	first := mustAttach(t, m, 1000).Session
	defer first.Detach()
	if _, err := m.Attach(context.Background(), testID, "key", 2000, nil); err != ErrSaveBusy {
		t.Fatalf("got %v", err)
	}
	if _, err := first.SnapshotNow(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-first.Kicked():
		t.Fatal("refused attach kicked owner")
	default:
	}
}

func TestShutdownReportsFailedFlush(t *testing.T) {
	m := testManager(t)
	s := mustAttach(t, m, 1000).Session
	if _, err := s.DevSetFuel(1000, 1); err != nil {
		t.Fatal(err)
	}
	if err := m.store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := m.Shutdown(context.Background()); err == nil {
		t.Fatal("shutdown reported success after save failed")
	}
}

func TestSnapshotsHaveIncreasingRevisions(t *testing.T) {
	m := testManager(t)
	s := mustAttach(t, m, 1000).Session
	defer s.Detach()
	first, err := s.SnapshotNow()
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.DevSetFuel(1000, 1)
	if err != nil {
		t.Fatal(err)
	}
	third, err := s.SnapshotNow()
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision >= second.Revision || second.Revision >= third.Revision {
		t.Fatalf("revisions %d, %d, %d", first.Revision, second.Revision, third.Revision)
	}
}

func TestFailedDisconnectKeepsDirtyActorForRetry(t *testing.T) {
	m := testManager(t)
	s := mustAttach(t, m, 1000).Session
	if err := m.store.Close(); err != nil {
		t.Fatal(err)
	}
	s.Detach()
	m.mu.Lock()
	retained := m.actors[s.actor.key] == s.actor
	m.mu.Unlock()
	if !retained {
		t.Fatal("disconnect discarded unsaved actor")
	}
	// Drain the deliberately failed actor before testManager's cleanup.
	if err := m.Shutdown(context.Background()); err == nil {
		t.Fatal("expected closed-store flush failure")
	}
}
