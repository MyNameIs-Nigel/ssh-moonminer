package sim_test

import (
	"math"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestThrusterGradesMateriallyReduceEscapeTime(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 71, 1000)
	s.Ships["skiff"].Grades.Thrusters = 0
	base := sim.EscapeMul(s, c)
	s.Ships["skiff"].Grades.Thrusters = sim.MaxGrade
	if got := sim.EscapeMul(s, c); !(got < base*0.55) {
		t.Fatalf("S thrusters escape multiplier = %.3f, want materially below E %.3f", got, base)
	}
}

func TestScannerGradesSpeedScansAndSIsInstantNearby(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 72, 1000)
	s.Ships["skiff"].Grades.Scanner = 0
	base := sim.ScannerScanSeconds(s, c, 5)
	s.Ships["skiff"].Grades.Scanner = 4
	if got := sim.ScannerScanSeconds(s, c, 5); !(got < base) {
		t.Fatalf("A scanner scan = %.3fs, want below E %.3fs", got, base)
	}
	s.Ships["skiff"].Grades.Scanner = sim.MaxGrade
	if got := sim.ScannerScanSeconds(s, c, 1.99); got != 0 {
		t.Fatalf("S scanner scan at 1.99km = %.3fs, want instant", got)
	}
	if got := sim.ScannerScanSeconds(s, c, 2); got <= 0 {
		t.Fatalf("S scanner scan at the non-instant 2km boundary = %.3fs, want positive", got)
	}

	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	s.Belt[0].Distance = 1.5
	s.Belt[0].Scanned = false
	fuelBefore := sim.FuelAmount(s, c)
	if err := sim.Scan(s, c, s.Belt[0].ID, 1000); err != nil {
		t.Fatal(err)
	}
	if s.Scan != nil || !s.Belt[0].Scanned {
		t.Fatalf("S close-range scan must finish in one action: scan=%+v asteroid=%+v", s.Scan, s.Belt[0])
	}
	if got, want := sim.FuelAmount(s, c), fuelBefore-c.Belt.ScanFuelCost; math.Abs(got-want) > 0.000001 {
		t.Fatalf("instant scan fuel = %.2f, want %.2f", got, want)
	}
}

func TestJammerDurationsMatchEveryGrade(t *testing.T) {
	c := testContent(t)
	want := []float64{3, 5, 8, 12, 15, 25}
	for grade, seconds := range want {
		if got := sim.JammerDurationSeconds(c, grade); got != seconds {
			t.Fatalf("grade %d jammer = %.0fs, want %.0fs", grade, got, seconds)
		}
	}
}

func TestMissilesUseAmmoCooldownAndDockReload(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 73, 1000)
	s.Credits = 100000
	if err := sim.AcquireShip(s, c, "warden"); err != nil {
		t.Fatal(err)
	}
	if err := sim.InstallSlotDevice(s, c, "warden", sim.SlotWeapon, 0, sim.ItemMissileLauncher, 0); err != nil {
		t.Fatal(err)
	}
	if err := sim.SwitchActiveShip(s, c, "warden"); err != nil {
		t.Fatal(err)
	}
	launcher := s.Ships["warden"].Weapon[0]
	if launcher.Missiles != 3 {
		t.Fatalf("E launcher starts with %d missiles, want 3", launcher.Missiles)
	}
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, s.Belt[0].ID, 1000); err != nil {
		t.Fatal(err)
	}
	c.Worlds[s.WorldIdx].PiratesAlwaysAttack = true
	s.Run.PirateDistance = 0
	if _, ended := sim.TickRun(s, c, 0, 1000); ended || s.Run == nil || s.Run.Combat == nil {
		t.Fatalf("expected combat, run=%+v ended=%v", s.Run, ended)
	}
	s.Run.Combat.PirateDPS = 0
	s.Run.Combat.PirateHull = 1000
	before := s.Run.Combat.PirateHull
	if _, err := sim.FireMissileWithOutcome(s, c, 1001); err != nil {
		t.Fatal(err)
	}
	if got, want := s.Run.Combat.PirateHull, before-sim.WeaponDamagePerShot(c, sim.ItemMissileLauncher, 0); got != want {
		t.Fatalf("guided missile damage = %.0f, want %.0f", before-got, before-want)
	}
	if launcher.Missiles != 2 || s.Run.Combat.MissileCooldown != 2 {
		t.Fatalf("missile fire state = missiles %d cooldown %.1f, want 2/2", launcher.Missiles, s.Run.Combat.MissileCooldown)
	}
	if _, err := sim.FireMissileWithOutcome(s, c, 1001); err != sim.ErrMissileCoolingDown {
		t.Fatalf("repeat missile fire = %v, want cooldown error", err)
	}
	if err := sim.FireWeapons(s, c, 1001); err != sim.ErrNoWeaponInstalled {
		t.Fatalf("F should only fire Pulse Lasers, got %v", err)
	}

	s.Run = nil
	sim.Dock(s, c)
	if launcher.Missiles != sim.MissileCapacity(c, launcher.Grade) {
		t.Fatalf("dock reload = %d, want %d", launcher.Missiles, sim.MissileCapacity(c, launcher.Grade))
	}
}
