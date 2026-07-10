package sim_test

import (
	"encoding/json"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestRouteGatesAndPermits(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)

	if got := sim.RouteLockReason(s, c, 1); got != "" { // Vesta starter belt
		t.Fatalf("starter route locked: %q", got)
	}
	if got := sim.RouteLockReason(s, c, 3); got != "NEED FUEL CAPACITY 120" { // Io
		t.Fatalf("Io lock = %q", got)
	}
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemFuelTank, 1); err != nil {
		t.Fatal(err)
	}
	if got := sim.RouteLockReason(s, c, 3); got != "" {
		t.Fatalf("fuel-tank gate did not clear: %q", got)
	}
	if got := sim.RouteLockReason(s, c, 0); got != "BUY NAV PERMIT 7000 cr" {
		t.Fatalf("Ceres lock = %q", got)
	}
	if err := sim.BuyDestinationPermit(s, c, "ceres_claims"); err != nil {
		t.Fatal(err)
	}
	if got := sim.RouteLockReason(s, c, 0); got != "NEED FUEL CAPACITY 140" {
		t.Fatalf("Ceres should retain its tank gate after permit purchase: %q", got)
	}
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 1, sim.ItemFuelTank, 1); err != nil {
		t.Fatal(err)
	}
	if got := sim.RouteLockReason(s, c, 0); got != "" {
		t.Fatalf("Ceres permit and tank gates did not clear: %q", got)
	}

	if got := sim.RouteLockReason(s, c, 2); got != "NEED FIGHTER-CLASS SHIP" {
		t.Fatalf("Eridani lock = %q", got)
	}
	if err := sim.AcquireShip(s, c, "warden"); err != nil {
		t.Fatal(err)
	}
	if err := sim.BuySystemPermit(s, c, "eridani"); err != nil {
		t.Fatal(err)
	}
	if got := sim.RouteLockReason(s, c, 2); got != "NEED FUEL CAPACITY 120" {
		t.Fatalf("Titan post-permit lock = %q", got)
	}
	if err := sim.InstallSlotDevice(s, c, "warden", sim.SlotUtility, 0, sim.ItemFuelTank, 1); err != nil {
		t.Fatal(err)
	}
	if got := sim.RouteLockReason(s, c, 2); got != "" {
		t.Fatalf("Eridani route remains locked: %q", got)
	}
}

func TestCargoSellsAtDockAndIsLostWithShip(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 2, 0)
	s.CargoUnits = 75
	s.CargoValue = 1200
	credits := s.Credits
	got, err := sim.SellCargo(s, c, 1)
	if err != nil || got != 1200 {
		t.Fatalf("SellCargo = (%d, %v)", got, err)
	}
	if s.Credits != credits+1200 || s.CargoValue != 0 || s.CargoUnits != 0 {
		t.Fatalf("sell did not clear cargo and pay credits: %+v", s)
	}
	if len(s.RunLog) != 1 || s.RunLog[0].CargoValueSold != 1200 {
		t.Fatalf("sale not recorded in ship log: %+v", s.RunLog)
	}

	s.CargoUnits = 75
	s.CargoValue = 900
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, s.Belt[0].ID, 1); err != nil {
		t.Fatal(err)
	}
	s.Run.CargoValue = 100
	s.Run.MinedUnits = 10
	s.Hull = 0
	out, ended := sim.TickRun(s, c, 0, 2)
	if !ended || out == nil || out.Record.CargoValueLost <= 900 {
		t.Fatalf("ship loss did not include stored cargo: ended=%v out=%+v", ended, out)
	}
	if s.CargoValue != 0 || s.CargoUnits != 0 {
		t.Fatalf("cargo survived ship loss: value=%d units=%f", s.CargoValue, s.CargoUnits)
	}
}

func TestVersionThreeSaveMigratesToSolWithEmptyCargo(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 3, 0)
	b, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	raw["version"] = float64(3)
	delete(raw, "system_id")
	delete(raw, "system_permits")
	delete(raw, "destination_permits")
	b, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := sim.DecodeState(b, c)
	if err != nil {
		t.Fatal(err)
	}
	if got.SystemID != "sol" || got.CargoValue != 0 || got.SystemPermits == nil || got.DestinationPermits == nil {
		t.Fatalf("bad v3 migration: %+v", got)
	}
}
