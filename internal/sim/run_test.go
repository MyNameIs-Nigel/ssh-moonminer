package sim_test

import (
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestOutcomes(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	_ = sim.Depart(s, c, 1)
	belt := s.Belt
	if len(belt) == 0 {
		t.Fatal("no belt")
	}
	ast := belt[0]
	s.Fuel = 200
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}

	s.Run.Drill = 50
	s.Run.Yield = ast.Value / 2
	rec := sim.Bail(s, c, 2000)
	if rec == nil || rec.Kind != sim.OutcomeBail {
		t.Fatalf("bail outcome: %+v", rec)
	}
	if s.Run != nil {
		t.Fatal("run should be cleared")
	}
}

func TestRaidedPriority(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	_ = sim.Depart(s, c, 0)
	ast := s.Belt[0]
	s.Fuel = 500
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, ast.ID, 1000); err != nil {
		t.Fatal(err)
	}
	s.Run.Pirate = 99.9
	s.Run.Drill = 99.9
	out, ended := sim.TickRun(s, c, 1.0, 2000)
	if !ended || out == nil || out.Kind != sim.OutcomeRaided {
		t.Fatalf("expected raided, got %+v ended=%v", out, ended)
	}
}

func TestBailNoOp(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)
	if sim.Bail(s, c, 0) != nil {
		t.Fatal("expected nil")
	}
}

func TestOverdriveFaster(t *testing.T) {
	c := testContent(t)
	runFuelBurn := func(over bool, ticks int) float64 {
		s := sim.New(c, 1, 0)
		_ = sim.Depart(s, c, 1)
		ast := s.Belt[0]
		ast.DrillSec = 10
		ast.Scanned = true
		s.Belt[0] = ast
		s.Fuel = 500
		if err := sim.Lock(s, c, ast.ID, 0); err != nil {
			t.Fatal(err)
		}
		if over {
			sim.SetOverdrive(s, true)
		}
		start := s.Fuel
		dt := 1.0 / float64(c.Mining.TickHz)
		for i := 0; i < ticks; i++ {
			out, done := sim.TickRun(s, c, dt, int64(i))
			if done {
				break
			}
			_ = out
		}
		return start - s.Fuel
	}
	fuelNormal := runFuelBurn(false, 20)
	fuelOver := runFuelBurn(true, 20)
	if fuelOver <= fuelNormal*1.5 {
		t.Fatalf("overdrive should burn noticeably more fuel: normal=%v over=%v", fuelNormal, fuelOver)
	}
}
