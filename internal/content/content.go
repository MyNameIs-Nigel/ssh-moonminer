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

// MiningConfig holds real-time mining, pirate, and escape constants for the
// manual mining run loop (docs/gameplay/02-mining-run-loop.md). There is no
// overdrive in this design — tension comes from time, radar, cargo load,
// events, and escape risk.
type MiningConfig struct {
	TickHz int `toml:"tick_hz"`

	// Pirate radar distance closes from 100 (far) to 0 (arrived).
	PirateApproachBase    float64 `toml:"pirate_approach_base"`
	PirateApproachPerTier float64 `toml:"pirate_approach_per_tier"`

	FuelDrainBase    float64 `toml:"fuel_drain_base"`
	FuelDrainPerTier float64 `toml:"fuel_drain_per_tier"`

	// Once pirates arrive, they roll tribute vs. an immediate attack.
	TributeChance          float64 `toml:"tribute_chance"`
	TributeDemandPct       float64 `toml:"tribute_demand_pct"`
	TributeDecisionSeconds float64 `toml:"tribute_decision_seconds"`

	// Escape sequence duration and pressure. Escaping is meant to read as a
	// short cooldown, not a second stress test — the real tension under
	// attack comes from AttackHullDamagePerSecond, not from the timer or
	// fuel burn.
	BaseEscapeSeconds         float64 `toml:"base_escape_seconds"`
	EscapeCargoExponent       float64 `toml:"escape_cargo_exponent"`
	CargoEscapePenaltySeconds float64 `toml:"cargo_escape_penalty_seconds"`
	// EscapeFuelDrainPerSec is intentionally much lower than FuelDrainBase —
	// the escape burn itself should barely sip the tank.
	EscapeFuelDrainPerSec          float64 `toml:"escape_fuel_drain_per_sec"`
	FuelOutEscapePenaltyPerSec     float64 `toml:"fuel_out_escape_penalty_per_sec"`
	FuelOutEscapePenaltyCapSeconds float64 `toml:"fuel_out_escape_penalty_cap_seconds"`
	AttackHullDamagePerSecond      float64 `toml:"attack_hull_damage_per_second"`

	// Fraction of an asteroid's units that must remain (of the original)
	// for a bailed/tribute-paid/escaped rock to stay in the belt instead of
	// being swept away as "too scattered to reacquire."
	RemnantKeepThreshold float64 `toml:"remnant_keep_threshold"`

	// Skill-check "drill calibration" minigame during mining: a countdown
	// appears every so often, and pressing the hotkey before it expires
	// grants the mining-progress bonus below. There is no moving target to
	// track — over SSH, timing a press against a sweeping marker is a
	// latency test, not a skill test.
	SkillCheckMinIntervalSeconds float64 `toml:"skill_check_min_interval_seconds"`
	SkillCheckMaxIntervalSeconds float64 `toml:"skill_check_max_interval_seconds"`
	SkillCheckWindowSeconds      float64 `toml:"skill_check_window_seconds"`
	// SkillCheckBonusPct is a fraction of the asteroid's *remaining* volume,
	// not a flat time amount — a flat "seconds of mining" bonus scales
	// inversely with drill speed and would gut small/fast asteroids almost
	// instantly while barely denting slow/large ones.
	SkillCheckBonusPct float64 `toml:"skill_check_bonus_pct"`

	// Fuzzed pirate ETA range shown on the mining-screen radar.
	EtaBaseUncertaintyPct   float64 `toml:"eta_base_uncertainty_pct"`
	EtaSurveyorReductionPct float64 `toml:"eta_surveyor_reduction_pct"`
	EtaMinUncertaintyPct    float64 `toml:"eta_min_uncertainty_pct"`
}

// EventsConfig tunes random mining/escape events. The lower the hull
// percentage, the higher the chance the next event is bad.
type EventsConfig struct {
	BaseChancePerMinute float64 `toml:"base_chance_per_minute"`
	LowHullChanceBonus  float64 `toml:"low_hull_chance_bonus"`
	BaseBadWeight       float64 `toml:"base_bad_weight"`
	LowHullBadWeight    float64 `toml:"low_hull_bad_weight"`
	CooldownSeconds     float64 `toml:"cooldown_seconds"`

	PowerOutageSeconds          float64 `toml:"power_outage_seconds"`
	RadarBlackoutSeconds        float64 `toml:"radar_blackout_seconds"`
	LifeSupportCountdownSeconds float64 `toml:"life_support_countdown_seconds"`
	LifeSupportBleedPerSecond   int     `toml:"life_support_bleed_per_second"`
	CargoShiftEffectSeconds     float64 `toml:"cargo_shift_effect_seconds"`
	CargoShiftEscapePct         float64 `toml:"cargo_shift_escape_pct"`
	ReactorSurgeSeconds         float64 `toml:"reactor_surge_seconds"`
	ReactorSurgeFuelMul         float64 `toml:"reactor_surge_fuel_mul"`
	ReactorSurgeHullHitChance   float64 `toml:"reactor_surge_hull_hit_chance"`
	ReactorSurgeHullHitAmount   int     `toml:"reactor_surge_hull_hit_amount"`
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

// StarterShipID is the free, always-rebuyable-at-zero-cost fallback ship
// every new pilot owns. gameplay/05-fleet-ships-and-shipyard-economy.md
// requires a ship with this ID, price 0.
const StarterShipID = "skiff"

// ShipModel is one purchasable hull design (gameplay/05's "4 starter
// ships"). Track *Start/*Cap fields are grades 0..5 (E..S) — the range this
// specific hull can ever install for that track. UtilitySlots/WeaponSlots
// are slot counts; every ship has exactly one Internal slot (not
// configurable per-ship, per gameplay/05).
type ShipModel struct {
	ID    string `toml:"id"`
	Name  string `toml:"name"`
	Brand string `toml:"brand"` // federation | alliance | independent
	Class string `toml:"class"` // miner | fighter | freighter

	Price   int     `toml:"price"`
	TierMul float64 `toml:"tier_mul"` // scales track-upgrade prices for this hull

	BaseMass  float64 `toml:"base_mass"`
	BaseCargo float64 `toml:"base_cargo"` // unit capacity before Extra Cargo devices

	UtilitySlots int `toml:"utility_slots"`
	WeaponSlots  int `toml:"weapon_slots"`

	ThrustersStart int `toml:"thrusters_start"`
	ThrustersCap   int `toml:"thrusters_cap"`
	HullStart      int `toml:"hull_start"`
	HullCap        int `toml:"hull_cap"`
	FuelEffStart   int `toml:"fuel_eff_start"`
	FuelEffCap     int `toml:"fuel_eff_cap"`
	PowerGenStart  int `toml:"power_gen_start"`
	PowerGenCap    int `toml:"power_gen_cap"`
	ScannerStart   int `toml:"scanner_start"`
	ScannerCap     int `toml:"scanner_cap"`
}

// FleetConfig holds the shared formulas driving ship-track grade prices and
// effects, the power/mass system, the Scanner distance lock, the
// distance-biased rarity nudge, the random-event hull gate, and ship
// buyback — see gameplay/05-fleet-ships-and-shipyard-economy.md.
type FleetConfig struct {
	GradePriceCurve float64 `toml:"grade_price_curve"`

	ThrustersBasePrice int `toml:"thrusters_base_price"`
	HullBasePrice      int `toml:"hull_base_price"`
	FuelEffBasePrice   int `toml:"fuel_eff_base_price"`
	PowerGenBasePrice  int `toml:"power_gen_base_price"`
	ScannerBasePrice   int `toml:"scanner_base_price"`

	ThrusterEscapeMul float64 `toml:"thruster_escape_mul"` // multiplicative, per grade
	FuelEffMul        float64 `toml:"fuel_eff_mul"`        // multiplicative, per grade
	HullPoolPerGrade  int     `toml:"hull_pool_per_grade"` // additive to the 100 base

	PowerCapacityBase     int     `toml:"power_capacity_base"`
	PowerCapacityPerGrade int     `toml:"power_capacity_per_grade"`
	PowerCurveExponent    float64 `toml:"power_curve_exponent"`

	ScannerLockBaseKm     float64 `toml:"scanner_lock_base_km"`
	ScannerLockPerGradeKm float64 `toml:"scanner_lock_per_grade_km"`
	ScannerLockMaxKm      float64 `toml:"scanner_lock_max_km"`
	// ScannerEtaBonusPct applies only at grades beyond B (grade 4=A, 5=S),
	// narrowing pirate ETA uncertainty an extra (grade-3)*this per grade.
	ScannerEtaBonusPct float64 `toml:"scanner_eta_bonus_pct"`

	MassFuelCoefficient   float64 `toml:"mass_fuel_coefficient"`
	MassEscapeCoefficient float64 `toml:"mass_escape_coefficient"`

	DistanceRarityBonusMax float64 `toml:"distance_rarity_bonus_max"`
	EventHullGatePct       float64 `toml:"event_hull_gate_pct"`
	BuybackPricePct        float64 `toml:"buyback_price_pct"`
}

// SlotsConfig holds slot-device pricing, power draw, mass, and effect
// magnitudes. Every *BasePrice scales via FleetConfig.GradePriceCurve^grade.
// Extra Cargo and Extra Fuel Tank intentionally have no power fields — they
// never draw power, the one explicit exception in the slot power system.
type SlotsConfig struct {
	CargoBasePrice    int     `toml:"cargo_base_price"`
	CargoPerGrade     float64 `toml:"cargo_per_grade"`
	CargoMassPerGrade float64 `toml:"cargo_mass_per_grade"`

	FuelTankBasePrice    int     `toml:"fuel_tank_base_price"`
	FuelTankPerGrade     float64 `toml:"fuel_tank_per_grade"`
	FuelTankMassPerGrade float64 `toml:"fuel_tank_mass_per_grade"`

	ShieldBasePrice    int     `toml:"shield_base_price"`
	ShieldPowerK       float64 `toml:"shield_power_k"`
	ShieldMassPerGrade float64 `toml:"shield_mass_per_grade"`
	ShieldHPPerGrade   float64 `toml:"shield_hp_per_grade"`

	ChaffBasePrice    int     `toml:"chaff_base_price"`
	ChaffPowerK       float64 `toml:"chaff_power_k"`
	ChaffMassPerGrade float64 `toml:"chaff_mass_per_grade"`
	ChaffBaseSeconds  float64 `toml:"chaff_base_seconds"`

	TurretBasePrice    int     `toml:"turret_base_price"`
	TurretPowerK       float64 `toml:"turret_power_k"`
	TurretMassPerGrade float64 `toml:"turret_mass_per_grade"`
	TurretPctPerGrade  float64 `toml:"turret_pct_per_grade"`

	InternalPowerK       float64 `toml:"internal_power_k"`
	InternalMassPerGrade float64 `toml:"internal_mass_per_grade"`

	SeismicBasePrice int `toml:"seismic_base_price"`
	SeismicScanCount int `toml:"seismic_scan_count"`

	FuelMinerBasePrice   int     `toml:"fuel_miner_base_price"`
	FuelMinerBasePct     float64 `toml:"fuel_miner_base_pct"`
	FuelMinerPerGradePct float64 `toml:"fuel_miner_per_grade_pct"`

	JammerBasePrice     int `toml:"jammer_base_price"`
	JammerGradeUsesStep int `toml:"jammer_grade_uses_step"`

	JumpDriveBasePrice int `toml:"jump_drive_base_price"`
}

type worldsFile struct {
	Worlds []World `toml:"worlds"`
}

type balanceFile struct {
	Pilot  PilotStart   `toml:"pilot"`
	Belt   BeltConfig   `toml:"belt"`
	Tiers  TierConfig   `toml:"tiers"`
	Mining MiningConfig `toml:"mining"`
	Events EventsConfig `toml:"events"`
	Port   PortConfig   `toml:"port"`
	Fleet  FleetConfig  `toml:"fleet"`
	Slots  SlotsConfig  `toml:"slots"`
	Ships  []ShipModel  `toml:"ships"`
}

// Content is immutable validated game configuration.
type Content struct {
	Worlds []World
	Pilot  PilotStart
	Belt   BeltConfig
	Tiers  TierConfig
	Mining MiningConfig
	Events EventsConfig
	Port   PortConfig
	Fleet  FleetConfig
	Slots  SlotsConfig
	Ships  []ShipModel
}

// ShipByID returns the ship model with the given ID, or nil.
func (c *Content) ShipByID(id string) *ShipModel {
	for i := range c.Ships {
		if c.Ships[i].ID == id {
			return &c.Ships[i]
		}
	}
	return nil
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
		Worlds: wf.Worlds,
		Pilot:  bf.Pilot,
		Belt:   bf.Belt,
		Tiers:  bf.Tiers,
		Mining: bf.Mining,
		Events: bf.Events,
		Port:   bf.Port,
		Fleet:  bf.Fleet,
		Slots:  bf.Slots,
		Ships:  bf.Ships,
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
	if c.Mining.TributeChance < 0 || c.Mining.TributeChance > 1 {
		return fmt.Errorf("content: mining tribute_chance must be within 0..1")
	}
	if c.Mining.TributeDemandPct < 0 || c.Mining.TributeDemandPct > 1 {
		return fmt.Errorf("content: mining tribute_demand_pct must be within 0..1")
	}
	if c.Mining.BaseEscapeSeconds <= 0 {
		return fmt.Errorf("content: mining base_escape_seconds must be positive")
	}
	if c.Mining.EscapeFuelDrainPerSec < 0 {
		return fmt.Errorf("content: mining escape_fuel_drain_per_sec must be non-negative")
	}
	if c.Mining.FuelOutEscapePenaltyCapSeconds < 0 {
		return fmt.Errorf("content: mining fuel_out_escape_penalty_cap_seconds must be non-negative")
	}
	if c.Mining.RemnantKeepThreshold < 0 || c.Mining.RemnantKeepThreshold > 1 {
		return fmt.Errorf("content: mining remnant_keep_threshold must be within 0..1")
	}
	if c.Mining.SkillCheckMinIntervalSeconds <= 0 || c.Mining.SkillCheckMaxIntervalSeconds < c.Mining.SkillCheckMinIntervalSeconds {
		return fmt.Errorf("content: mining skill_check_min/max_interval_seconds must be positive and ascending")
	}
	if c.Mining.SkillCheckWindowSeconds <= 0 {
		return fmt.Errorf("content: mining skill_check_window_seconds must be positive")
	}
	if c.Mining.SkillCheckBonusPct <= 0 || c.Mining.SkillCheckBonusPct > 1 {
		return fmt.Errorf("content: mining skill_check_bonus_pct must be within (0..1]")
	}
	if c.Mining.EtaBaseUncertaintyPct < c.Mining.EtaMinUncertaintyPct || c.Mining.EtaMinUncertaintyPct < 0 {
		return fmt.Errorf("content: mining eta_base_uncertainty_pct must be >= eta_min_uncertainty_pct >= 0")
	}
	if c.Port.RefuelPerPoint < 1 || c.Port.RepairPerPoint < 1 {
		return fmt.Errorf("content: port prices must be positive")
	}
	if err := c.validateFleet(); err != nil {
		return err
	}
	return nil
}

var validBrands = map[string]bool{"federation": true, "alliance": true, "independent": true}
var validClasses = map[string]bool{"miner": true, "fighter": true, "freighter": true}

const maxGrade = 5

func (c *Content) validateFleet() error {
	if c.Fleet.GradePriceCurve <= 1 {
		return fmt.Errorf("content: fleet grade_price_curve must be > 1")
	}
	if c.Fleet.PowerCapacityBase < 0 || c.Fleet.PowerCapacityPerGrade < 0 {
		return fmt.Errorf("content: fleet power capacity fields must be non-negative")
	}
	if c.Fleet.PowerCurveExponent <= 0 {
		return fmt.Errorf("content: fleet power_curve_exponent must be positive")
	}
	if c.Fleet.ScannerLockBaseKm <= 0 || c.Fleet.ScannerLockMaxKm < c.Fleet.ScannerLockBaseKm {
		return fmt.Errorf("content: fleet scanner_lock_base_km must be positive and <= scanner_lock_max_km")
	}
	if c.Fleet.EventHullGatePct <= 0 || c.Fleet.EventHullGatePct > 1 {
		return fmt.Errorf("content: fleet event_hull_gate_pct must be within (0..1]")
	}
	if c.Fleet.BuybackPricePct <= 0 || c.Fleet.BuybackPricePct >= 1 {
		return fmt.Errorf("content: fleet buyback_price_pct must be within (0..1)")
	}
	if len(c.Ships) < 4 {
		return fmt.Errorf("content: need at least 4 ships, got %d", len(c.Ships))
	}
	seen := make(map[string]bool, len(c.Ships))
	haveStarter := false
	for i, sm := range c.Ships {
		if sm.ID == "" {
			return fmt.Errorf("content: ship #%d has no id", i+1)
		}
		if seen[sm.ID] {
			return fmt.Errorf("content: duplicate ship id %q", sm.ID)
		}
		seen[sm.ID] = true
		if sm.Name == "" {
			return fmt.Errorf("content: ship %q has no name", sm.ID)
		}
		if !validBrands[sm.Brand] {
			return fmt.Errorf("content: ship %q has invalid brand %q", sm.ID, sm.Brand)
		}
		if !validClasses[sm.Class] {
			return fmt.Errorf("content: ship %q has invalid class %q", sm.ID, sm.Class)
		}
		if sm.Price < 0 {
			return fmt.Errorf("content: ship %q price must be non-negative", sm.ID)
		}
		if sm.TierMul <= 0 {
			return fmt.Errorf("content: ship %q tier_mul must be positive", sm.ID)
		}
		if sm.BaseMass <= 0 || sm.BaseCargo <= 0 {
			return fmt.Errorf("content: ship %q base_mass/base_cargo must be positive", sm.ID)
		}
		if sm.UtilitySlots < 0 || sm.WeaponSlots < 0 {
			return fmt.Errorf("content: ship %q slot counts must be non-negative", sm.ID)
		}
		tracks := []struct {
			name        string
			start, cap_ int
		}{
			{"thrusters", sm.ThrustersStart, sm.ThrustersCap},
			{"hull", sm.HullStart, sm.HullCap},
			{"fuel_eff", sm.FuelEffStart, sm.FuelEffCap},
			{"power_gen", sm.PowerGenStart, sm.PowerGenCap},
			{"scanner", sm.ScannerStart, sm.ScannerCap},
		}
		for _, tr := range tracks {
			if tr.start < 0 || tr.cap_ > maxGrade || tr.start > tr.cap_ {
				return fmt.Errorf("content: ship %q track %s start/cap out of range (start=%d cap=%d)", sm.ID, tr.name, tr.start, tr.cap_)
			}
		}
		if sm.ID == StarterShipID {
			haveStarter = true
			if sm.Price != 0 {
				return fmt.Errorf("content: starter ship %q must be free", StarterShipID)
			}
		}
	}
	if !haveStarter {
		return fmt.Errorf("content: missing required starter ship %q", StarterShipID)
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
