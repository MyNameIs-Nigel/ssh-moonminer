// Package content loads and validates game data from TOML.
package content

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/mynameis-nigel/ssh-moonminer/data"
)

// World is one destination on the star chart.
type World struct {
	Name        string  `toml:"name"`
	Sub         string  `toml:"sub"`
	Ring        string  `toml:"ring"`
	PirateLabel string  `toml:"pirate_label"`
	PirateMul   float64 `toml:"pirate_mul"`
	RarityBias  float64 `toml:"rarity_bias"`
	TravelFuel  int     `toml:"travel_fuel"`
	RarityLabel string  `toml:"rarity_label"`
	Desc        string  `toml:"desc"`
	Art         string  `toml:"art"`
}

// PilotStart is the new-pilot starting resources.
type PilotStart struct {
	StartCredits int `toml:"start_credits"`
	StartFuel    int `toml:"start_fuel"`
	StartHull    int `toml:"start_hull"`
}

// BeltConfig tunes procedural belt generation.
type BeltConfig struct {
	AsteroidsPerBelt int      `toml:"asteroids_per_belt"`
	VolumeMin        int      `toml:"volume_min"`
	VolumeMax        int      `toml:"volume_max"`
	VolumeStep       int      `toml:"volume_step"`
	DrillSecMin      float64  `toml:"drill_sec_min"`
	DrillSecMax      float64  `toml:"drill_sec_max"`
	DrillSecPerVol   float64  `toml:"drill_sec_per_volume"`
	ValuePerVolume   float64  `toml:"value_per_volume"`
	ValueStep        int      `toml:"value_step"`
	DistanceMin      float64  `toml:"distance_min"`
	DistanceMax      float64  `toml:"distance_max"`
	FuelPerKm        float64  `toml:"fuel_per_km"`
	FuelCostMin      int      `toml:"fuel_cost_min"`
	FuelCostMax      int      `toml:"fuel_cost_max"`
	FuelCostTierBon  int      `toml:"fuel_cost_tier_bonus"`
	ScanFuelCost     float64  `toml:"scan_fuel_cost"`
	ScanSecPerKm     float64  `toml:"scan_sec_per_km"`
	RiskMin          int      `toml:"risk_min"`
	RiskMax          int      `toml:"risk_max"`
	RiskBase         int      `toml:"risk_base"`
	RiskPerTier      int      `toml:"risk_per_tier"`
	NamePrefixes     []string `toml:"name_prefixes"`
}

// TierConfig holds rarity tier definitions.
type TierConfig struct {
	Labels []string  `toml:"labels"`
	Glyphs []string  `toml:"glyphs"`
	Mults  []float64 `toml:"mults"`
}

// MiningConfig holds real-time mining constants.
type MiningConfig struct {
	TickHz            int     `toml:"tick_hz"`
	PirateRateBase    float64 `toml:"pirate_rate_base"`
	PirateRatePerTier float64 `toml:"pirate_rate_per_tier"`
	FuelDrainBase     float64 `toml:"fuel_drain_base"`
	FuelDrainPerTier  float64 `toml:"fuel_drain_per_tier"`
	OverdriveDrillMul float64 `toml:"overdrive_drill_mul"`
	OverdriveFuelMul  float64 `toml:"overdrive_fuel_mul"`
	RaidedYieldKeep   float64 `toml:"raided_yield_keep"`
	RaidedHullDamage  int     `toml:"raided_hull_damage"`
	StrandedYieldKeep float64 `toml:"stranded_yield_keep"`
}

// PortConfig holds port service pricing.
type PortConfig struct {
	RefuelPerPoint           int     `toml:"refuel_per_point"`
	RepairPerPoint           int     `toml:"repair_per_point"`
	DrydockSurchargeMul      float64 `toml:"drydock_surcharge_mul"`
	InsuranceCredits         int     `toml:"insurance_credits"`
	InsuranceFuelThreshold   int     `toml:"insurance_fuel_threshold"`
	InsuranceCreditThreshold int     `toml:"insurance_credit_threshold"`
}

// UpgradeConfig holds ship upgrade pricing and effects.
type UpgradeConfig struct {
	MaxLevel         int     `toml:"max_level"`
	DrillBase        int     `toml:"drill_base"`
	TankBase         int     `toml:"tank_base"`
	PlatingBase      int     `toml:"plating_base"`
	DamperBase       int     `toml:"damper_base"`
	SurveyorBase     int     `toml:"surveyor_base"`
	DrillRateMul     float64 `toml:"drill_rate_mul"`
	TankBonus        int     `toml:"tank_bonus"`
	PlatingReduction int     `toml:"plating_reduction"`
	PlatingFloor     int     `toml:"plating_floor"`
	DamperMul        float64 `toml:"damper_mul"`
	SurveyorBonus    int     `toml:"surveyor_bonus"`
	BaseTank         int     `toml:"base_tank"`
}

type worldsFile struct {
	Worlds []World `toml:"worlds"`
}

type balanceFile struct {
	Pilot    PilotStart    `toml:"pilot"`
	Belt     BeltConfig    `toml:"belt"`
	Tiers    TierConfig    `toml:"tiers"`
	Mining   MiningConfig  `toml:"mining"`
	Port     PortConfig    `toml:"port"`
	Upgrades UpgradeConfig `toml:"upgrades"`
}

// Content is immutable validated game configuration.
type Content struct {
	Worlds   []World
	Pilot    PilotStart
	Belt     BeltConfig
	Tiers    TierConfig
	Mining   MiningConfig
	Port     PortConfig
	Upgrades UpgradeConfig
}

// Load reads content from overrideDir or embedded files.
func Load(overrideDir string) (*Content, error) {
	var fsys fs.FS = data.FS
	if overrideDir != "" {
		fsys = os.DirFS(overrideDir)
	}
	var wf worldsFile
	if err := decodeTOML(fsys, "worlds.toml", &wf); err != nil {
		return nil, err
	}
	var bf balanceFile
	if err := decodeTOML(fsys, "balance.toml", &bf); err != nil {
		return nil, err
	}
	c := &Content{
		Worlds:   wf.Worlds,
		Pilot:    bf.Pilot,
		Belt:     bf.Belt,
		Tiers:    bf.Tiers,
		Mining:   bf.Mining,
		Port:     bf.Port,
		Upgrades: bf.Upgrades,
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func decodeTOML(fsys fs.FS, name string, v any) error {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("content: read %s: %w", name, err)
	}
	if err := toml.Unmarshal(b, v); err != nil {
		return fmt.Errorf("content: parse %s: %w", name, err)
	}
	return nil
}

func (c *Content) validate() error {
	if len(c.Worlds) < 4 {
		return fmt.Errorf("content: need at least 4 worlds, got %d", len(c.Worlds))
	}
	for i, w := range c.Worlds {
		if w.Name == "" {
			return fmt.Errorf("content: world #%d has no name", i+1)
		}
		if w.TravelFuel < 1 {
			return fmt.Errorf("content: world %q travel_fuel must be positive", w.Name)
		}
		if w.PirateMul <= 0 || w.RarityBias < 0 {
			return fmt.Errorf("content: world %q has invalid multipliers", w.Name)
		}
	}
	if len(c.Tiers.Labels) != 4 || len(c.Tiers.Glyphs) != 4 || len(c.Tiers.Mults) != 4 {
		return fmt.Errorf("content: tiers must have exactly 4 entries")
	}
	for i := 1; i < len(c.Tiers.Mults); i++ {
		if c.Tiers.Mults[i] <= c.Tiers.Mults[i-1] {
			return fmt.Errorf("content: tier multipliers must be strictly ascending")
		}
	}
	if c.Belt.AsteroidsPerBelt < 1 {
		return fmt.Errorf("content: asteroids_per_belt must be positive")
	}
	if c.Belt.DistanceMin <= 0 || c.Belt.DistanceMax <= c.Belt.DistanceMin {
		return fmt.Errorf("content: belt distance_min/distance_max must be positive and ascending")
	}
	if c.Belt.ScanSecPerKm <= 0 {
		return fmt.Errorf("content: belt scan_sec_per_km must be positive")
	}
	if c.Mining.TickHz < 1 {
		return fmt.Errorf("content: mining tick_hz must be positive")
	}
	if c.Port.RefuelPerPoint < 1 || c.Port.RepairPerPoint < 1 {
		return fmt.Errorf("content: port prices must be positive")
	}
	if c.Upgrades.MaxLevel < 1 {
		return fmt.Errorf("content: upgrades max_level must be positive")
	}
	return nil
}

// WorldByIndex returns world at index or nil.
func (c *Content) WorldByIndex(idx int) *World {
	if idx < 0 || idx >= len(c.Worlds) {
		return nil
	}
	return &c.Worlds[idx]
}
