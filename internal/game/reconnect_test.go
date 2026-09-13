package game

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/store"
)

func testManager(t *testing.T) *Manager {
	t.Helper()
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "moonminer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewManager(st, c, logger, time.Hour, PolicyTakeover)
	t.Cleanup(func() {
		if err := m.Shutdown(context.Background()); err != nil {
			t.Errorf("cleanup shutdown: %v", err)
		}
	})
	return m
}

var testID = identity.SessionIdentity{Fingerprint: "SHA256:test", Slot: "default"}

func mustAttach(t *testing.T, m *Manager, now int64) AttachResult {
	t.Helper()
	res, err := m.Attach(context.Background(), testID, "ssh-ed25519 AAAA test", now, func(string) {})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if res.Created {
		res.Session.actor.do(func() { res.Session.actor.state.Seed = 42 })
	}
	return res
}

// scannableRock returns the nearest contact the starter skiff's scanner can
// actually reach. New saves get a random seed, so a given belt may hold nothing
// in range at all; when that happens it docks and departs again for a fresh
// roll rather than skipping the test.
func scannableRock(t *testing.T, m *Manager, s *Session, worldIdx int) sim.Asteroid {
	t.Helper()
	c := m.Content()
	for attempt := 0; attempt < 20; attempt++ {
		snap, err := s.SnapshotNow()
		if err != nil {
			t.Fatal(err)
		}
		st := snap.State
		best := -1
		for i := range st.Belt {
			if st.Belt[i].Scanned {
				return st.Belt[i]
			}
			if sim.IsOutOfRange(&st, c, &st.Belt[i]) {
				continue
			}
			if best < 0 || st.Belt[i].Distance < st.Belt[best].Distance {
				best = i
			}
		}
		if best >= 0 {
			return st.Belt[best]
		}
		// Nothing in range in this belt — re-roll it.
		if _, err := s.Dock(1000); err != nil {
			t.Fatalf("dock: %v", err)
		}
		if _, err := s.DevSetFuel(1000, 500); err != nil {
			t.Fatalf("set fuel: %v", err)
		}
		if _, err := s.Depart(1000, worldIdx); err != nil {
			t.Fatalf("depart: %v", err)
		}
	}
	t.Fatal("no belt roll produced a contact within scanner range")
	return sim.Asteroid{}
}

// waitScanned blocks until the actor's tick loop finishes an in-flight scan.
func waitScanned(t *testing.T, s *Session, asteroidID int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		snap, err := s.SnapshotNow()
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range snap.State.Belt {
			if a.ID == asteroidID && a.Scanned {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("scan of asteroid %d never completed", asteroidID)
}

// TestReattachRestoresBeltLocation is the end-to-end half of
// docs/framework/05: a pilot who departs and drops the connection comes back
// at the same belt, with the same rocks, and without the belt being re-rolled.
func TestReattachRestoresBeltLocation(t *testing.T) {
	m := testManager(t)

	first := mustAttach(t, m, 1000)
	if !first.Created {
		t.Fatal("first attach did not create a pilot")
	}
	if _, err := first.Session.Depart(1000, 1); err != nil {
		t.Fatalf("depart: %v", err)
	}
	before, err := first.Session.SnapshotNow()
	if err != nil {
		t.Fatal(err)
	}
	if before.State.IsDocked() {
		t.Fatal("pilot still reads as docked after departing")
	}
	first.Session.Detach()

	second := mustAttach(t, m, 2000)
	if second.Created {
		t.Fatal("reattach created a second pilot instead of loading the save")
	}
	after, err := second.Session.SnapshotNow()
	if err != nil {
		t.Fatal(err)
	}
	defer second.Session.Detach()

	if after.State.IsDocked() {
		t.Fatal("reattached pilot was silently docked — the belt-side state was lost")
	}
	if after.State.WorldIdx != before.State.WorldIdx || after.State.SystemID != before.State.SystemID {
		t.Fatalf("location drifted across reconnect: %d/%s -> %d/%s",
			before.State.WorldIdx, before.State.SystemID,
			after.State.WorldIdx, after.State.SystemID)
	}
	if after.State.BeltCount != before.State.BeltCount {
		t.Fatalf("belt was re-rolled across reconnect: BeltCount %d -> %d",
			before.State.BeltCount, after.State.BeltCount)
	}
	if len(after.State.Belt) != len(before.State.Belt) {
		t.Fatalf("belt size drifted: %d -> %d", len(before.State.Belt), len(after.State.Belt))
	}
	for i := range before.State.Belt {
		if after.State.Belt[i] != before.State.Belt[i] {
			t.Fatalf("rock %d drifted across reconnect: %+v -> %+v",
				i, before.State.Belt[i], after.State.Belt[i])
		}
	}

	// And the reconnected pilot has a legal move: dock, then the port opens.
	if _, err := second.Session.Depart(2000, 1); err != sim.ErrInBelt {
		t.Fatalf("Depart from the belt: got %v, want ErrInBelt", err)
	}
	if _, err := second.Session.Dock(2000); err != nil {
		t.Fatalf("dock: %v", err)
	}
	docked, err := second.Session.SnapshotNow()
	if err != nil {
		t.Fatal(err)
	}
	if !docked.State.IsDocked() {
		t.Fatal("Dock did not return the pilot to the chart")
	}
}

// TestDetachMidRunResolvesAndLeavesNotice pins both halves of the reconnect
// rule at once: the run does not survive (framework/03 — a disconnect is not
// a free pause) but the location does, and the pilot is told what happened.
func TestDetachMidRunResolvesAndLeavesNotice(t *testing.T) {
	m := testManager(t)

	first := mustAttach(t, m, 1000)
	if _, err := first.Session.Depart(1000, 1); err != nil {
		t.Fatalf("depart: %v", err)
	}
	snap, err := first.Session.SnapshotNow()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.State.Belt) == 0 {
		t.Fatal("departed into an empty belt")
	}
	// Lock refuses an unscanned rock, and the starter skiff's scanner caps
	// below the instant-scan grade, so this genuinely waits on the actor's
	// 4 Hz tick. The nearest contact is the cheapest one to wait for.
	target := scannableRock(t, m, first.Session, 1)
	if !target.Scanned {
		if _, err := first.Session.Scan(1000, target.ID); err != nil {
			t.Fatalf("scan: %v", err)
		}
		waitScanned(t, first.Session, target.ID)
	}
	if _, err := first.Session.Lock(1000, target.ID); err != nil {
		t.Fatalf("lock: %v", err)
	}
	mid, err := first.Session.SnapshotNow()
	if err != nil {
		t.Fatal(err)
	}
	if mid.State.Run == nil {
		t.Fatal("lock did not start a run")
	}
	worldBefore := mid.State.WorldIdx

	first.Session.Detach()

	second := mustAttach(t, m, 2000)
	defer second.Session.Detach()
	after, err := second.Session.SnapshotNow()
	if err != nil {
		t.Fatal(err)
	}

	if after.State.Run != nil {
		t.Fatal("a mining run survived the disconnect — a dropped link must not be a free pause")
	}
	if after.State.WorldIdx != worldBefore {
		t.Fatalf("pilot did not reattach at the belt they were working: %d -> %d",
			worldBefore, after.State.WorldIdx)
	}
	if len(after.State.RunLog) == 0 {
		t.Fatal("the autopilot resolution left no ship's-log entry")
	}
	if !after.State.RunLog[0].Disconnected {
		t.Fatal("the autopilot's ship's-log entry is indistinguishable from a run the pilot flew")
	}
	if after.State.DisconnectNotice == nil {
		t.Fatal("the reconnecting pilot is told nothing about what their autopilot did")
	}

	// Shown once: acknowledging it persists, so a further reconnect is quiet.
	if _, err := second.Session.AckDisconnectNotice(2000); err != nil {
		t.Fatalf("ack: %v", err)
	}
	acked, err := second.Session.SnapshotNow()
	if err != nil {
		t.Fatal(err)
	}
	if acked.State.DisconnectNotice != nil {
		t.Fatal("acknowledging the notice did not clear it")
	}
	if !acked.State.RunLog[0].Disconnected {
		t.Fatal("acknowledging the notice cleared the permanent ship's-log flag")
	}
}

// TestFrequentRequestsDoNotStarveTheTick is a regression test for a live bug
// found while writing the reconnect tests: the actor loop called
// mineTick.Reset on every iteration, and Reset restarts a ticker's period from
// zero. Any client sending requests faster than the 4 Hz tick therefore
// starved the tick completely — a player mashing mining pressure points froze
// their own drill, the pirate approach and the escape timer, and an in-flight
// scan never finished. The ticker is now only started/stopped on an edge.
func TestFrequentRequestsDoNotStarveTheTick(t *testing.T) {
	m := testManager(t)
	res := mustAttach(t, m, 1000)
	defer res.Session.Detach()

	if _, err := res.Session.Depart(1000, 1); err != nil {
		t.Fatalf("depart: %v", err)
	}
	target := scannableRock(t, m, res.Session, 1)
	if target.Scanned {
		t.Fatal("fixed scan fixture must start unscanned")
	}
	if _, err := res.Session.Scan(1000, target.ID); err != nil {
		t.Fatalf("scan: %v", err)
	}

	// Poll far faster than the tick interval. Before the fix this loop alone
	// was enough to keep the tick from ever firing.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		cur, err := res.Session.SnapshotNow()
		if err != nil {
			t.Fatal(err)
		}
		if cur.State.Scan == nil {
			found := false
			for _, rock := range cur.State.Belt {
				if rock.ID == target.ID && rock.Scanned {
					found = true
				}
			}
			if !found {
				t.Fatal("scan disappeared without revealing its target")
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("scan never completed while the session was polling faster than the tick interval")
}

// The takeover and shutdown paths are framework/03 policy rules, covered here
// because this package had no tests at all and the reconnect work depends on
// attach/detach being sound.
func TestTakeoverKicksTheOlderSession(t *testing.T) {
	m := testManager(t)

	first := mustAttach(t, m, 1000)
	second := mustAttach(t, m, 1001)
	defer second.Session.Detach()

	select {
	case reason := <-first.Session.Kicked():
		if reason == "" {
			t.Fatal("takeover delivered an empty kick reason")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("takeover did not kick the first session")
	}
}

func TestAttachRefusedDuringShutdown(t *testing.T) {
	m := testManager(t)
	sess := mustAttach(t, m, 1000)
	_ = sess

	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if _, err := m.Attach(context.Background(), testID, "ssh-ed25519 AAAA test", 2000, func(string) {}); err != ErrShuttingDown {
		t.Fatalf("attach during shutdown: got %v, want ErrShuttingDown", err)
	}
}
