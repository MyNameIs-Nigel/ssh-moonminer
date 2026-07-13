package sim

import "github.com/mynameis-nigel/ssh-moonminer/internal/content"

// BuySystemPermit permanently opens a system transfer for this pilot.
func BuySystemPermit(s *State, c *content.Content, systemID string) error {
	if !s.IsDocked() {
		return ErrInBelt
	}
	system := c.SystemByID(systemID)
	if system == nil {
		return ErrInvalidSystem
	}
	if system.StartsUnlocked || s.SystemPermits[systemID] {
		return ErrAlreadyUnlocked
	}
	if source := c.SystemByID(s.SystemID); source == nil || (source.ID != system.ID && !systemsLinked(source, system.ID)) {
		return ErrRouteLocked
	}
	model := c.ShipByID(s.ActiveShipID)
	if system.RequiredShipClass != "" && (model == nil || model.Class != system.RequiredShipClass) {
		return ErrRouteLocked
	}
	if s.Credits < system.TransferFee {
		return ErrInsufficientFunds
	}
	if s.SystemPermits == nil {
		s.SystemPermits = make(map[string]bool)
	}
	s.Credits -= system.TransferFee
	s.Stats.CreditsSpent += system.TransferFee
	s.SystemPermits[systemID] = true
	return nil
}

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
	if system == nil || (!system.StartsUnlocked && !s.SystemPermits[w.SystemID]) {
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

// SellCargo converts the entire docked cargo hold into credits and records
// the sale separately from the mining runs that loaded the hold.
func SellCargo(s *State, c *content.Content, now int64) (int, error) {
	if !s.IsDocked() {
		return 0, ErrInBelt
	}
	if s.CargoValue <= 0 {
		return 0, ErrCargoEmpty
	}
	value := s.CargoValue
	s.Credits += value
	s.CargoValue = 0
	s.CargoUnits = 0
	s.Stats.CreditsEarned += value
	s.Stats.CargoValueSold += value
	// Selling recovered cargo is a documented recovery milestone: the pilot
	// has converted a viable escape route, so a future real softlock may use
	// the safety net again.
	s.Settings.InsuranceUsed = false
	systemName := s.SystemID
	if system := c.SystemByID(s.SystemID); system != nil {
		systemName = system.Name
	}
	appendRunLog(s, RunRecord{When: now, World: systemName, Asteroid: "DOCK SALE", Outcome: "cargo_sold", CargoValueSold: value})
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
	s.Settings.InsuranceUsed = true
	s.Stats.InsuranceClaims++
	return nil
}

// UpdateSettings applies tweak changes.
func UpdateSettings(s *State, fn func(*Settings)) {
	fn(&s.Settings)
}
