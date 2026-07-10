package sim

import (
	"math"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// SlotKind identifies which of a ship's three slot kinds an item belongs
// in. Internal is always exactly one slot per ship; Utility/Weapon counts
// vary per ship model (Weapon may be zero).
type SlotKind int

const (
	SlotUtility SlotKind = iota
	SlotWeapon
	SlotInternal
)

// Slot item IDs — the fixed catalog from
// docs/gameplay/05-fleet-ships-and-shipyard-economy.md. Not content-data
// driven (like TrackName) since each has a distinct effect formula.
const (
	ItemCargo     = "cargo"
	ItemFuelTank  = "fuel_tank"
	ItemShield    = "shield"
	ItemChaff     = "chaff"
	ItemTurret    = "turret"
	ItemSeismic   = "seismic_sensors"
	ItemFuelMiner = "fuel_miner"
	ItemJammer    = "pirate_jammer"
	ItemJumpDrive = "jump_drive"
)

// SlotItemKind reports which slot kind an item installs into.
func SlotItemKind(itemID string) SlotKind {
	switch itemID {
	case ItemTurret:
		return SlotWeapon
	case ItemSeismic, ItemFuelMiner, ItemJammer, ItemJumpDrive:
		return SlotInternal
	default:
		return SlotUtility
	}
}

// SlotItemName returns the slot item's display name.
func SlotItemName(itemID string) string {
	switch itemID {
	case ItemCargo:
		return "EXTRA CARGO"
	case ItemFuelTank:
		return "FUEL TANK"
	case ItemShield:
		return "SHIELD"
	case ItemChaff:
		return "CHAFF LAUNCHER"
	case ItemTurret:
		return "DEFENSE TURRET"
	case ItemSeismic:
		return "SEISMIC SENSORS"
	case ItemFuelMiner:
		return "FUEL MINER"
	case ItemJammer:
		return "PIRATE JAMMER"
	case ItemJumpDrive:
		return "JUMP DRIVE"
	}
	return itemID
}

// SlotItemLocked reports whether the item is unavailable for purchase. Every
// current catalog item is usable; the Jump Drive now unlocks Eridani Drift.
func SlotItemLocked(itemID string) bool { return false }

// utilityItems/weaponItems/internalItems list the buyable items per slot
// kind, in catalog display order.
var utilityItems = []string{ItemCargo, ItemFuelTank, ItemShield, ItemChaff}
var weaponItems = []string{ItemTurret}
var internalItems = []string{ItemSeismic, ItemFuelMiner, ItemJammer, ItemJumpDrive}

// SlotItemsFor returns the buyable item catalog for a slot kind.
func SlotItemsFor(kind SlotKind) []string {
	switch kind {
	case SlotWeapon:
		return weaponItems
	case SlotInternal:
		return internalItems
	default:
		return utilityItems
	}
}

func slotItemBasePrice(c *content.Content, itemID string) int {
	sc := c.Slots
	switch itemID {
	case ItemCargo:
		return sc.CargoBasePrice
	case ItemFuelTank:
		return sc.FuelTankBasePrice
	case ItemShield:
		return sc.ShieldBasePrice
	case ItemChaff:
		return sc.ChaffBasePrice
	case ItemTurret:
		return sc.TurretBasePrice
	case ItemSeismic:
		return sc.SeismicBasePrice
	case ItemFuelMiner:
		return sc.FuelMinerBasePrice
	case ItemJammer:
		return sc.JammerBasePrice
	case ItemJumpDrive:
		return sc.JumpDriveBasePrice
	}
	return 0
}

// SlotItemPrice returns the credit cost to buy itemID at grade (0..5):
// base * fleet.GradePriceCurve^grade.
func SlotItemPrice(c *content.Content, itemID string, grade int) int {
	base := slotItemBasePrice(c, itemID)
	return int(math.Round(float64(base) * math.Pow(c.Fleet.GradePriceCurve, float64(grade))))
}

// SlotItemPower returns itemID's power draw at grade. Cargo/Fuel Tank/Jump
// Drive always return 0 — the explicit "never costs power" exception for
// structural (non-electronic) capacity items, and Jump Drive is unpowered
// while locked.
func SlotItemPower(c *content.Content, itemID string, grade int) int {
	g := math.Pow(float64(grade+1), c.Fleet.PowerCurveExponent)
	sc := c.Slots
	switch itemID {
	case ItemShield:
		return int(math.Round(sc.ShieldPowerK * g))
	case ItemChaff:
		return int(math.Round(sc.ChaffPowerK * g))
	case ItemTurret:
		return int(math.Round(sc.TurretPowerK * g))
	case ItemSeismic, ItemFuelMiner, ItemJammer:
		return int(math.Round(sc.InternalPowerK * g))
	default: // ItemCargo, ItemFuelTank, ItemJumpDrive
		return 0
	}
}

// SlotItemMass returns itemID's hull-mass contribution at grade.
func SlotItemMass(c *content.Content, itemID string, grade int) float64 {
	n := float64(grade + 1)
	sc := c.Slots
	switch itemID {
	case ItemCargo:
		return sc.CargoMassPerGrade * n
	case ItemFuelTank:
		return sc.FuelTankMassPerGrade * n
	case ItemShield:
		return sc.ShieldMassPerGrade * n
	case ItemChaff:
		return sc.ChaffMassPerGrade * n
	case ItemTurret:
		return sc.TurretMassPerGrade * n
	case ItemSeismic, ItemFuelMiner, ItemJammer, ItemJumpDrive:
		return sc.InternalMassPerGrade * n
	}
	return 0
}

func deviceSlice(inst *ShipInstance, kind SlotKind) []*SlotDevice {
	switch kind {
	case SlotUtility:
		return inst.Utility
	case SlotWeapon:
		return inst.Weapon
	default:
		return nil
	}
}

// PowerCapacity returns the active ship's power budget from its Power
// Generator grade: power_capacity_base + power_capacity_per_grade*grade.
func PowerCapacity(s *State, c *content.Content) int {
	return PowerCapacityFor(s, c, s.ActiveShipID)
}

// PowerCapacityFor returns shipID's power budget from its own Power
// Generator grade — unlike PowerCapacity, not limited to the active ship,
// so the shipyard can preview a parked hangar ship's budget.
func PowerCapacityFor(s *State, c *content.Content, shipID string) int {
	inst := s.Ships[shipID]
	if inst == nil {
		return c.Fleet.PowerCapacityBase
	}
	return c.Fleet.PowerCapacityBase + c.Fleet.PowerCapacityPerGrade*inst.Grades.PowerGen
}

// InstalledPower returns the sum power draw of every device installed on
// shipID.
func InstalledPower(s *State, c *content.Content, shipID string) int {
	inst := s.Ships[shipID]
	if inst == nil {
		return 0
	}
	total := 0
	for _, d := range inst.Utility {
		if d != nil {
			total += SlotItemPower(c, d.ItemID, d.Grade)
		}
	}
	for _, d := range inst.Weapon {
		if d != nil {
			total += SlotItemPower(c, d.ItemID, d.Grade)
		}
	}
	if inst.Internal != nil {
		total += SlotItemPower(c, inst.Internal.ItemID, inst.Internal.Grade)
	}
	return total
}

// InstalledMass returns shipID's total mass: model base mass plus every
// installed device's mass contribution.
func InstalledMass(s *State, c *content.Content, shipID string) float64 {
	inst := s.Ships[shipID]
	if inst == nil {
		return 0
	}
	model := c.ShipByID(inst.ModelID)
	if model == nil {
		return 0
	}
	total := model.BaseMass
	for _, d := range inst.Utility {
		if d != nil {
			total += SlotItemMass(c, d.ItemID, d.Grade)
		}
	}
	for _, d := range inst.Weapon {
		if d != nil {
			total += SlotItemMass(c, d.ItemID, d.Grade)
		}
	}
	if inst.Internal != nil {
		total += SlotItemMass(c, inst.Internal.ItemID, inst.Internal.Grade)
	}
	return total
}

// massRatio returns the active ship's current mass divided by its unladen
// base mass (always >= 1).
func massRatio(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil {
		return 1
	}
	model := c.ShipByID(inst.ModelID)
	if model == nil || model.BaseMass <= 0 {
		return 1
	}
	return InstalledMass(s, c, s.ActiveShipID) / model.BaseMass
}

// FuelDrainMul is the active ship's overall fuel-burn multiplier: the
// Fuel-Efficiency track multiplier compounded with the mass penalty from
// installed devices.
func FuelDrainMul(s *State, c *content.Content) float64 {
	r := massRatio(s, c)
	return fuelEffMul(s, c) * (1 + c.Fleet.MassFuelCoefficient*(r-1))
}

// EscapeMul is the active ship's overall escape-time multiplier: the
// Thrusters track multiplier compounded with the mass penalty from
// installed devices.
func EscapeMul(s *State, c *content.Content) float64 {
	r := massRatio(s, c)
	return thrusterMul(s, c) * (1 + c.Fleet.MassEscapeCoefficient*(r-1))
}

// FuelCapacity returns the active ship's fuel tank size: the pilot base
// tank (content.Pilot.StartFuel) plus every installed Extra Fuel Tank
// device's bonus.
func FuelCapacity(s *State, c *content.Content) float64 {
	base := float64(c.Pilot.StartFuel)
	inst := ActiveShip(s)
	if inst == nil {
		return base
	}
	for _, d := range inst.Utility {
		if d != nil && d.ItemID == ItemFuelTank {
			base += c.Slots.FuelTankPerGrade * float64(d.Grade+1)
		}
	}
	return base
}

// CargoCapacityUnits returns the active ship's mining-hold capacity in
// asteroid-volume units: the model's base cargo plus every installed Extra
// Cargo device's bonus. Mining an asteroid stops (as if depleted) once
// MinedUnits reaches min(asteroid.Volume, this).
func CargoCapacityUnits(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil {
		return math.MaxFloat64
	}
	model := c.ShipByID(inst.ModelID)
	cap_ := 0.0
	if model != nil {
		cap_ = model.BaseCargo
	}
	for _, d := range inst.Utility {
		if d != nil && d.ItemID == ItemCargo {
			cap_ += c.Slots.CargoPerGrade * float64(d.Grade+1)
		}
	}
	return cap_
}

func findDevice(devices []*SlotDevice, itemID string) *SlotDevice {
	for _, d := range devices {
		if d != nil && d.ItemID == itemID {
			return d
		}
	}
	return nil
}

// ShieldMaxHP returns the active ship's Shield absorption buffer size (0 if
// none is installed).
func ShieldMaxHP(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil {
		return 0
	}
	return ShieldMaxHPFor(s, c, s.ActiveShipID)
}

// ShieldMaxHPFor returns shipID's Shield absorption capacity.
func ShieldMaxHPFor(s *State, c *content.Content, shipID string) float64 {
	inst := s.Ships[shipID]
	if inst == nil {
		return 0
	}
	d := findDevice(inst.Utility, ItemShield)
	if d == nil {
		return 0
	}
	return c.Slots.ShieldHPPerGrade * float64(d.Grade+1)
}

// ShieldStatus returns the active ship's current shield charge, capacity, and
// whether the shield burst and now needs dock service for a full recharge.
func ShieldStatus(s *State, c *content.Content) (hp, maxHP float64, damaged bool) {
	inst := ActiveShip(s)
	if inst == nil {
		return 0, 0, false
	}
	maxHP = ShieldMaxHP(s, c)
	if maxHP <= 0 {
		return 0, 0, false
	}
	return clamp(inst.ShieldHP, 0, maxHP), maxHP, inst.ShieldDamaged
}

// ShieldRechargeSeconds returns how long the active shield takes to recover
// from empty in the belt: 90 seconds at E, 15 seconds at S, linearly between.
func ShieldRechargeSeconds(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil {
		return 0
	}
	d := findDevice(inst.Utility, ItemShield)
	if d == nil {
		return 0
	}
	grade := clampInt(d.Grade, 0, MaxGrade)
	return c.Slots.ShieldRechargeESeconds +
		(c.Slots.ShieldRechargeSSeconds-c.Slots.ShieldRechargeESeconds)*float64(grade)/float64(MaxGrade)
}

// ShieldRechargeETA returns the seconds until the active shield reaches its
// current belt-side cap. A burst shield's cap is its emergency 25% charge;
// docking is required to restore the remaining capacity.
func ShieldRechargeETA(s *State, c *content.Content) float64 {
	hp, maxHP, damaged := ShieldStatus(s, c)
	if s.WorldIdx < 0 || maxHP <= 0 {
		return 0
	}
	cap_ := maxHP
	if damaged {
		cap_ *= c.Slots.ShieldBurstReturnPct
	}
	if hp >= cap_ {
		return 0
	}
	seconds := ShieldRechargeSeconds(s, c)
	if seconds <= 0 {
		return 0
	}
	return (cap_ - hp) * seconds / maxHP
}

// TickBelt advances belt-only systems. Shields recharge only between mining
// runs, never while the ship is drilling or fleeing.
func TickBelt(s *State, c *content.Content, dt float64) {
	if s.WorldIdx < 0 || s.Run != nil || dt <= 0 || math.IsNaN(dt) {
		return
	}
	inst := ActiveShip(s)
	if inst == nil {
		return
	}
	maxHP := ShieldMaxHP(s, c)
	if maxHP <= 0 {
		inst.ShieldHP = 0
		inst.ShieldDamaged = false
		return
	}
	cap_ := maxHP
	if inst.ShieldDamaged {
		cap_ *= c.Slots.ShieldBurstReturnPct
	}
	if inst.ShieldHP >= cap_ {
		return
	}
	seconds := ShieldRechargeSeconds(s, c)
	if seconds <= 0 {
		return
	}
	inst.ShieldHP = math.Min(cap_, inst.ShieldHP+maxHP*dt/seconds)
}

// restoreBurstShieldOnBeltReturn applies the limited emergency recovery for
// a shield that was fully depleted during the just-finished run.
func restoreBurstShieldOnBeltReturn(s *State, c *content.Content) {
	inst := ActiveShip(s)
	if inst == nil || !inst.ShieldDamaged {
		return
	}
	maxHP := ShieldMaxHP(s, c)
	if maxHP <= 0 {
		inst.ShieldHP = 0
		inst.ShieldDamaged = false
		return
	}
	inst.ShieldHP = math.Min(maxHP, maxHP*c.Slots.ShieldBurstReturnPct)
}

// restoreShipShieldFull is the dock/installation service action.
func restoreShipShieldFull(s *State, c *content.Content, shipID string) {
	inst := s.Ships[shipID]
	if inst == nil {
		return
	}
	inst.ShieldHP = ShieldMaxHPFor(s, c, shipID)
	inst.ShieldDamaged = false
}

func normalizeShipShield(s *State, c *content.Content, shipID string) {
	inst := s.Ships[shipID]
	if inst == nil {
		return
	}
	maxHP := ShieldMaxHPFor(s, c, shipID)
	if maxHP <= 0 {
		inst.ShieldHP = 0
		inst.ShieldDamaged = false
		return
	}
	inst.ShieldHP = clamp(inst.ShieldHP, 0, maxHP)
}

// ChaffDurationSeconds returns the active ship's Chaff Launcher suppression
// window (0 if none installed).
func ChaffDurationSeconds(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil {
		return 0
	}
	d := findDevice(inst.Utility, ItemChaff)
	if d == nil {
		return 0
	}
	return c.Slots.ChaffBaseSeconds + float64(d.Grade)
}

// hasChaff reports whether the active ship has a Chaff Launcher installed.
func hasChaff(s *State) bool {
	inst := ActiveShip(s)
	if inst == nil {
		return false
	}
	return findDevice(inst.Utility, ItemChaff) != nil
}

// AttackDamageMul returns the active ship's Defense Turret mitigation
// multiplier applied to incoming attack damage (1 = no mitigation, i.e. no
// turret installed or no weapon slot).
func AttackDamageMul(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil {
		return 1
	}
	d := findDevice(inst.Weapon, ItemTurret)
	if d == nil {
		return 1
	}
	pct := c.Slots.TurretPctPerGrade * float64(d.Grade+1)
	if pct > 1 {
		pct = 1
	}
	return 1 - pct
}

// FuelMinerRefundPct returns the fraction of a mined Rare+ asteroid's
// FuelCost refunded while mining it (0 unless Fuel Miner is the active
// ship's internal module).
func FuelMinerRefundPct(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil || inst.Internal == nil || inst.Internal.ItemID != ItemFuelMiner {
		return 0
	}
	return c.Slots.FuelMinerBasePct + c.Slots.FuelMinerPerGradePct*float64(inst.Internal.Grade)
}

// seismicMinTierGuarantee returns the minimum asteroid tier Seismic Sensors
// guarantees among its free pre-scans, based on the module's grade.
func seismicMinTierGuarantee(grade int) int {
	switch {
	case grade >= 4:
		return 2 // Rare+
	case grade >= 2:
		return 1 // Uncommon+
	default:
		return 0
	}
}

// jammerMaxCharges returns how many asteroids a Pirate Jammer of the given
// grade can suppress before it must be rearmed at dock.
func jammerMaxCharges(c *content.Content, grade int) int {
	step := c.Slots.JammerGradeUsesStep
	if step <= 0 {
		return 1
	}
	return 1 + grade/step
}

// RearmJammer resets the active ship's Pirate Jammer to full charges. Free
// and instant — called whenever the pilot docks (see Dock in belt.go).
func RearmJammer(s *State, c *content.Content) {
	inst := ActiveShip(s)
	if inst == nil || inst.Internal == nil || inst.Internal.ItemID != ItemJammer {
		return
	}
	inst.JammerCharges = jammerMaxCharges(c, inst.Internal.Grade)
}

// deviceSlot resolves the addressable **SlotDevice for kind/index on inst —
// a pointer to the exact map/slice/field slot itself, so Install/Remove can
// share one read-modify-write path instead of a three-way switch each.
// index is ignored for SlotInternal, which has exactly one slot.
func deviceSlot(inst *ShipInstance, kind SlotKind, index int) (**SlotDevice, error) {
	switch kind {
	case SlotUtility:
		if index < 0 || index >= len(inst.Utility) {
			return nil, ErrInvalidSlotIndex
		}
		return &inst.Utility[index], nil
	case SlotWeapon:
		if index < 0 || index >= len(inst.Weapon) {
			return nil, ErrInvalidSlotIndex
		}
		return &inst.Weapon[index], nil
	case SlotInternal:
		return &inst.Internal, nil
	}
	return nil, ErrInvalidSlotItem
}

// InstallSlotDevice buys and installs itemID at grade into shipID's slot
// kind at index (index is ignored for Internal, which has exactly one
// slot). Requires docked, ownership, a valid index, power headroom, and
// affordability; the target slot must be empty. Call StoreSlotDevice or
// SellSlotDevice first when changing a loadout, so a module can never be
// silently destroyed by an install.
func InstallSlotDevice(s *State, c *content.Content, shipID string, kind SlotKind, index int, itemID string, grade int) error {
	if !s.IsDocked() {
		return ErrInBelt
	}
	if SlotItemLocked(itemID) {
		return ErrItemLocked
	}
	if SlotItemKind(itemID) != kind {
		return ErrInvalidSlotItem
	}
	if grade < 0 || grade > MaxGrade {
		return ErrInvalidSlotItem
	}
	inst := s.Ships[shipID]
	if inst == nil {
		return ErrNotOwned
	}
	target, err := deviceSlot(inst, kind, index)
	if err != nil {
		return err
	}
	if *target != nil {
		return ErrSlotOccupied
	}

	price := SlotItemPrice(c, itemID, grade)
	if s.Credits < price {
		return ErrInsufficientFunds
	}
	capacity := PowerCapacityFor(s, c, shipID)
	powerWithout := InstalledPower(s, c, shipID) - devicePower(c, *target)
	if powerWithout+SlotItemPower(c, itemID, grade) > capacity {
		return ErrPowerExceeded
	}

	s.Credits -= price
	s.Stats.CreditsSpent += price
	*target = &SlotDevice{ItemID: itemID, Grade: grade}
	if itemID == ItemShield {
		restoreShipShieldFull(s, c, shipID)
	}
	if itemID == ItemJammer {
		inst.JammerCharges = jammerMaxCharges(c, grade)
	}
	return nil
}

func devicePower(c *content.Content, d *SlotDevice) int {
	if d == nil {
		return 0
	}
	return SlotItemPower(c, d.ItemID, d.Grade)
}

// SlotItemSellValue returns the credit refund for selling itemID at grade
// back to the shipyard: c.Slots.SellValuePct of its current buy price
// (data/balance.toml, default 95%), rounded to the nearest credit.
func SlotItemSellValue(c *content.Content, itemID string, grade int) int {
	return int(math.Round(float64(SlotItemPrice(c, itemID, grade)) * c.Slots.SellValuePct))
}

// StoreSlotDevice removes an installed device from shipID's slot and adds
// it to the pilot's account-wide inventory, freeing its power/mass
// immediately with no credit refund — it can be re-equipped for free later
// via InstallSlotDeviceFromInventory. Docked-only.
func StoreSlotDevice(s *State, c *content.Content, shipID string, kind SlotKind, index int) error {
	_, target, err := resolveOccupiedSlot(s, shipID, kind, index)
	if err != nil {
		return err
	}
	s.Inventory = append(s.Inventory, *target)
	*target = nil
	normalizeShipShield(s, c, shipID)
	return nil
}

// SellSlotDevice removes an installed device from shipID's slot and
// refunds SlotItemSellValue (c.Slots.SellValuePct of its buy price) in
// credits. Docked-only.
func SellSlotDevice(s *State, c *content.Content, shipID string, kind SlotKind, index int) error {
	_, target, err := resolveOccupiedSlot(s, shipID, kind, index)
	if err != nil {
		return err
	}
	s.Credits += SlotItemSellValue(c, (*target).ItemID, (*target).Grade)
	*target = nil
	normalizeShipShield(s, c, shipID)
	return nil
}

// resolveOccupiedSlot is the shared docked/owned/occupied validation for
// StoreSlotDevice and SellSlotDevice.
func resolveOccupiedSlot(s *State, shipID string, kind SlotKind, index int) (*ShipInstance, **SlotDevice, error) {
	if !s.IsDocked() {
		return nil, nil, ErrInBelt
	}
	inst := s.Ships[shipID]
	if inst == nil {
		return nil, nil, ErrNotOwned
	}
	target, err := deviceSlot(inst, kind, index)
	if err != nil {
		return nil, nil, err
	}
	if *target == nil {
		return nil, nil, ErrSlotEmpty
	}
	return inst, target, nil
}

// InstallSlotDeviceFromInventory moves a previously-stored device out of
// the pilot's inventory and into shipID's slot at index, for free (it was
// already paid for). Requires docked, ownership, a valid inventory index
// matching kind, power headroom, and an empty target slot. Call
// StoreSlotDevice or SellSlotDevice first when changing a loadout.
func InstallSlotDeviceFromInventory(s *State, c *content.Content, shipID string, kind SlotKind, index, invIndex int) error {
	if !s.IsDocked() {
		return ErrInBelt
	}
	if invIndex < 0 || invIndex >= len(s.Inventory) || s.Inventory[invIndex] == nil {
		return ErrInvalidInventory
	}
	d := s.Inventory[invIndex]
	if SlotItemKind(d.ItemID) != kind {
		return ErrInvalidSlotItem
	}
	if SlotItemLocked(d.ItemID) {
		return ErrItemLocked
	}
	inst := s.Ships[shipID]
	if inst == nil {
		return ErrNotOwned
	}
	target, err := deviceSlot(inst, kind, index)
	if err != nil {
		return err
	}
	if *target != nil {
		return ErrSlotOccupied
	}

	capacity := PowerCapacityFor(s, c, shipID)
	powerWithout := InstalledPower(s, c, shipID) - devicePower(c, *target)
	if powerWithout+SlotItemPower(c, d.ItemID, d.Grade) > capacity {
		return ErrPowerExceeded
	}

	*target = d
	last := len(s.Inventory) - 1
	copy(s.Inventory[invIndex:], s.Inventory[invIndex+1:])
	s.Inventory[last] = nil // drop the moved pointer so the vacated backing-array slot doesn't keep it alive
	s.Inventory = s.Inventory[:last]
	if d.ItemID == ItemShield {
		restoreShipShieldFull(s, c, shipID)
	}
	if d.ItemID == ItemJammer {
		inst.JammerCharges = jammerMaxCharges(c, d.Grade)
	}
	return nil
}
