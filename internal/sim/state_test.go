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
	got, err := sim.DecodeState(b, c)
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

func TestNewPilotDefaultsToOreScan(t *testing.T) {
	if got := sim.New(testContent(t), 42, 1000).Settings.BeltView; got != sim.BeltViewOreScan {
		t.Fatalf("new-pilot belt view = %v, want ore scan", got)
	}
}

func TestVersionFourChaffMigratesToArmedEMPLauncher(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 42, 1000)
	s.Version = 4
	s.Ships["skiff"].Utility[0] = &sim.SlotDevice{ItemID: "chaff", Grade: 2}
	s.Inventory = []*sim.SlotDevice{{ItemID: "chaff", Grade: 4}}

	b, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := sim.DecodeState(b, c)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []*sim.SlotDevice{got.Ships["skiff"].Utility[0], got.Inventory[0]} {
		if d.ItemID != sim.ItemEMPLauncher || !d.EMPArmed {
			t.Fatalf("legacy chaff was not migrated to an armed EMP launcher: %+v", d)
		}
	}
}

func TestVersionSixMigratesRatingAndMassDriver(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 45, 1000)
	s.Version = 6
	s.Ships["skiff"].Internal = &sim.SlotDevice{ItemID: "jump_drive", Grade: 2}
	s.Ships["skiff"].Weapon = []*sim.SlotDevice{{ItemID: "mass_driver", Grade: 3}}
	s.Inventory = []*sim.SlotDevice{{ItemID: "mass_driver", Grade: 1}}

	b, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := sim.DecodeState(b, c)
	if err != nil {
		t.Fatal(err)
	}
	ship := got.Ships["skiff"]
	if ship.Internal != nil || got.JumpClass != 0 {
		t.Fatal("legacy drive did not become Class E")
	}

	if d := ship.Weapon[0]; d.ItemID != sim.ItemMissileLauncher || d.Missiles != sim.MissileCapacity(c, d.Grade) {
		t.Fatalf("legacy fitted mass driver migration = %+v", d)
	}
	if d := got.Inventory[0]; d.ItemID != sim.ItemMissileLauncher || d.Missiles != sim.MissileCapacity(c, d.Grade) {
		t.Fatalf("legacy stored mass driver migration = %+v", d)
	}
	if got.Version != sim.StateVersion {
		t.Fatalf("state version = %d, want %d", got.Version, sim.StateVersion)
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
	err := sim.Depart(s, c, 1) // Vesta needs 6
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
		if a.Risk < bc.RiskMin || a.Risk > bc.RiskMax {
			t.Fatalf("risk out of range: %d", a.Risk)
		}
	}
}
