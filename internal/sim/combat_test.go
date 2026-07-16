package sim_test

import (
	"math"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func startAutocannonCombat(t *testing.T, c *content.Content) (*sim.State, int) {
	t.Helper()
	s := sim.New(c, 41, 1000)
	s.Credits = 100000
	if err := sim.AcquireShip(s, c, "warden"); err != nil {
		t.Fatal(err)
	}
	s.Ships["warden"].Weapon[0] = &sim.SlotDevice{ItemID: sim.ItemTurret, Grade: 0}
	if err := sim.SwitchActiveShip(s, c, "warden"); err != nil {
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
	c.Worlds[s.WorldIdx].PiratesAlwaysAttack = true
	s.Run.PirateDistance = 0
	if _, ended := sim.TickRun(s, c, 0, 1000); ended || s.Run == nil || s.Run.Phase != sim.PhaseCombat {
		t.Fatalf("expected an active combat, run=%+v ended=%v", s.Run, ended)
	}
	return s, ast.ID
}

func TestAutocannonDamagesContinuouslyAndEndsRunOnKill(t *testing.T) {
	c := testContent(t)
	s, _ := startAutocannonCombat(t, c)

	dps := sim.AutocannonDamagePerSecond(s, c)
	if dps <= 0 {
		t.Fatal("expected an installed autocannon to have positive DPS")
	}
	before := s.Run.Combat.PirateHull
	if out, ended := sim.TickRun(s, c, 1, 1001); ended || out != nil {
		t.Fatalf("autocannon should not win this first tick: out=%+v ended=%v", out, ended)
	}
	if got := s.Run.Combat.PirateHull; math.Abs(got-(before-dps)) > 0.000001 {
		t.Fatalf("autocannon damage after 1 second = %.6f, want %.6f", before-got, dps)
	}

	s.Run.Combat.PirateHull = dps / 2
	out, ended := sim.TickRun(s, c, 1, 1002)
	if !ended || out == nil || out.Kind != sim.OutcomePirateDestroyed {
		t.Fatalf("autocannon kill should end at the summary outcome, out=%+v ended=%v", out, ended)
	}
	if s.Run != nil {
		t.Fatal("pirate kill should clear the active run before returning to the belt")
	}
	if out.Record.PirateDestroyed == "" || out.Record.BountyEarned <= 0 || s.BountyVouchers != out.Record.BountyEarned {
		t.Fatalf("kill summary did not retain pirate and bounty: outcome=%+v vouchers=%d", out, s.BountyVouchers)
	}
}

func TestGuidedMissileKillReturnsSummaryOutcome(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 43, 1000)
	s.Credits = 100000
	if err := sim.AcquireShip(s, c, "warden"); err != nil {
		t.Fatal(err)
	}
	s.Ships["warden"].Weapon[0] = &sim.SlotDevice{ItemID: sim.ItemMissileLauncher, Grade: 0, Missiles: sim.MissileCapacity(c, 0)}
	if err := sim.SwitchActiveShip(s, c, "warden"); err != nil {
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
	c.Worlds[s.WorldIdx].PiratesAlwaysAttack = true
	s.Run.PirateDistance = 0
	if _, ended := sim.TickRun(s, c, 0, 1000); ended || s.Run == nil || s.Run.Combat == nil {
		t.Fatalf("expected an active combat, run=%+v ended=%v", s.Run, ended)
	}
	s.Run.Combat.PirateHull = 1
	out, err := sim.FireMissileWithOutcome(s, c, 1001)
	if err != nil || out == nil || out.Kind != sim.OutcomePirateDestroyed {
		t.Fatalf("manual kill outcome = %+v, err=%v", out, err)
	}
	if s.Run != nil {
		t.Fatal("missile pirate kill should clear the active run")
	}
}

func TestJammerSuppressesPirateApproachUntilCountdownExpires(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 42, 1000)
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	ast := s.Belt[0]
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	s.Run.JammerRemaining = 2
	s.Run.PirateDistance = 100
	if out, ended := sim.TickRun(s, c, 1, 1001); ended || out != nil {
		t.Fatalf("jammed run should not resolve: out=%+v ended=%v", out, ended)
	}
	if s.Run.PirateDistance != 100 || s.Run.JammerRemaining != 1 {
		t.Fatalf("jammer should freeze approach for the first second: %+v", s.Run)
	}
	sim.TickRun(s, c, 2, 1003)
	if s.Run == nil || s.Run.Phase != sim.PhaseMining || s.Run.PirateDistance >= 100 {
		t.Fatalf("pirates should resume approaching after the jammer expires: %+v", s.Run)
	}
}

func TestHeatSinkRaisesShipHeatCapacityByGrade(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 42, 1000)
	base := c.Combat.HeatCapacity
	if got := sim.HeatCapacity(s, c); got != base {
		t.Fatalf("base heat capacity = %.0f, want %.0f", got, base)
	}
	s.Ships["skiff"].Internal = &sim.SlotDevice{ItemID: sim.ItemHeatSink, Grade: 2}
	want := base + 3*c.Slots.HeatSinkCapacityPerGrade
	if got := sim.HeatCapacity(s, c); got != want {
		t.Fatalf("C-grade heat sink capacity = %.0f, want %.0f", got, want)
	}
}

func TestHeatSinkDelaysManualWeaponOverheat(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 42, 1000)
	s.Credits = 100000
	if err := sim.AcquireShip(s, c, "warden"); err != nil {
		t.Fatal(err)
	}
	s.Ships["warden"].Weapon[0] = &sim.SlotDevice{ItemID: sim.ItemPulseLaser, Grade: 0}
	s.Ships["warden"].Internal = &sim.SlotDevice{ItemID: sim.ItemHeatSink, Grade: 0}
	if err := sim.SwitchActiveShip(s, c, "warden"); err != nil {
		t.Fatal(err)
	}
	s.Run = &sim.ActiveRun{
		Phase: sim.PhaseCombat,
		Combat: &sim.CombatState{
			PirateName: "TARGET",
			PirateHull: 10000,
		},
	}
	for shot := 1; shot <= 10; shot++ {
		if _, err := sim.FireWeaponsWithOutcome(s, c, int64(shot)); err != nil {
			t.Fatalf("shot %d: %v", shot, err)
		}
	}
	if s.Run.Combat.LockRemaining > 0 {
		t.Fatalf("E-grade heat sink should keep ten pulse shots below its %.0f capacity", sim.HeatCapacity(s, c))
	}
	if _, err := sim.FireWeaponsWithOutcome(s, c, 11); err != nil {
		t.Fatal(err)
	}
	if s.Run.Combat.LockRemaining <= 0 || s.Run.Combat.Heat != sim.HeatCapacity(s, c) {
		t.Fatalf("eleventh pulse shot should overheat at sink capacity: %+v", s.Run.Combat)
	}
}

func TestAutocannonFastForwardStillAllowsPirateToWin(t *testing.T) {
	c := testContent(t)
	s, _ := startAutocannonCombat(t, c)
	// A giant simulation tick is used during EmergencyResolve. The pirate has
	// enough DPS to destroy the ship before the turret can finish it.
	s.Hull = 1
	s.Run.Combat.PirateDPS = 100
	out := sim.EmergencyResolve(s, c, 1001)
	if out == nil || out.Kind != sim.OutcomeShipLost {
		t.Fatalf("fast-forwarded combat should preserve pirate damage, out=%+v", out)
	}
}
