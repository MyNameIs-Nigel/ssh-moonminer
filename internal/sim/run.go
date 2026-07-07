package sim

import (
	"math"
	"time"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// OutcomeKind identifies run resolution type.
type OutcomeKind string

const (
	OutcomeClean    OutcomeKind = "clean"
	OutcomeBail     OutcomeKind = "bail"
	OutcomeRaided   OutcomeKind = "raided"
	OutcomeStranded OutcomeKind = "stranded"
)

// RunOutcome is returned when a run ends.
type RunOutcome struct {
	Kind        OutcomeKind
	Record      RunRecord
	Description string
	Label       string
}

var outcomeMeta = map[OutcomeKind]struct{ Label, Desc string }{
	OutcomeClean:    {"CLEAN EXTRACTION", "Vein drilled to the core. Full cargo secured and clear of contacts."},
	OutcomeBail:     {"CARGO SECURED", "Disengaged early and ran. You keep every credit already mined."},
	OutcomeRaided:   {"RAIDED", "Pirates boarded mid-drill. Lost cargo and took hull damage breaking away."},
	OutcomeStranded: {"STRANDED", "Tanks dry. Emergency tow sold your hold at a loss to cover the fee."},
}

// Lock begins a mining run on the selected asteroid.
func Lock(s *State, c *content.Content, asteroidID int, now int64) error {
	if s.WorldIdx < 0 {
		return ErrNotDocked
	}
	if s.Run != nil {
		return ErrActiveRun
	}
	if s.Hull <= 0 {
		return ErrHullBreached
	}
	ast, _ := FindAsteroid(s, asteroidID)
	if ast == nil {
		return ErrInvalidAsteroid
	}
	if !ast.Scanned {
		return ErrNotScanned
	}
	if s.Fuel < float64(ast.FuelCost) {
		return ErrInsufficientFuel
	}
	s.Fuel -= float64(ast.FuelCost)
	s.Run = &ActiveRun{
		AsteroidID: asteroidID,
		StartedAt:  now,
	}
	return nil
}

// Scan begins a sensor scan on the given asteroid. Distance determines both
// the scan's duration (content.Belt.ScanSecPerKm per km) and, indirectly via
// GenerateBelt, its flight fuel cost — but the scan itself only costs the
// flat content.Belt.ScanFuelCost. Completion is driven by TickScan.
func Scan(s *State, c *content.Content, asteroidID int, now int64) error {
	if s.WorldIdx < 0 {
		return ErrNotDocked
	}
	if s.Run != nil {
		return ErrActiveRun
	}
	if s.Scan != nil {
		return ErrScanInProgress
	}
	ast, _ := FindAsteroid(s, asteroidID)
	if ast == nil {
		return ErrInvalidAsteroid
	}
	if ast.Scanned {
		return ErrAlreadyScanned
	}
	cost := c.Belt.ScanFuelCost
	if s.Fuel < cost {
		return ErrInsufficientFuel
	}
	s.Fuel -= cost
	s.Scan = &ActiveScan{
		AsteroidID: asteroidID,
		Duration:   ast.Distance * c.Belt.ScanSecPerKm,
	}
	return nil
}

// TickScan advances an in-progress scan by dt seconds, revealing the
// asteroid's details once the scan completes.
func TickScan(s *State, dt float64) {
	sc := s.Scan
	if sc == nil {
		return
	}
	sc.Elapsed += dt
	if sc.Elapsed < sc.Duration {
		return
	}
	if ast, _ := FindAsteroid(s, sc.AsteroidID); ast != nil {
		ast.Scanned = true
	}
	s.Scan = nil
}

// SetOverdrive toggles overdrive on the active run.
func SetOverdrive(s *State, on bool) {
	if s.Run == nil {
		return
	}
	if on && !s.Run.Overdrive {
		s.Run.Overdrove = true
	}
	s.Run.Overdrive = on
}

// Bail ends the run early, banking accrued yield.
func Bail(s *State, c *content.Content, now int64) *RunOutcome {
	if s.Run == nil {
		return nil
	}
	return resolveRun(s, c, OutcomeBail, now)
}

// TickRun advances the mining run by dt seconds.
func TickRun(s *State, c *content.Content, dt float64, now int64) (*RunOutcome, bool) {
	run := s.Run
	if run == nil {
		return nil, false
	}
	ast, _ := FindAsteroid(s, run.AsteroidID)
	if ast == nil {
		s.Run = nil
		return nil, false
	}

	mc := c.Mining
	drillRate := 100.0 / ast.DrillSec * effectiveDrillRate(s, c)
	pirateRate := (float64(ast.Risk) / 100.0) * (mc.PirateRateBase + float64(ast.Tier)*mc.PirateRatePerTier) * s.Settings.PirateAggression * effectiveDamperMul(s, c)
	fuelDrain := mc.FuelDrainBase + float64(ast.Tier)*mc.FuelDrainPerTier

	drillMul := 1.0
	fuelMul := 1.0
	if run.Overdrive {
		drillMul = mc.OverdriveDrillMul
		fuelMul = mc.OverdriveFuelMul
	}

	run.Drill = math.Min(100, run.Drill+drillRate*dt*drillMul)
	s.Fuel = math.Max(0, s.Fuel-fuelDrain*dt*fuelMul)
	run.Pirate = math.Min(100, run.Pirate+pirateRate*dt)
	run.Yield = int(math.Round(float64(ast.Value) * run.Drill / 100))

	if run.Pirate >= 100 {
		out := resolveRun(s, c, OutcomeRaided, now)
		return out, true
	}
	if s.Fuel <= 0 {
		out := resolveRun(s, c, OutcomeStranded, now)
		return out, true
	}
	if run.Drill >= 100 {
		out := resolveRun(s, c, OutcomeClean, now)
		return out, true
	}
	return nil, false
}

// TickInterval returns the mining tick duration from content.
func TickInterval(c *content.Content) time.Duration {
	hz := c.Mining.TickHz
	if hz < 1 {
		hz = 8
	}
	return time.Second / time.Duration(hz)
}

func resolveRun(s *State, c *content.Content, kind OutcomeKind, now int64) *RunOutcome {
	run := s.Run
	if run == nil {
		return nil
	}
	ast, _ := FindAsteroid(s, run.AsteroidID)
	if ast == nil {
		s.Run = nil
		return nil
	}

	mc := c.Mining
	var banked int
	switch kind {
	case OutcomeClean:
		banked = ast.Value
	case OutcomeBail:
		banked = run.Yield
	case OutcomeRaided:
		banked = int(math.Round(float64(run.Yield) * mc.RaidedYieldKeep))
		dmg := mc.RaidedHullDamage - s.Upgrades.Plating*c.Upgrades.PlatingReduction
		if dmg < c.Upgrades.PlatingFloor {
			dmg = c.Upgrades.PlatingFloor
		}
		s.Hull = max(0, s.Hull-dmg)
	case OutcomeStranded:
		banked = int(math.Round(float64(run.Yield) * mc.StrandedYieldKeep))
	}

	s.Credits += banked
	RemoveAsteroid(s, run.AsteroidID)

	worldName := ""
	if w := c.WorldByIndex(s.WorldIdx); w != nil {
		worldName = w.Name
	}

	meta := outcomeMeta[kind]
	rec := RunRecord{
		When: now, World: worldName, Asteroid: ast.Name, Tier: ast.Tier,
		Outcome: string(kind), Banked: banked, DrillPct: int(run.Drill),
		Overdrove: run.Overdrove,
	}
	appendRunLog(s, rec)
	updateStatsOnOutcome(s, kind, banked, ast.Tier)

	if kind == OutcomeClean {
		s.Settings.InsuranceUsed = false
	}

	s.Run = nil
	return &RunOutcome{Kind: kind, Record: rec, Label: meta.Label, Description: meta.Desc}
}

func appendRunLog(s *State, rec RunRecord) {
	s.RunLog = append([]RunRecord{rec}, s.RunLog...)
	if len(s.RunLog) > 20 {
		s.RunLog = s.RunLog[:20]
	}
}

func updateStatsOnOutcome(s *State, kind OutcomeKind, banked, tier int) {
	s.Stats.RunsTotal++
	s.Stats.CreditsEarned += banked
	switch kind {
	case OutcomeClean:
		s.Stats.RunsClean++
	case OutcomeBail:
		s.Stats.RunsBailed++
	case OutcomeRaided:
		s.Stats.RunsRaided++
	case OutcomeStranded:
		s.Stats.RunsStranded++
	}
	if tier == 3 {
		s.Stats.LegendariesMined++
	}
}
