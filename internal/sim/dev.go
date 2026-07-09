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
	s.Fuel = v
}

// DevSetHull sets hull directly, clamped to [0, 100].
func DevSetHull(s *State, v int) {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	s.Hull = v
}

// DevSetGodMode toggles hull-damage immunity.
func DevSetGodMode(s *State, on bool) {
	s.DevGodMode = on
}

// DevMaxUpgrades sets every upgrade track to its max level, free of charge.
func DevMaxUpgrades(s *State, c *content.Content) {
	s.Upgrades = Upgrades{
		Drill:    c.Upgrades.MaxLevel,
		Tank:     c.Upgrades.MaxLevel,
		Plating:  c.Upgrades.MaxLevel,
		Damper:   c.Upgrades.MaxLevel,
		Surveyor: c.Upgrades.MaxLevel,
	}
}
