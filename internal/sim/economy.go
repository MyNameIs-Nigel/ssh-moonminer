package sim

import (
	"math"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// Refuel fills fuel tank as much as credits allow.
func Refuel(s *State, c *content.Content) error {
	if !s.IsDocked() {
		return ErrInBelt
	}
	tank := TankSize(s, c)
	if s.Fuel >= tank {
		return ErrAlreadyFull
	}
	if s.Credits <= 0 {
		return ErrInsufficientFunds
	}
	missing := tank - s.Fuel
	fullCost := int(math.Round(missing * float64(c.Port.RefuelPerPoint)))
	if s.Credits >= fullCost {
		s.Credits -= fullCost
		s.Stats.CreditsSpent += fullCost
		s.Fuel = tank
	} else {
		pts := float64(s.Credits) / float64(c.Port.RefuelPerPoint)
		spent := s.Credits
		s.Credits = 0
		s.Stats.CreditsSpent += spent
		s.Fuel += pts
	}
	return nil
}

// Repair restores hull to 100.
func Repair(s *State, c *content.Content) error {
	if !s.IsDocked() {
		return ErrInBelt
	}
	if s.Hull >= 100 {
		return ErrAlreadyFull
	}
	cost := RepairCost(s, c)
	if s.Credits < cost {
		return ErrInsufficientFunds
	}
	s.Credits -= cost
	s.Hull = 100
	s.Stats.CreditsSpent += cost
	return nil
}

// InsuranceAdvance grants softlock protection credits.
func InsuranceAdvance(s *State, c *content.Content) error {
	if !InsuranceEligible(s, c) {
		return ErrNotEligible
	}
	s.Credits = c.Port.InsuranceCredits
	s.Settings.InsuranceUsed = true
	s.Stats.InsuranceClaims++
	return nil
}

// UpdateSettings applies tweak changes.
func UpdateSettings(s *State, fn func(*Settings)) {
	fn(&s.Settings)
}
