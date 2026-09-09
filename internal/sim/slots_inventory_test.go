package sim_test

import (
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

// TestStoreSlotDeviceMovesToInventoryWithNoRefund covers the shipyard's
// "store" remove choice: the device leaves the ship slot with no credit
// change and becomes re-equippable later via InstallSlotDeviceFromInventory.
func TestStoreSlotDeviceMovesToInventoryWithNoRefund(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemShield, 1); err != nil {
		t.Fatal(err)
	}
	creditsBefore := s.Credits

	if err := sim.StoreSlotDevice(s, c, "skiff", sim.SlotUtility, 0); err != nil {
		t.Fatal(err)
	}
	if s.Credits != creditsBefore {
		t.Fatalf("store should not change credits: before=%d after=%d", creditsBefore, s.Credits)
	}
	if s.Ships["skiff"].Utility[0] != nil {
		t.Fatal("expected the utility slot to be empty after storing")
	}
	if len(s.Inventory) != 1 || s.Inventory[0].ItemID != sim.ItemShield || s.Inventory[0].Grade != 1 {
		t.Fatalf("expected the shield (grade 1) to land in inventory, got %+v", s.Inventory)
	}
}

// TestSellSlotDeviceRefunds95Pct covers the shipyard's "sell" remove choice.
func TestSellSlotDeviceRefunds95Pct(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemShield, 1); err != nil {
		t.Fatal(err)
	}
	price := sim.SlotItemPrice(c, sim.ItemShield, 1)
	creditsBefore := s.Credits

	if err := sim.SellSlotDevice(s, c, "skiff", sim.SlotUtility, 0); err != nil {
		t.Fatal(err)
	}
	wantRefund := sim.SlotItemSellValue(c, sim.ItemShield, 1)
	if wantRefund <= 0 || wantRefund >= price {
		t.Fatalf("expected sell value strictly between 0 and the %d cr buy price, got %d", price, wantRefund)
	}
	if s.Credits != creditsBefore+wantRefund {
		t.Fatalf("expected credits to increase by %d, got %d -> %d", wantRefund, creditsBefore, s.Credits)
	}
	if s.Ships["skiff"].Utility[0] != nil {
		t.Fatal("expected the utility slot to be empty after selling")
	}
	if len(s.Inventory) != 0 {
		t.Fatalf("selling must not add to inventory, got %+v", s.Inventory)
	}
}

// TestStoreAndSellRequireAnOccupiedSlot covers the empty-slot guard shared
// by both remove paths.
func TestStoreAndSellRequireAnOccupiedSlot(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)

	if err := sim.StoreSlotDevice(s, c, "skiff", sim.SlotUtility, 0); err != sim.ErrSlotEmpty {
		t.Fatalf("expected ErrSlotEmpty storing an empty slot, got %v", err)
	}
	if err := sim.SellSlotDevice(s, c, "skiff", sim.SlotUtility, 0); err != sim.ErrSlotEmpty {
		t.Fatalf("expected ErrSlotEmpty selling an empty slot, got %v", err)
	}
}

// TestInstallSlotDeviceRejectsReplacingAnInstalledModule ensures callers
// cannot silently destroy a module by installing another one over it.
func TestInstallSlotDeviceRejectsReplacingAnInstalledModule(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemShield, 1); err != nil {
		t.Fatal(err)
	}
	creditsBefore := s.Credits

	err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemEMPLauncher, 0)
	if err != sim.ErrSlotOccupied {
		t.Fatalf("expected ErrSlotOccupied, got %v", err)
	}
	if s.Credits != creditsBefore {
		t.Fatalf("rejected replacement changed credits: before=%d after=%d", creditsBefore, s.Credits)
	}
	d := s.Ships["skiff"].Utility[0]
	if d == nil || d.ItemID != sim.ItemShield || d.Grade != 1 {
		t.Fatalf("rejected replacement changed installed module: %+v", d)
	}
}

// TestInstallSlotDeviceFromInventoryRejectsReplacingAnInstalledModule covers
// the same safeguard for a stored module being re-equipped.
func TestInstallSlotDeviceFromInventoryRejectsReplacingAnInstalledModule(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemShield, 0); err != nil {
		t.Fatal(err)
	}
	s.Inventory = []*sim.SlotDevice{{ItemID: sim.ItemEMPLauncher, Grade: 1, EMPArmed: true}}

	err := sim.InstallSlotDeviceFromInventory(s, c, "skiff", sim.SlotUtility, 0, 0)
	if err != sim.ErrSlotOccupied {
		t.Fatalf("expected ErrSlotOccupied, got %v", err)
	}
	if len(s.Inventory) != 1 || s.Inventory[0].ItemID != sim.ItemEMPLauncher {
		t.Fatalf("rejected replacement changed inventory: %+v", s.Inventory)
	}
	if d := s.Ships["skiff"].Utility[0]; d == nil || d.ItemID != sim.ItemShield {
		t.Fatalf("rejected replacement changed installed module: %+v", d)
	}
}

// TestInstallSlotDeviceFromInventoryIsFreeAndAccountWide covers re-equipping
// a stored device on a *different* owned ship than it was removed from,
// confirming the inventory pool is account-wide, not per-ship, and that the
// re-install costs no credits.
func TestInstallSlotDeviceFromInventoryIsFreeAndAccountWide(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemShield, 1); err != nil {
		t.Fatal(err)
	}
	if err := sim.StoreSlotDevice(s, c, "skiff", sim.SlotUtility, 0); err != nil {
		t.Fatal(err)
	}
	if err := sim.AcquireShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	creditsBefore := s.Credits

	if err := sim.InstallSlotDeviceFromInventory(s, c, "cicada", sim.SlotUtility, 0, 0); err != nil {
		t.Fatal(err)
	}
	if s.Credits != creditsBefore {
		t.Fatalf("installing from inventory should be free: before=%d after=%d", creditsBefore, s.Credits)
	}
	if len(s.Inventory) != 0 {
		t.Fatalf("expected the device to leave inventory once installed, got %+v", s.Inventory)
	}
	d := s.Ships["cicada"].Utility[0]
	if d == nil || d.ItemID != sim.ItemShield || d.Grade != 1 {
		t.Fatalf("expected cicada's utility slot 0 to hold the stored shield (grade 1), got %+v", d)
	}
}

// TestInstallSlotDeviceFromInventoryRejectsWrongKindAndOverPower covers the
// two validation paths specific to the inventory install (kind mismatch,
// power overflow) beyond what InstallSlotDevice already tests.
func TestInstallSlotDeviceFromInventoryRejectsWrongKindAndOverPower(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 1000)
	s.Credits = 100000
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemShield, 0); err != nil {
		t.Fatal(err)
	}
	if err := sim.StoreSlotDevice(s, c, "skiff", sim.SlotUtility, 0); err != nil {
		t.Fatal(err)
	}

	// Wrong kind: inventory holds a Utility item, targeting an Internal slot.
	if err := sim.InstallSlotDeviceFromInventory(s, c, "skiff", sim.SlotInternal, 0, 0); err != sim.ErrInvalidSlotItem {
		t.Fatalf("expected ErrInvalidSlotItem for a kind mismatch, got %v", err)
	}

	// Invalid inventory index.
	if err := sim.InstallSlotDeviceFromInventory(s, c, "skiff", sim.SlotUtility, 0, 5); err != sim.ErrInvalidInventory {
		t.Fatalf("expected ErrInvalidInventory for an out-of-range index, got %v", err)
	}

	// Over power: fill the skiff's other utility slot close to its 10-power
	// budget, then try to re-equip the stored (now inventory[0]) shield into
	// the empty slot — free re-installs must respect power headroom exactly
	// like a paid install does.
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 1, sim.ItemEMPLauncher, 5); err != nil {
		t.Fatal(err)
	}
	if err := sim.InstallSlotDeviceFromInventory(s, c, "skiff", sim.SlotUtility, 0, 0); err != sim.ErrPowerExceeded {
		t.Fatalf("expected ErrPowerExceeded, got %v", err)
	}
}

// Legacy route devices are removed at the save boundary (TestV7RatingMigrationAndClone).
func TestRetiredRouteDeviceCannotBeInstalled(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)
	s.Inventory = []*sim.SlotDevice{{ItemID: "jump_drive"}}
	if err := sim.InstallSlotDeviceFromInventory(s, c, "skiff", sim.SlotUtility, 0, 0); err != sim.ErrItemLocked {
		t.Fatalf("retired device install = %v", err)
	}
}

func TestSlotItemSellValueIs95PctRoundedOfPrice(t *testing.T) {
	c := testContent(t)
	price := sim.SlotItemPrice(c, sim.ItemCargo, 3)
	got := sim.SlotItemSellValue(c, sim.ItemCargo, 3)
	want := int(float64(price)*0.95 + 0.5)
	if got != want {
		t.Fatalf("SlotItemSellValue(cargo, 3) = %d, want %d (95%% of %d cr)", got, want, price)
	}
}
