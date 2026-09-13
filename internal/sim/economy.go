package sim

import "github.com/mynameis-nigel/ssh-moonminer/internal/content"

// BuyDestinationPermit permanently opens a locally gated destination.
func BuyDestinationPermit(s *State, c *content.Content, destinationID string) error {
	if !s.IsDocked() {
		return ErrInBelt
	}
	w := c.WorldByID(destinationID)
	if w == nil {
		return ErrInvalidWorld
	}
	if w.StartsUnlocked || s.DestinationPermits[w.ID] {
		return ErrAlreadyUnlocked
	}
	system := c.SystemByID(w.SystemID)
	if system == nil || w.SystemID != s.SystemID {
		return ErrRouteLocked
	}
	if s.Credits < w.PermitFee {
		return ErrInsufficientFunds
	}
	if s.DestinationPermits == nil {
		s.DestinationPermits = make(map[string]bool)
	}
	s.Credits -= w.PermitFee
	s.Stats.CreditsSpent += w.PermitFee
	s.DestinationPermits[w.ID] = true
	return nil
}

// SellCargo converts the entire docked cargo hold into credits and redeems
// any bounty vouchers earned from destroyed pirates in the same
// transaction, recording the sale separately from the mining runs that
// loaded the hold. Vouchers are not cargo — docs/gameplay/07-pirate-combat-
// and-bounties.md's deliberate carve-out — so they can pay out even when
// the hold is empty.
func SellCargo(s *State, c *content.Content, now int64) (int, error) {
	if !s.IsDocked() {
		return 0, ErrInBelt
	}
	if s.CargoValue <= 0 && s.BountyVouchers <= 0 {
		return 0, ErrCargoEmpty
	}
	cargoValue := s.CargoValue
	bounty := s.BountyVouchers
	value := cargoValue + bounty
	s.Credits += value
	for id, value := range s.CargoOrigins {
		frontierRecord(s, id).CargoValueSold += value
	}
	s.CargoOrigins = nil
	s.CargoValue = 0
	s.CargoUnits = 0
	s.BountyVouchers = 0
	s.Stats.CreditsEarned += value
	s.Stats.CargoValueSold += cargoValue
	// Selling recovered cargo is a documented recovery milestone: the pilot
	// has converted a viable escape route, so a future real softlock may use
	// the safety net again.
	s.Settings.InsuranceUsed = false
	systemName := s.SystemID
	if system := c.SystemByID(s.SystemID); system != nil {
		systemName = system.Name
	}
	appendRunLog(s, RunRecord{When: now, World: systemName, Asteroid: "DOCK SALE", Outcome: "cargo_sold", CargoValueSold: cargoValue, BountyEarned: bounty})
	return value, nil
}

// Refuel buys whole fuel points at the port's whole-credit rate, filling
// detachable tanks before the hull reserve. A fractional shortfall costs one
// final point so the tank can always be topped off.
func Refuel(s *State, c *content.Content) error {
	syncActiveConditionFromLegacy(s, c)
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
	pointsNeeded := refuelPointsNeeded(tank, s.Fuel)
	if pointsNeeded <= 0 {
		return ErrAlreadyFull
	}
	points := min(s.Credits/c.Port.RefuelPerPoint, pointsNeeded)
	if points <= 0 {
		return ErrInsufficientFunds
	}
	spent := points * c.Port.RefuelPerPoint
	s.Credits -= spent
	s.Stats.CreditsSpent += spent
	AddFuel(s, c, float64(points))
	return nil
}

// Repair restores hull to the active ship's max.
func Repair(s *State, c *content.Content) error {
	syncActiveConditionFromLegacy(s, c)
	if !s.IsDocked() {
		return ErrInBelt
	}
	max := MaxHull(s, c)
	if s.Hull >= max {
		return ErrAlreadyFull
	}
	cost := RepairCost(s, c)
	if s.Credits < cost {
		return ErrInsufficientFunds
	}
	s.Credits -= cost
	setActiveHull(s, c, max)
	s.Stats.CreditsSpent += cost
	return nil
}

// InsuranceAdvance grants softlock protection credits.
func InsuranceAdvance(s *State, c *content.Content) error {
	if !InsuranceEligible(s, c) {
		return ErrNotEligible
	}
	s.Credits = c.Port.InsuranceCredits
	if sys := c.SystemByID(s.SystemID); sys != nil {
		s.Credits = max(s.Credits, sys.SalvageAdvance)
	}
	s.Settings.InsuranceUsed = true
	s.Stats.InsuranceClaims++
	return nil
}

// UpdateSettings applies tweak changes.
func UpdateSettings(s *State, fn func(*Settings)) {
	fn(&s.Settings)
}
