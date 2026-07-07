package sim

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// GenerateBelt creates procedural asteroids for the given world.
func GenerateBelt(s *State, c *content.Content, worldIdx int) []Asteroid {
	w := c.WorldByIndex(worldIdx)
	if w == nil {
		return nil
	}
	bc := c.Belt
	n := bc.AsteroidsPerBelt + s.Upgrades.Surveyor*effectiveSurveyorBonus(c)
	rng := newRNG(s.Seed, s.BeltCount)
	bias := w.RarityBias

	weights := []float64{
		0.46 - bias*0.5,
		0.30,
		0.16 + bias*0.6,
		0.08 + bias*0.4,
	}

	rocks := make([]Asteroid, n)
	for i := 0; i < n; i++ {
		tier := rollTier(rng, weights)
		vol := int(math.Round(clamp(rng.Float64()*float64(bc.VolumeMax-bc.VolumeMin)+float64(bc.VolumeMin), float64(bc.VolumeMin), float64(bc.VolumeMax)) / float64(bc.VolumeStep))) * bc.VolumeStep
		drill := round1(clamp(float64(vol)/bc.DrillSecPerVol, bc.DrillSecMin, bc.DrillSecMax))
		tierMult := c.Tiers.Mults[tier]
		value := int(math.Round(float64(vol)*bc.ValuePerVolume*tierMult/float64(bc.ValueStep))) * bc.ValueStep
		fuelCost := int(math.Round(clamp(float64(bc.FuelCostMin)+rng.Float64()*14+float64(tier*bc.FuelCostTierBon), float64(bc.FuelCostMin), float64(bc.FuelCostMax))))
		risk := int(math.Round(clamp(w.PirateMul*float64(bc.RiskBase+tier*bc.RiskPerTier)+rng.Float64()*16, float64(bc.RiskMin), float64(bc.RiskMax))))
		dots := clampInt(int(math.Ceil(float64(risk)/20)), 1, 5)
		size := "sm"
		if vol > 3000 {
			size = "lg"
		} else if vol > 1300 {
			size = "md"
		}
		prefix := bc.NamePrefixes[rng.Intn(len(bc.NamePrefixes))]
		name := fmt.Sprintf("%s-%d", prefix, 1000+rng.Intn(9000))
		x := int(math.Round(rng.Float64()*74 + 10))
		y := int(math.Round(rng.Float64()*68 + 12))

		rocks[i] = Asteroid{
			ID: i + 1, Name: name, Tier: tier, Volume: vol,
			DrillSec: drill, Value: value, FuelCost: fuelCost,
			Risk: risk, Dots: dots, Size: size, X: x, Y: y,
		}
	}
	sort.Slice(rocks, func(i, j int) bool { return rocks[i].FuelCost < rocks[j].FuelCost })
	s.BeltCount++
	return rocks
}

func rollTier(rng *rand.Rand, weights []float64) int {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	r := rng.Float64() * total
	cum := 0.0
	for i, w := range weights {
		cum += w
		if r <= cum {
			return i
		}
	}
	return len(weights) - 1
}

// Depart travels to a world and generates a belt.
func Depart(s *State, c *content.Content, worldIdx int) error {
	if worldIdx < 0 || worldIdx >= len(c.Worlds) {
		return ErrInvalidWorld
	}
	if s.Hull <= 0 {
		return ErrHullBreached
	}
	if s.Run != nil {
		return ErrActiveRun
	}
	w := c.Worlds[worldIdx]
	travel := float64(w.TravelFuel)
	if s.Fuel < travel {
		return ErrInsufficientFuel
	}
	s.Fuel -= travel
	s.WorldIdx = worldIdx
	s.Belt = GenerateBelt(s, c, worldIdx)
	return nil
}

// Rescan regenerates the current belt.
func Rescan(s *State, c *content.Content) {
	if s.WorldIdx < 0 {
		return
	}
	s.Belt = GenerateBelt(s, c, s.WorldIdx)
}

// Dock returns to the star chart.
func Dock(s *State) {
	if s.Run != nil {
		return
	}
	s.WorldIdx = -1
	s.Belt = nil
}

// FindAsteroid returns asteroid by ID.
func FindAsteroid(s *State, id int) (*Asteroid, int) {
	for i := range s.Belt {
		if s.Belt[i].ID == id {
			return &s.Belt[i], i
		}
	}
	return nil, -1
}

// RemoveAsteroid removes asteroid by ID from belt.
func RemoveAsteroid(s *State, id int) {
	for i := range s.Belt {
		if s.Belt[i].ID == id {
			s.Belt = append(s.Belt[:i], s.Belt[i+1:]...)
			return
		}
	}
}
