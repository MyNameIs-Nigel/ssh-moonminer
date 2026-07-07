package sim

import (
	"math"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// UpgradeTrack identifies a ship upgrade line.
type UpgradeTrack int

const (
	UpgradeDrill UpgradeTrack = iota
	UpgradeTank
	UpgradePlating
	UpgradeDamper
	UpgradeSurveyor
)

// UpgradePrice returns cost for the next level of a track.
func UpgradePrice(c *content.Content, track UpgradeTrack, level int) int {
	uc := c.Upgrades
	var base int
	switch track {
	case UpgradeDrill:
		base = uc.DrillBase
	case UpgradeTank:
		base = uc.TankBase
	case UpgradePlating:
		base = uc.PlatingBase
	case UpgradeDamper:
		base = uc.DamperBase
	case UpgradeSurveyor:
		base = uc.SurveyorBase
	}
	return int(float64(base) * math.Pow(3, float64(level)))
}

// UpgradeLevel returns current level for track.
func UpgradeLevel(s *State, track UpgradeTrack) int {
	switch track {
	case UpgradeDrill:
		return s.Upgrades.Drill
	case UpgradeTank:
		return s.Upgrades.Tank
	case UpgradePlating:
		return s.Upgrades.Plating
	case UpgradeDamper:
		return s.Upgrades.Damper
	case UpgradeSurveyor:
		return s.Upgrades.Surveyor
	}
	return 0
}

// BuyUpgrade purchases the next level of a track.
func BuyUpgrade(s *State, c *content.Content, track UpgradeTrack) error {
	if !s.IsDocked() {
		return ErrInBelt
	}
	lvl := UpgradeLevel(s, track)
	if lvl >= c.Upgrades.MaxLevel {
		return ErrMaxUpgrade
	}
	price := UpgradePrice(c, track, lvl)
	if s.Credits < price {
		return ErrInsufficientFunds
	}
	s.Credits -= price
	s.Stats.CreditsSpent += price
	switch track {
	case UpgradeDrill:
		s.Upgrades.Drill++
	case UpgradeTank:
		s.Upgrades.Tank++
	case UpgradePlating:
		s.Upgrades.Plating++
	case UpgradeDamper:
		s.Upgrades.Damper++
	case UpgradeSurveyor:
		s.Upgrades.Surveyor++
	}
	return nil
}

func effectiveDrillRate(s *State, c *content.Content) float64 {
	mul := 1.0
	for i := 0; i < s.Upgrades.Drill; i++ {
		mul *= c.Upgrades.DrillRateMul
	}
	return mul
}

func effectiveDamperMul(s *State, c *content.Content) float64 {
	mul := 1.0
	for i := 0; i < s.Upgrades.Damper; i++ {
		mul *= c.Upgrades.DamperMul
	}
	return mul
}

func effectiveSurveyorBonus(c *content.Content) int {
	return c.Upgrades.SurveyorBonus
}

// TrackName returns display name for upgrade track.
func TrackName(track UpgradeTrack) string {
	switch track {
	case UpgradeDrill:
		return "DRILL HEAD"
	case UpgradeTank:
		return "FUEL TANK"
	case UpgradePlating:
		return "HULL PLATING"
	case UpgradeDamper:
		return "SIG DAMPER"
	case UpgradeSurveyor:
		return "SURVEYOR"
	}
	return ""
}
