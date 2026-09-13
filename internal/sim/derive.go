package sim

import (
	"fmt"
	"math"
	"strings"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// TankSize returns the active ship's effective fuel tank capacity.
func TankSize(s *State, c *content.Content) float64 {
	return FuelCapacity(s, c)
}

// RefuelCost returns credits to fill the tank. Fuel is consumed in fractional
// units during a run but sold in whole points, so any non-zero shortfall needs
// one final point rather than being rounded down to a free, unusable refuel.
func RefuelCost(s *State, c *content.Content) int {
	return refuelPointsNeeded(TankSize(s, c), FuelAmount(s, c)) * c.Port.RefuelPerPoint
}

const fuelRoundingEpsilon = 0.000001

func refuelPointsNeeded(tank, fuel float64) int {
	missing := tank - fuel
	if missing <= fuelRoundingEpsilon {
		return 0
	}
	return int(math.Ceil(missing - fuelRoundingEpsilon))
}

// RepairCost returns credits to repair hull to the active ship's max.
func RepairCost(s *State, c *content.Content) int {
	max := MaxHull(s, c)
	hull := ShipHull(s, s.ActiveShipID)
	if hull >= max {
		return 0
	}
	missing := max - hull
	rate := float64(c.Port.RepairPerPoint)
	if hull == 0 {
		rate *= c.Port.DrydockSurchargeMul
	}
	return int(math.Round(float64(missing) * rate))
}

// CanDepart reports whether the pilot can afford travel to worldIdx.
func CanDepart(s *State, c *content.Content, worldIdx int) bool {
	if ShipHull(s, s.ActiveShipID) <= 0 || worldIdx < 0 || worldIdx >= len(c.Worlds) {
		return false
	}
	return RemainingCargoCapacity(s, c) > 0 &&
		RouteLockReason(s, c, worldIdx) == "" && FuelAmount(s, c) >= float64(c.Worlds[worldIdx].TravelFuel)
}

// RemainingCargoCapacity reports the unused volume in the active ship's
// physical hold. Cargo carried between runs already occupies that hold;
// in-progress run cargo is accounted for by MiningRunCapacity.
func RemainingCargoCapacity(s *State, c *content.Content) float64 {
	return math.Max(0, CargoCapacityUnits(s, c)-s.CargoUnits)
}

// MiningRunCapacity is the maximum volume that the current run can extract
// before either the asteroid is exhausted or the active hold is full. Keeping
// it here makes the simulation and the mining HUD use one capacity rule.
func MiningRunCapacity(s *State, c *content.Content, ast *Asteroid) float64 {
	if ast == nil {
		return 0
	}
	return math.Min(float64(ast.Volume), RemainingCargoCapacity(s, c))
}

// RunMiningCapacity is retained for callers that need to know how much this
// run can load before its hold fills. It is deliberately not the resource HUD
// denominator: a full hold must still show the asteroid's real ore remaining.
func RunMiningCapacity(s *State, c *content.Content, ast *Asteroid, run *ActiveRun) float64 {
	if ast == nil {
		return 0
	}
	extracted := RunExtractedUnits(run)
	held := RunHeldUnits(run)
	free := math.Max(0, RemainingCargoCapacity(s, c)-held)
	return math.Min(float64(ast.Volume), extracted+free)
}

// RunHeldUnits returns current-run cargo still aboard. It understands the
// short-lived pre-accounting MinedUnits field so rendering fixtures and dev
// tools remain readable while all real simulation paths use HeldUnits.
func RunHeldUnits(run *ActiveRun) float64 {
	if run == nil {
		return 0
	}
	if run.HeldUnits == 0 && run.ExtractedUnits == 0 && run.MinedUnits > 0 {
		return run.MinedUnits
	}
	return run.HeldUnits
}

// RunExtractedUnits returns volume removed from the asteroid.
func RunExtractedUnits(run *ActiveRun) float64 {
	if run == nil {
		return 0
	}
	if run.ExtractedUnits == 0 && run.HeldUnits == 0 && run.MinedUnits > 0 {
		return run.MinedUnits
	}
	return run.ExtractedUnits
}

// RouteLockReason returns player-facing route gate text. An empty string
// means the destination is unlocked and compatible with the active ship.
func RouteLockReason(s *State, c *content.Content, worldIdx int) string {
	w := c.WorldByIndex(worldIdx)
	if w == nil {
		return "INVALID DESTINATION"
	}
	system := c.SystemByID(w.SystemID)
	if system == nil {
		return "INVALID SYSTEM"
	}
	source := c.SystemByID(s.SystemID)
	if source == nil {
		return "INVALID CURRENT SYSTEM"
	}
	if source.ID != system.ID {
		return "NOT IN SYSTEM — JUMP TO " + system.Name + " FIRST"
	}
	if !w.StartsUnlocked && !s.DestinationPermits[w.ID] {
		return fmt.Sprintf("BUY NAV PERMIT %d cr", w.PermitFee)
	}
	if w.RequiredShipClass != "" {
		model := c.ShipByID(s.ActiveShipID)
		if model == nil || model.Class != w.RequiredShipClass {
			return "NEED " + strings.ToUpper(w.RequiredShipClass) + "-CLASS SHIP"
		}
	}
	if FuelCapacity(s, c) < w.RequiredFuelCapacity {
		return fmt.Sprintf("NEED FUEL CAPACITY %.0f", w.RequiredFuelCapacity)
	}
	return ""
}

func systemsLinked(system *content.System, destinationID string) bool {
	if system == nil {
		return false
	}
	for _, id := range system.Links {
		if id == destinationID {
			return true
		}
	}
	return false
}

// HasActiveSlotItem reports whether the active ship has the given device
// installed in any of its slots.
func HasActiveSlotItem(s *State, itemID string) bool {
	return ShipHasSlotItem(s, s.ActiveShipID, itemID)
}

// ShipHasSlotItem reports whether a named owned hull has a device installed.
func ShipHasSlotItem(s *State, shipID, itemID string) bool {
	inst := s.Ships[shipID]
	if inst == nil {
		return false
	}
	for _, d := range inst.Utility {
		if d != nil && d.ItemID == itemID {
			return true
		}
	}
	for _, d := range inst.Weapon {
		if d != nil && d.ItemID == itemID {
			return true
		}
	}
	return internalDevice(inst, itemID) != nil
}

// InsuranceEligible reports softlock protection availability.
func InsuranceEligible(s *State, c *content.Content) bool {
	if !s.IsDocked() || s.Settings.InsuranceUsed {
		return false
	}
	// The advance is a softlock escape hatch, not a fleet subsidy. A pilot
	// must be on the lone starter hull, carrying no sellable cargo, unable to
	// fuel even the cheapest trip, and below the published recovery threshold.
	if s.ActiveShipID != content.StarterShipID || localShipCount(s) != 1 || !OwnsShip(s, content.StarterShipID) {
		return false
	}
	if s.CargoUnits > 0 || s.CargoValue > 0 {
		return false
	}
	return s.Credits < c.Port.InsuranceCreditThreshold &&
		FuelAmount(s, c) < float64(localMinTravelFuel(s, c)) &&
		int(FuelAmount(s, c)) < max(c.Port.InsuranceFuelThreshold, localMinTravelFuel(s, c))
}

// MinTravelFuel returns cheapest world travel cost.
func MinTravelFuel(c *content.Content) int {
	min := c.Worlds[0].TravelFuel
	for _, w := range c.Worlds[1:] {
		if w.TravelFuel < min {
			min = w.TravelFuel
		}
	}
	return min
}
