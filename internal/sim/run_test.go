package sim_test

import (
	"math"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestBailOrDepartStartsEscapeNotImmediateOutcome(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	ast := s.Belt[0]
	s.Fuel = 500
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	if err := sim.BailOrDepart(s, c, 2000); err != nil {
		t.Fatalf("BailOrDepart: %v", err)
	}
	if s.Run == nil {
		t.Fatal("run cleared too early — bail should start an escape, not resolve immediately")
	}
	if s.Run.Phase != sim.PhaseEscaping {
		t.Fatalf("expected escaping phase, got %v", s.Run.Phase)
	}
	if s.Run.Intent != sim.OutcomeBailed {
		t.Fatalf("expected bailed intent before depletion, got %v", s.Run.Intent)
	}
}

func TestDepartAfterDepletion(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	_ = sim.Depart(s, c, 1)
	ast := s.Belt[0]
	ast.DrillSec = 1
	s.Belt[0] = ast
	s.Belt[0].Scanned = true
	s.Fuel = 500
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	// Mine well past depletion in small ticks so pirate distance also moves
	// deterministically but shouldn't have reached 0 yet in this seed/asteroid.
	dt := 0.25
	for i := 0; i < 40 && s.Run != nil && s.Run.Phase == sim.PhaseMining; i++ {
		sim.TickRun(s, c, dt, 1000+int64(i))
	}
	if s.Run == nil {
		t.Fatal("run resolved on its own — mining must never auto-resolve on depletion")
	}
	if s.Run.Phase != sim.PhaseMining {
		t.Skip("pirates arrived before depletion in this scenario; not exercising depart path")
	}
	if s.Run.MinedUnits < float64(ast.Volume) {
		t.Fatalf("expected asteroid depleted, mined %v of %v", s.Run.MinedUnits, ast.Volume)
	}
	if err := sim.BailOrDepart(s, c, 2000); err != nil {
		t.Fatal(err)
	}
	if s.Run.Intent != sim.OutcomeDeparted {
		t.Fatalf("expected departed intent after depletion, got %v", s.Run.Intent)
	}
}

func TestBailOrDepartNoOp(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)
	if err := sim.BailOrDepart(s, c, 0); err != nil {
		t.Fatalf("expected nil error on no-op, got %v", err)
	}
	if s.Run != nil {
		t.Fatal("expected no run to be created")
	}
}

func TestBailOrDepartInvalidOnceEscaping(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	_ = sim.Depart(s, c, 1)
	ast := s.Belt[0]
	s.Fuel = 500
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	if err := sim.BailOrDepart(s, c, 1000); err != nil {
		t.Fatal(err)
	}
	if err := sim.BailOrDepart(s, c, 1001); err != sim.ErrInvalidRunPhase {
		t.Fatalf("expected ErrInvalidRunPhase once escaping, got %v", err)
	}
}

func forcePirateArrival(s *sim.State) {
	s.Run.PirateDistance = 0
}

func TestPirateActionRolledDeterministically(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	_ = sim.Depart(s, c, 1)
	ast := s.Belt[0]
	s.Fuel = 500
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	forcePirateArrival(s)
	sim.TickRun(s, c, 0.25, 1000)
	if s.Run == nil {
		t.Fatal("run should still be active")
	}
	if s.Run.Phase != sim.PhaseTribute && s.Run.Phase != sim.PhaseEscaping {
		t.Fatalf("expected tribute or escaping phase once pirates arrive, got %v", s.Run.Phase)
	}
}

func TestAcceptTributeRemovesCargoAndAvoidsAttack(t *testing.T) {
	c := testContent(t)
	// Try a handful of seeds to find one that rolls a tribute demand.
	for seed := uint64(1); seed < 50; seed++ {
		s := sim.New(c, seed, 1000)
		_ = sim.Depart(s, c, 1)
		ast := s.Belt[0]
		s.Fuel = 500
		s.Belt[0].Scanned = true
		if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
			t.Fatal(err)
		}
		s.Run.MinedUnits = float64(ast.Volume) * 0.5
		s.Run.CargoValue = ast.Value / 2
		forcePirateArrival(s)
		sim.TickRun(s, c, 0.25, 1000)
		if s.Run == nil || s.Run.Phase != sim.PhaseTribute {
			continue
		}
		before := s.Run.CargoValue
		if err := sim.AcceptTribute(s, c, 2000); err != nil {
			t.Fatal(err)
		}
		if s.Run.CargoValue >= before {
			t.Fatalf("expected cargo value to drop after tribute, before=%d after=%d", before, s.Run.CargoValue)
		}
		if s.Run.UnderAttack {
			t.Fatal("accepting tribute should not trigger an attack")
		}
		if s.Run.Phase != sim.PhaseEscaping {
			t.Fatalf("expected escaping phase after tribute accepted, got %v", s.Run.Phase)
		}
		return
	}
	t.Skip("no seed in range rolled a tribute demand — tribute_chance may need review")
}

func TestRefuseTributeStartsAttack(t *testing.T) {
	c := testContent(t)
	for seed := uint64(1); seed < 50; seed++ {
		s := sim.New(c, seed, 1000)
		_ = sim.Depart(s, c, 1)
		ast := s.Belt[0]
		s.Fuel = 500
		s.Belt[0].Scanned = true
		if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
			t.Fatal(err)
		}
		forcePirateArrival(s)
		sim.TickRun(s, c, 0.25, 1000)
		if s.Run == nil || s.Run.Phase != sim.PhaseTribute {
			continue
		}
		if err := sim.RefuseTribute(s, c, 2000); err != nil {
			t.Fatal(err)
		}
		if !s.Run.UnderAttack {
			t.Fatal("expected UnderAttack after refusing tribute")
		}
		if s.Run.Phase != sim.PhaseEscaping {
			t.Fatalf("expected escaping phase after refusal, got %v", s.Run.Phase)
		}
		return
	}
	t.Skip("no seed in range rolled a tribute demand")
}

func TestShipLostResetsUpgradesKeepsCredits(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Credits = 5000
	s.Upgrades.Drill = 3
	s.Upgrades.Plating = 2
	_ = sim.Depart(s, c, 1)
	ast := s.Belt[0]
	s.Fuel = 500
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	if err := sim.BailOrDepart(s, c, 1000); err != nil {
		t.Fatal(err)
	}
	s.Run.UnderAttack = true
	s.Hull = 0
	out, ended := sim.TickRun(s, c, 0.25, 2000)
	if !ended || out == nil || out.Kind != sim.OutcomeShipLost {
		t.Fatalf("expected ship_lost, got %+v ended=%v", out, ended)
	}
	if s.Run != nil {
		t.Fatal("run should be cleared")
	}
	if s.Upgrades != (sim.Upgrades{}) {
		t.Fatalf("expected upgrades reset on ship loss, got %+v", s.Upgrades)
	}
	if s.Hull != c.Pilot.StartHull {
		t.Fatalf("expected hull respawn to %d, got %d", c.Pilot.StartHull, s.Hull)
	}
	if s.Fuel != float64(c.Pilot.StartFuel) {
		t.Fatalf("expected fuel respawn to %d, got %v", c.Pilot.StartFuel, s.Fuel)
	}
	if s.Credits != 5000 {
		t.Fatalf("banked credits must survive ship loss, got %d", s.Credits)
	}
	if !s.IsDocked() {
		t.Fatal("expected respawn to dock the pilot")
	}
	if s.Stats.ShipsLost != 1 {
		t.Fatalf("expected ship-loss stats incremented, got %+v", s.Stats)
	}
}

func TestFullCargoEscapesSlowerThanEmptyCargo(t *testing.T) {
	c := testContent(t)

	escapeReqFor := func(minedFrac float64) float64 {
		s := sim.New(c, 7, 1000)
		_ = sim.Depart(s, c, 1)
		ast := s.Belt[0]
		s.Fuel = 500
		s.Belt[0].Scanned = true
		if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
			t.Fatal(err)
		}
		s.Run.MinedUnits = float64(ast.Volume) * minedFrac
		s.Run.CargoValue = int(float64(ast.Value) * minedFrac)
		if err := sim.BailOrDepart(s, c, 2000); err != nil {
			t.Fatal(err)
		}
		return s.Run.EscapeSecondsRequired
	}

	empty := escapeReqFor(0.0)
	full := escapeReqFor(1.0)
	if full <= empty {
		t.Fatalf("full cargo should take materially longer to escape: empty=%v full=%v", empty, full)
	}
}

func TestNumericFloorsAtDtEdgeCases(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	_ = sim.Depart(s, c, 1)
	ast := s.Belt[0]
	s.Fuel = 500
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}

	// dt = 0 should not panic or produce NaN.
	sim.TickRun(s, c, 0, 1000)
	if s.Run == nil {
		t.Fatal("run should survive a zero-length tick")
	}
	if math.IsNaN(s.Run.PirateDistance) || math.IsNaN(s.Fuel) {
		t.Fatal("dt=0 produced NaN")
	}

	// Huge dt should clamp rather than go negative or NaN.
	sim.TickRun(s, c, 1e9, 2000)
	if s.Fuel < 0 || math.IsNaN(s.Fuel) {
		t.Fatalf("huge dt produced invalid fuel: %v", s.Fuel)
	}
	if s.Hull < 0 {
		t.Fatalf("huge dt produced negative hull: %d", s.Hull)
	}
}

func TestLowHullIncreasesEventChance(t *testing.T) {
	c := testContent(t)

	countEvents := func(hull int, seeds int) int {
		total := 0
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			s := sim.New(c, seed, 1000)
			_ = sim.Depart(s, c, 1)
			ast := s.Belt[0]
			ast.Risk = 8 // keep pirates from arriving mid-loop for this sample
			s.Belt[0] = ast
			s.Belt[0].Scanned = true
			s.Fuel = 500
			s.Hull = hull
			if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
				t.Fatal(err)
			}
			seenEvent := false
			for i := 0; i < 6 && s.Run != nil && s.Run.Phase == sim.PhaseMining; i++ {
				sim.TickRun(s, c, 1.0, 1000+int64(i))
				if s.Run != nil && len(s.Run.EventLog) > 0 {
					seenEvent = true
					break
				}
			}
			if seenEvent {
				total++
			}
		}
		return total
	}

	lowHullCount := countEvents(15, 150)
	highHullCount := countEvents(100, 150)
	if lowHullCount <= highHullCount {
		t.Fatalf("expected low-hull event rate to exceed high-hull rate: low=%d high=%d", lowHullCount, highHullCount)
	}
}
