package sim

import (
	"encoding/json"
	"fmt"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// StateVersion is the current save schema version. Version 5 renames the
// Chaff Launcher to EMP Launcher and persists its armed state.
const StateVersion = 5

// BeltViewMode is the default belt rendering mode.
type BeltViewMode int

const (
	BeltViewTiles BeltViewMode = iota
	BeltViewOreScan
	BeltViewRadar
)

// Settings are player tweak preferences persisted in the save.
type Settings struct {
	BeltView         BeltViewMode `json:"belt_view"`
	PirateAggression float64      `json:"pirate_aggression"`
	HighContrast     bool         `json:"high_contrast"`
	ASCIISafe        bool         `json:"ascii_safe"`
	ReducedMotion    bool         `json:"reduced_motion"`
	InsuranceUsed    bool         `json:"insurance_used"`
}

// TrackGrades holds a ship's five stat-track grades (0..5, E..S) — see
// gameplay/05-fleet-ships-and-shipyard-economy.md.
type TrackGrades struct {
	Thrusters int `json:"thrusters"`
	Hull      int `json:"hull"`
	FuelEff   int `json:"fuel_eff"`
	PowerGen  int `json:"power_gen"`
	Scanner   int `json:"scanner"`
}

// SlotDevice is one installed Utility/Weapon/Internal item and its grade
// (0..5, E..S).
type SlotDevice struct {
	ItemID   string `json:"item_id"`
	Grade    int    `json:"grade"`
	EMPArmed bool   `json:"emp_armed,omitempty"`
}

// ShipInstance is one owned, persistent hangar ship: its model, its own
// stat grades, and its own slot loadout. Utility/Weapon slices are
// fixed-length (model.UtilitySlots/WeaponSlots) with nil entries for empty
// slots; Internal is always exactly one slot per ship.
type ShipInstance struct {
	ModelID string `json:"model_id"`

	Grades   TrackGrades   `json:"grades"`
	Utility  []*SlotDevice `json:"utility,omitempty"`
	Weapon   []*SlotDevice `json:"weapon,omitempty"`
	Internal *SlotDevice   `json:"internal,omitempty"`

	// JammerCharges is how many more asteroids this run's Pirate Jammer
	// internal module can suppress before it must be rearmed at dock.
	// Rearmed to full on Dock(); irrelevant unless Internal is the jammer.
	JammerCharges int `json:"jammer_charges"`

	// ShieldHP persists the active Shield module's remaining absorption pool
	// between mining runs. ShieldDamaged is set when that pool is depleted;
	// a burst shield can only recover to its belt-side emergency charge until
	// the ship reaches dock for service.
	ShieldHP      float64 `json:"shield_hp"`
	ShieldDamaged bool    `json:"shield_damaged"`
}

// Stats tracks lifetime pilot statistics.
type Stats struct {
	RunsTotal            int     `json:"runs_total"`
	RunsDeparted         int     `json:"runs_departed"`
	RunsBailed           int     `json:"runs_bailed"`
	RunsTributePaid      int     `json:"runs_tribute_paid"`
	RunsEscapedUnderFire int     `json:"runs_escaped_under_fire"`
	ShipsLost            int     `json:"ships_lost"`
	ShipsPurchased       int     `json:"ships_purchased"`
	CreditsEarned        int     `json:"credits_earned"`
	CreditsSpent         int     `json:"credits_spent"`
	CargoValueSold       int     `json:"cargo_value_sold"`
	LegendariesMined     int     `json:"legendaries_mined"`
	FuelBurned           float64 `json:"fuel_burned"`
	InsuranceClaims      int     `json:"insurance_claims"`
	FirstSeen            int64   `json:"first_seen"`
	LastSeen             int64   `json:"last_seen"`
}

// RunRecord is one ship's-log entry.
type RunRecord struct {
	When                 int64    `json:"when"`
	World                string   `json:"world"`
	Asteroid             string   `json:"asteroid"`
	Tier                 int      `json:"tier"`
	Outcome              string   `json:"outcome"`
	CargoValueRecovered  int      `json:"cargo_value_recovered"`
	CargoValueLost       int      `json:"cargo_value_lost"`
	CargoValueJettisoned int      `json:"cargo_value_jettisoned,omitempty"`
	HullDelta            int      `json:"hull_delta"`
	FuelDelta            float64  `json:"fuel_delta"`
	Depleted             bool     `json:"depleted"`
	CargoValueSold       int      `json:"cargo_value_sold"`
	Events               []string `json:"events,omitempty"`
}

// Asteroid is one belt contact.
type Asteroid struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Tier     int     `json:"tier"`
	Volume   int     `json:"volume"`
	DrillSec float64 `json:"drill_sec"`
	Value    int     `json:"value"`
	FuelCost int     `json:"fuel_cost"`
	Risk     int     `json:"risk"`
	Dots     int     `json:"dots"`
	Size     string  `json:"size"`
	X        int     `json:"x"`
	Y        int     `json:"y"`
	Distance float64 `json:"distance"`
	Scanned  bool    `json:"scanned"`
}

// RunPhase identifies the stage of an in-progress mining run.
type RunPhase int

const (
	// PhaseMining is the drill transferring asteroid units into cargo.
	PhaseMining RunPhase = iota
	// PhaseTribute is pirates waiting for accept/refuse.
	PhaseTribute
	// PhaseEscaping is the ship trying to leave, possibly under attack.
	PhaseEscaping
)

// PirateAction is the pirates' chosen behavior once they arrive.
type PirateAction int

const (
	PirateActionNone PirateAction = iota
	PirateActionTribute
	PirateActionAttack
)

// EventKind identifies a random mining/escape event.
type EventKind string

const (
	EventPowerOutage        EventKind = "power_outage"
	EventRadarBlackout      EventKind = "radar_blackout"
	EventLifeSupportFailure EventKind = "life_support_failure"
	EventCargoShift         EventKind = "cargo_shift"
	EventReactorSurge       EventKind = "reactor_surge"
)

// HUDTreatment tells the TUI how to render the currently active event
// without it needing to re-derive event meaning.
type HUDTreatment string

const (
	HUDNone      HUDTreatment = ""
	HUDBlackout  HUDTreatment = "blackout"
	HUDStatic    HUDTreatment = "static"
	HUDRedPulse  HUDTreatment = "red_pulse"
	HUDJitter    HUDTreatment = "jitter"
	HUDAmberGlow HUDTreatment = "amber_glow"
)

// RunEvent is the currently active random event on a run, if any.
type RunEvent struct {
	Kind         EventKind    `json:"kind"`
	Remaining    float64      `json:"remaining"`
	HUDTreatment HUDTreatment `json:"hud_treatment"`
}

// RunEventRecord is a historical log entry of an event/pirate decision that
// occurred during the run, kept for the run summary.
type RunEventRecord struct {
	Kind EventKind `json:"kind"`
	At   int64     `json:"at"`
}

// SkillCheck is a periodic "drill calibration" prompt during mining: a
// countdown of Window seconds starts, and hitting AttemptSkillCheck any time
// before Elapsed reaches Window grants a mining-progress bonus. Missing
// costs nothing — it just auto-expires.
type SkillCheck struct {
	Elapsed float64 `json:"elapsed"`
	Window  float64 `json:"window"`
}

// ActiveRun is in-progress mining state (never persisted non-nil).
type ActiveRun struct {
	AsteroidID int      `json:"asteroid_id"`
	Phase      RunPhase `json:"phase"`

	// ExtractedUnits is permanently removed from the asteroid. HeldUnits is
	// the portion still aboard this ship; tribute may reduce it, but never
	// changes extraction/remnant accounting. JettisonedUnits records the
	// current asteroid's extracted cargo surrendered to pirates.
	ExtractedUnits  float64 `json:"extracted_units"`
	HeldUnits       float64 `json:"held_units"`
	JettisonedUnits float64 `json:"jettisoned_units"`
	CargoValue      int     `json:"cargo_value"`

	// MinedUnits is retained only for transient test/dev compatibility with
	// pre-accounting callers. Simulation code mirrors it to HeldUnits and
	// always uses ExtractedUnits/HeldUnits for authoritative behavior.
	MinedUnits float64 `json:"mined_units,omitempty"`

	TributeCargoBefore   int `json:"tribute_cargo_before,omitempty"`
	TributeDemand        int `json:"tribute_demand,omitempty"`
	TributeCargoRetained int `json:"tribute_cargo_retained,omitempty"`

	PirateDistance float64      `json:"pirate_distance"`
	PirateAction   PirateAction `json:"pirate_action"`
	// PirateBearing is a cosmetic 0..1 direction rolled once at Lock, giving
	// the radar-widget blip a fixed approach angle instead of teleporting.
	PirateBearing float64 `json:"pirate_bearing"`
	// PirateETAMin/Max are a deliberately fuzzed arrival estimate (seconds),
	// recomputed each mining tick; the TUI must never render the true,
	// exact arrival time derived from PirateDistance.
	PirateETAMin float64 `json:"pirate_eta_min"`
	PirateETAMax float64 `json:"pirate_eta_max"`

	Intent      OutcomeKind `json:"intent"` // bailed/departed decided when escape starts
	TributePaid bool        `json:"tribute_paid"`

	// SkillCheck is the currently active "drill calibration" minigame
	// prompt, if any. NextSkillCheckIn counts down to the next prompt.
	SkillCheck       *SkillCheck `json:"skill_check,omitempty"`
	NextSkillCheckIn float64     `json:"next_skill_check_in"`

	EscapeSecondsRequired     float64 `json:"escape_seconds_required"`
	BaseEscapeSecondsRequired float64 `json:"base_escape_seconds_required"`
	FuelOutSeconds            float64 `json:"fuel_out_seconds"`
	EscapeSecondsElapsed      float64 `json:"escape_seconds_elapsed"`
	UnderAttack               bool    `json:"under_attack"`

	TributeSecondsElapsed float64 `json:"tribute_seconds_elapsed"`

	ActiveEvent *RunEvent        `json:"active_event,omitempty"`
	EventLog    []RunEventRecord `json:"event_log,omitempty"`

	LifeSupportBreached  bool    `json:"life_support_breached"`
	CargoShiftPenaltyMul float64 `json:"cargo_shift_penalty_mul"`
	EventCooldown        float64 `json:"event_cooldown"`
	HullDamageCarry      float64 `json:"hull_damage_carry"`
	TickCount            uint64  `json:"tick_count"`

	StartHull int     `json:"start_hull"`
	StartFuel float64 `json:"start_fuel"`
	StartedAt int64   `json:"started_at"`

	// EMPDeployed limits a run to one EMP Launcher. EMPActive/EMPRemaining
	// delay pirate action while the pilot keeps mining after contact.
	EMPDeployed  bool    `json:"emp_deployed"`
	EMPActive    bool    `json:"emp_active"`
	EMPRemaining float64 `json:"emp_remaining"`
	// PirateImmune is set at Lock time when a Pirate Jammer charge was
	// consumed for this asteroid — pirates never approach for the run.
	PirateImmune bool `json:"pirate_immune"`
}

// ActiveScan is an in-progress sensor scan (never persisted non-nil).
type ActiveScan struct {
	AsteroidID int     `json:"asteroid_id"`
	Elapsed    float64 `json:"elapsed"`
	Duration   float64 `json:"duration"`
}

// State is the full authoritative save.
type State struct {
	Version   int        `json:"version"`
	Seed      uint64     `json:"seed"`
	BeltCount uint64     `json:"belt_count"`
	Credits   int        `json:"credits"`
	Fuel      float64    `json:"fuel"`
	Hull      int        `json:"hull"`
	WorldIdx  int        `json:"world_idx"`
	Belt      []Asteroid `json:"belt,omitempty"`

	// SystemID is the dock/belt system the pilot currently occupies. The
	// permit maps only record purchases; content-defined starter routes need
	// not be written into every save.
	SystemID           string          `json:"system_id"`
	SystemPermits      map[string]bool `json:"system_permits,omitempty"`
	DestinationPermits map[string]bool `json:"destination_permits,omitempty"`
	CargoUnits         float64         `json:"cargo_units,omitempty"`
	CargoValue         int             `json:"cargo_value,omitempty"`

	// Ships is the pilot's hangar: owned ship models keyed by ModelID, each
	// with its own persistent grades and loadout. ActiveShipID selects
	// which one is currently flown (its Hull/Fuel condition is State.Hull/
	// State.Fuel above — hangar ships otherwise sit fully maintained).
	// ShipsUnlocked records every model ever owned, so re-acquiring one
	// after losing it prices as a 25% buyback forever after, not a
	// time-limited window. See
	// docs/gameplay/05-fleet-ships-and-shipyard-economy.md.
	Ships         map[string]*ShipInstance `json:"ships"`
	ActiveShipID  string                   `json:"active_ship_id"`
	ShipsUnlocked map[string]bool          `json:"ships_unlocked,omitempty"`

	// Inventory holds slot devices removed via "store" (as opposed to sold)
	// rather than discarded — an account-wide pool any owned ship can
	// re-equip from for free via the shipyard's item picker. Not
	// ship-scoped, so moving a device between hangar ships costs nothing
	// once it's been stored.
	Inventory []*SlotDevice `json:"inventory,omitempty"`

	Settings Settings    `json:"settings"`
	Stats    Stats       `json:"stats"`
	RunLog   []RunRecord `json:"run_log,omitempty"`
	Run      *ActiveRun  `json:"run,omitempty"`
	Scan     *ActiveScan `json:"scan,omitempty"`

	// DevGodMode disables hull damage. It is a dev-server-only debug flag.
	// It has a real json tag so it survives Clone()'s JSON round-trip
	// in-memory, but Encode() (used for the persisted DB payload) always
	// zeroes it first so it can never leak into a saved pilot file.
	DevGodMode bool `json:"dev_god_mode,omitempty"`
}

// Snapshot is a value copy for rendering.
type Snapshot struct {
	State State
}

// New creates a fresh pilot save, owning the free starter ship.
func New(c *content.Content, seed uint64, now int64) *State {
	s := &State{
		Version:   StateVersion,
		Seed:      seed,
		BeltCount: 0,
		Credits:   c.Pilot.StartCredits,
		WorldIdx:  -1,
		SystemID:  "sol",
		Settings: Settings{
			BeltView:         BeltViewTiles,
			PirateAggression: 1.0,
		},
		Stats: Stats{FirstSeen: now, LastSeen: now},
	}
	grantStarterShip(s, c)
	s.Fuel = FuelCapacity(s, c)
	s.Hull = MaxHull(s, c)
	return s
}

// Encode serializes state for storage. Run is cleared before encode.
func (s *State) Encode() ([]byte, error) {
	copy := s.Clone()
	copy.Run = nil
	copy.Scan = nil
	copy.DevGodMode = false
	return json.Marshal(copy)
}

// Clone returns a deep copy of the state.
func (s *State) Clone() *State {
	if s == nil {
		return nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	var out State
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return &out
}

// DecodeState parses and upgrades stored state. c is required to build a
// fresh starter ship when migrating a pre-fleet (version < 2) save.
func DecodeState(b []byte, c *content.Content) (*State, error) {
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("sim: decode state: %w", err)
	}
	savedVersion := s.Version
	if savedVersion > StateVersion {
		return nil, fmt.Errorf("sim: unsupported state version %d", s.Version)
	}
	if s.Version < StateVersion {
		s.Version = StateVersion
	}
	if s.Settings.PirateAggression == 0 {
		s.Settings.PirateAggression = 1.0
	}
	if s.SystemID == "" {
		s.SystemID = "sol"
		if s.WorldIdx >= 0 {
			if world := c.WorldByIndex(s.WorldIdx); world != nil {
				s.SystemID = world.SystemID
			}
		}
	}
	if s.SystemPermits == nil {
		s.SystemPermits = make(map[string]bool)
	}
	if s.DestinationPermits == nil {
		s.DestinationPermits = make(map[string]bool)
	}
	activeUnresolvable := s.Ships == nil || s.ActiveShipID == "" || s.Ships[s.ActiveShipID] == nil ||
		c.ShipByID(s.Ships[s.ActiveShipID].ModelID) == nil
	if activeUnresolvable {
		// Pre-fleet save (or a corrupted/dangling active-ship reference):
		// grant a fresh starter ship rather than leaving ActiveShip()
		// unresolvable. Old per-track Upgrades levels have no clean mapping
		// onto the new grade/slot system and are intentionally not carried
		// over.
		s.Ships = nil
		s.ActiveShipID = ""
		grantStarterShip(&s, c)
		s.Fuel = FuelCapacity(&s, c)
		s.Hull = MaxHull(&s, c)
	} else {
		// Drop any other hangar entries referencing a ship model no longer
		// in content (e.g. removed from data/*.toml) so later lookups like
		// CargoCapacityUnits can't silently resolve against a nil model.
		for id, inst := range s.Ships {
			if id != s.ActiveShipID && c.ShipByID(inst.ModelID) == nil {
				delete(s.Ships, id)
				delete(s.ShipsUnlocked, id)
			}
		}
	}
	s.Run = nil  // never restore mid-run
	s.Scan = nil // never restore mid-scan
	if savedVersion < 3 {
		// Old saves reset a shield on every Lock and therefore have no
		// persistent charge. Give installed shields a full starting charge
		// rather than treating every existing pilot's shield as burst.
		for shipID := range s.Ships {
			if ShieldMaxHPFor(&s, c, shipID) > 0 {
				restoreShipShieldFull(&s, c, shipID)
			}
		}
	}
	if savedVersion < 5 {
		migrateEMPLaunchers(&s)
	}
	return &s, nil
}

// migrateEMPLaunchers preserves old Chaff Launcher purchases as loaded EMP
// Launchers. It handles installed devices and shipyard storage alike.
func migrateEMPLaunchers(s *State) {
	migrateDevice := func(d *SlotDevice) {
		if d != nil && d.ItemID == "chaff" {
			d.ItemID = ItemEMPLauncher
			d.EMPArmed = true
		}
	}
	for _, ship := range s.Ships {
		if ship == nil {
			continue
		}
		for _, d := range ship.Utility {
			migrateDevice(d)
		}
		for _, d := range ship.Weapon {
			migrateDevice(d)
		}
		migrateDevice(ship.Internal)
	}
	for _, d := range s.Inventory {
		migrateDevice(d)
	}
}

// IsDocked reports whether the pilot is at the star chart.
func (s *State) IsDocked() bool { return s.WorldIdx < 0 }
