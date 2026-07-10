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
	return RouteLockReason(s, c, worldIdx) == "" && s.Fuel >= float64(c.Worlds[worldIdx].TravelFuel)
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
	if !system.StartsUnlocked && !s.SystemPermits[system.ID] {
		return fmt.Sprintf("BUY TRANSFER %d cr", system.TransferFee)
	}
	if !w.StartsUnlocked && !s.DestinationPermits[w.ID] {
		return fmt.Sprintf("BUY NAV PERMIT %d cr", w.PermitFee)
	}
	if FuelCapacity(s, c) < w.RequiredFuelCapacity {
		return fmt.Sprintf("NEED FUEL CAPACITY %.0f", w.RequiredFuelCapacity)
	}
	return ""
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
