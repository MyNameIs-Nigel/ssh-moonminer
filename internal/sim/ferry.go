package sim

import (
	"math"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

func FerryPrice(s *State, c *content.Content, shipID, to string) (int, error) {
	if !s.IsDocked() {
		return 0, ErrInBelt
	}
	inst := s.Ships[shipID]
	if inst == nil {
		return 0, ErrNotOwned
	}
	if shipID == s.ActiveShipID {
		return 0, ErrNotEligible
	}
	if c.SystemByID(to) == nil {
		return 0, ErrInvalidSystem
	}
	if inst.SystemID == to {
		return 0, ErrNotEligible
	}
	if f := s.Frontier[to]; f == nil || !f.Visited {
		return 0, ErrRouteLocked
	}
	// Dijkstra keeps the service correct if the explicit network gains branches.
	costs := map[string]float64{inst.SystemID: 0}
	done := map[string]bool{}
	for {
		id := ""
		best := math.Inf(1)
		for _, sys := range c.Systems {
			v, ok := costs[sys.ID]
			if ok && !done[sys.ID] && v < best {
				id = sys.ID
				best = v
			}
		}
		if id == "" {
			return 0, ErrRouteLocked
		}
		if id == to {
			return int(math.Round(best * c.Ferry.Markup * float64(c.Port.RefuelPerPoint))), nil
		}
		done[id] = true
		for _, next := range c.SystemByID(id).Links {
			g := c.GateBetween(id, next)
			if g == nil || s.JumpClass < g.RequiredRating {
				continue
			}
			v := best + JumpFuel(s, c, shipID, *g)
			old, ok := costs[next]
			if !ok || v < old {
				costs[next] = v
			}
		}
	}
}
func FerryShip(s *State, c *content.Content, shipID, to string) error {
	price, err := FerryPrice(s, c, shipID, to)
	if err != nil {
		return err
	}
	if s.Credits < price {
		return ErrInsufficientFunds
	}
	s.Credits -= price
	s.Stats.CreditsSpent += price
	s.Ships[shipID].SystemID = to
	return nil
}
