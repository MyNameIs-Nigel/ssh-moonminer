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

// RefuelCost returns credits to fill the tank.
func RefuelCost(s *State, c *content.Content) int {
	missing := TankSize(s, c) - s.Fuel
	if missing <= 0 {
		return 0
	}
	return int(math.Round(missing * float64(c.Port.RefuelPerPoint)))
}

// RepairCost returns credits to repair hull to the active ship's max.
func RepairCost(s *State, c *content.Content) int {
	max := MaxHull(s, c)
	if s.Hull >= max {
		return 0
	}
	missing := max - s.Hull
	rate := float64(c.Port.RepairPerPoint)
	if s.Hull == 0 {
		rate *= c.Port.DrydockSurchargeMul
	}
	return int(math.Round(float64(missing) * rate))
}

// CanDepart reports whether the pilot can afford travel to worldIdx.
func CanDepart(s *State, c *content.Content, worldIdx int) bool {
	if s.Hull <= 0 || worldIdx < 0 || worldIdx >= len(c.Worlds) {
		return false
	}
	return RemainingCargoCapacity(s, c) > 0 &&
		RouteLockReason(s, c, worldIdx) == "" && s.Fuel >= float64(c.Worlds[worldIdx].TravelFuel)
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

// RunMiningCapacity is the resource cap currently shown for an active run.
// It includes what the run has already extracted plus the hold space it can
// still occupy, so the resource meter reaches zero exactly with RunDepleted.
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
	if system.RequiredShipClass != "" {
		model := c.ShipByID(s.ActiveShipID)
		if model == nil || model.Class != system.RequiredShipClass {
			return "NEED " + strings.ToUpper(system.RequiredShipClass) + "-CLASS SHIP"
		}
	}
	if system.RequiredItemID != "" && !HasActiveSlotItem(s, system.RequiredItemID) {
		return "NEED " + strings.ToUpper(strings.ReplaceAll(system.RequiredItemID, "_", " "))
	}
	if !system.StartsUnlocked && !s.SystemPermits[system.ID] {
		return fmt.Sprintf("BUY TRANSFER %d cr", system.TransferFee)
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

// HasActiveSlotItem reports whether the active ship has the given device
// installed in any of its slots.
func HasActiveSlotItem(s *State, itemID string) bool {
	inst := ActiveShip(s)
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
	return inst.Internal != nil && inst.Internal.ItemID == itemID
}

// InsuranceEligible reports softlock protection availability.
func InsuranceEligible(s *State, c *content.Content) bool {
	if !s.IsDocked() || s.Settings.InsuranceUsed {
		return false
	}
	return s.Credits < c.Port.InsuranceCreditThreshold &&
		int(s.Fuel) < c.Port.InsuranceFuelThreshold
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
