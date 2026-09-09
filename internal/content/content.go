// Package content loads and validates game data from TOML.
package content

import (
	"fmt"
	"io/fs"
	"math"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/mynameis-nigel/ssh-moonminer/data"
)

// System is one large travel region on the star chart.
type System struct {
	SalvageAdvance    int      `toml:"salvage_advance"`
	ID                string   `toml:"id"`
	Name              string   `toml:"name"`
	Links             []string `toml:"links"`
	StartsUnlocked    bool     `toml:"starts_unlocked"`
	RequiredRating    int      `toml:"required_rating"`
	Signature         string   `toml:"signature"`
	Pressure          string   `toml:"pressure"`
	Hostile           bool     `toml:"hostile"`
	RequiredShipClass string   `toml:"required_ship_class"`
}

// World is one mineable destination on the star chart. The retained Go name
// keeps the deterministic belt API compact while TOML calls these entries
// destinations.
type World struct {
	ID                   string  `toml:"id"`
	SystemID             string  `toml:"system_id"`
	Name                 string  `toml:"name"`
	Sub                  string  `toml:"sub"`
	Ring                 string  `toml:"ring"`
	PirateLabel          string  `toml:"pirate_label"`
	PirateMul            float64 `toml:"pirate_mul"`
	RarityBias           float64 `toml:"rarity_bias"`
	TravelFuel           int     `toml:"travel_fuel"`
	RarityLabel          string  `toml:"rarity_label"`
	Desc                 string  `toml:"desc"`
	Art                  string  `toml:"art"`
	StartsUnlocked       bool    `toml:"starts_unlocked"`
	RequiredFuelCapacity float64 `toml:"required_fuel_capacity"`
	PermitFee            int     `toml:"permit_fee"`
	RequiredShipClass    string  `toml:"required_ship_class"`
	DrillTimeMul         float64 `toml:"drill_time_mul"`
	PirateStartDistance  float64 `toml:"pirate_start_distance"`
	PiratesAlwaysAttack  bool    `toml:"pirates_always_attack"`
	PirateAttackMul      float64 `toml:"pirate_attack_mul"`
	InstabilityPerSecond float64 `toml:"instability_per_second"`
}

// Pirate is one named roster entry — docs/gameplay/07-pirate-combat-and-
// bounties.md. Spawn is a weighted pick (salt 7100, rolled at Lock) among
// entries whose [MinThreat, MaxThreat] band covers
// threat = asteroid.Risk/100 * world.PirateMul.
type Pirate struct {
	ID              string  `toml:"id"`
	Name            string  `toml:"name"`
	Hull            float64 `toml:"hull"`
	DamagePerSecond float64 `toml:"damage_per_second"`
	Maneuver        float64 `toml:"maneuver"`
	Bounty          int     `toml:"bounty"`
	MinThreat       float64 `toml:"min_threat"`
	MaxThreat       float64 `toml:"max_threat"`
	Weight          float64 `toml:"weight"`
}

// CombatConfig tunes the tactical-scope pirate-combat minigame — docs/
// gameplay/07-pirate-combat-and-bounties.md. Bearing/Range/Solution are a
// deterministic function of the run's TickCount so replay stays exact.
type CombatConfig struct {
	HeatCapacity        float64 `toml:"heat_capacity"`
	HeatDecayPerSecond  float64 `toml:"heat_decay_per_second"`
	OverheatLockSeconds float64 `toml:"overheat_lock_seconds"`
	ArcCenter           float64 `toml:"arc_center"`
	ArcHalfWidth        float64 `toml:"arc_half_width"`
	SolutionRangeFloor  float64 `toml:"solution_range_floor"`
	RangeMin            float64 `toml:"range_min"`
	ManeuverW1          float64 `toml:"maneuver_w1"`
	ManeuverW2          float64 `toml:"maneuver_w2"`
	ManeuverW3          float64 `toml:"maneuver_w3"`
	ManeuverAmp1        float64 `toml:"maneuver_amp1"`
	ManeuverAmp2        float64 `toml:"maneuver_amp2"`
	OddsAvgSolutionBase float64 `toml:"odds_avg_solution_base"`
	OddsFloor           float64 `toml:"odds_floor"`
	OddsCeiling         float64 `toml:"odds_ceiling"`
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

	// Pressure-point skill checks during mining. Every run rolls one to three
	// points on the asteroid; each point gets one timed interaction and a hit
	// removes a meaningful fraction of the ore still left in the rock.
	SkillCheckMinIntervalSeconds float64 `toml:"skill_check_min_interval_seconds"`
	SkillCheckMaxIntervalSeconds float64 `toml:"skill_check_max_interval_seconds"`
	SkillCheckWindowSeconds      float64 `toml:"skill_check_window_seconds"`
	PressurePointsMin            int     `toml:"pressure_points_min"`
	PressurePointsMax            int     `toml:"pressure_points_max"`
	// SkillCheckBonusPct is a fraction of the asteroid's remaining volume.
	// This makes every successful pressure point substantial without making a
	// small or fast asteroid disproportionately valuable.
	SkillCheckBonusPct float64 `toml:"skill_check_bonus_pct"`

	// FuelAsteroidChance is independent of distance and rarity. The roll is
	// deterministic for a generated belt, but only an equipped Fuel Miner can
	// identify the result on the mining screen.
	FuelAsteroidChance float64 `toml:"fuel_asteroid_chance"`

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
// are slot counts; InternalSlots is one except on the two-slot Lantern.
type ShipModel struct {
	ID    string `toml:"id"`
	Name  string `toml:"name"`
	Brand string `toml:"brand"` // federation | alliance | independent
	Class string `toml:"class"` // miner | fighter | freighter

	Price   int     `toml:"price"`
	TierMul float64 `toml:"tier_mul"` // scales track-upgrade prices for this hull

	BaseMass  float64 `toml:"base_mass"`
	BaseCargo float64 `toml:"base_cargo"` // unit capacity before Extra Cargo devices

	UtilitySlots  int      `toml:"utility_slots"`
	WeaponSlots   int      `toml:"weapon_slots"`
	InternalSlots int      `toml:"internal_slots"`
	SoldIn        []string `toml:"sold_in"`
	NoBuyback     bool     `toml:"no_buyback"`
	NoShield      bool     `toml:"no_shield"`
	DockRepairPct float64  `toml:"dock_repair_pct"`

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
	// ScannerScanMul compounds per Scanner grade; lower is faster.
	ScannerScanMul       float64 `toml:"scanner_scan_mul"`
	ScannerInstantScanKm float64 `toml:"scanner_instant_scan_km"`
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
	// SellValuePct is the shipyard's remove-and-sell refund fraction (of the
	// device's current buy price), used by sim.SlotItemSellValue.
	SellValuePct float64 `toml:"sell_value_pct"`

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
	// ShieldRechargeESeconds / ShieldRechargeSSeconds are the full-recharge
	// times at E and S grade; intermediate grades interpolate linearly.
	ShieldRechargeESeconds float64 `toml:"shield_recharge_e_seconds"`
	ShieldRechargeSSeconds float64 `toml:"shield_recharge_s_seconds"`
	// ShieldBurstReturnPct caps a burst shield's belt-side recovery until it
	// is serviced at dock.
	ShieldBurstReturnPct float64 `toml:"shield_burst_return_pct"`

	SeismicOverchargeBasePrice     int     `toml:"seismic_overcharge_base_price"`
	SeismicOverchargePowerK        float64 `toml:"seismic_overcharge_power_k"`
	SeismicOverchargeMassPerGrade  float64 `toml:"seismic_overcharge_mass_per_grade"`
	SeismicOverchargeSpeedE        float64 `toml:"seismic_overcharge_speed_e"`
	SeismicOverchargeSpeedPerGrade float64 `toml:"seismic_overcharge_speed_per_grade"`

	EMPLauncherBasePrice      int     `toml:"emp_launcher_base_price"`
	EMPLauncherPowerK         float64 `toml:"emp_launcher_power_k"`
	EMPLauncherMassPerGrade   float64 `toml:"emp_launcher_mass_per_grade"`
	EMPLauncherDeployESeconds float64 `toml:"emp_launcher_deploy_e_seconds"`
	EMPLauncherDeploySSeconds float64 `toml:"emp_launcher_deploy_s_seconds"`

	// Weapon catalog (docs/gameplay/07-pirate-combat-and-bounties.md). The
	// autocannon fires continuously; the Missile Launcher and Pulse Laser use
	// separate player-triggered actions.
	TurretBasePrice             int     `toml:"turret_base_price"`
	TurretPowerK                float64 `toml:"turret_power_k"`
	TurretMassPerGrade          float64 `toml:"turret_mass_per_grade"`
	TurretDamagePerShotBase     float64 `toml:"turret_damage_per_shot_base"`
	TurretDamagePerShotPerGrade float64 `toml:"turret_damage_per_shot_per_grade"`
	TurretShotsPerSecond        float64 `toml:"turret_shots_per_second"`

	MissileLauncherBasePrice     int     `toml:"missile_launcher_base_price"`
	MissileLauncherPowerK        float64 `toml:"missile_launcher_power_k"`
	MissileLauncherMassPerGrade  float64 `toml:"missile_launcher_mass_per_grade"`
	MissileDamagePerShotBase     float64 `toml:"missile_damage_per_shot_base"`
	MissileDamagePerShotPerGrade float64 `toml:"missile_damage_per_shot_per_grade"`
	// MissileCapacityByGrade has one E..S capacity entry. Missiles are
	// always hits and use MissileCooldownSeconds rather than heat.
	MissileCapacityByGrade []int   `toml:"missile_capacity_by_grade"`
	MissileCooldownSeconds float64 `toml:"missile_cooldown_seconds"`

	PulseLaserBasePrice             int     `toml:"pulse_laser_base_price"`
	PulseLaserPowerK                float64 `toml:"pulse_laser_power_k"`
	PulseLaserMassPerGrade          float64 `toml:"pulse_laser_mass_per_grade"`
	PulseLaserDamagePerShotBase     float64 `toml:"pulse_laser_damage_per_shot_base"`
	PulseLaserDamagePerShotPerGrade float64 `toml:"pulse_laser_damage_per_shot_per_grade"`
	PulseLaserHeatPerShot           float64 `toml:"pulse_laser_heat_per_shot"`
	PulseLaserSolutionBonus         float64 `toml:"pulse_laser_solution_bonus"`

	InternalPowerK       float64 `toml:"internal_power_k"`
	InternalMassPerGrade float64 `toml:"internal_mass_per_grade"`

	SeismicBasePrice int `toml:"seismic_base_price"`
	SeismicScanCount int `toml:"seismic_scan_count"`

	FuelMinerBasePrice       int     `toml:"fuel_miner_base_price"`
	FuelMinerERecoveryMul    float64 `toml:"fuel_miner_e_recovery_mul"`
	FuelMinerPerGradeGainMul float64 `toml:"fuel_miner_per_grade_gain_mul"`

	JammerBasePrice       int       `toml:"jammer_base_price"`
	JammerGradeUsesStep   int       `toml:"jammer_grade_uses_step"`
	JammerDurationSeconds []float64 `toml:"jammer_duration_seconds"`

	HeatSinkBasePrice        int     `toml:"heat_sink_base_price"`
	HeatSinkCapacityPerGrade float64 `toml:"heat_sink_capacity_per_grade"`

	LegacyDriveBasePrice int `toml:"legacy_drive_base_price"`
}

type worldsFile struct {
	Systems      []System `toml:"systems"`
	Destinations []World  `toml:"destinations"`
}

type balanceFile struct {
	Jump    JumpConfig   `toml:"jump"`
	Ferry   FerryConfig  `toml:"ferry"`
	Gates   []Gate       `toml:"gates"`
	Ratings []Rating     `toml:"ratings"`
	Pilot   PilotStart   `toml:"pilot"`
	Belt    BeltConfig   `toml:"belt"`
	Tiers   TierConfig   `toml:"tiers"`
	Mining  MiningConfig `toml:"mining"`
	Combat  CombatConfig `toml:"combat"`
	Events  EventsConfig `toml:"events"`
	Port    PortConfig   `toml:"port"`
	Fleet   FleetConfig  `toml:"fleet"`
	Slots   SlotsConfig  `toml:"slots"`
	Ships   []ShipModel  `toml:"ships"`
}

type piratesFile struct {
	Pirates []Pirate `toml:"pirates"`
}

// Content is immutable validated game configuration.
type Content struct {
	Jump    JumpConfig
	Ferry   FerryConfig
	Gates   []Gate
	Ratings []Rating
	Systems []System
	Worlds  []World
	Pilot   PilotStart
	Belt    BeltConfig
	Tiers   TierConfig
	Mining  MiningConfig
	Combat  CombatConfig
	Events  EventsConfig
	Port    PortConfig
	Fleet   FleetConfig
	Slots   SlotsConfig
	Ships   []ShipModel
	Pirates []Pirate
}

// ShipByID returns the ship model with the given ID, or nil.
func (c *Content) ShipByID(id string) *ShipModel {
	id, _, _ = strings.Cut(id, "@")
	for i := range c.Ships {
		if c.Ships[i].ID == id {
			return &c.Ships[i]
		}
	}
	return nil
}

// PirateByID returns the pirate roster entry with the given ID, or nil.
func (c *Content) PirateByID(id string) *Pirate {
	for i := range c.Pirates {
		if c.Pirates[i].ID == id {
			return &c.Pirates[i]
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
	var pf piratesFile
	if err := decodeTOML(fsys, "pirates.toml", &pf); err != nil {
		return nil, err
	}
	c := &Content{
		Systems: wf.Systems,
		Jump:    bf.Jump, Ferry: bf.Ferry, Gates: bf.Gates, Ratings: bf.Ratings,
		Worlds:  wf.Destinations,
		Pilot:   bf.Pilot,
		Belt:    bf.Belt,
		Tiers:   bf.Tiers,
		Mining:  bf.Mining,
		Combat:  bf.Combat,
		Events:  bf.Events,
		Port:    bf.Port,
		Fleet:   bf.Fleet,
		Slots:   bf.Slots,
		Ships:   bf.Ships,
		Pirates: pf.Pirates,
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
	metadata, err := toml.Decode(string(b), v)
	if err != nil {
		return fmt.Errorf("content: parse %s: %w", name, err)
	}
	if keys := metadata.Undecoded(); len(keys) > 0 {
		return fmt.Errorf("content: %s has unknown keys: %v", name, keys)
	}
	return nil
}

func (c *Content) validate() error {
	if err := c.validateJump(); err != nil {
		return err
	}
	// TOML supports nan/inf; ordinary range comparisons do not reject NaN.
	// Validate all numeric leaves once at boot, including future tuning fields.
	if err := validateFinite(reflect.ValueOf(*c), "content"); err != nil {
		return err
	}
	if c.Pilot.StartFuel <= 0 || c.Pilot.StartHull <= 0 || c.Pilot.StartCredits < 0 {
		return fmt.Errorf("content: pilot fuel/hull must be positive and credits non-negative")
	}
	if len(c.Belt.NamePrefixes) == 0 {
		return fmt.Errorf("content: belt name_prefixes must not be empty")
	}
	if c.Belt.VolumeMin <= 0 || c.Belt.VolumeMax < c.Belt.VolumeMin || c.Belt.VolumeStep <= 0 || c.Belt.ValueStep <= 0 {
		return fmt.Errorf("content: belt volume bounds and volume/value steps must be positive and ordered")
	}
	if c.Belt.DrillSecPerVol <= 0 || c.Belt.DrillSecMin <= 0 || c.Belt.DrillSecMax < c.Belt.DrillSecMin || c.Belt.ValuePerVolume <= 0 {
		return fmt.Errorf("content: belt drill bounds/divisor and value_per_volume must be positive and ordered")
	}
	if c.Belt.ScanFuelCost < 0 {
		return fmt.Errorf("content: belt scan_fuel_cost must be non-negative")
	}
	if c.Mining.TickHz > int(time.Second) {
		return fmt.Errorf("content: mining tick_hz exceeds timer resolution")
	}

	if len(c.Systems) < 2 {
		return fmt.Errorf("content: need at least 2 systems, got %d", len(c.Systems))
	}
	systems := make(map[string]bool, len(c.Systems))
	for _, system := range c.Systems {
		if system.ID == "" || system.Name == "" {
			return fmt.Errorf("content: system id and name are required")
		}
		if systems[system.ID] {
			return fmt.Errorf("content: duplicate system id %q", system.ID)
		}
		if system.RequiredRating < -1 {
			return fmt.Errorf("content: system %q required_rating must be at least -1", system.ID)
		}
		if system.RequiredShipClass != "" && !validClasses[system.RequiredShipClass] {
			return fmt.Errorf("content: system %q has invalid required_ship_class %q", system.ID, system.RequiredShipClass)
		}

		systems[system.ID] = true
	}
	for _, system := range c.Systems {
		if len(system.Links) == 0 {
			return fmt.Errorf("content: system %q must have at least one outbound link", system.ID)
		}
		for _, linkedID := range system.Links {
			if linkedID == system.ID || !systems[linkedID] {
				return fmt.Errorf("content: system %q has invalid outbound link %q", system.ID, linkedID)
			}
			linked := c.SystemByID(linkedID)
			if linked == nil || !containsString(linked.Links, system.ID) {
				return fmt.Errorf("content: system link %q -> %q must be reciprocal", system.ID, linkedID)
			}
		}
	}
	if len(c.Worlds) < 4 {
		return fmt.Errorf("content: need at least 4 worlds, got %d", len(c.Worlds))
	}
	destinations := make(map[string]bool, len(c.Worlds))
	for i, w := range c.Worlds {
		if w.ID == "" || w.Name == "" {
			return fmt.Errorf("content: world #%d has no name", i+1)
		}
		if destinations[w.ID] || !systems[w.SystemID] {
			return fmt.Errorf("content: destination %q has duplicate id or unknown system", w.ID)
		}
		if w.PermitFee < 0 || w.RequiredFuelCapacity < 0 {
			return fmt.Errorf("content: destination %q has invalid progression gate", w.ID)
		}
		if w.RequiredShipClass != "" && !validClasses[w.RequiredShipClass] {
			return fmt.Errorf("content: destination %q has invalid required_ship_class %q", w.ID, w.RequiredShipClass)
		}
		if w.DrillTimeMul < 0 || w.PirateStartDistance < 0 || w.PirateAttackMul < 0 {
			return fmt.Errorf("content: destination %q has invalid danger modifiers", w.ID)
		}
		destinations[w.ID] = true
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
	if c.Mining.PressurePointsMin < 1 || c.Mining.PressurePointsMax < c.Mining.PressurePointsMin || c.Mining.PressurePointsMax > 3 {
		return fmt.Errorf("content: mining pressure_points_min/max must be 1..3 and ascending")
	}
	if c.Mining.SkillCheckBonusPct <= 0 || c.Mining.SkillCheckBonusPct > 1 {
		return fmt.Errorf("content: mining skill_check_bonus_pct must be within (0..1]")
	}
	if c.Mining.FuelAsteroidChance < 0 || c.Mining.FuelAsteroidChance > 1 {
		return fmt.Errorf("content: mining fuel_asteroid_chance must be within 0..1")
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
	if err := c.validateCombat(); err != nil {
		return err
	}
	return nil
}

// validateCombat checks the pirate roster and [combat] tuning — docs/
// gameplay/07-pirate-combat-and-bounties.md.
func (c *Content) validateCombat() error {
	if len(c.Pirates) == 0 {
		return fmt.Errorf("content: need at least 1 pirate, got 0")
	}
	seen := make(map[string]bool, len(c.Pirates))
	for _, p := range c.Pirates {
		if p.ID == "" || p.Name == "" {
			return fmt.Errorf("content: pirate has no id/name")
		}
		if seen[p.ID] {
			return fmt.Errorf("content: duplicate pirate id %q", p.ID)
		}
		seen[p.ID] = true
		if p.Hull <= 0 || p.DamagePerSecond <= 0 || p.Maneuver <= 0 || p.Weight <= 0 {
			return fmt.Errorf("content: pirate %q hull/damage_per_second/maneuver/weight must be positive", p.ID)
		}
		if p.Bounty < 0 {
			return fmt.Errorf("content: pirate %q bounty must be non-negative", p.ID)
		}
		if p.MinThreat < 0 || p.MaxThreat < p.MinThreat {
			return fmt.Errorf("content: pirate %q min_threat/max_threat must be non-negative and ascending", p.ID)
		}
	}
	cc := c.Combat
	if cc.HeatCapacity <= 0 || cc.HeatDecayPerSecond <= 0 || cc.OverheatLockSeconds <= 0 {
		return fmt.Errorf("content: combat heat_capacity/heat_decay_per_second/overheat_lock_seconds must be positive")
	}
	if cc.ArcHalfWidth <= 0 || cc.ArcHalfWidth > 0.5 {
		return fmt.Errorf("content: combat arc_half_width must be within (0..0.5]")
	}
	if cc.SolutionRangeFloor < 0 || cc.SolutionRangeFloor > 1 || cc.RangeMin < 0 || cc.RangeMin > 1 {
		return fmt.Errorf("content: combat solution_range_floor/range_min must be within 0..1")
	}
	if cc.OddsFloor < 0 || cc.OddsCeiling > 1 || cc.OddsCeiling < cc.OddsFloor {
		return fmt.Errorf("content: combat odds_floor/odds_ceiling must be within 0..1 and ascending")
	}
	if cc.OddsAvgSolutionBase <= 0 {
		return fmt.Errorf("content: combat odds_avg_solution_base must be positive")
	}
	if c.Slots.TurretShotsPerSecond <= 0 {
		return fmt.Errorf("content: slots turret_shots_per_second must be positive")
	}
	if len(c.Slots.JammerDurationSeconds) != maxGrade+1 {
		return fmt.Errorf("content: slots jammer_duration_seconds must have %d E..S entries", maxGrade+1)
	}
	for grade, seconds := range c.Slots.JammerDurationSeconds {
		if seconds <= 0 {
			return fmt.Errorf("content: slots jammer_duration_seconds[%d] must be positive", grade)
		}
	}
	if len(c.Slots.MissileCapacityByGrade) != maxGrade+1 {
		return fmt.Errorf("content: slots missile_capacity_by_grade must have %d E..S entries", maxGrade+1)
	}
	for grade, capacity := range c.Slots.MissileCapacityByGrade {
		if capacity < 3 || capacity > 10 || (grade > 0 && capacity < c.Slots.MissileCapacityByGrade[grade-1]) {
			return fmt.Errorf("content: slots missile_capacity_by_grade must ascend within 3..10")
		}
	}
	if c.Slots.MissileDamagePerShotBase <= 0 || c.Slots.MissileDamagePerShotPerGrade < 0 ||
		c.Slots.MissileCooldownSeconds <= 0 {
		return fmt.Errorf("content: missile launcher damage and cooldown must be positive")
	}
	if c.Slots.HeatSinkCapacityPerGrade <= 0 {
		return fmt.Errorf("content: slots heat_sink_capacity_per_grade must be positive")
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// SystemByID returns a system by its stable content ID.
func (c *Content) SystemByID(id string) *System {
	for i := range c.Systems {
		if c.Systems[i].ID == id {
			return &c.Systems[i]
		}
	}
	return nil
}

var validBrands = map[string]bool{"federation": true, "alliance": true, "independent": true, "frontier": true}
var validClasses = map[string]bool{"miner": true, "fighter": true, "freighter": true}

const maxGrade = 5

func (c *Content) validateFleet() error {
	if c.Fleet.GradePriceCurve <= 1 {
		return fmt.Errorf("content: fleet grade_price_curve must be > 1")
	}
	if c.Fleet.ThrusterEscapeMul <= 0 || c.Fleet.ThrusterEscapeMul > 1 {
		return fmt.Errorf("content: fleet thruster_escape_mul must be within (0..1]")
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
	if c.Fleet.ScannerScanMul <= 0 || c.Fleet.ScannerScanMul > 1 || c.Fleet.ScannerInstantScanKm < 0 {
		return fmt.Errorf("content: fleet scanner scan multiplier must be within (0..1] and instant range non-negative")
	}
	if c.Fleet.EventHullGatePct <= 0 || c.Fleet.EventHullGatePct > 1 {
		return fmt.Errorf("content: fleet event_hull_gate_pct must be within (0..1]")
	}
	if c.Fleet.BuybackPricePct <= 0 || c.Fleet.BuybackPricePct >= 1 {
		return fmt.Errorf("content: fleet buyback_price_pct must be within (0..1)")
	}
	if c.Slots.SellValuePct <= 0 || c.Slots.SellValuePct >= 1 {
		return fmt.Errorf("content: slots sell_value_pct must be within (0..1)")
	}
	if c.Slots.ShieldRechargeESeconds < 15 || c.Slots.ShieldRechargeSSeconds < 15 ||
		c.Slots.ShieldRechargeESeconds < c.Slots.ShieldRechargeSSeconds ||
		c.Slots.ShieldRechargeESeconds > 90 {
		return fmt.Errorf("content: shield recharge must be 15..90 seconds, slowest at E")
	}
	if c.Slots.ShieldBurstReturnPct <= 0 || c.Slots.ShieldBurstReturnPct > 1 {
		return fmt.Errorf("content: shield_burst_return_pct must be within (0..1]")
	}
	if c.Slots.SeismicOverchargeBasePrice <= 0 || c.Slots.SeismicOverchargePowerK <= 0 ||
		c.Slots.SeismicOverchargeMassPerGrade <= 0 || c.Slots.SeismicOverchargeSpeedE <= 1 ||
		c.Slots.SeismicOverchargeSpeedPerGrade < 0 {
		return fmt.Errorf("content: seismic overcharge price/power/mass/speed values are invalid")
	}
	if c.Slots.FuelMinerBasePrice <= 0 || c.Slots.FuelMinerERecoveryMul <= 0 || c.Slots.FuelMinerPerGradeGainMul < 0 {
		return fmt.Errorf("content: fuel miner price/recovery values are invalid")
	}
	if c.Slots.EMPLauncherDeployESeconds <= 0 || c.Slots.EMPLauncherDeploySSeconds < c.Slots.EMPLauncherDeployESeconds {
		return fmt.Errorf("content: EMP launcher deploy seconds must be positive and slowest at S")
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

// WorldByID returns a destination by its stable content ID.
func (c *Content) WorldByID(id string) *World {
	for i := range c.Worlds {
		if c.Worlds[i].ID == id {
			return &c.Worlds[i]
		}
	}
	return nil
}

// Reflection is confined to startup validation; simulation hot paths do not use it.
func validateFinite(v reflect.Value, path string) error {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := validateFinite(v.Field(i), path+"."+v.Type().Field(i).Name); err != nil {
				return err
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if err := validateFinite(v.Index(i), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.Float64:
		if math.IsNaN(v.Float()) || math.IsInf(v.Float(), 0) {
			return fmt.Errorf("%s must be finite", path)
		}
	}
	return nil
}
