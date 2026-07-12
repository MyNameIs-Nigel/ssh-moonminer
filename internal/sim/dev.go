package sim

import "github.com/mynameis-nigel/ssh-moonminer/internal/content"

// The functions in this file back the dev-server's debug overlay. They
// bypass normal economy rules (cost, docking, upgrade order) on purpose —
// production code must never call them.

// DevSetCredits sets credits directly, floored at zero.
func DevSetCredits(s *State, v int) {
	if v < 0 {
		v = 0
	}
	s.Credits = v
}

// DevSetFuel sets fuel directly, clamped to [0, tank size].
func DevSetFuel(s *State, c *content.Content, v float64) {
	tank := TankSize(s, c)
	if v < 0 {
		v = 0
	}
	if v > tank {
		v = tank
	}
	setFuelAmountFor(s, c, s.ActiveShipID, v)
	syncActiveConditionMirror(s, c)
}

// DevSetHull sets hull directly, clamped to [0, active ship's max].
func DevSetHull(s *State, c *content.Content, v int) {
	if v < 0 {
		v = 0
	}
	max := MaxHull(s, c)
	if v > max {
		v = max
	}
	setActiveHull(s, c, v)
}

// DevSetGodMode toggles hull-damage immunity.
func DevSetGodMode(s *State, on bool) {
	s.DevGodMode = on
}

// DevMaxShip sets the active ship's stat tracks to their model caps, free
// of charge.
func DevMaxShip(s *State, c *content.Content) {
	inst := ActiveShip(s)
	if inst == nil {
		return
	}
	model := c.ShipByID(inst.ModelID)
	if model == nil {
		return
	}
	inst.Grades = TrackGrades{
		Thrusters: model.ThrustersCap,
		Hull:      model.HullCap,
		FuelEff:   model.FuelEffCap,
		PowerGen:  model.PowerGenCap,
		Scanner:   model.ScannerCap,
	}
}
