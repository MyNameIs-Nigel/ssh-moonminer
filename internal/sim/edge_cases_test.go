package sim_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestFullCargoRejectsDepartAndLockWithoutSpendingFuel(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 11, 0)
	s.CargoUnits = sim.CargoCapacityUnits(s, c)
	before := s.Clone()
	if err := sim.Depart(s, c, 1); err != sim.ErrCargoFull {
		t.Fatalf("Depart full hold error = %v, want ErrCargoFull", err)
	}
	if !reflect.DeepEqual(s, before) {
		t.Fatalf("full-hold departure mutated state:\n got=%+v\nwant=%+v", s, before)
	}

	s.CargoUnits = 0
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	ast := s.Belt[0]
	s.Belt[0].Scanned = true
	s.CargoUnits = sim.CargoCapacityUnits(s, c)
	before = s.Clone()
	if err := sim.Lock(s, c, ast.ID, 1); err != sim.ErrCargoFull {
		t.Fatalf("Lock full hold error = %v, want ErrCargoFull", err)
	}
	if !reflect.DeepEqual(s, before) {
		t.Fatalf("full-hold lock mutated state:\n got=%+v\nwant=%+v", s, before)
	}
}

func TestCargoCapacityActionsCannotStrandCargo(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 12, 0)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemCargo, 0); err != nil {
		t.Fatal(err)
	}
	s.CargoUnits = sim.CargoCapacityUnits(s, c)
	if err := sim.StoreSlotDevice(s, c, "skiff", sim.SlotUtility, 0); err != sim.ErrCargoDoesNotFit {
		t.Fatalf("store cargo module error = %v, want ErrCargoDoesNotFit", err)
	}
	if s.Ships["skiff"].Utility[0] == nil {
		t.Fatal("rejected cargo-module storage removed the module")
	}

	if err := sim.AcquireShip(s, c, "mule"); err != nil {
		t.Fatal(err)
	}
	if err := sim.SwitchActiveShip(s, c, "skiff"); err != nil {
		t.Fatal(err)
	}
	// The MULE can carry this load, but the Skiff cannot once its extension
	// is removed. Switching from the large hold back to a bare Skiff must be
	// rejected before cargo is transferred.
	s.CargoUnits = sim.CargoCapacityUnitsFor(s, c, "mule")
	if err := sim.SwitchActiveShip(s, c, "mule"); err != nil {
		t.Fatal(err)
	}
	if err := sim.SwitchActiveShip(s, c, "skiff"); err != sim.ErrCargoDoesNotFit {
		t.Fatalf("switch into smaller hold error = %v, want ErrCargoDoesNotFit", err)
	}
}

func TestEquipmentDuplicatesAndTurretsHaveDefinedEffects(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 13, 0)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemShield, 0); err != nil {
		t.Fatal(err)
	}
	credits := s.Credits
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 1, sim.ItemShield, 0); err != sim.ErrDuplicateItem {
		t.Fatalf("second shield error = %v, want ErrDuplicateItem", err)
	}
	if s.Credits != credits {
		t.Fatalf("rejected second shield changed credits: got %d want %d", s.Credits, credits)
	}

	if err := sim.AcquireShip(s, c, "warden"); err != nil {
		t.Fatal(err)
	}
	if err := sim.SwitchActiveShip(s, c, "warden"); err != nil {
		t.Fatal(err)
	}
	s.Ships["warden"].Weapon[0] = &sim.SlotDevice{ItemID: sim.ItemTurret, Grade: 0}
	s.Ships["warden"].Weapon[1] = &sim.SlotDevice{ItemID: sim.ItemTurret, Grade: 0}
	want := math.Max(0.1, math.Pow(1-c.Slots.TurretPctPerGrade, 2))
	if got := sim.AttackDamageMul(s, c); math.Abs(got-want) > 0.000001 {
		t.Fatalf("two turret multiplier = %.6f, want %.6f", got, want)
	}
}

func TestSaleReplacementUsesRefundAtomically(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 14, 0)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemShield, 0); err != nil {
		t.Fatal(err)
	}
	price := sim.SlotItemPrice(c, sim.ItemEMPLauncher, 0)
	refund := sim.SlotItemSellValue(c, sim.ItemShield, 0)
	s.Credits = price - refund
	if err := sim.ReplaceSlotDeviceWithPurchase(s, c, "skiff", sim.SlotUtility, 0, sim.ItemEMPLauncher, 0); err != nil {
		t.Fatal(err)
	}
	if s.Credits != 0 || s.Ships["skiff"].Utility[0].ItemID != sim.ItemEMPLauncher {
		t.Fatalf("sale replacement = credits %d, device %+v", s.Credits, s.Ships["skiff"].Utility[0])
	}

	s.Credits = 0
	before := s.Clone()
	if err := sim.ReplaceSlotDeviceWithPurchase(s, c, "skiff", sim.SlotUtility, 0, sim.ItemFuelTank, 5); err != sim.ErrInsufficientFunds {
		t.Fatalf("unaffordable replacement error = %v, want ErrInsufficientFunds", err)
	}
	if !reflect.DeepEqual(s, before) {
		t.Fatalf("rejected replacement mutated state:\n got=%+v\nwant=%+v", s, before)
	}
}

func TestScannerRangeIncludesExactBoundary(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 15, 0)
	lock := sim.ScannerLockKm(s, c)
	for _, tc := range []struct {
		distance float64
		wantOut  bool
	}{
		{lock - 0.1, false},
		{lock, false},
		{lock + 0.1, true},
	} {
		ast := sim.Asteroid{Distance: tc.distance}
		if got := sim.IsOutOfRange(s, c, &ast); got != tc.wantOut {
			t.Errorf("distance %.1f out of range = %v, want %v", tc.distance, got, tc.wantOut)
		}
	}
}

func TestTributeUsesAllCargoWithoutRestoringAsteroidOre(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 16, 0)
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	ast := s.Belt[0]
	ast.Volume = 1000
	ast.Value = 1000
	ast.DrillSec = 100
	ast.Scanned = true
	s.Belt[0] = ast
	if err := sim.Lock(s, c, ast.ID, 1); err != nil {
		t.Fatal(err)
	}
	// This cargo came from earlier rocks. The active run holds a smaller
	// amount from the current asteroid, which makes the old loophole obvious.
	s.CargoUnits, s.CargoValue = 600, 600
	s.Run.Phase = sim.PhaseTribute
	s.Run.ExtractedUnits, s.Run.HeldUnits = 400, 400
	s.Run.MinedUnits = 400 // compatibility mirror must not drive remnants
	s.Run.CargoValue = 400

	totalBefore := s.CargoValue + s.Run.CargoValue
	demand := int(math.Round(float64(totalBefore) * c.Mining.TributeDemandPct))
	if err := sim.AcceptTribute(s, c, 2); err != nil {
		t.Fatal(err)
	}
	if got := s.CargoValue + s.Run.CargoValue; got != totalBefore-demand {
		t.Fatalf("tribute retained %d cargo value, want %d", got, totalBefore-demand)
	}
	if s.CargoValue >= 600 {
		t.Fatalf("tribute ignored previously held cargo: state cargo value %d", s.CargoValue)
	}
	if s.Run.ExtractedUnits != 400 {
		t.Fatalf("tribute changed extracted volume to %.0f; want 400", s.Run.ExtractedUnits)
	}
	if s.Run.HeldUnits >= 400 || s.Run.JettisonedUnits <= 0 {
		t.Fatalf("tribute did not separate current held/jettisoned cargo: %+v", s.Run)
	}
	if s.Run.TributeDemand != demand || s.Run.TributeCargoRetained != totalBefore-demand {
		t.Fatalf("tribute record = demand %d retained %d, want %d/%d", s.Run.TributeDemand, s.Run.TributeCargoRetained, demand, totalBefore-demand)
	}

	s.Run.BaseEscapeSecondsRequired = 0
	s.Run.EscapeSecondsRequired = 0
	if _, ended := sim.TickRun(s, c, 0, 3); !ended {
		t.Fatal("expected tribute escape to resolve")
	}
	remnant, _ := sim.FindAsteroid(s, ast.ID)
	if remnant == nil || remnant.Volume != 600 || remnant.Value != 600 {
		t.Fatalf("remnant after 400 extracted = %+v, want volume/value 600", remnant)
	}
	if s.CargoValue != totalBefore-demand {
		t.Fatalf("resolved cargo value = %d, want %d", s.CargoValue, totalBefore-demand)
	}
	if len(s.RunLog) == 0 || s.RunLog[0].CargoValueJettisoned != demand {
		t.Fatalf("tribute loss missing from run log: %+v", s.RunLog)
	}
}

func TestTributeCannotBecomeFreeWithOnlyPriorCargo(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 17, 0)
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	ast := s.Belt[0]
	ast.Scanned = true
	s.Belt[0] = ast
	if err := sim.Lock(s, c, ast.ID, 1); err != nil {
		t.Fatal(err)
	}
	s.CargoUnits, s.CargoValue = 100, 900
	s.Run.Phase = sim.PhaseTribute
	before := s.CargoValue
	if err := sim.AcceptTribute(s, c, 2); err != nil {
		t.Fatal(err)
	}
	if s.Run.TributeDemand <= 0 || s.CargoValue >= before {
		t.Fatalf("prior cargo should pay tribute: demand=%d value %d -> %d", s.Run.TributeDemand, before, s.CargoValue)
	}
}
