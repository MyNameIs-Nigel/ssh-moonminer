package sim_test

import (
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

// Gate direction and lifetime credentials are covered by jump_test.go. The
// old installed-key removal guard is deliberately retired in schema v8.

func TestInsuranceAdvanceIsOnlyForActualStarterSoftlocks(t *testing.T) {
	c := testContent(t)
	newEligible := func() *sim.State {
		s := sim.New(c, 32, 0)
		s.Credits = c.Port.InsuranceCreditThreshold - 1
		sim.DevSetFuel(s, c, 0)
		return s
	}
	if !sim.InsuranceEligible(newEligible(), c) {
		t.Fatal("expected bare, empty starter ship to be eligible")
	}

	for name, alter := range map[string]func(*sim.State){
		"cargo":   func(s *sim.State) { s.CargoUnits, s.CargoValue = 1, 1 },
		"fuel":    func(s *sim.State) { sim.DevSetFuel(s, c, float64(c.Port.InsuranceFuelThreshold)) },
		"credits": func(s *sim.State) { s.Credits = c.Port.InsuranceCreditThreshold },
		"used":    func(s *sim.State) { s.Settings.InsuranceUsed = true },
		"fleet": func(s *sim.State) {
			s.Credits = 100000
			if err := sim.AcquireShip(s, c, "cicada"); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := newEligible()
			alter(s)
			if sim.InsuranceEligible(s, c) {
				t.Fatal("ineligible pilot qualified for insurance")
			}
		})
	}

	s := newEligible()
	if err := sim.InsuranceAdvance(s, c); err != nil {
		t.Fatal(err)
	}
	if s.Credits != c.Port.InsuranceCredits {
		t.Fatalf("advance credits = %d, want %d", s.Credits, c.Port.InsuranceCredits)
	}
	if err := sim.Refuel(s, c); err != nil {
		t.Fatal(err)
	}
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatalf("advance should fund one cheapest starter trip: %v", err)
	}

	// Recovery milestones, not a particular run outcome, reset the claim.
	sim.Dock(s, c)
	s.Settings.InsuranceUsed = true
	s.CargoUnits, s.CargoValue = 1, 10
	if _, err := sim.SellCargo(s, c, 1); err != nil {
		t.Fatal(err)
	}
	if s.Settings.InsuranceUsed {
		t.Fatal("selling cargo should reset the recovery claim")
	}
	s.Settings.InsuranceUsed = true
	s.Credits = 100000
	if err := sim.AcquireShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	if s.Settings.InsuranceUsed {
		t.Fatal("buying a paid ship should reset the recovery claim")
	}
}

func TestThreatAssistChangesPirateApproachOnly(t *testing.T) {
	c := testContent(t)
	distanceAfter := func(assist float64) float64 {
		s := sim.New(c, 33, 0)
		s.Settings.PirateAggression = assist
		if err := sim.Depart(s, c, 1); err != nil {
			t.Fatal(err)
		}
		s.Belt[0].Scanned = true
		if err := sim.Lock(s, c, s.Belt[0].ID, 1); err != nil {
			t.Fatal(err)
		}
		credits, cargo := s.Credits, s.CargoValue
		sim.TickRun(s, c, 1, 2)
		if s.Credits != credits || s.CargoValue != cargo {
			t.Fatalf("threat assist changed rewards: credits %d/%d cargo %d/%d", credits, s.Credits, cargo, s.CargoValue)
		}
		return s.Run.PirateDistance
	}
	if slow, fast := distanceAfter(0.5), distanceAfter(2.0); slow <= fast {
		t.Fatalf("threat assist did not change approach speed: 0.5=%v 2.0=%v", slow, fast)
	}
}
