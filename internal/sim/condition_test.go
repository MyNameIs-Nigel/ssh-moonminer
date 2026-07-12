package sim_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestShipConditionPersistsAcrossSwitchesAndPurchaseDoesNotActivate(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 21, 0)
	s.Credits = 100000
	sim.DevSetFuel(s, c, 37)
	sim.DevSetHull(s, c, 63)

	if err := sim.AcquireShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	if s.ActiveShipID != "skiff" {
		t.Fatalf("purchase silently activated %q", s.ActiveShipID)
	}
	if got := sim.ShipFuelAmount(s, c, "cicada"); got != sim.ShipFuelCapacity(s, c, "cicada") {
		t.Fatalf("new Cicada fuel = %.0f, want fully serviced %.0f", got, sim.ShipFuelCapacity(s, c, "cicada"))
	}
	if got := sim.ShipHull(s, "cicada"); got != sim.MaxHullFor(s, c, "cicada") {
		t.Fatalf("new Cicada hull = %d, want %d", got, sim.MaxHullFor(s, c, "cicada"))
	}

	if err := sim.SwitchActiveShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	sim.DevSetFuel(s, c, 14)
	sim.DevSetHull(s, c, 52)
	if err := sim.SwitchActiveShip(s, c, "skiff"); err != nil {
		t.Fatal(err)
	}
	if got := sim.FuelAmount(s, c); got != 37 || sim.ShipHull(s, "skiff") != 63 {
		t.Fatalf("Skiff condition was serviced on switch: fuel %.0f hull %d", got, sim.ShipHull(s, "skiff"))
	}
	if err := sim.SwitchActiveShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	if got := sim.FuelAmount(s, c); got != 14 || sim.ShipHull(s, "cicada") != 52 {
		t.Fatalf("Cicada condition was serviced on switch: fuel %.0f hull %d", got, sim.ShipHull(s, "cicada"))
	}
}

func TestFuelTankKeepsReserveWhenStoredAndInstalledElsewhere(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 22, 0)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemFuelTank, 0); err != nil {
		t.Fatal(err)
	}
	tankCap := sim.FuelTankCapacity(c, s.Ships["skiff"].Utility[0])
	sim.DevSetFuel(s, c, sim.TankSize(s, c))
	if got := s.Ships["skiff"].Utility[0].Fuel; got != tankCap {
		t.Fatalf("filled tank = %.0f, want %.0f", got, tankCap)
	}
	if err := sim.StoreSlotDevice(s, c, "skiff", sim.SlotUtility, 0); err != nil {
		t.Fatal(err)
	}
	if got := sim.FuelAmount(s, c); got != float64(c.Pilot.StartFuel) {
		t.Fatalf("stored tank left active fuel %.0f, want base reserve %d", got, c.Pilot.StartFuel)
	}
	if len(s.Inventory) != 1 || s.Inventory[0].Fuel != tankCap {
		t.Fatalf("stored tank did not retain its reserve: %+v", s.Inventory)
	}

	if err := sim.AcquireShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	if err := sim.InstallSlotDeviceFromInventory(s, c, "cicada", sim.SlotUtility, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := sim.SwitchActiveShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	if got := sim.FuelAmount(s, c); got != float64(c.Pilot.StartFuel)+tankCap {
		t.Fatalf("reinstalled tank fuel = %.0f, want %.0f", got, float64(c.Pilot.StartFuel)+tankCap)
	}
	if err := sim.SellSlotDevice(s, c, "cicada", sim.SlotUtility, 0); err != nil {
		t.Fatal(err)
	}
	if got := sim.FuelAmount(s, c); got != float64(c.Pilot.StartFuel) {
		t.Fatalf("selling tank retained fuel: %.0f", got)
	}
}

func TestVersionFiveConditionMigratesIntoHullAndTank(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 23, 0)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemFuelTank, 0); err != nil {
		t.Fatal(err)
	}
	sim.DevSetFuel(s, c, sim.TankSize(s, c))
	sim.DevSetHull(s, c, 62)
	b, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	raw["version"] = float64(5)
	ships := raw["ships"].(map[string]any)
	skiff := ships["skiff"].(map[string]any)
	delete(skiff, "hull")
	delete(skiff, "base_fuel")
	utility := skiff["utility"].([]any)
	delete(utility[0].(map[string]any), "fuel")
	b, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := sim.DecodeState(b, c)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != sim.StateVersion || sim.ShipHull(got, "skiff") != 62 {
		t.Fatalf("condition migration failed: version=%d hull=%d", got.Version, sim.ShipHull(got, "skiff"))
	}
	if fuel := sim.FuelAmount(got, c); math.Abs(fuel-sim.TankSize(got, c)) > 0.000001 {
		t.Fatalf("migration fuel = %.2f, want %.2f", fuel, sim.TankSize(got, c))
	}
	if tank := got.Ships["skiff"].Utility[0].Fuel; tank <= 0 {
		t.Fatalf("migration did not assign reserve to tank: %.2f", tank)
	}
}

func TestRefuelBuysWholeFuelPoints(t *testing.T) {
	c := testContent(t)
	for _, tc := range []struct {
		credits int
		fuel    float64
		want    float64
		spent   int
		err     error
	}{
		{0, 0, 0, 0, sim.ErrInsufficientFunds},
		{1, 0, 0, 0, sim.ErrInsufficientFunds},
		{c.Port.RefuelPerPoint - 1, 0, 0, 0, sim.ErrInsufficientFunds},
		{c.Port.RefuelPerPoint, 0, 1, c.Port.RefuelPerPoint, nil},
		{c.Pilot.StartFuel * c.Port.RefuelPerPoint, 0, float64(c.Pilot.StartFuel), c.Pilot.StartFuel * c.Port.RefuelPerPoint, nil},
		{c.Port.RefuelPerPoint, 99.5, 99.5, 0, sim.ErrAlreadyFull},
	} {
		s := sim.New(c, 24, 0)
		s.Credits = tc.credits
		sim.DevSetFuel(s, c, tc.fuel)
		before := s.Credits
		err := sim.Refuel(s, c)
		if err != tc.err {
			t.Errorf("credits=%d fuel=%.1f: Refuel error=%v, want %v", tc.credits, tc.fuel, err, tc.err)
			continue
		}
		if got := sim.FuelAmount(s, c); math.Abs(got-tc.want) > 0.000001 || before-s.Credits != tc.spent {
			t.Errorf("credits=%d fuel=%.1f: got fuel %.2f spent %d, want %.2f/%d", tc.credits, tc.fuel, got, before-s.Credits, tc.want, tc.spent)
		}
	}
}

func TestShipLossSelectsSkiffWithoutServicingIt(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 25, 0)
	s.Credits = 100000
	if err := sim.AcquireShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	sim.DevSetFuel(s, c, 31)
	sim.DevSetHull(s, c, 44)
	if err := sim.SwitchActiveShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, s.Belt[0].ID, 1); err != nil {
		t.Fatal(err)
	}
	s.Hull = 0
	if _, ended := sim.TickRun(s, c, 0, 2); !ended {
		t.Fatal("expected destroyed Cicada to resolve")
	}
	if s.ActiveShipID != "skiff" || sim.ShipHull(s, "skiff") != 44 || sim.FuelAmount(s, c) != 31 {
		t.Fatalf("recovery serviced or chose the wrong backup: active=%s hull=%d fuel=%.0f", s.ActiveShipID, sim.ShipHull(s, "skiff"), sim.FuelAmount(s, c))
	}
}
