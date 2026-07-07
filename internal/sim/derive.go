package sim

import (
	"math"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// TankSize returns effective fuel tank capacity.
func TankSize(s *State, c *content.Content) float64 {
	return float64(c.Upgrades.BaseTank + s.Upgrades.Tank*c.Upgrades.TankBonus)
}

// RefuelCost returns credits to fill the tank.
func RefuelCost(s *State, c *content.Content) int {
	missing := TankSize(s, c) - s.Fuel
	if missing <= 0 {
		return 0
	}
	return int(math.Round(missing * float64(c.Port.RefuelPerPoint)))
}

// RepairCost returns credits to repair hull to 100.
func RepairCost(s *State, c *content.Content) int {
	if s.Hull >= 100 {
		return 0
	}
	missing := 100 - s.Hull
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
	return s.Fuel >= float64(c.Worlds[worldIdx].TravelFuel)
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
