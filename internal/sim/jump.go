package sim

import (
	"fmt"
	"math"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

type JumpResult struct {
	OriginID     string  `json:"origin_id"`
	SystemID     string  `json:"system_id"`
	FuelCost     float64 `json:"fuel_cost"`
	FuelAfter    float64 `json:"fuel_after"`
	HullAfter    int     `json:"hull_after"`
	Stress       int     `json:"stress"`
	DriftChance  float64 `json:"drift_chance"`
	Drift        string  `json:"drift,omitempty"`
	FirstArrival bool    `json:"first_arrival"`
	LowHull      bool    `json:"low_hull"`
}

func JumpFuel(s *State, c *content.Content, shipID string, g content.Gate) float64 {
	return g.JumpFuelBase * math.Pow(InstalledMass(s, c, shipID)/c.Jump.ReferenceMass, c.Jump.MassExponent)
}
func JumpLockReason(s *State, c *content.Content, to string) string {
	source, dest := c.SystemByID(s.SystemID), c.SystemByID(to)
	if source == nil || dest == nil {
		return "INVALID SYSTEM"
	}
	g := c.GateBetween(s.SystemID, to)
	if !systemsLinked(source, to) || g == nil {
		return "NO CHARTED ROUTE FROM " + source.Name + " TO " + dest.Name
	}
	if s.JumpClass < g.RequiredRating {
		return "NEED JUMP RATING CLASS " + GradeLetter(g.RequiredRating)
	}
	inst := ActiveShip(s)
	if inst == nil || inst.SystemID != s.SystemID {
		return "SHIP IS NOT AT THIS DOCK"
	}
	fuel := JumpFuel(s, c, s.ActiveShipID, *g)
	if fuel > FuelCapacity(s, c) {
		return fmt.Sprintf("JUMP FUEL %.1f · TANK %.1f", fuel, FuelCapacity(s, c))
	}
	if fuel > FuelAmount(s, c) {
		return fmt.Sprintf("REFUEL — JUMP NEEDS %.1f", fuel)
	}
	return ""
}

// PreviewJump describes guaranteed costs. Drift is deliberately not rolled until
// the committed action: the overlay labels this baseline and the possible risks.
func PreviewJump(s *State, c *content.Content, to string) (*JumpResult, error) {
	if !s.IsDocked() {
		return nil, ErrInBelt
	}
	if s.Run != nil {
		return nil, ErrActiveRun
	}
	if reason := JumpLockReason(s, c, to); reason != "" {
		return nil, fmt.Errorf("%w: %s", ErrRouteLocked, reason)
	}
	if ShipHull(s, s.ActiveShipID) <= 0 {
		return nil, ErrHullBreached
	}
	gate := c.GateBetween(s.SystemID, to)
	fuel := JumpFuel(s, c, s.ActiveShipID, *gate)
	stress := int(math.Ceil(float64(MaxHull(s, c)) * gate.HullStressPct))
	hull := max(c.Jump.HullFloor, ShipHull(s, s.ActiveShipID)-stress)
	return &JumpResult{OriginID: s.SystemID, SystemID: to, FuelCost: fuel, FuelAfter: FuelAmount(s, c) - fuel, HullAfter: hull, Stress: stress, DriftChance: gate.DriftChance * (1 - c.Jump.ScannerDriftReduction*float64(ActiveShip(s).Grades.Scanner)), LowHull: float64(hull)/float64(MaxHull(s, c)) < c.Fleet.EventHullGatePct}, nil
}

// Jump commits transit once on the actor, independently of animation/reconnect.
func Jump(s *State, c *content.Content, to string, now int64) (*JumpResult, error) {
	result, err := PreviewJump(s, c, to)
	if err != nil {
		return nil, err
	}
	gate := c.GateBetween(s.SystemID, to)
	j := c.Jump
	rng := newRNG(s.Seed, 900000+s.JumpCount)
	s.JumpCount++
	if rng.Float64() < result.DriftChance {
		mis := 0.
		if s.CrossedGates[gate.From+":"+gate.To] {
			mis = gate.MisalignmentWeight
		}
		roll := rng.Float64() * (j.HardTranslationWeight + j.FuelBloomWeight + j.HotArrivalWeight + mis)
		switch {
		case roll < j.HardTranslationWeight:
			result.Drift = "HARD TRANSLATION"
			damage := j.HardTranslationMin + rng.Float64()*(j.HardTranslationMax-j.HardTranslationMin)
			result.HullAfter = max(j.HullFloor, result.HullAfter-int(math.Ceil(float64(MaxHull(s, c))*damage)))
		case roll < j.HardTranslationWeight+j.FuelBloomWeight:
			result.Drift = "FUEL BLOOM"
			result.FuelAfter *= 1 - (j.FuelBloomMin + rng.Float64()*(j.FuelBloomMax-j.FuelBloomMin))
		case roll < j.HardTranslationWeight+j.FuelBloomWeight+j.HotArrivalWeight:
			result.Drift = "HOT ARRIVAL"
			if s.HotArrivals == nil {
				s.HotArrivals = map[string]bool{}
			}
			s.HotArrivals[to] = true
		default:
			result.Drift = "MISALIGNMENT"
			result.SystemID = s.SystemID
		}
	}
	ConsumeFuel(s, c, FuelAmount(s, c)-result.FuelAfter)
	setActiveHull(s, c, result.HullAfter)
	s.Stats.FuelBurned += result.FuelCost
	s.SystemID = result.SystemID
	ActiveShip(s).SystemID = s.SystemID
	s.Belt = nil
	s.Scan = nil
	if result.SystemID == to {
		if s.CrossedGates == nil {
			s.CrossedGates = map[string]bool{}
		}
		s.CrossedGates[gate.From+":"+gate.To] = true
		result.FirstArrival = visitSystem(s, to, now)
	}
	result.LowHull = float64(result.HullAfter)/float64(MaxHull(s, c)) < c.Fleet.EventHullGatePct
	s.LastJump = cloneValue(result)
	return result, nil
}
