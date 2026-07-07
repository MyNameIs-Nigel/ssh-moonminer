package sim_test

import (
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func testContent(t *testing.T) *content.Content {
	t.Helper()
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 42, 1000)
	s.Credits = 5000
	s.WorldIdx = 1
	s.Belt = sim.GenerateBelt(s, c, 1)

	b, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := sim.DecodeState(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Credits != 5000 || got.WorldIdx != 1 || len(got.Belt) == 0 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if got.Run != nil {
		t.Fatal("run should not persist")
	}
}

func TestGenerateBeltDeterministic(t *testing.T) {
	c := testContent(t)
	a := sim.New(c, 99, 0)
	b := sim.New(c, 99, 0)
	beltA := sim.GenerateBelt(a, c, 0)
	beltB := sim.GenerateBelt(b, c, 0)
	if len(beltA) != len(beltB) {
		t.Fatalf("belt lengths differ")
	}
	for i := range beltA {
		if beltA[i] != beltB[i] {
			t.Fatalf("belt[%d] differs: %+v vs %+v", i, beltA[i], beltB[i])
		}
	}
}

func TestDepartInsufficientFuel(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)
	s.Fuel = 5
	err := sim.Depart(s, c, 0) // Ceres needs 8
	if err != sim.ErrInsufficientFuel {
		t.Fatalf("got %v", err)
	}
	if s.WorldIdx != -1 {
		t.Fatal("state mutated")
	}
}

func TestBeltRanges(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 12345, 0)
	belt := sim.GenerateBelt(s, c, 2)
	bc := c.Belt
	for _, a := range belt {
		if a.Volume < bc.VolumeMin || a.Volume > bc.VolumeMax {
			t.Fatalf("volume out of range: %d", a.Volume)
		}
		if a.FuelCost < bc.FuelCostMin || a.FuelCost > bc.FuelCostMax {
			t.Fatalf("fuel cost out of range: %d", a.FuelCost)
		}
		if a.Risk < bc.RiskMin || a.Risk > bc.RiskMax {
			t.Fatalf("risk out of range: %d", a.Risk)
		}
	}
}
