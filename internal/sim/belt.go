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
	n := bc.AsteroidsPerBelt
	rng := newRNG(s.Seed, s.BeltCount)

	rocks := make([]Asteroid, n)
	for i := 0; i < n; i++ {
		// Distance is rolled before tier so a farther contact can nudge its
		// own rarity weights (gameplay/05's small distance-biased rarity
		// nudge) — order matters for TestGenerateBeltDeterministic-style
		// reproducibility, not for correctness, since both draws come from
		// the same seeded stream either way.
		distance := round1(clamp(rng.Float64()*(bc.DistanceMax-bc.DistanceMin)+bc.DistanceMin, bc.DistanceMin, bc.DistanceMax))
		distBias := 0.0
		if bc.DistanceMax > 0 {
			distBias = (distance / bc.DistanceMax) * c.Fleet.DistanceRarityBonusMax
		}
		bias := w.RarityBias + distBias
		weights := []float64{
			0.46 - bias*0.5,
			0.30,
			0.16 + bias*0.6,
			0.08 + bias*0.4,
		}
		tier := rollTier(rng, weights)
		vol := int(math.Round(clamp(rng.Float64()*float64(bc.VolumeMax-bc.VolumeMin)+float64(bc.VolumeMin), float64(bc.VolumeMin), float64(bc.VolumeMax))/float64(bc.VolumeStep))) * bc.VolumeStep
		drill := round1(clamp(float64(vol)/bc.DrillSecPerVol, bc.DrillSecMin, bc.DrillSecMax))
		tierMult := c.Tiers.Mults[tier]
		value := int(math.Round(float64(vol)*bc.ValuePerVolume*tierMult/float64(bc.ValueStep))) * bc.ValueStep
		fuelCost := int(math.Round(clamp(distance*bc.FuelPerKm+float64(tier*bc.FuelCostTierBon), float64(bc.FuelCostMin), float64(bc.FuelCostMax))))
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
			Distance: distance,
		}
	}
	sort.Slice(rocks, func(i, j int) bool { return rocks[i].Distance < rocks[j].Distance })
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
	if reason := RouteLockReason(s, c, worldIdx); reason != "" {
		return ErrRouteLocked
	}
	travel := float64(w.TravelFuel)
	if s.Fuel < travel {
		return ErrInsufficientFuel
	}
	s.Fuel -= travel
	s.WorldIdx = worldIdx
	s.SystemID = w.SystemID
	s.Belt = GenerateBelt(s, c, worldIdx)
	applySeismicSensors(s, c)
	return nil
}

// Dock returns to the star chart and rearms the active ship's Pirate
// Jammer, if installed — a free, instant service performed only at dock.
func Dock(s *State, c *content.Content) {
	if s.Run != nil {
		return
	}
	s.WorldIdx = -1
	s.Belt = nil
	s.Scan = nil
	RearmJammer(s, c)
	restoreShipShieldFull(s, c, s.ActiveShipID)
}

// applySeismicSensors pre-scans free asteroids on belt arrival when the
// active ship's internal module is Seismic Sensors, per
// gameplay/05-fleet-ships-and-shipyard-economy.md. Higher grades guarantee
// a minimum tier among the pre-scanned picks. Candidates are restricted to
// the ship's own Scanner range — the free pre-scan is a convenience, not a
// way to bypass the Scanner distance lock that gates Lock()/manual Scan().
func applySeismicSensors(s *State, c *content.Content) {
	inst := ActiveShip(s)
	if inst == nil || inst.Internal == nil || inst.Internal.ItemID != ItemSeismic {
		return
	}
	n := c.Slots.SeismicScanCount
	if n <= 0 || len(s.Belt) == 0 {
		return
	}
	lockKm := ScannerLockKm(s, c)
	candidates := make([]int, 0, len(s.Belt))
	for i := range s.Belt {
		if s.Belt[i].Distance < lockKm {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		return
	}
	if n > len(candidates) {
		n = len(candidates)
	}
	rng := newRNG(s.Seed, s.BeltCount*2654435761+1)
	order := rng.Perm(len(candidates))
	picked := make([]int, 0, n)
	for _, oi := range order[:n] {
		idx := candidates[oi]
		s.Belt[idx].Scanned = true
		picked = append(picked, idx)
	}

	minTier := seismicMinTierGuarantee(inst.Internal.Grade)
	if minTier == 0 {
		return
	}
	for _, idx := range picked {
		if s.Belt[idx].Tier >= minTier {
			return // guarantee already satisfied
		}
	}
	bestIdx, bestTier := -1, -1
	for _, ci := range candidates {
		if s.Belt[ci].Scanned {
			continue
		}
		if s.Belt[ci].Tier >= minTier && s.Belt[ci].Tier > bestTier {
			bestIdx, bestTier = ci, s.Belt[ci].Tier
		}
	}
	if bestIdx < 0 {
		return // no in-range asteroid in this belt meets the guarantee
	}
	for _, idx := range picked {
		if s.Belt[idx].Tier < minTier {
			s.Belt[idx].Scanned = false
			s.Belt[bestIdx].Scanned = true
			return
		}
	}
}

// IsOutOfRange reports whether an asteroid is beyond the active ship's
// Scanner lock distance — it cannot be scanned or targeted.
func IsOutOfRange(s *State, c *content.Content, ast *Asteroid) bool {
	return ast.Distance >= ScannerLockKm(s, c)
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
