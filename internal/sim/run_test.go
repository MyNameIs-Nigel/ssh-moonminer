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
	ast.Volume = 200 // smaller than the starter hold: exercise true rock depletion
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
	// Only actual rock exhaustion enables DEPART. A full hold pauses the drill
	// but preserves a non-zero resource meter and requires BAIL.
	if !sim.RunDepleted(s, c, &ast, s.Run) {
		t.Fatalf("expected asteroid depletion, mined %v of volume %v cargo cap %v",
			s.Run.MinedUnits, ast.Volume, sim.CargoCapacityUnits(s, c))
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

func TestEMPLauncherDelaysPiratesAndSpendsHighestGradeFirst(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	_ = sim.Depart(s, c, 1)
	ast := s.Belt[0]
	s.Fuel = 500
	s.Belt[0].Scanned = true
	// Direct setup intentionally bypasses the starter ship's power budget:
	// this test is about selection and consumption of two fitted launchers.
	s.Ships["skiff"].Utility[0] = &sim.SlotDevice{ItemID: sim.ItemEMPLauncher, Grade: 0, EMPArmed: true}
	s.Ships["skiff"].Utility[1] = &sim.SlotDevice{ItemID: sim.ItemEMPLauncher, Grade: sim.MaxGrade, EMPArmed: true}
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	forcePirateArrival(s)
	if _, ended := sim.TickRun(s, c, 0, 1000); ended {
		t.Fatal("EMP deployment must keep the run mining")
	}
	if !s.Run.EMPActive || s.Run.EMPRemaining != 10 || s.Run.Phase != sim.PhaseMining {
		t.Fatalf("expected S-grade EMP delay while mining, got %+v", s.Run)
	}
	if s.Ships["skiff"].Utility[1].EMPArmed || !s.Ships["skiff"].Utility[0].EMPArmed {
		t.Fatalf("expected the highest grade launcher, and only it, to be spent: %+v", s.Ships["skiff"].Utility)
	}
	minedBefore := s.Run.MinedUnits
	sim.TickRun(s, c, 1, 1001)
	if s.Run.Phase != sim.PhaseMining || s.Run.MinedUnits <= minedBefore {
		t.Fatalf("mining should continue during EMP delay: %+v", s.Run)
	}

	// Once its timer expires, the same asteroid cannot trigger the lower-grade
	// launcher. Make the pirate action deterministic so the resulting escape
	// is unambiguous.
	c.Worlds[s.WorldIdx].PiratesAlwaysAttack = true
	sim.TickRun(s, c, 9, 1010)
	if s.Run.Phase != sim.PhaseCombat || !s.Run.EMPDeployed {
		t.Fatalf("expected pirates to act after the delay, got %+v", s.Run)
	}
	if s.Run.Combat == nil || !s.Run.Combat.EscapeStarted {
		t.Fatal("expected the unarmed skiff's escape burn to already be running once pirates attack")
	}
	if !s.Ships["skiff"].Utility[0].EMPArmed {
		t.Fatal("a second EMP launcher must remain armed during the same asteroid run")
	}
}

func TestDockRearamsSpentEMPLaunchers(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Ships["skiff"].Utility[0] = &sim.SlotDevice{ItemID: sim.ItemEMPLauncher, Grade: 0}
	s.Ships["skiff"].Utility[1] = &sim.SlotDevice{ItemID: sim.ItemEMPLauncher, Grade: sim.MaxGrade}
	_ = sim.Depart(s, c, 1)
	sim.Dock(s, c)
	for _, d := range s.Ships["skiff"].Utility {
		if !d.EMPArmed {
			t.Fatalf("dock must rearm every installed EMP launcher: %+v", d)
		}
	}
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
	if s.Run.Phase != sim.PhaseTribute && s.Run.Phase != sim.PhaseCombat {
		t.Fatalf("expected tribute or combat phase once pirates arrive, got %v", s.Run.Phase)
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
		if s.Run.Phase != sim.PhaseCombat {
			t.Fatalf("expected combat phase after refusal, got %v", s.Run.Phase)
		}
		if s.Run.Combat == nil || !s.Run.Combat.EscapeStarted {
			t.Fatal("expected the escape burn to already be running after refusing tribute")
		}
		return
	}
	t.Skip("no seed in range rolled a tribute demand")
}

func TestShipLostResetsHangarEntryKeepsCredits(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Ships["skiff"].Grades.Thrusters = 3 // direct mutation — must not cost credits
	s.Credits = 5000
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
	if !sim.OwnsShip(s, "skiff") {
		t.Fatal("expected a fresh starter ship to be granted on loss of the only ship")
	}
	if sim.TrackGrade(s, "skiff", sim.TrackThrusters) != c.ShipByID("skiff").ThrustersStart {
		t.Fatalf("expected the respawned skiff's grades reset to stock, got thrusters=%d", sim.TrackGrade(s, "skiff", sim.TrackThrusters))
	}
	if s.Hull != sim.MaxHull(s, c) {
		t.Fatalf("expected hull respawn to max (%d), got %d", sim.MaxHull(s, c), s.Hull)
	}
	if s.Fuel != sim.TankSize(s, c) {
		t.Fatalf("expected fuel respawn to a full tank (%v), got %v", sim.TankSize(s, c), s.Fuel)
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

func TestShipLostFallsBackToBestRemainingOwnedShip(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Credits = 100000
	if err := sim.AcquireShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	if err := sim.SwitchActiveShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	if s.ActiveShipID != "cicada" {
		t.Fatalf("expected cicada active, got %s", s.ActiveShipID)
	}
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
	if _, ended := sim.TickRun(s, c, 0.25, 2000); !ended {
		t.Fatal("expected the run to resolve")
	}
	if sim.OwnsShip(s, "cicada") {
		t.Fatal("the destroyed cicada should be removed from the hangar")
	}
	if s.ActiveShipID != "skiff" {
		t.Fatalf("expected fallback to the still-owned skiff, got %s", s.ActiveShipID)
	}
	if !sim.IsBuyback(s, "cicada") {
		t.Fatal("expected cicada to now price as a buyback")
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

// TestAttackDamagePerSecondUnmitigatedByTurret confirms the combat rework:
// the Defense/Autocannon Turret no longer passively mitigates incoming
// pirate damage (that job now belongs entirely to the Shield) — it became
// an always-firing weapon covered by combat_test.go instead.
func TestAttackDamagePerSecondUnmitigatedByTurret(t *testing.T) {
	c := testContent(t)
	for _, grade := range []int{0, 1, 2, 3, 4} {
		s := sim.New(c, 1, 1000)
		s.Credits = 100000
		if err := sim.AcquireShip(s, c, "warden"); err != nil {
			t.Fatal(err)
		}
		if err := sim.InstallSlotDevice(s, c, "warden", sim.SlotWeapon, 0, sim.ItemTurret, grade); err != nil {
			t.Fatal(err)
		}
		if err := sim.SwitchActiveShip(s, c, "warden"); err != nil {
			t.Fatal(err)
		}
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
		startHull := s.Hull
		for i := 0; i < 4; i++ {
			sim.TickRun(s, c, 0.25, 1000+int64(i))
		}
		lost := startHull - s.Hull

		want := int(c.Mining.AttackHullDamagePerSecond)
		if lost != want {
			t.Fatalf("turret grade=%d: expected full %d hull lost over 1s at %.1f dmg/s (no mitigation), got %d",
				grade, want, c.Mining.AttackHullDamagePerSecond, lost)
		}
	}
}

func TestFuelOutEscapePenaltyCaps(t *testing.T) {
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
	s.Fuel = 0

	// Check the cap invariant against the run's *current* base each tick,
	// since a cargo_shift event can legitimately grow BaseEscapeSecondsRequired
	// mid-escape — the cap must still hold relative to that updated base.
	resolved := false
	for i := 0; i < 400; i++ {
		sim.TickRun(s, c, 0.25, 1000+int64(i))
		if s.Run == nil {
			resolved = true
			break
		}
		want := s.Run.BaseEscapeSecondsRequired + c.Mining.FuelOutEscapePenaltyCapSeconds
		if s.Run.EscapeSecondsRequired > want+0.01 {
			t.Fatalf("escape requirement exceeded the fuel-out cap: got %v want <= %v", s.Run.EscapeSecondsRequired, want)
		}
	}
	if !resolved {
		t.Fatal("escape never resolved over 100 simulated seconds — fuel-out penalty may not be capped")
	}
}

func TestAttemptSkillCheckHitGrantsBonusAndConsumesCheck(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	_ = sim.Depart(s, c, 1)
	ast := s.Belt[0]
	s.Fuel = 500
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}

	// Any press while a check is active is a hit — there's no zone/timing to
	// miss within the window, only a deadline to beat.
	s.Run.SkillCheck = &sim.SkillCheck{Window: 3.0}
	before := s.Run.MinedUnits
	if err := sim.AttemptSkillCheck(s, c, 2000); err != nil {
		t.Fatal(err)
	}
	if s.Run.SkillCheck != nil {
		t.Fatal("expected the check to be consumed after an attempt")
	}
	if s.Run.MinedUnits <= before {
		t.Fatalf("a press on an active check should grant a mining bonus: before=%v after=%v", before, s.Run.MinedUnits)
	}
	if s.Run.NextSkillCheckIn <= 0 {
		t.Fatal("expected the next-check countdown to be re-rolled after an attempt")
	}
}

func TestAttemptSkillCheckNoActiveCheckIsNoop(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	_ = sim.Depart(s, c, 1)
	ast := s.Belt[0]
	s.Fuel = 500
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	s.Run.SkillCheck = nil
	before := s.Run.MinedUnits
	if err := sim.AttemptSkillCheck(s, c, 2000); err != nil {
		t.Fatal(err)
	}
	if s.Run.MinedUnits != before {
		t.Fatal("pressing with no active check should have no effect")
	}
}

func TestSkillCheckExpiryHasNoEffectOnShip(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	_ = sim.Depart(s, c, 1)
	ast := s.Belt[0]
	s.Fuel = 500
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	s.Run.SkillCheck = &sim.SkillCheck{Window: 3.0}
	startHull := s.Hull

	// Let the countdown run out without ever pressing the hotkey.
	if _, ended := sim.TickRun(s, c, 3.1, 2000); ended {
		t.Fatal("a missed skill check must not end the run")
	}
	if s.Run.SkillCheck != nil {
		t.Fatal("expected the check to auto-expire once Elapsed reaches Window")
	}
	if s.Hull != startHull {
		t.Fatalf("a missed skill check must not damage the ship: hull %d -> %d", startHull, s.Hull)
	}
	if s.Run.NextSkillCheckIn <= 0 {
		t.Fatal("expected the next-check countdown to be re-rolled after expiry")
	}
}

func TestPirateETANarrowsWithHighScannerGrade(t *testing.T) {
	c := testContent(t)

	widthFor := func(grade int) float64 {
		s := sim.New(c, 1, 1000)
		s.Credits = 200000
		if err := sim.AcquireShip(s, c, "cicada"); err != nil {
			t.Fatal(err)
		}
		for sim.TrackGrade(s, "cicada", sim.TrackScanner) < grade {
			if err := sim.BuyShipTrack(s, c, "cicada", sim.TrackScanner); err != nil {
				t.Fatal(err)
			}
		}
		if err := sim.SwitchActiveShip(s, c, "cicada"); err != nil {
			t.Fatal(err)
		}
		_ = sim.Depart(s, c, 1)
		ast := s.Belt[0]
		s.Fuel = 500
		s.Belt[0].Scanned = true
		if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
			t.Fatal(err)
		}
		return s.Run.PirateETAMax - s.Run.PirateETAMin
	}

	base := widthFor(3)     // B grade — no ETA bonus yet (only A/S narrow further)
	upgraded := widthFor(5) // S grade — max ETA bonus
	if upgraded >= base {
		t.Fatalf("expected high Scanner grades to narrow the pirate ETA range: base=%v upgraded=%v", base, upgraded)
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
	if highHullCount != 0 {
		t.Fatalf("expected zero events at/above the hull gate, got %d", highHullCount)
	}
}

// TestCargoCapStopsMiningAndLeavesTruthfulRemnant covers the full-hold edge
// case: the drill pauses with real ore remaining, the player must BAIL (not
// DEPART), and the asteroid keeps exactly that remnant.
func TestCargoCapDepletionStopsMiningAndLeavesRemnant(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	_ = sim.Depart(s, c, 1)
	ast := s.Belt[0]
	ast.Volume = 4600 // force it well above the starter skiff's cargo cap
	s.Belt[0] = ast
	s.Belt[0].Scanned = true
	s.Fuel = 500
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	// This test exercises cargo behavior only; a pirate interruption would
	// make the timing unrelated to whether the hold is respected.
	s.Run.PirateDistance = 1e9
	cargoCap := sim.CargoCapacityUnits(s, c)
	if cargoCap >= float64(ast.Volume) {
		t.Fatalf("test setup invalid: cargo cap %v must be below asteroid volume %v", cargoCap, ast.Volume)
	}

	dt := 0.25
	for i := 0; i < 400 && s.Run != nil && !sim.RunCargoFull(s, c, s.Run); i++ {
		sim.TickRun(s, c, dt, 1000+int64(i))
	}
	if s.Run == nil || s.Run.Phase != sim.PhaseMining {
		t.Fatal("run ended before the hold filled")
	}
	if !sim.RunCargoFull(s, c, s.Run) || sim.RunDepleted(s, c, &ast, s.Run) {
		t.Fatalf("expected a full hold with ore remaining, mined %v of cap %v", s.Run.MinedUnits, cargoCap)
	}
	minedBeforeResolve := s.Run.MinedUnits
	fuelBeforeStop := s.Fuel
	if _, ended := sim.TickRun(s, c, 10, 1999); ended || s.Run == nil || s.Run.Phase != sim.PhaseMining {
		t.Fatal("a full hold should stop mining, not end the run")
	}
	if s.Run.MinedUnits != minedBeforeResolve || s.Fuel != fuelBeforeStop {
		t.Fatalf("full hold continued mining: units %.2f -> %.2f, fuel %.2f -> %.2f", minedBeforeResolve, s.Run.MinedUnits, fuelBeforeStop, s.Fuel)
	}
	if err := sim.BailOrDepart(s, c, 2000); err != nil {
		t.Fatal(err)
	}
	if s.Run.Intent != sim.OutcomeBailed {
		t.Fatalf("expected bailed intent once the hold is full, got %v", s.Run.Intent)
	}
	s.Run.BaseEscapeSecondsRequired = 0
	s.Run.EscapeSecondsRequired = 0
	out, ended := sim.TickRun(s, c, 0.1, 3000)
	if !ended || out == nil {
		t.Fatal("expected the escape to resolve")
	}

	remaining, _ := sim.FindAsteroid(s, ast.ID)
	if remaining == nil {
		t.Fatalf("cargo-cap-triggered bail destroyed the whole asteroid — %v units were never mined and should have survived as a remnant", float64(ast.Volume)-minedBeforeResolve)
	}
	if remaining.Volume >= ast.Volume {
		t.Fatalf("expected the remnant to shrink below the original volume, got %d (was %d)", remaining.Volume, ast.Volume)
	}
}

// TestSwitchActiveShipPreservesJammerCharges ensures a hangar switch is not a
// hidden dock service; only Dock may rearm the active ship's countermeasures.
func TestSwitchActiveShipPreservesJammerCharges(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Credits = 100000
	if err := sim.AcquireShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	if err := sim.InstallSlotDevice(s, c, "cicada", sim.SlotInternal, 0, sim.ItemJammer, 4); err != nil {
		t.Fatal(err)
	}
	if err := sim.SwitchActiveShip(s, c, "skiff"); err != nil {
		t.Fatal(err)
	}
	// Drain cicada's jammer charges to 0 while it's parked, simulating a
	// prior trip that used them all up without redocking on cicada.
	s.Ships["cicada"].JammerCharges = 0

	if err := sim.SwitchActiveShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	if got := s.Ships["cicada"].JammerCharges; got != 0 {
		t.Fatalf("switching should preserve depleted jammer charges, got %d", got)
	}
	sim.Dock(s, c)
	if got := s.Ships["cicada"].JammerCharges; got == 0 {
		t.Fatal("docking should rearm the active jammer")
	}
}

// TestJammerGradeCPlusGrantsExtraCharge locks in gameplay/05's documented
// "grade C+ grants 1 + floor(g/3) uses" — grade C (2) must already show the
// bonus, not just grade B (3).
func TestJammerGradeCPlusGrantsExtraCharge(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Credits = 100000
	if err := sim.AcquireShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	if err := sim.InstallSlotDevice(s, c, "cicada", sim.SlotInternal, 0, sim.ItemJammer, 2); err != nil {
		t.Fatal(err)
	}
	sim.RearmJammer(s, c)
	if got := s.Ships["cicada"].JammerCharges; got < 2 {
		t.Fatalf("expected grade C (2) to already grant a second charge, got %d", got)
	}
}

func TestJammerChargeStartsTimedSuppressionAtLock(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotInternal, 0, sim.ItemJammer, 0); err != nil {
		t.Fatal(err)
	}
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	ast := s.Belt[0]
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	if got, want := s.Run.JammerRemaining, sim.JammerDurationSeconds(c, 0); got != want {
		t.Fatalf("jammer countdown = %.1f, want %.1f", got, want)
	}
	if got := s.Ships["skiff"].JammerCharges; got != 0 {
		t.Fatalf("jammer charges after lock = %d, want 0", got)
	}
}

func TestPressurePointsAreDeterministicAndResolveToHitOrMiss(t *testing.T) {
	c := testContent(t)
	c.Mining.PressurePointsMin = 3
	c.Mining.PressurePointsMax = 3
	s := sim.New(c, 99, 1000)
	_ = sim.Depart(s, c, 1)
	ast := s.Belt[0]
	ast.Scanned = true
	s.Belt[0] = ast
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	if len(s.Run.PressurePoints) != 3 {
		t.Fatalf("pressure-point count = %d, want 3", len(s.Run.PressurePoints))
	}
	seen := map[int]bool{}
	for _, point := range s.Run.PressurePoints {
		if seen[point.Position] {
			t.Fatalf("duplicate pressure-point position: %+v", s.Run.PressurePoints)
		}
		seen[point.Position] = true
	}

	s.Run.NextSkillCheckIn = 0
	s.Run.PirateDistance = 1e9
	sim.TickRun(s, c, 0, 1001)
	if s.Run.SkillCheck == nil {
		t.Fatal("expected the first pressure point to light up")
	}
	point := s.Run.SkillCheck.PressurePoint
	if s.Run.PressurePoints[point].Status != sim.PressurePointActive {
		t.Fatalf("lit point state = %q, want active", s.Run.PressurePoints[point].Status)
	}
	before := s.Run.ExtractedUnits
	if err := sim.AttemptSkillCheck(s, c, 1002); err != nil {
		t.Fatal(err)
	}
	if s.Run.PressurePoints[point].Status != sim.PressurePointHit {
		t.Fatalf("hit point state = %q, want hit", s.Run.PressurePoints[point].Status)
	}
	if s.Run.ExtractedUnits <= before {
		t.Fatalf("pressure-point hit did not fracture ore: %.1f -> %.1f", before, s.Run.ExtractedUnits)
	}

	miss := 1
	s.Run.SkillCheck = &sim.SkillCheck{Window: 1, PressurePoint: miss}
	s.Run.PressurePoints[miss].Status = sim.PressurePointActive
	sim.TickRun(s, c, 1, 1003)
	if s.Run.PressurePoints[miss].Status != sim.PressurePointMissed {
		t.Fatalf("expired pressure point state = %q, want missed", s.Run.PressurePoints[miss].Status)
	}
}

func TestNoPressurePointAppearsAfterMiningStops(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 100, 1000)
	_ = sim.Depart(s, c, 1)
	ast := s.Belt[0]
	ast.Volume = 100
	ast.Scanned = true
	s.Belt[0] = ast
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	s.Run.ExtractedUnits, s.Run.HeldUnits, s.Run.MinedUnits = 100, 100, 100
	s.Run.NextSkillCheckIn = 0
	s.Run.PirateDistance = 1e9
	sim.TickRun(s, c, 1, 1001)
	if s.Run.SkillCheck != nil {
		t.Fatalf("depleted asteroid spawned a pressure point: %+v", s.Run.SkillCheck)
	}
}

func TestFuelMinerOnlyRecoversFuelOnConfiguredFuelAsteroids(t *testing.T) {
	c := testContent(t)
	c.Mining.FuelAsteroidChance = 1 // proves the chance comes from content, not rarity/distance.
	s := sim.New(c, 101, 1000)
	s.Ships["skiff"].Internal = &sim.SlotDevice{ItemID: sim.ItemFuelMiner, Grade: 0}
	_ = sim.Depart(s, c, 1)
	ast := s.Belt[0]
	ast.Scanned = true
	s.Belt[0] = ast
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	if !s.Run.FuelAsteroid {
		t.Fatal("fuel chance of 1 did not produce a fuel asteroid")
	}
	s.Run.PirateDistance = 1e9
	fuelBefore := s.Fuel
	sim.TickRun(s, c, 1, 1001)
	if math.Abs(s.Fuel-fuelBefore) > 0.000001 {
		t.Fatalf("E Fuel Miner should exactly offset drill burn: %.6f -> %.6f", fuelBefore, s.Fuel)
	}

	s.Ships["skiff"].Internal.Grade = 1
	fuelBefore = s.Fuel
	sim.TickRun(s, c, 1, 1002)
	if s.Fuel <= fuelBefore {
		t.Fatalf("D Fuel Miner should create net fuel on a fuel asteroid: %.6f -> %.6f", fuelBefore, s.Fuel)
	}
}

func TestSeismicOverchargeAcceleratesMiningAndIsUnique(t *testing.T) {
	c := testContent(t)
	mineAfterSecond := func(overcharge bool) float64 {
		s := sim.New(c, 102, 1000)
		if overcharge {
			s.Credits = 100000
			if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemSeismicOvercharge, 0); err != nil {
				t.Fatal(err)
			}
		}
		_ = sim.Depart(s, c, 1)
		ast := s.Belt[0]
		ast.Scanned = true
		s.Belt[0] = ast
		if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
			t.Fatal(err)
		}
		s.Run.PirateDistance = 1e9
		sim.TickRun(s, c, 1, 1001)
		return s.Run.ExtractedUnits
	}
	if boosted, base := mineAfterSecond(true), mineAfterSecond(false); boosted <= base {
		t.Fatalf("seismic overcharge did not accelerate mining: base %.2f, boosted %.2f", base, boosted)
	}

	s := sim.New(c, 103, 1000)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemSeismicOvercharge, 0); err != nil {
		t.Fatal(err)
	}
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 1, sim.ItemSeismicOvercharge, 0); err != sim.ErrDuplicateItem {
		t.Fatalf("second seismic overcharge error = %v, want ErrDuplicateItem", err)
	}
}
