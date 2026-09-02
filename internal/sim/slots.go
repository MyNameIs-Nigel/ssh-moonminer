package sim

import (
	"math"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// SlotKind identifies which of a ship's four slot kinds an item belongs in.
// Internal and Jump Drive are each exactly one slot per ship; Utility/Weapon
// counts vary per ship model (Weapon may be zero).
type SlotKind int

const (
	SlotUtility SlotKind = iota
	SlotWeapon
	SlotInternal
	SlotJumpDrive
)

// Slot item IDs — the fixed catalog from
// docs/gameplay/05-fleet-ships-and-shipyard-economy.md. Not content-data
// driven (like TrackName) since each has a distinct effect formula.
const (
	ItemCargo             = "cargo"
	ItemFuelTank          = "fuel_tank"
	ItemShield            = "shield"
	ItemSeismicOvercharge = "seismic_overcharge"
	ItemEMPLauncher       = "emp_launcher"
	ItemTurret            = "turret"
	ItemMissileLauncher   = "missile_launcher"
	// ItemMassDriver is retained as an API alias for callers compiled against
	// pre-1.6.1 code. Stored v6 JSON used the literal "mass_driver", handled
	// by legacyItemMassDriver during migration below.
	ItemMassDriver = ItemMissileLauncher
	ItemPulseLaser = "pulse_laser"
	ItemSeismic    = "seismic_sensors"
	ItemFuelMiner  = "fuel_miner"
	ItemJammer     = "pirate_jammer"
	ItemHeatSink   = "heat_sink"
	ItemJumpDrive  = "jump_drive"
)

const legacyItemMassDriver = "mass_driver"

// SlotItemKind reports which slot kind an item installs into.
func SlotItemKind(itemID string) SlotKind {
	switch itemID {
	case ItemTurret, ItemMissileLauncher, ItemPulseLaser:
		return SlotWeapon
	case ItemSeismic, ItemFuelMiner, ItemJammer, ItemHeatSink:
		return SlotInternal
	case ItemJumpDrive:
		return SlotJumpDrive
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
	case ItemSeismicOvercharge:
		return "SEISMIC OVERCHARGE"
	case ItemEMPLauncher:
		return "EMP LAUNCHER"
	case ItemTurret:
		return "AUTOCANNON TURRET"
	case ItemMissileLauncher:
		return "MISSILE LAUNCHER"
	case ItemPulseLaser:
		return "PULSE LASER"
	case ItemSeismic:
		return "SEISMIC SENSORS"
	case ItemFuelMiner:
		return "FUEL MINER"
	case ItemJammer:
		return "PIRATE JAMMER"
	case ItemHeatSink:
		return "HEAT SINK"
	case ItemJumpDrive:
		return "JUMP DRIVE"
	}
	return itemID
}

// SlotItemDesc returns a one-line summary of what the item does
// (docs/gameplay/05-fleet-ships-and-shipyard-economy.md "Slot device
// catalog"), shown in the Shipyard when the item is selected or highlighted
// in the install picker.
func SlotItemDesc(itemID string) string {
	switch itemID {
	case ItemCargo:
		return "Adds cargo capacity. Draws no power."
	case ItemFuelTank:
		return "Adds fuel capacity. Draws no power."
	case ItemShield:
		return "Absorbs pirate damage before your hull does; recharges between runs."
	case ItemSeismicOvercharge:
		return "Unique. Raises drill speed well above a bare hull's."
	case ItemEMPLauncher:
		return "Auto-deploys on pirate contact, delaying their approach."
	case ItemTurret:
		return "Fires continuously for steady passive damage."
	case ItemMissileLauncher:
		return "G fires one guided, 100% accurate missile per shot."
	case ItemPulseLaser:
		return "F fires every fitted laser as one heat-limited volley."
	case ItemSeismic:
		return "Pre-scans 3 in-range asteroids for free on lock."
	case ItemFuelMiner:
		return "Chance to recover fuel burned while drilling."
	case ItemJammer:
		return "Suppresses pirate approach for a time after lock."
	case ItemHeatSink:
		return "Raises weapon heat capacity before overheat lock."
	case ItemJumpDrive:
		return "Opens Eridani Drift while installed. Draws no power."
	}
	return ""
}

// SlotItemLocked reports whether the item is unavailable for purchase. Every
// current catalog item is usable; the Jump Drive now unlocks Eridani Drift.
func SlotItemLocked(itemID string) bool { return false }

// utilityItems/weaponItems/internalItems list the buyable items per slot
// kind, in catalog display order.
var utilityItems = []string{ItemCargo, ItemFuelTank, ItemShield, ItemSeismicOvercharge, ItemEMPLauncher}
var weaponItems = []string{ItemTurret, ItemMissileLauncher, ItemPulseLaser}
var internalItems = []string{ItemSeismic, ItemFuelMiner, ItemJammer, ItemHeatSink}
var jumpDriveItems = []string{ItemJumpDrive}

// SlotItemsFor returns the buyable item catalog for a slot kind.
func SlotItemsFor(kind SlotKind) []string {
	switch kind {
	case SlotWeapon:
		return weaponItems
	case SlotInternal:
		return internalItems
	case SlotJumpDrive:
		return jumpDriveItems
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
	case ItemSeismicOvercharge:
		return sc.SeismicOverchargeBasePrice
	case ItemEMPLauncher:
		return sc.EMPLauncherBasePrice
	case ItemTurret:
		return sc.TurretBasePrice
	case ItemMissileLauncher:
		return sc.MissileLauncherBasePrice
	case ItemPulseLaser:
		return sc.PulseLaserBasePrice
	case ItemSeismic:
		return sc.SeismicBasePrice
	case ItemFuelMiner:
		return sc.FuelMinerBasePrice
	case ItemJammer:
		return sc.JammerBasePrice
	case ItemHeatSink:
		return sc.HeatSinkBasePrice
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

// SlotItemPower returns itemID's power draw at grade. Cargo, Fuel Tank, and
// Jump Drive always return 0 — the explicit structural/unpowered exceptions.
func SlotItemPower(c *content.Content, itemID string, grade int) int {
	g := math.Pow(float64(grade+1), c.Fleet.PowerCurveExponent)
	sc := c.Slots
	switch itemID {
	case ItemShield:
		return int(math.Round(sc.ShieldPowerK * g))
	case ItemSeismicOvercharge:
		return int(math.Round(sc.SeismicOverchargePowerK * g))
	case ItemEMPLauncher:
		return int(math.Round(sc.EMPLauncherPowerK * g))
	case ItemTurret:
		return int(math.Round(sc.TurretPowerK * g))
	case ItemMissileLauncher:
		return int(math.Round(sc.MissileLauncherPowerK * g))
	case ItemPulseLaser:
		return int(math.Round(sc.PulseLaserPowerK * g))
	case ItemSeismic, ItemFuelMiner, ItemJammer, ItemHeatSink:
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
	case ItemSeismicOvercharge:
		return sc.SeismicOverchargeMassPerGrade * n
	case ItemEMPLauncher:
		return sc.EMPLauncherMassPerGrade * n
	case ItemTurret:
		return sc.TurretMassPerGrade * n
	case ItemMissileLauncher:
		return sc.MissileLauncherMassPerGrade * n
	case ItemPulseLaser:
		return sc.PulseLaserMassPerGrade * n
	case ItemSeismic, ItemFuelMiner, ItemJammer, ItemHeatSink, ItemJumpDrive:
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
	if inst.JumpDrive != nil {
		total += SlotItemPower(c, inst.JumpDrive.ItemID, inst.JumpDrive.Grade)
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
	if inst.JumpDrive != nil {
		total += SlotItemMass(c, inst.JumpDrive.ItemID, inst.JumpDrive.Grade)
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
	return fuelCapacityFor(s, c, s.ActiveShipID)
}

// CargoCapacityUnits returns the active ship's mining-hold capacity in
// asteroid-volume units: the model's base cargo plus every installed Extra
// Cargo device's bonus. Mining stops when the held current-run cargo plus
// already-boarded cargo reaches this physical capacity.
func CargoCapacityUnits(s *State, c *content.Content) float64 {
	return CargoCapacityUnitsFor(s, c, s.ActiveShipID)
}

// CargoCapacityUnitsFor returns the hold capacity of any owned ship. It is
// used before docked fleet/loadout actions so cargo can never be left in a
// ship that cannot physically contain it.
func CargoCapacityUnitsFor(s *State, c *content.Content, shipID string) float64 {
	inst := s.Ships[shipID]
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

func hasOtherDevice(inst *ShipInstance, kind SlotKind, exclude int, itemID string) bool {
	if inst == nil {
		return false
	}
	devices := deviceSlice(inst, kind)
	for i, d := range devices {
		if i != exclude && d != nil && d.ItemID == itemID {
			return true
		}
	}
	return false
}

// SlotItemIsUnique declares categories that may be installed only once per
// ship. Extra cargo/fuel and EMP launchers deliberately stack; weapons stack
// their damage/heat additively in FireWeapons.
func SlotItemIsUnique(itemID string) bool {
	return itemID == ItemShield || itemID == ItemSeismicOvercharge
}

func validateUniqueSlotItem(inst *ShipInstance, kind SlotKind, index int, itemID string) error {
	if SlotItemIsUnique(itemID) && hasOtherDevice(inst, kind, index, itemID) {
		return ErrDuplicateItem
	}
	return nil
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

// EMPDeploySeconds returns how long an EMP Launcher delays pirates after
// they arrive. It scales linearly from E through S.
func EMPDeploySeconds(c *content.Content, grade int) float64 {
	grade = clampInt(grade, 0, MaxGrade)
	return c.Slots.EMPLauncherDeployESeconds +
		(c.Slots.EMPLauncherDeploySSeconds-c.Slots.EMPLauncherDeployESeconds)*float64(grade)/float64(MaxGrade)
}

// deployEMPLauncher arms the active run with the highest-grade loaded EMP
// Launcher. Ties retain utility-slot order. Each launcher is spent until the
// ship returns to dock, and a run may deploy at most one of them.
func deployEMPLauncher(s *State, c *content.Content, run *ActiveRun, now int64) bool {
	if run.EMPDeployed {
		return false
	}
	inst := ActiveShip(s)
	if inst == nil {
		return false
	}
	var selected *SlotDevice
	for _, d := range inst.Utility {
		if d != nil && d.ItemID == ItemEMPLauncher && d.EMPArmed &&
			(selected == nil || d.Grade > selected.Grade) {
			selected = d
		}
	}
	if selected == nil {
		return false
	}
	selected.EMPArmed = false
	run.EMPDeployed = true
	run.EMPActive = true
	run.EMPRemaining = EMPDeploySeconds(c, selected.Grade)
	run.EventLog = append(run.EventLog, RunEventRecord{Kind: "emp_deployed", At: now})
	return true
}

// RearmEMPLaunchers loads every installed EMP Launcher on the active ship.
// Docking, activating a parked ship, and installing one all provide this
// free service.
func RearmEMPLaunchers(s *State) {
	inst := ActiveShip(s)
	if inst == nil {
		return
	}
	for _, d := range inst.Utility {
		if d != nil && d.ItemID == ItemEMPLauncher {
			d.EMPArmed = true
		}
	}
}

// HasWeapon reports whether the active ship has at least one weapon
// installed — gates [F] FIGHT at the tribute prompt.
func HasWeapon(s *State) bool {
	inst := ActiveShip(s)
	if inst == nil {
		return false
	}
	for _, d := range inst.Weapon {
		if d != nil {
			return true
		}
	}
	return false
}

// HasManualWeapon reports whether the active ship has a weapon the pilot can
// fire. Autocannon turrets fire continuously and are therefore excluded.
func HasManualWeapon(s *State) bool {
	return HasPulseLaser(s)
}

// HasPulseLaser reports whether the active ship can fire the F-key pulse
// volley. Missile launchers use their own G-key action and cooldown.
func HasPulseLaser(s *State) bool {
	inst := ActiveShip(s)
	if inst == nil {
		return false
	}
	for _, d := range inst.Weapon {
		if d != nil && d.ItemID == ItemPulseLaser {
			return true
		}
	}
	return false
}

// HasMissileLauncher reports whether the active ship has at least one
// fitted launcher, whether or not it currently has ammunition remaining.
func HasMissileLauncher(s *State) bool {
	inst := ActiveShip(s)
	return inst != nil && findDevice(inst.Weapon, ItemMissileLauncher) != nil
}

// WeaponDamagePerShot returns itemID's damage dealt per volley at grade
// (0..5). Zero for non-weapon items.
func WeaponDamagePerShot(c *content.Content, itemID string, grade int) float64 {
	sc := c.Slots
	n := float64(grade)
	switch itemID {
	case ItemTurret:
		return sc.TurretDamagePerShotBase + sc.TurretDamagePerShotPerGrade*n
	case ItemMissileLauncher:
		return sc.MissileDamagePerShotBase + sc.MissileDamagePerShotPerGrade*n
	case ItemPulseLaser:
		return sc.PulseLaserDamagePerShotBase + sc.PulseLaserDamagePerShotPerGrade*n
	}
	return 0
}

// WeaponHeatPerShot returns itemID's heat-capacitor cost per manual volley.
// Autocannon turrets never use the shared heat capacitor.
func WeaponHeatPerShot(c *content.Content, itemID string, _ int) float64 {
	switch itemID {
	case ItemPulseLaser:
		return c.Slots.PulseLaserHeatPerShot
	}
	return 0
}

// WeaponSolutionBonus returns itemID's flat addition to the manual firing
// solution (e.g. the Pulse Laser's accuracy edge).
func WeaponSolutionBonus(c *content.Content, itemID string, _ int) float64 {
	switch itemID {
	case ItemPulseLaser:
		return c.Slots.PulseLaserSolutionBonus
	}
	return 0
}

// InstalledWeaponVolley sums the active ship's installed weapons' damage,
// heat cost, and solution bonus. It is kept for loadout previews; combat
// firing uses InstalledManualWeaponVolley so autocannons are not double-fired.
func InstalledWeaponVolley(s *State, c *content.Content) (damage, heat, solutionBonus float64) {
	inst := ActiveShip(s)
	if inst == nil {
		return 0, 0, 0
	}
	for _, d := range inst.Weapon {
		if d == nil {
			continue
		}
		damage += WeaponDamagePerShot(c, d.ItemID, d.Grade)
		heat += WeaponHeatPerShot(c, d.ItemID, d.Grade)
		solutionBonus += WeaponSolutionBonus(c, d.ItemID, d.Grade)
	}
	return damage, heat, solutionBonus
}

// InstalledManualWeaponVolley sums the Pulse Lasers that fire when the pilot
// presses F. Missile launchers are always fired one missile at a time via G.
func InstalledManualWeaponVolley(s *State, c *content.Content) (damage, heat, solutionBonus float64) {
	inst := ActiveShip(s)
	if inst == nil {
		return 0, 0, 0
	}
	for _, d := range inst.Weapon {
		if d == nil || d.ItemID != ItemPulseLaser {
			continue
		}
		damage += WeaponDamagePerShot(c, d.ItemID, d.Grade)
		heat += WeaponHeatPerShot(c, d.ItemID, d.Grade)
		solutionBonus += WeaponSolutionBonus(c, d.ItemID, d.Grade)
	}
	return damage, heat, solutionBonus
}

// MissileCapacity returns the loaded missile count for one launcher grade.
// Content supplies one explicit E..S entry so balance changes never rely on
// accidental rounding.
func MissileCapacity(c *content.Content, grade int) int {
	if len(c.Slots.MissileCapacityByGrade) == 0 {
		return 0
	}
	grade = clampInt(grade, 0, MaxGrade)
	if grade >= len(c.Slots.MissileCapacityByGrade) {
		grade = len(c.Slots.MissileCapacityByGrade) - 1
	}
	return max(0, c.Slots.MissileCapacityByGrade[grade])
}

// MissileAmmo returns the total loaded ammunition across the active ship's
// missile launchers. A ship may fit a launcher alongside pulse lasers.
func MissileAmmo(s *State) int {
	inst := ActiveShip(s)
	if inst == nil {
		return 0
	}
	total := 0
	for _, d := range inst.Weapon {
		if d != nil && d.ItemID == ItemMissileLauncher {
			total += max(0, d.Missiles)
		}
	}
	return total
}

// MissileCapacityFor returns the total fully-loaded ammunition capacity for
// one owned ship.
func MissileCapacityFor(s *State, c *content.Content, shipID string) int {
	inst := s.Ships[shipID]
	if inst == nil {
		return 0
	}
	total := 0
	for _, d := range inst.Weapon {
		if d != nil && d.ItemID == ItemMissileLauncher {
			total += MissileCapacity(c, d.Grade)
		}
	}
	return total
}

// normalizeShipMissiles keeps persisted ammunition valid after a launcher is
// stored, sold, replaced, or loaded from an older save.
func normalizeShipMissiles(s *State, c *content.Content, shipID string) {
	inst := s.Ships[shipID]
	if inst == nil {
		return
	}
	for _, d := range inst.Weapon {
		if d == nil || d.ItemID != ItemMissileLauncher {
			continue
		}
		d.Missiles = clampInt(d.Missiles, 0, MissileCapacity(c, d.Grade))
	}
}

// RearmMissilesForShip services every fitted launcher on one ship. It is
// intentionally called only by dock service and a new launcher installation.
func RearmMissilesForShip(s *State, c *content.Content, shipID string) {
	inst := s.Ships[shipID]
	if inst == nil {
		return
	}
	for _, d := range inst.Weapon {
		if d != nil && d.ItemID == ItemMissileLauncher {
			d.Missiles = MissileCapacity(c, d.Grade)
		}
	}
}

// RearmMissiles services the active ship at dock, matching shield and EMP
// servicing semantics.
func RearmMissiles(s *State, c *content.Content) {
	RearmMissilesForShip(s, c, s.ActiveShipID)
}

// nextLoadedMissileLauncher selects the highest-grade loaded launcher. Ties
// use fitted-slot order, making each G press deterministic without RNG.
func nextLoadedMissileLauncher(s *State) *SlotDevice {
	inst := ActiveShip(s)
	if inst == nil {
		return nil
	}
	var selected *SlotDevice
	for _, d := range inst.Weapon {
		if d != nil && d.ItemID == ItemMissileLauncher && d.Missiles > 0 &&
			(selected == nil || d.Grade > selected.Grade) {
			selected = d
		}
	}
	return selected
}

// AutocannonDamagePerSecond sums the always-on damage dealt by installed
// autocannon turrets. It has no firing solution, heat, or RNG component.
func AutocannonDamagePerSecond(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil || c.Slots.TurretShotsPerSecond <= 0 {
		return 0
	}
	dps := 0.0
	for _, d := range inst.Weapon {
		if d != nil && d.ItemID == ItemTurret {
			dps += WeaponDamagePerShot(c, d.ItemID, d.Grade) * c.Slots.TurretShotsPerSecond
		}
	}
	return dps
}

// HeatCapacity returns the active ship's shared manual-weapon heat capacitor.
// A Heat Sink in the single internal slot adds capacity by module grade.
func HeatCapacity(s *State, c *content.Content) float64 {
	capacity := c.Combat.HeatCapacity
	inst := ActiveShip(s)
	if inst != nil && inst.Internal != nil && inst.Internal.ItemID == ItemHeatSink {
		capacity += c.Slots.HeatSinkCapacityPerGrade * float64(inst.Internal.Grade+1)
	}
	return capacity
}

// FuelMinerRecoveryMul returns how much of the fuel a working drill burns is
// recovered from a fuel asteroid. E returns exactly 1× the actual burn; each
// higher grade adds a net gain. It is zero unless the active ship has a Fuel
// Miner in its internal slot.
func FuelMinerRecoveryMul(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil || inst.Internal == nil || inst.Internal.ItemID != ItemFuelMiner {
		return 0
	}
	return c.Slots.FuelMinerERecoveryMul + c.Slots.FuelMinerPerGradeGainMul*float64(inst.Internal.Grade)
}

// MiningSpeedMul returns the drilling multiplier from a single equipped
// Seismic Overcharge. Its mass and mass-driver-class power requirement are
// intentionally handled by the normal slot systems, so the speed comes with
// fuel and loadout tradeoffs rather than a separate runtime cost.
func MiningSpeedMul(s *State, c *content.Content) float64 {
	inst := ActiveShip(s)
	if inst == nil {
		return 1
	}
	d := findDevice(inst.Utility, ItemSeismicOvercharge)
	if d == nil {
		return 1
	}
	return c.Slots.SeismicOverchargeSpeedE +
		c.Slots.SeismicOverchargeSpeedPerGrade*float64(clampInt(d.Grade, 0, MaxGrade))
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

// JammerDurationSeconds returns a grade's authored pirate-approach
// suppression duration. The explicit per-grade table supports the large S
// grade bump without distorting the lower grades.
func JammerDurationSeconds(c *content.Content, grade int) float64 {
	if len(c.Slots.JammerDurationSeconds) == 0 {
		return 0
	}
	grade = clampInt(grade, 0, MaxGrade)
	if grade >= len(c.Slots.JammerDurationSeconds) {
		grade = len(c.Slots.JammerDurationSeconds) - 1
	}
	return c.Slots.JammerDurationSeconds[grade]
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
// index is ignored for SlotInternal and SlotJumpDrive, which each have one
// dedicated slot.
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
	case SlotJumpDrive:
		return &inst.JumpDrive, nil
	}
	return nil, ErrInvalidSlotItem
}

// InstallSlotDevice buys and installs itemID at grade into shipID's slot
// kind at index (index is ignored for Internal/Jump Drive, which each have
// one slot). Requires docked, ownership, a valid index, power headroom, and
// affordability; the target slot must be empty. Call StoreSlotDevice or
// SellSlotDevice first when changing a loadout, so a module can never be
// silently destroyed by an install.
func InstallSlotDevice(s *State, c *content.Content, shipID string, kind SlotKind, index int, itemID string, grade int) error {
	syncActiveConditionFromLegacy(s, c)
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
	if err := validateUniqueSlotItem(inst, kind, index, itemID); err != nil {
		return err
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
	finishDeviceInstallation(s, c, shipID, *target)
	if shipID == s.ActiveShipID {
		syncActiveConditionMirror(s, c)
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
	syncActiveConditionFromLegacy(s, c)
	_, target, err := resolveOccupiedSlot(s, shipID, kind, index)
	if err != nil {
		return err
	}
	if err := validateDeviceRemovalCargo(s, c, shipID, *target); err != nil {
		return err
	}
	if err := validateDeviceRemovalRouteKey(s, c, shipID, *target, nil); err != nil {
		return err
	}
	s.Inventory = append(s.Inventory, *target)
	*target = nil
	normalizeShipShield(s, c, shipID)
	normalizeShipMissiles(s, c, shipID)
	if shipID == s.ActiveShipID {
		syncActiveConditionMirror(s, c)
	}
	return nil
}

// SellSlotDevice removes an installed device from shipID's slot and
// refunds SlotItemSellValue (c.Slots.SellValuePct of its buy price) in
// credits. Docked-only.
func SellSlotDevice(s *State, c *content.Content, shipID string, kind SlotKind, index int) error {
	syncActiveConditionFromLegacy(s, c)
	_, target, err := resolveOccupiedSlot(s, shipID, kind, index)
	if err != nil {
		return err
	}
	if err := validateDeviceRemovalCargo(s, c, shipID, *target); err != nil {
		return err
	}
	if err := validateDeviceRemovalRouteKey(s, c, shipID, *target, nil); err != nil {
		return err
	}
	s.Credits += SlotItemSellValue(c, (*target).ItemID, (*target).Grade)
	*target = nil
	normalizeShipShield(s, c, shipID)
	normalizeShipMissiles(s, c, shipID)
	if shipID == s.ActiveShipID {
		syncActiveConditionMirror(s, c)
	}
	return nil
}

func validateDeviceRemovalCargo(s *State, c *content.Content, shipID string, d *SlotDevice) error {
	if shipID != s.ActiveShipID {
		return nil
	}
	capacityAfterRemoval := cargoCapacityAfterChange(s, c, shipID, d, nil)
	if s.CargoUnits > capacityAfterRemoval {
		return ErrCargoDoesNotFit
	}
	return nil
}

func validateDeviceRemovalRouteKey(s *State, c *content.Content, shipID string, old, replacement *SlotDevice) error {
	if shipID != s.ActiveShipID || old == nil {
		return nil
	}
	system := c.SystemByID(s.SystemID)
	if system == nil || system.RequiredItemID == "" || old.ItemID != system.RequiredItemID {
		return nil
	}
	if replacement != nil && replacement.ItemID == system.RequiredItemID {
		return nil
	}
	return ErrRouteKeyRequired
}

// cargoCapacityAfterChange calculates a ship's capacity with old removed and
// replacement installed. It is side-effect free so rejected shipyard actions
// leave the pilot's physical cargo state untouched.
func cargoCapacityAfterChange(s *State, c *content.Content, shipID string, old, replacement *SlotDevice) float64 {
	capacity := CargoCapacityUnitsFor(s, c, shipID)
	if old != nil && old.ItemID == ItemCargo {
		capacity -= c.Slots.CargoPerGrade * float64(old.Grade+1)
	}
	if replacement != nil && replacement.ItemID == ItemCargo {
		capacity += c.Slots.CargoPerGrade * float64(replacement.Grade+1)
	}
	return capacity
}

func validateCargoAfterReplacement(s *State, c *content.Content, shipID string, old, replacement *SlotDevice) error {
	if shipID == s.ActiveShipID && s.CargoUnits > cargoCapacityAfterChange(s, c, shipID, old, replacement) {
		return ErrCargoDoesNotFit
	}
	return nil
}

func finishDeviceInstallation(s *State, c *content.Content, shipID string, d *SlotDevice) {
	if d == nil {
		return
	}
	inst := s.Ships[shipID]
	switch d.ItemID {
	case ItemShield:
		restoreShipShieldFull(s, c, shipID)
	case ItemJammer:
		if inst != nil {
			inst.JammerCharges = jammerMaxCharges(c, d.Grade)
		}
	case ItemEMPLauncher:
		d.EMPArmed = true
	case ItemMissileLauncher:
		RearmMissilesForShip(s, c, shipID)
	}
}

// ReplaceSlotDeviceWithPurchase atomically sells the occupied target slot and
// buys a catalog replacement. The old module's refund is available to this
// transaction, but all validation happens before any credits or loadout state
// change.
func ReplaceSlotDeviceWithPurchase(s *State, c *content.Content, shipID string, kind SlotKind, index int, itemID string, grade int) error {
	syncActiveConditionFromLegacy(s, c)
	if !s.IsDocked() {
		return ErrInBelt
	}
	if SlotItemLocked(itemID) || SlotItemKind(itemID) != kind || grade < 0 || grade > MaxGrade {
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
	if *target == nil {
		return ErrSlotEmpty
	}
	old := *target
	if err := validateUniqueSlotItem(inst, kind, index, itemID); err != nil {
		return err
	}
	replacement := &SlotDevice{ItemID: itemID, Grade: grade}
	if err := validateCargoAfterReplacement(s, c, shipID, old, replacement); err != nil {
		return err
	}
	if err := validateDeviceRemovalRouteKey(s, c, shipID, old, replacement); err != nil {
		return err
	}
	powerWithout := InstalledPower(s, c, shipID) - devicePower(c, old)
	if powerWithout+SlotItemPower(c, itemID, grade) > PowerCapacityFor(s, c, shipID) {
		return ErrPowerExceeded
	}
	price := SlotItemPrice(c, itemID, grade)
	refund := SlotItemSellValue(c, old.ItemID, old.Grade)
	if s.Credits+refund < price {
		return ErrInsufficientFunds
	}
	s.Credits += refund - price
	s.Stats.CreditsSpent += price
	*target = replacement
	finishDeviceInstallation(s, c, shipID, replacement)
	normalizeShipShield(s, c, shipID)
	normalizeShipMissiles(s, c, shipID)
	if shipID == s.ActiveShipID {
		syncActiveConditionMirror(s, c)
	}
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
	syncActiveConditionFromLegacy(s, c)
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
	if err := validateUniqueSlotItem(inst, kind, index, d.ItemID); err != nil {
		return err
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
	finishDeviceInstallation(s, c, shipID, d)
	if shipID == s.ActiveShipID {
		syncActiveConditionMirror(s, c)
	}
	return nil
}
