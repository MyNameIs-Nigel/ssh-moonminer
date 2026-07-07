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

// Upgrades holds ship upgrade levels (0-4 per track).
type Upgrades struct {
	Drill    int `json:"drill"`
	Tank     int `json:"tank"`
	Plating  int `json:"plating"`
	Damper   int `json:"damper"`
	Surveyor int `json:"surveyor"`
}

// Stats tracks lifetime pilot statistics.
type Stats struct {
	RunsTotal       int     `json:"runs_total"`
	RunsClean       int     `json:"runs_clean"`
	RunsBailed      int     `json:"runs_bailed"`
	RunsRaided      int     `json:"runs_raided"`
	RunsStranded    int     `json:"runs_stranded"`
	CreditsEarned   int     `json:"credits_earned"`
	CreditsSpent    int     `json:"credits_spent"`
	LegendariesMined int    `json:"legendaries_mined"`
	FuelBurned      float64 `json:"fuel_burned"`
	InsuranceClaims int     `json:"insurance_claims"`
	FirstSeen       int64   `json:"first_seen"`
	LastSeen        int64   `json:"last_seen"`
}

// RunRecord is one ship's-log entry.
type RunRecord struct {
	When      int64  `json:"when"`
	World     string `json:"world"`
	Asteroid  string `json:"asteroid"`
	Tier      int    `json:"tier"`
	Outcome   string `json:"outcome"`
	Banked    int    `json:"banked"`
	DrillPct  int    `json:"drill_pct"`
	Overdrove bool   `json:"overdrove"`
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
}

// ActiveRun is in-progress mining state (never persisted non-nil).
type ActiveRun struct {
	AsteroidID int     `json:"asteroid_id"`
	Drill      float64 `json:"drill"`
	Pirate     float64 `json:"pirate"`
	Yield      int     `json:"yield"`
	Overdrive  bool    `json:"overdrive"`
	StartedAt  int64   `json:"started_at"`
	Overdrove  bool    `json:"overdrove"`
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
	s.Run = nil // never restore mid-run
	return &s, nil
}

// IsDocked reports whether the pilot is at the star chart.
func (s *State) IsDocked() bool { return s.WorldIdx < 0 }
