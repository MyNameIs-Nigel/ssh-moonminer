package sim

import (
	"encoding/json"
	"fmt"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// StateVersion is the current save schema version.
const StateVersion = 1

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

// Upgrades holds ship upgrade levels (0-4 per track). Upgrades represent
// installed equipment on the pilot's active (disposable) ship — they are
// wiped to zero on ship loss, per the "ships are lives" design.
type Upgrades struct {
	Drill    int `json:"drill"`
	Tank     int `json:"tank"`
	Plating  int `json:"plating"`
	Damper   int `json:"damper"`
	Surveyor int `json:"surveyor"`
}

// Stats tracks lifetime pilot statistics.
type Stats struct {
	RunsTotal            int     `json:"runs_total"`
	RunsDeparted         int     `json:"runs_departed"`
	RunsBailed           int     `json:"runs_bailed"`
	RunsTributePaid      int     `json:"runs_tribute_paid"`
	RunsEscapedUnderFire int     `json:"runs_escaped_under_fire"`
	ShipsLost            int     `json:"ships_lost"`
	CreditsEarned        int     `json:"credits_earned"`
	CreditsSpent         int     `json:"credits_spent"`
	LegendariesMined     int     `json:"legendaries_mined"`
	FuelBurned           float64 `json:"fuel_burned"`
	InsuranceClaims      int     `json:"insurance_claims"`
	FirstSeen            int64   `json:"first_seen"`
	LastSeen             int64   `json:"last_seen"`
}

// RunRecord is one ship's-log entry.
type RunRecord struct {
	When                int64    `json:"when"`
	World               string   `json:"world"`
	Asteroid            string   `json:"asteroid"`
	Tier                int      `json:"tier"`
	Outcome             string   `json:"outcome"`
	CargoValueRecovered int      `json:"cargo_value_recovered"`
	CargoValueLost      int      `json:"cargo_value_lost"`
	HullDelta           int      `json:"hull_delta"`
	FuelDelta           float64  `json:"fuel_delta"`
	Depleted            bool     `json:"depleted"`
	Events              []string `json:"events,omitempty"`
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

	MinedUnits float64 `json:"mined_units"`
	CargoValue int     `json:"cargo_value"`

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
}

// ActiveScan is an in-progress sensor scan (never persisted non-nil).
type ActiveScan struct {
	AsteroidID int     `json:"asteroid_id"`
	Elapsed    float64 `json:"elapsed"`
	Duration   float64 `json:"duration"`
}

// State is the full authoritative save.
type State struct {
	Version   int         `json:"version"`
	Seed      uint64      `json:"seed"`
	BeltCount uint64      `json:"belt_count"`
	Credits   int         `json:"credits"`
	Fuel      float64     `json:"fuel"`
	Hull      int         `json:"hull"`
	WorldIdx  int         `json:"world_idx"`
	Belt      []Asteroid  `json:"belt,omitempty"`
	Upgrades  Upgrades    `json:"upgrades"`
	Settings  Settings    `json:"settings"`
	Stats     Stats       `json:"stats"`
	RunLog    []RunRecord `json:"run_log,omitempty"`
	Run       *ActiveRun  `json:"run,omitempty"`
	Scan      *ActiveScan `json:"scan,omitempty"`

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

// New creates a fresh pilot save.
func New(c *content.Content, seed uint64, now int64) *State {
	return &State{
		Version:   StateVersion,
		Seed:      seed,
		BeltCount: 0,
		Credits:   c.Pilot.StartCredits,
		Fuel:      float64(c.Pilot.StartFuel),
		Hull:      c.Pilot.StartHull,
		WorldIdx:  -1,
		Settings: Settings{
			BeltView:         BeltViewTiles,
			PirateAggression: 1.0,
		},
		Stats: Stats{FirstSeen: now, LastSeen: now},
	}
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

// DecodeState parses and upgrades stored state.
func DecodeState(b []byte) (*State, error) {
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("sim: decode state: %w", err)
	}
	if s.Version > StateVersion {
		return nil, fmt.Errorf("sim: unsupported state version %d", s.Version)
	}
	if s.Version < StateVersion {
		s.Version = StateVersion
	}
	if s.Settings.PirateAggression == 0 {
		s.Settings.PirateAggression = 1.0
	}
	s.Run = nil  // never restore mid-run
	s.Scan = nil // never restore mid-scan
	return &s, nil
}

// IsDocked reports whether the pilot is at the star chart.
func (s *State) IsDocked() bool { return s.WorldIdx < 0 }
