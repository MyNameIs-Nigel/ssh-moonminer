package content

import (
	"fmt"
	"math"
)

type JumpConfig struct {
	ReferenceMass         float64 `toml:"reference_mass"`
	MassExponent          float64 `toml:"mass_exponent"`
	ScannerDriftReduction float64 `toml:"scanner_drift_reduction"`
	HullFloor             int     `toml:"hull_floor"`
	CountdownSeconds      int     `toml:"countdown_seconds"`
	HardTranslationMin    float64 `toml:"hard_translation_min"`
	HardTranslationMax    float64 `toml:"hard_translation_max"`
	FuelBloomMin          float64 `toml:"fuel_bloom_min"`
	FuelBloomMax          float64 `toml:"fuel_bloom_max"`
	HotArrivalETAMul      float64 `toml:"hot_arrival_eta_mul"`
	HardTranslationWeight float64 `toml:"hard_translation_weight"`
	FuelBloomWeight       float64 `toml:"fuel_bloom_weight"`
	HotArrivalWeight      float64 `toml:"hot_arrival_weight"`
}
type FerryConfig struct {
	Markup float64 `toml:"markup"`
}
type Gate struct {
	From               string  `toml:"from"`
	To                 string  `toml:"to"`
	RequiredRating     int     `toml:"required_rating"`
	JumpFuelBase       float64 `toml:"jump_fuel_base"`
	HullStressPct      float64 `toml:"hull_stress_pct"`
	DriftChance        float64 `toml:"drift_chance"`
	MisalignmentWeight float64 `toml:"misalignment_weight"`
}
type Rating struct {
	Class               int    `toml:"class"`
	Name                string `toml:"name"`
	Opens               string `toml:"opens"`
	CertifyIn           string `toml:"certify_in"`
	Price               int    `toml:"price"`
	ReqCargoSoldSystem  string `toml:"req_cargo_sold_system"`
	ReqCargoSoldValue   int    `toml:"req_cargo_sold_value"`
	ReqRunsDestination  string `toml:"req_runs_destination"`
	ReqRunsCount        int    `toml:"req_runs_count"`
	ReqPiratesDestroyed int    `toml:"req_pirates_destroyed"`
	ReqLegendariesMined int    `toml:"req_legendaries_mined"`
}

func (c *Content) GateBetween(from, to string) *Gate {
	for i := range c.Gates {
		g := &c.Gates[i]
		if g.From == from && g.To == to || g.To == from && g.From == to {
			return g
		}
	}
	return nil
}
func (c *Content) validateJump() error {
	bad := func(s string) error { return fmt.Errorf("content: jump progression: %s", s) }
	j := c.Jump
	if j.ReferenceMass <= 0 || j.MassExponent <= 0 || j.HullFloor != 1 || j.CountdownSeconds < 0 || j.ScannerDriftReduction < 0 || j.ScannerDriftReduction > 0.2 || c.Ferry.Markup <= 1 {
		return bad("invalid jump or ferry tuning")
	}
	if j.HardTranslationMin < 0 || j.HardTranslationMax < j.HardTranslationMin || j.HardTranslationMax > 1 || j.FuelBloomMin < 0 || j.FuelBloomMax < j.FuelBloomMin || j.FuelBloomMax > 1 || j.HotArrivalETAMul <= 0 || j.HotArrivalETAMul >= 1 || j.HardTranslationWeight <= 0 || j.FuelBloomWeight <= 0 || j.HotArrivalWeight <= 0 {
		return bad("invalid drift table")
	}
	if len(c.Ratings) != 3 || len(c.Gates) != 3 {
		return bad("beta needs three ratings and gates")
	}
	seen := map[string]bool{}
	for _, g := range c.Gates {
		a, b := c.SystemByID(g.From), c.SystemByID(g.To)
		if a == nil || b == nil || !containsString(a.Links, g.To) || !containsString(b.Links, g.From) || g.From == g.To {
			return bad("unknown or unlinked gate")
		}
		key := g.From + ":" + g.To
		reverse := g.To + ":" + g.From
		if seen[key] || seen[reverse] {
			return bad("duplicate gate")
		}
		seen[key] = true
		if g.RequiredRating < 0 || g.RequiredRating > 2 || g.RequiredRating != max(a.RequiredRating, b.RequiredRating) || g.JumpFuelBase <= 0 || g.HullStressPct <= 0 || g.HullStressPct > 1 || g.DriftChance < 0 || g.DriftChance > 1 || g.MisalignmentWeight < 0 || g.RequiredRating < 2 && g.MisalignmentWeight > 0 {
			return bad("invalid gate tuning")
		}
		skiff := c.ShipByID(StarterShipID)
		if skiff == nil || g.JumpFuelBase*math.Pow(skiff.BaseMass/j.ReferenceMass, j.MassExponent) >= float64(c.Pilot.StartFuel) {
			return bad("gate strands free Skiff")
		}
	}
	for _, sys := range c.Systems {
		for _, to := range sys.Links {
			if c.GateBetween(sys.ID, to) == nil {
				return bad("link has no gate")
			}
		}
	}
	for i, r := range c.Ratings {
		g := c.GateBetween(r.CertifyIn, r.Opens)
		if r.Class != i || g == nil || g.RequiredRating != i || c.SystemByID(r.CertifyIn).RequiredRating != i-1 || r.Price <= 0 {
			return bad("invalid rating ladder")
		}
		if r.ReqCargoSoldValue < 0 || r.ReqRunsCount < 0 || r.ReqPiratesDestroyed < 0 || r.ReqLegendariesMined < 0 || r.ReqCargoSoldValue+r.ReqRunsCount+r.ReqPiratesDestroyed+r.ReqLegendariesMined == 0 {
			return bad("rating needs frontier work")
		}
		if r.ReqCargoSoldValue > 0 && r.ReqCargoSoldSystem != r.CertifyIn {
			return bad("cargo qualification outside frontier")
		}
		if r.ReqRunsCount > 0 {
			w := c.WorldByID(r.ReqRunsDestination)
			if w == nil || w.SystemID != r.CertifyIn {
				return bad("run qualification outside frontier")
			}
		}
	}
	for _, m := range c.Ships {
		if len(m.SoldIn) == 0 || m.InternalSlots < 1 || m.InternalSlots > 2 || m.DockRepairPct < 0 || m.DockRepairPct > 1 {
			return bad("invalid hull services")
		}
		for _, id := range m.SoldIn {
			if c.SystemByID(id) == nil {
				return bad("unknown hull market")
			}
		}
	}
	for _, w := range c.Worlds {
		if w.InstabilityPerSecond < 0 || w.InstabilityPerSecond > 1 {
			return bad("invalid belt instability")
		}
	}
	return nil
}
