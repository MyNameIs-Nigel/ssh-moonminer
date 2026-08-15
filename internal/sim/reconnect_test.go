package sim_test

import (
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

// TestDepartRequiresDocked covers docs/framework/05: a pilot already out at a
// belt must not be able to "depart" again. Before the guard this charged
// TravelFuel a second time and rolled a fresh belt over the one the save had
// preserved across the disconnect.
func TestDepartRequiresDocked(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	if s.IsDocked() {
		t.Fatal("Depart left the pilot docked")
	}

	fuelBefore := s.Fuel
	beltCountBefore := s.BeltCount
	beltBefore := append([]sim.Asteroid(nil), s.Belt...)
	worldBefore, systemBefore := s.WorldIdx, s.SystemID

	if err := sim.Depart(s, c, 1); err != sim.ErrInBelt {
		t.Fatalf("Depart while in belt: got %v, want ErrInBelt", err)
	}

	// A refused departure must mutate nothing at all.
	if s.Fuel != fuelBefore {
		t.Fatalf("refused Depart consumed fuel: %.2f -> %.2f", fuelBefore, s.Fuel)
	}
	if s.BeltCount != beltCountBefore {
		t.Fatalf("refused Depart advanced BeltCount: %d -> %d", beltCountBefore, s.BeltCount)
	}
	if s.WorldIdx != worldBefore || s.SystemID != systemBefore {
		t.Fatalf("refused Depart moved the ship: world %d/%s -> %d/%s",
			worldBefore, systemBefore, s.WorldIdx, s.SystemID)
	}
	if len(s.Belt) != len(beltBefore) {
		t.Fatalf("refused Depart regenerated the belt: %d rocks -> %d", len(beltBefore), len(s.Belt))
	}
	for i := range beltBefore {
		if s.Belt[i] != beltBefore[i] {
			t.Fatalf("refused Depart altered rock %d: %+v -> %+v", i, beltBefore[i], s.Belt[i])
		}
	}
}

// TestDepartAfterDockAllowed pins the intended route between belts: the only
// way from one belt to another is dock (where the rearm belongs), then depart.
func TestDepartAfterDockAllowed(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	sim.Dock(s, c)
	if !s.IsDocked() {
		t.Fatal("Dock did not return the pilot to the chart")
	}
	s.Fuel = 500
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatalf("Depart after Dock: %v", err)
	}
}

// TestDepartInBeltWithFullHoldIsRefusedNotCargoFull pins the specific
// unrecoverable case from docs/framework/05: with a full hold, ErrCargoFull
// used to be the error a stranded pilot saw, which reads as "sell your cargo"
// — advice they could not act on, because selling is docked-gated too. The
// honest error is ErrInBelt.
func TestDepartInBeltWithFullHoldIsRefusedNotCargoFull(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	s.CargoUnits = sim.CargoCapacityUnits(s, c)
	s.CargoValue = 400
	if sim.RemainingCargoCapacity(s, c) > 0 {
		t.Fatal("hold is not actually full")
	}
	if err := sim.Depart(s, c, 1); err != sim.ErrInBelt {
		t.Fatalf("full-hold Depart while in belt: got %v, want ErrInBelt", err)
	}
	// And the recovery route the fix guarantees: dock, then sell.
	sim.Dock(s, c)
	if _, err := sim.SellCargo(s, c, 2000); err != nil {
		t.Fatalf("SellCargo after docking: %v", err)
	}
}

// TestEmergencyResolveFlagsRunAndLeavesNotice covers the reconnect notice:
// a run the disconnect autopilot closed is marked in the ship's log and left
// as a one-shot notice for the next session, which acknowledges it once.
func TestEmergencyResolveFlagsRunAndLeavesNotice(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	s.Fuel = 500
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, s.Belt[0].ID, 1000); err != nil {
		t.Fatal(err)
	}
	// Mine a little so there is something to recover, then drop the link.
	sim.TickRun(s, c, 2, 1002)
	if s.Run == nil {
		t.Fatal("run ended before the simulated disconnect")
	}

	logBefore := len(s.RunLog)
	out := sim.EmergencyResolve(s, c, 2000)
	if s.Run != nil {
		t.Fatal("EmergencyResolve left the run active")
	}
	if len(s.RunLog) <= logBefore {
		t.Fatalf("EmergencyResolve appended no ship's-log entry (%d -> %d)", logBefore, len(s.RunLog))
	}
	if !s.RunLog[0].Disconnected {
		t.Fatal("newest ship's-log entry is not flagged Disconnected")
	}
	if out != nil && !out.Record.Disconnected {
		t.Fatal("returned outcome record is not flagged Disconnected")
	}
	if s.DisconnectNotice == nil {
		t.Fatal("no reconnect notice was left for the next session")
	}
	if s.DisconnectNotice.Outcome != s.RunLog[0].Outcome {
		t.Fatalf("notice outcome %q does not match the logged run %q",
			s.DisconnectNotice.Outcome, s.RunLog[0].Outcome)
	}

	// The notice survives a save round-trip — outliving the session that made
	// it is the whole point — and is cleared exactly once on acknowledgement.
	payload, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := sim.DecodeState(payload, c)
	if err != nil {
		t.Fatal(err)
	}
	if restored.DisconnectNotice == nil {
		t.Fatal("reconnect notice did not survive Encode/DecodeState")
	}
	if !restored.RunLog[0].Disconnected {
		t.Fatal("Disconnected flag did not survive Encode/DecodeState")
	}
	sim.AckDisconnectNotice(restored)
	if restored.DisconnectNotice != nil {
		t.Fatal("AckDisconnectNotice did not clear the notice")
	}
	if !restored.RunLog[0].Disconnected {
		t.Fatal("acknowledging the notice cleared the permanent ship's-log flag")
	}
}

// TestEmergencyResolveShipLostRespawnsDocked guards the one case where
// restoring the pilot's belt would be wrong: if the autopilot lost the ship
// there is no belt to go back to, so the respawn must read as docked and the
// TUI must route the next session to the chart.
func TestEmergencyResolveShipLostRespawnsDocked(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	s.Fuel = 500
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, s.Belt[0].ID, 1000); err != nil {
		t.Fatal(err)
	}
	if err := sim.BailOrDepart(s, c, 1000); err != nil {
		t.Fatal(err)
	}
	// The deterministic death recipe used elsewhere in this package.
	s.Run.UnderAttack = true
	s.Hull = 0

	out := sim.EmergencyResolve(s, c, 2000)
	if out == nil || out.Kind != sim.OutcomeShipLost {
		t.Fatalf("expected ship_lost, got %+v", out)
	}
	if !s.IsDocked() {
		t.Fatal("a lost ship must respawn docked — there is no belt to restore to")
	}
	if len(s.Belt) != 0 {
		t.Fatalf("respawn left %d rocks in the belt", len(s.Belt))
	}
	if s.DisconnectNotice == nil || s.DisconnectNotice.Outcome != string(sim.OutcomeShipLost) {
		t.Fatalf("the pilot is not told they lost the ship: %+v", s.DisconnectNotice)
	}
}

// TestEmergencyResolveWithNoRunIsInert guards the common case: most detaches
// happen with no run in flight and must leave no notice behind.
func TestEmergencyResolveWithNoRunIsInert(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	if out := sim.EmergencyResolve(s, c, 2000); out != nil {
		t.Fatalf("EmergencyResolve with no active run returned %+v", out)
	}
	if s.DisconnectNotice != nil {
		t.Fatal("EmergencyResolve with no active run left a notice")
	}
}

// TestDisconnectPreservesBeltLocation is the storage-level half of the
// reconnect fix: WorldIdx, SystemID and the belt itself must round-trip, so
// the pilot can be put back where they were.
func TestDisconnectPreservesBeltLocation(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 7, 1000)
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	payload, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := sim.DecodeState(payload, c)
	if err != nil {
		t.Fatal(err)
	}
	if restored.IsDocked() {
		t.Fatal("restored pilot reads as docked despite being at a belt")
	}
	if restored.WorldIdx != s.WorldIdx || restored.SystemID != s.SystemID {
		t.Fatalf("location drifted: %d/%s -> %d/%s",
			s.WorldIdx, s.SystemID, restored.WorldIdx, restored.SystemID)
	}
	if restored.BeltCount != s.BeltCount {
		t.Fatalf("BeltCount drifted: %d -> %d", s.BeltCount, restored.BeltCount)
	}
	if len(restored.Belt) != len(s.Belt) {
		t.Fatalf("belt size drifted: %d -> %d", len(s.Belt), len(restored.Belt))
	}
	for i := range s.Belt {
		if restored.Belt[i] != s.Belt[i] {
			t.Fatalf("rock %d drifted: %+v -> %+v", i, s.Belt[i], restored.Belt[i])
		}
	}
}
