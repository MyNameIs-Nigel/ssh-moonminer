package sim_test

import (
	"math"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func installTestShield(t *testing.T, grade int) (*sim.State, float64) {
	t.Helper()
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemShield, grade); err != nil {
		t.Fatal(err)
	}
	_, maxHP, damaged := sim.ShieldStatus(s, c)
	if maxHP <= 0 || damaged {
		t.Fatalf("new shield status = max=%v damaged=%v, want charged and intact", maxHP, damaged)
	}
	return s, maxHP
}

func TestShieldAbsorbsAttackBeforeHull(t *testing.T) {
	c := testContent(t)
	s, maxHP := installTestShield(t, 0)
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, s.Belt[0].ID, 1000); err != nil {
		t.Fatal(err)
	}
	if err := sim.BailOrDepart(s, c, 1000); err != nil {
		t.Fatal(err)
	}
	s.Run.UnderAttack = true
	s.Run.EscapeSecondsRequired = 1e9 // keep the run open for this focused damage check
	startHull := s.Hull
	dt := maxHP / (2 * c.Mining.AttackHullDamagePerSecond)
	sim.TickRun(s, c, dt, 1001)

	hp, _, damaged := sim.ShieldStatus(s, c)
	if hp <= 0 || hp >= maxHP {
		t.Fatalf("shield should absorb a partial attack: hp=%v max=%v", hp, maxHP)
	}
	if damaged {
		t.Fatal("a partially depleted shield must not be marked damaged")
	}
	if s.Hull != startHull {
		t.Fatalf("shielded partial attack damaged hull: %d -> %d", startHull, s.Hull)
	}
}

func TestShieldBeltRechargeAndBurstCap(t *testing.T) {
	c := testContent(t)
	s, maxHP := installTestShield(t, 0)
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	inst := s.Ships[s.ActiveShipID]

	inst.ShieldHP = 0
	if got := sim.ShieldRechargeETA(s, c); math.Abs(got-c.Slots.ShieldRechargeESeconds) > 0.001 {
		t.Fatalf("empty E shield ETA = %v, want %v", got, c.Slots.ShieldRechargeESeconds)
	}
	sim.TickBelt(s, c, c.Slots.ShieldRechargeESeconds)
	hp, _, damaged := sim.ShieldStatus(s, c)
	if math.Abs(hp-maxHP) > 0.001 || damaged {
		t.Fatalf("E shield should fully recharge in %.0fs: hp=%v max=%v damaged=%v", c.Slots.ShieldRechargeESeconds, hp, maxHP, damaged)
	}

	inst.ShieldHP = 0
	inst.ShieldDamaged = true
	sim.TickBelt(s, c, c.Slots.ShieldRechargeESeconds)
	hp, _, damaged = sim.ShieldStatus(s, c)
	wantBurst := maxHP * c.Slots.ShieldBurstReturnPct
	if math.Abs(hp-wantBurst) > 0.001 || !damaged {
		t.Fatalf("burst shield belt recovery = %v damaged=%v, want %v and damaged", hp, damaged, wantBurst)
	}

	sim.Dock(s, c)
	hp, _, damaged = sim.ShieldStatus(s, c)
	if math.Abs(hp-maxHP) > 0.001 || damaged {
		t.Fatalf("dock service = %v damaged=%v, want %v and intact", hp, damaged, maxHP)
	}
}

func TestShieldRechargeGradeBounds(t *testing.T) {
	c := testContent(t)
	for _, grade := range []int{0, sim.MaxGrade} {
		s := sim.New(c, 1, 1000)
		// This test only checks the grade-to-time mapping. Installing an S
		// shield through the shipyard would be invalid on the starter's
		// E-grade power budget.
		s.Ships[s.ActiveShipID].Utility[0] = &sim.SlotDevice{ItemID: sim.ItemShield, Grade: grade}
		got := sim.ShieldRechargeSeconds(s, c)
		want := c.Slots.ShieldRechargeESeconds
		if grade == sim.MaxGrade {
			want = c.Slots.ShieldRechargeSSeconds
		}
		if got != want {
			t.Fatalf("grade %d recharge = %v, want %v", grade, got, want)
		}
	}
}
