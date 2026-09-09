package sim

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// StateVersion is the save schema. Version 8 adds pilot ratings, frontier
// records, physical ship locations, and removes installed route devices.
const StateVersion = 8

// BeltViewMode is the default belt rendering mode.
type BeltViewMode int

const (
	BeltViewTiles BeltViewMode = iota
	BeltViewOreScan
	BeltViewRadar
)

// Settings are player tweak preferences persisted in the save. PirateAggression
// is retained as a stable save key but is presented as an accessibility-style
// Pirate Threat Assist; it changes pirate approach speed only, never rewards.
type Settings struct {
	BeltView         BeltViewMode `json:"belt_view"`
	PirateAggression float64      `json:"pirate_aggression"`
	HighContrast     bool         `json:"high_contrast"`
	ASCIISafe        bool         `json:"ascii_safe"`
	ReducedMotion    bool         `json:"reduced_motion"`
	WrapLongText     bool         `json:"wrap_long_text"`
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

// SlotDevice is one installed Utility/Weapon/Internal item and
// its grade (0..5, E..S).
type SlotDevice struct {
	ItemID   string `json:"item_id"`
	Grade    int    `json:"grade"`
	EMPArmed bool   `json:"emp_armed,omitempty"`
	// Missiles is the loaded ammunition for a Missile Launcher. It is ignored
	// for every other device and is refilled when the fitted ship docks.
	Missiles int `json:"missiles,omitempty"`
	// Fuel belongs to an Extra Fuel Tank, never to a generic pilot pool.
	// It is ignored for all other device types.
	Fuel float64 `json:"fuel,omitempty"`
}

// ShipInstance is one owned, persistent hangar ship: its model, its own
// stat grades, and its own slot loadout. Utility/Weapon slices are
// fixed-length (model.UtilitySlots/WeaponSlots) with nil entries for empty
// slots. Internal retains the first slot for save compatibility; Lantern adds
// a second slot in AdditionalInternal.
type ShipInstance struct {
	ModelID  string `json:"model_id"`
	SystemID string `json:"system_id"`
	// Hull and BaseFuel are physical condition on this specific hull. Extra
	// Fuel Tank reserves are carried by their individual SlotDevices.
	Hull     int     `json:"hull"`
	BaseFuel float64 `json:"base_fuel"`

	Grades             TrackGrades   `json:"grades"`
	Utility            []*SlotDevice `json:"utility,omitempty"`
	Weapon             []*SlotDevice `json:"weapon,omitempty"`
	Internal           *SlotDevice   `json:"internal,omitempty"`
	AdditionalInternal []*SlotDevice `json:"additional_internal,omitempty"`

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
	PiratesDestroyed     int     `json:"pirates_destroyed"`
	BountyCreditsEarned  int     `json:"bounty_credits_earned"`
	FuelBurned           float64 `json:"fuel_burned"`
	InsuranceClaims      int     `json:"insurance_claims"`
	FirstSeen            int64   `json:"first_seen"`
	LastSeen             int64   `json:"last_seen"`
}

// RunRecord is one ship's-log entry.
type RunRecord struct {
	SystemID             string   `json:"system_id,omitempty"`
	DestinationID        string   `json:"destination_id,omitempty"`
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
	PirateDestroyed      string   `json:"pirate_destroyed,omitempty"`
	BountyEarned         int      `json:"bounty_earned,omitempty"`

	// Disconnected marks a run the disconnect/shutdown autopilot resolved
	// (EmergencyResolve) rather than one the pilot flew to its end. Permanent
	// and informational: the ship's log shows it so an outcome nobody chose is
	// never mistaken for one that was. Old records decode as false, so this
	// needs no StateVersion bump. See
	// docs/framework/05-reconnect-and-location-restore.md.
	Disconnected bool `json:"disconnected,omitempty"`
}

// Asteroid is one belt contact.
type Asteroid struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Tier     int     `json:"tier"`
	Volume   int     `json:"volume"`
	DrillSec float64 `json:"drill_sec"`
	Value    int     `json:"value"`
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
	// PhaseTribute is pirates waiting for accept/refuse/fight.
	PhaseTribute
	// PhaseEscaping is the ship trying to leave, possibly under attack.
	PhaseEscaping
	// PhaseCombat is an active tactical-scope engagement with the run's
	// named pirate — docs/gameplay/07-pirate-combat-and-bounties.md. The
	// escape burn runs alongside it only once Combat.EscapeStarted is true.
	PhaseCombat
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

// CombatState is a transient tactical-scope engagement against the run's
// named pirate — docs/gameplay/07-pirate-combat-and-bounties.md. It exists
// only while ActiveRun.Phase == PhaseCombat (ActiveRun itself is never
// persisted). Bearing/Range/Solution are recomputed each tick as a pure
// function of TickCount so replay stays exact.
type CombatState struct {
	PirateName    string  `json:"pirate_name"`
	PirateHull    float64 `json:"pirate_hull"`
	PirateMaxHull float64 `json:"pirate_max_hull"`
	// PirateDPS is the roster damage_per_second already scaled by the
	// world's PirateAttackMul, so tickCombat can apply it directly.
	PirateDPS float64 `json:"pirate_dps"`
	Bounty    int     `json:"bounty"`
	// OddsPct is EstimateOdds at combat start (0..100), frozen for the
	// header — it does not track mid-fight damage taken/dealt.
	OddsPct int `json:"odds_pct"`
	// Maneuver is the roster maneuver stat, copied in at combat start so
	// tickCombat never needs a content lookup by PirateID.
	Maneuver float64 `json:"maneuver"`
	Bearing  float64 `json:"bearing"`
	Range    float64 `json:"range"`
	Solution float64 `json:"solution"`
	Heat     float64 `json:"heat"`
	// LockRemaining > 0 means weapons are overheat-locked.
	LockRemaining float64 `json:"lock_remaining"`
	// MissileCooldown is independent of the laser heat capacitor. It is
	// transient with the combat encounter, while ammunition lives on the
	// fitted launcher device.
	MissileCooldown float64 `json:"missile_cooldown"`
	// Phase1/Phase2 are maneuver phase offsets rolled once at combat start.
	Phase1 float64 `json:"phase1"`
	Phase2 float64 `json:"phase2"`
	// EscapeStarted is the Fight-vs-Run difference: Refuse and an unarmed
	// immediate attack pre-start the burn; Fight and an armed immediate
	// attack wait for CombatEscape (B/Enter).
	EscapeStarted bool     `json:"escape_started"`
	Log           []string `json:"log,omitempty"`
	ShotsFired    int      `json:"shots_fired"`
	ShotsHit      int      `json:"shots_hit"`
}

// PressurePointStatus is the visible result of one fracture point on an
// asteroid. Points are transient run state: a fresh visit generates a fresh
// surface, and an interrupted run is never restored from disk.
type PressurePointStatus string

const (
	PressurePointDormant PressurePointStatus = "dormant"
	PressurePointActive  PressurePointStatus = "active"
	PressurePointHit     PressurePointStatus = "hit"
	PressurePointMissed  PressurePointStatus = "missed"
)

// PressurePoint is one marked surface location. Position is a 0..7 index on
// the rendered asteroid perimeter; Status is authored by the pure sim and
// merely displayed by the TUI.
type PressurePoint struct {
	Position int                 `json:"position"`
	Status   PressurePointStatus `json:"status"`
}

// SkillCheck is the timed interaction for the currently active pressure
// point. Any press before Elapsed reaches Window is a hit; a timeout marks
// that point missed and does not damage the ship.
type SkillCheck struct {
	Elapsed       float64 `json:"elapsed"`
	Window        float64 `json:"window"`
	PressurePoint int     `json:"pressure_point,omitempty"`
}

// ActiveRun is in-progress mining state (never persisted non-nil).
type ActiveRun struct {
	BeltStability float64  `json:"belt_stability"`
	AsteroidID    int      `json:"asteroid_id"`
	Phase         RunPhase `json:"phase"`

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
	// PirateID is the named roster pirate rolled at Lock (salt 7100) — the
	// same hunter foretold by the radar approach is the one combat starts
	// against. Combat is nil until the pirate arrives.
	PirateID string       `json:"pirate_id,omitempty"`
	Combat   *CombatState `json:"combat,omitempty"`
	// PirateDestroyed/BountyEarned carry a combat win into the resulting run
	// record and summary.
	PirateDestroyed string `json:"pirate_destroyed,omitempty"`
	BountyEarned    int    `json:"bounty_earned,omitempty"`
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

	// PressurePoints are generated deterministically at Lock. SkillCheck is
	// the currently lit point, while NextSkillCheckIn spaces out the reveals.
	PressurePoints   []PressurePoint `json:"pressure_points,omitempty"`
	SkillCheck       *SkillCheck     `json:"skill_check,omitempty"`
	NextSkillCheckIn float64         `json:"next_skill_check_in"`
	// FuelAsteroid is rolled independently from the asteroid's rarity and
	// distance. It is only rendered for an equipped Fuel Miner.
	FuelAsteroid   bool `json:"fuel_asteroid,omitempty"`
	AsteroidSprite int  `json:"asteroid_sprite,omitempty"`

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
	// JammerRemaining is the time left on the Pirate Jammer charge consumed
	// at Lock. Pirate approach is suspended until it reaches zero.
	JammerRemaining float64 `json:"jammer_remaining"`
}

// ActiveScan is an in-progress sensor scan (never persisted non-nil).
type ActiveScan struct {
	AsteroidID int     `json:"asteroid_id"`
	Elapsed    float64 `json:"elapsed"`
	Duration   float64 `json:"duration"`
}

// State is the full authoritative save.
type State struct {
	Version   int    `json:"version"`
	Seed      uint64 `json:"seed"`
	BeltCount uint64 `json:"belt_count"`
	Credits   int    `json:"credits"`
	// Fuel/Hull are legacy active-ship mirrors retained for v5 migration and
	// compatibility with dev/test callers. ShipInstance owns the authoritative
	// condition and Encode synchronizes these values before persistence.
	Fuel     float64    `json:"fuel"`
	Hull     int        `json:"hull"`
	WorldIdx int        `json:"world_idx"`
	Belt     []Asteroid `json:"belt,omitempty"`

	// SystemID is the dock/belt system the pilot currently occupies. The
	// permit maps only record purchases; content-defined starter routes need
	// not be written into every save.
	SystemID           string                     `json:"system_id"`
	JumpClass          int                        `json:"jump_class"`
	JumpCount          uint64                     `json:"jump_count"`
	Frontier           map[string]*FrontierRecord `json:"frontier,omitempty"`
	CargoOrigins       map[string]int             `json:"cargo_origins,omitempty"`
	ShipsLostAt        map[string]string          `json:"ships_lost_at,omitempty"`
	HotArrivals        map[string]bool            `json:"hot_arrivals,omitempty"`
	CrossedGates       map[string]bool            `json:"crossed_gates,omitempty"`
	LastJump           *JumpResult                `json:"last_jump,omitempty"`
	DestinationPermits map[string]bool            `json:"destination_permits,omitempty"`
	CargoUnits         float64                    `json:"cargo_units,omitempty"`
	CargoValue         int                        `json:"cargo_value,omitempty"`

	// BountyVouchers are confirmed pirate kills awaiting dock redemption
	// (the C key sells cargo and redeems vouchers together). Unlike cargo,
	// vouchers survive ship loss — docs/gameplay/07-pirate-combat-and-
	// bounties.md's deliberate carve-out from "cargo is not money".
	BountyVouchers int `json:"bounty_vouchers,omitempty"`

	// Ships is the pilot's hangar: owned ship models keyed by ModelID, each
	// with its own persistent grades, loadout, hull and fuel condition.
	// ActiveShipID selects which one is currently flown.
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

	// DisconnectNotice is the outcome of a run EmergencyResolve closed while
	// the pilot was gone, held so the next session can tell them what their
	// autopilot did. Unlike Run and Scan it deliberately survives Encode() —
	// outliving the session that produced it is the entire point. One-shot:
	// the TUI shows it once and calls AckDisconnectNotice, which clears it.
	// See docs/framework/05-reconnect-and-location-restore.md.
	DisconnectNotice *RunRecord `json:"disconnect_notice,omitempty"`

	// DevGodMode disables hull damage. It is a dev-server-only debug flag.
	// Clone preserves it in memory, but Encode (the persisted DB payload) always
	// zeroes it first so it can never leak into a saved pilot file.
	DevGodMode bool `json:"dev_god_mode,omitempty"`
}

// Snapshot is a value copy for rendering.
type Snapshot struct {
	// Revision orders actor snapshots across tick and action delivery channels.
	Revision uint64
	State    State
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
		JumpClass: -1,
		Settings: Settings{
			BeltView:         BeltViewOreScan,
			PirateAggression: 1.0,
		},
		Stats: Stats{FirstSeen: now, LastSeen: now},
	}
	visitSystem(s, "sol", now)
	grantStarterShip(s, c)
	s.Fuel = FuelCapacity(s, c)
	s.Hull = MaxHull(s, c)
	syncActiveConditionFromLegacy(s, c)
	return s
}

// Encode serializes state for storage. Run is cleared before encode.
func (s *State) Encode() ([]byte, error) {
	copy := *s
	syncActiveConditionMirrorForEncode(&copy)
	copy.Run = nil
	copy.Scan = nil
	copy.DevGodMode = false
	return json.Marshal(&copy)
}

func syncActiveConditionMirrorForEncode(s *State) {
	inst := ActiveShip(s)
	if inst == nil {
		return
	}
	s.Hull = inst.Hull
	s.Fuel = inst.BaseFuel
	for _, d := range inst.Utility {
		if d != nil && d.ItemID == ItemFuelTank {
			s.Fuel += d.Fuel
		}
	}
}

// Clone returns a deep copy of the state.
func (s *State) Clone() *State {
	if s == nil {
		return nil
	}
	out := *s
	out.Belt = slices.Clone(s.Belt)
	out.Frontier = maps.Clone(s.Frontier)
	for id, f := range s.Frontier {
		out.Frontier[id] = cloneValue(f)
		if f != nil {
			out.Frontier[id].RunsByDestination = maps.Clone(f.RunsByDestination)
		}
	}
	out.CargoOrigins = maps.Clone(s.CargoOrigins)
	out.ShipsLostAt = maps.Clone(s.ShipsLostAt)
	out.HotArrivals = maps.Clone(s.HotArrivals)
	out.CrossedGates = maps.Clone(s.CrossedGates)
	out.LastJump = cloneValue(s.LastJump)
	out.DestinationPermits = maps.Clone(s.DestinationPermits)
	out.ShipsUnlocked = maps.Clone(s.ShipsUnlocked)
	out.Ships = maps.Clone(s.Ships)
	for id, ship := range s.Ships {
		copy := cloneValue(ship)
		if copy != nil {
			copy.Utility = cloneDevices(ship.Utility)
			copy.Weapon = cloneDevices(ship.Weapon)
			copy.Internal = cloneValue(ship.Internal)
			copy.AdditionalInternal = cloneDevices(ship.AdditionalInternal)
		}
		out.Ships[id] = copy
	}
	out.Inventory = cloneDevices(s.Inventory)
	out.RunLog = slices.Clone(s.RunLog)
	for i := range out.RunLog {
		out.RunLog[i].Events = slices.Clone(s.RunLog[i].Events)
	}
	out.DisconnectNotice = cloneValue(s.DisconnectNotice)
	if out.DisconnectNotice != nil {
		out.DisconnectNotice.Events = slices.Clone(s.DisconnectNotice.Events)
	}
	out.Scan = cloneValue(s.Scan)
	out.Run = cloneValue(s.Run)
	if r := out.Run; r != nil {
		r.PressurePoints = slices.Clone(s.Run.PressurePoints)
		r.SkillCheck = cloneValue(s.Run.SkillCheck)
		r.ActiveEvent = cloneValue(s.Run.ActiveEvent)
		r.EventLog = slices.Clone(s.Run.EventLog)
		r.Combat = cloneValue(s.Run.Combat)
		if r.Combat != nil {
			r.Combat.Log = slices.Clone(s.Run.Combat.Log)
		}
	}
	return &out
}

func cloneValue[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneDevices(devices []*SlotDevice) []*SlotDevice {
	out := slices.Clone(devices)
	for i, d := range devices {
		out[i] = cloneValue(d)
	}
	return out
}

// DecodeState parses and upgrades stored state. c is required to build a
// fresh starter ship when migrating a pre-fleet (version < 2) save.
func DecodeState(b []byte, c *content.Content) (*State, error) {
	var s State
	migrated, driveCount, oldPermit, migrationErr := retireLegacyDrives(b)
	if migrationErr != nil {
		return nil, migrationErr
	}
	if err := json.Unmarshal(migrated, &s); err != nil {
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
			if id != s.ActiveShipID && (inst == nil || c.ShipByID(inst.ModelID) == nil) {
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
	if savedVersion < 6 {
		migrateShipConditions(&s, c)
	} else {
		for shipID := range s.Ships {
			normalizeShipCondition(&s, c, shipID)
		}
		syncActiveConditionMirror(&s, c)
	}
	if savedVersion < 7 {
		migrateUpgradeRework(&s, c)
	} else {
		for shipID := range s.Ships {
			normalizeShipMissiles(&s, c, shipID)
		}
	}
	if savedVersion < 8 {
		migrateFrontier(&s, c, driveCount, oldPermit)
	}
	for _, ship := range s.Ships {
		if ship.SystemID == "" {
			ship.SystemID = s.SystemID
		}
		n := c.ShipByID(ship.ModelID).InternalSlots - 1
		for len(ship.AdditionalInternal) < n {
			ship.AdditionalInternal = append(ship.AdditionalInternal, nil)
		}
	}
	for id := range s.Frontier {
		frontierRecord(&s, id)
	}
	syncActiveConditionMirror(&s, c)
	return &s, nil
}

// migrateShipConditions assigns the old active aggregate fuel/hull to its
// active hull, distributing fuel into detachable tanks first. Pre-v6 saves
// modeled parked ships as fully maintained, so their condition is initialized
// full rather than silently damaged or empty.
func migrateShipConditions(s *State, c *content.Content) {
	for shipID := range s.Ships {
		if shipID == s.ActiveShipID {
			inst := s.Ships[shipID]
			inst.Hull = clampInt(s.Hull, 0, MaxHullFor(s, c, shipID))
			setFuelAmountFor(s, c, shipID, s.Fuel)
			continue
		}
		inst := s.Ships[shipID]
		inst.Hull = MaxHullFor(s, c, shipID)
		setFuelAmountFor(s, c, shipID, fuelCapacityFor(s, c, shipID))
	}
	syncActiveConditionMirror(s, c)
}

func normalizeShipCondition(s *State, c *content.Content, shipID string) {
	inst := s.Ships[shipID]
	if inst == nil {
		return
	}
	inst.Hull = clampInt(inst.Hull, 0, MaxHullFor(s, c, shipID))
	setFuelAmountFor(s, c, shipID, fuelAmountFor(s, c, shipID))
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

// migrateUpgradeRework converts legacy Mass Drivers into loaded missile launchers.
func migrateUpgradeRework(s *State, c *content.Content) {
	migrateDevice := func(d *SlotDevice) {
		if d != nil && d.ItemID == legacyItemMassDriver {
			d.ItemID = ItemMissileLauncher
			d.Missiles = MissileCapacity(c, d.Grade)
		}
	}
	for shipID, ship := range s.Ships {
		if ship == nil {
			continue
		}
		for _, d := range ship.Utility {
			migrateDevice(d)
		}
		for _, d := range ship.Weapon {
			migrateDevice(d)
		}

		RearmMissilesForShip(s, c, shipID)
	}
	for _, d := range s.Inventory {
		migrateDevice(d)
	}
}

// IsDocked reports whether the pilot is at the star chart.
func (s *State) IsDocked() bool { return s.WorldIdx < 0 }
