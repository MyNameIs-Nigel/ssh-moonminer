package sim

import (
	"fmt"
	"math"
	"time"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// normalizeRunAccounting upgrades transient pre-accounting fixtures to the
// explicit extracted/held model. Active runs are never persisted, but this
// keeps deterministic tests and dev tooling written against MinedUnits from
// silently changing their meaning during the transition.
func normalizeRunAccounting(run *ActiveRun) {
	if run == nil {
		return
	}
	if run.ExtractedUnits == 0 && run.HeldUnits == 0 && run.MinedUnits > 0 {
		run.ExtractedUnits = run.MinedUnits
		run.HeldUnits = run.MinedUnits
	}
	if run.ExtractedUnits < run.HeldUnits {
		run.ExtractedUnits = run.HeldUnits
	}
	run.ExtractedUnits = math.Max(0, run.ExtractedUnits)
	run.HeldUnits = clamp(run.HeldUnits, 0, run.ExtractedUnits)
	run.JettisonedUnits = math.Max(0, run.JettisonedUnits)
	run.MinedUnits = run.HeldUnits
}

// OutcomeKind identifies run resolution type.
type OutcomeKind string

const (
	OutcomeDeparted         OutcomeKind = "departed"
	OutcomeBailed           OutcomeKind = "bailed"
	OutcomeTributePaid      OutcomeKind = "tribute_paid"
	OutcomeEscapedUnderFire OutcomeKind = "escaped_under_fire"
	OutcomePirateDestroyed  OutcomeKind = "pirate_destroyed"
	OutcomeShipLost         OutcomeKind = "ship_lost"
)

// RunOutcome is returned when a run ends.
type RunOutcome struct {
	Kind        OutcomeKind
	Record      RunRecord
	Description string
	Label       string
}

var outcomeMeta = map[OutcomeKind]struct{ Label, Desc string }{
	OutcomeDeparted:         {"DEPARTED", "Asteroid depleted. Cargo sealed. No clean run is safe until the dock buys it."},
	OutcomeBailed:           {"BAILED", "Cut the drill and ran with a partial hold."},
	OutcomeTributePaid:      {"TRIBUTE PAID", "Cargo jettisoned. Pirates took the easy money and let the ship run."},
	OutcomeEscapedUnderFire: {"ESCAPED UNDER FIRE", "Hull torn open, engines screaming, cargo still aboard."},
	OutcomePirateDestroyed:  {"PIRATE DESTROYED", "Threat neutralized. Cargo sealed and bounty voucher confirmed."},
	OutcomeShipLost:         {"CONNECTION LOST", "The ship and its hold are gone."},
}

// Lock begins a mining run on the selected asteroid.
func Lock(s *State, c *content.Content, asteroidID int, now int64) error {
	syncActiveConditionFromLegacy(s, c)
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
	if RemainingCargoCapacity(s, c) <= 0 {
		return ErrCargoFull
	}
	if s.Fuel < float64(ast.FuelCost) {
		return ErrInsufficientFuel
	}
	ConsumeFuel(s, c, float64(ast.FuelCost))
	pirateStartDistance := 100.0
	if w := c.WorldByIndex(s.WorldIdx); w != nil && w.PirateStartDistance > 0 {
		pirateStartDistance = w.PirateStartDistance
	}
	s.Run = &ActiveRun{
		AsteroidID:           asteroidID,
		Phase:                PhaseMining,
		PirateDistance:       pirateStartDistance,
		CargoShiftPenaltyMul: 1.0,
		StartHull:            s.Hull,
		StartFuel:            s.Fuel,
		StartedAt:            now,
	}
	if inst := ActiveShip(s); inst != nil && inst.Internal != nil && inst.Internal.ItemID == ItemJammer && inst.JammerCharges > 0 {
		inst.JammerCharges--
		s.Run.JammerRemaining = JammerDurationSeconds(c, inst.Internal.Grade)
	}
	s.Run.PirateBearing = runRNG(s, s.Run, 7000).Float64()
	s.Run.PirateID = rollPirateID(s, c, s.Run, ast)
	rollPressurePoints(s, c, s.Run)
	s.Run.AsteroidSprite = runRNG(s, s.Run, 6300).Intn(3)
	// Fuel content is a separate geological roll: it deliberately does not
	// reuse tier, distance, value, or flight-cost RNG so those signals cannot
	// be reverse-engineered into a fuel-asteroid predictor.
	s.Run.FuelAsteroid = runRNG(s, s.Run, 7200).Float64() < c.Mining.FuelAsteroidChance
	rollNextSkillCheckIn(s, c, s.Run)
	updatePirateETA(s, c, s.Run, ast)
	return nil
}

// Scan begins a sensor scan on the given asteroid. Distance determines both
// the scan's duration (content.Belt.ScanSecPerKm per km) and, indirectly via
// GenerateBelt, its flight fuel cost — but the scan itself only costs the
// flat content.Belt.ScanFuelCost. Completion is driven by TickScan.
func Scan(s *State, c *content.Content, asteroidID int, now int64) error {
	syncActiveConditionFromLegacy(s, c)
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
	if IsOutOfRange(s, c, ast) {
		return ErrOutOfRange
	}
	cost := c.Belt.ScanFuelCost
	if s.Fuel < cost {
		return ErrInsufficientFuel
	}
	ConsumeFuel(s, c, cost)
	duration := ScannerScanSeconds(s, c, ast.Distance)
	if duration <= 0 {
		// An S scanner's close-range instant scan must complete in the same
		// intent, not wait for the actor's next 250 ms tick.
		ast.Scanned = true
		return nil
	}
	s.Scan = &ActiveScan{AsteroidID: asteroidID, Duration: duration}
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

// BailOrDepart starts the escape sequence. It is the yellow BAIL action
// while resources remain on the asteroid and the green DEPART action once
// it is depleted; the sim doesn't distinguish the two calls, only the
// resulting intent. Safe to call with state.Run == nil (a no-op) because
// disconnect races a just-finished run. It is also a no-op once the run has
// already left the mining phase (tribute/escaping), matching "no menu
// actions are accepted" once fleeing has begun.
func BailOrDepart(s *State, c *content.Content, now int64) error {
	run := s.Run
	if run == nil {
		return nil
	}
	if run.Phase != PhaseMining {
		return ErrInvalidRunPhase
	}
	normalizeRunAccounting(run)
	ast, _ := FindAsteroid(s, run.AsteroidID)
	if ast == nil {
		s.Run = nil
		return nil
	}
	startEscape(s, c, ast, false)
	return nil
}

// AcceptTribute jettisons the demanded cargo percentage and starts escaping
// without triggering an immediate attack. Valid only during the tribute
// phase.
func AcceptTribute(s *State, c *content.Content, now int64) error {
	run := s.Run
	if run == nil {
		return ErrNoActiveRun
	}
	if run.Phase != PhaseTribute {
		return ErrInvalidRunPhase
	}
	normalizeRunAccounting(run)
	ast, _ := FindAsteroid(s, run.AsteroidID)
	if ast == nil {
		s.Run = nil
		return nil
	}
	totalCargo := s.CargoValue + run.CargoValue
	demand := int(math.Round(float64(totalCargo) * c.Mining.TributeDemandPct))
	demand = clampInt(demand, 0, totalCargo)
	fromStored := proportionalCargoLoss(demand, s.CargoValue, totalCargo)
	fromRun := demand - fromStored
	if fromRun > run.CargoValue {
		fromStored += fromRun - run.CargoValue
		fromRun = run.CargoValue
	}
	storedUnitsLost := cargoUnitsForValueLoss(s.CargoUnits, s.CargoValue, fromStored)
	runUnitsLost := cargoUnitsForValueLoss(run.HeldUnits, run.CargoValue, fromRun)
	s.CargoValue -= fromStored
	s.CargoUnits = math.Max(0, s.CargoUnits-storedUnitsLost)
	run.CargoValue -= fromRun
	run.HeldUnits = math.Max(0, run.HeldUnits-runUnitsLost)
	run.JettisonedUnits += runUnitsLost
	run.MinedUnits = run.HeldUnits // compatibility mirror; never remnant accounting
	run.TributeCargoBefore = totalCargo
	run.TributeDemand = demand
	run.TributeCargoRetained = s.CargoValue + run.CargoValue
	run.TributePaid = true
	run.EventLog = append(run.EventLog, RunEventRecord{Kind: EventKind(fmt.Sprintf("tribute paid: total %d, demand %d, retained %d", totalCargo, demand, run.TributeCargoRetained)), At: now})
	startEscape(s, c, ast, false)
	return nil
}

func proportionalCargoLoss(demand, portion, total int) int {
	if demand <= 0 || portion <= 0 || total <= 0 {
		return 0
	}
	return clampInt(int(math.Round(float64(demand)*float64(portion)/float64(total))), 0, min(demand, portion))
}

func cargoUnitsForValueLoss(units float64, value, loss int) float64 {
	if units <= 0 || value <= 0 || loss <= 0 {
		return 0
	}
	return clamp(units*float64(loss)/float64(value), 0, units)
}

// RefuseTribute rejects the pirates' demand and starts a fight-for-your-life
// escape. Valid only during the tribute phase.
func RefuseTribute(s *State, c *content.Content, now int64) error {
	run := s.Run
	if run == nil {
		return ErrNoActiveRun
	}
	if run.Phase != PhaseTribute {
		return ErrInvalidRunPhase
	}
	ast, _ := FindAsteroid(s, run.AsteroidID)
	if ast == nil {
		s.Run = nil
		return nil
	}
	run.EventLog = append(run.EventLog, RunEventRecord{Kind: "tribute_refused", At: now})
	startCombat(s, c, run, ast, true, now)
	return nil
}

func startEscape(s *State, c *content.Content, ast *Asteroid, underAttack bool) {
	run := s.Run
	normalizeRunAccounting(run)
	run.Phase = PhaseEscaping
	if underAttack {
		run.UnderAttack = true
	}
	configureEscape(s, c, run, ast)
}

// configureEscape computes the escape-burn duration from cargo load and
// starts its timer, without touching Phase — startEscape uses it to enter
// PhaseEscaping, and combat.go's startCombat/CombatEscape use it to start
// the burn alongside an ongoing PhaseCombat engagement (docs/gameplay/07-
// pirate-combat-and-bounties.md's "burn matrix").
func configureEscape(s *State, c *content.Content, run *ActiveRun, ast *Asteroid) {
	if run.Intent == "" {
		if RunDepleted(s, c, ast, run) {
			run.Intent = OutcomeDeparted
		} else {
			run.Intent = OutcomeBailed
		}
	}
	cargoLoadRatio := 0.0
	if cap_ := CargoCapacityUnits(s, c); cap_ > 0 {
		cargoLoadRatio = clamp((s.CargoUnits+run.HeldUnits)/cap_, 0, 1)
	}
	mc := c.Mining
	req := mc.BaseEscapeSeconds + math.Pow(cargoLoadRatio, mc.EscapeCargoExponent)*mc.CargoEscapePenaltySeconds
	if run.CargoShiftPenaltyMul <= 0 {
		run.CargoShiftPenaltyMul = 1.0
	}
	run.BaseEscapeSecondsRequired = req * run.CargoShiftPenaltyMul * EscapeMul(s, c)
	run.FuelOutSeconds = 0
	run.EscapeSecondsRequired = run.BaseEscapeSecondsRequired
	run.EscapeSecondsElapsed = 0
}

// TickRun advances the mining run by dt seconds.
func TickRun(s *State, c *content.Content, dt float64, now int64) (*RunOutcome, bool) {
	run := s.Run
	if run == nil {
		return nil, false
	}
	syncActiveConditionFromLegacy(s, c)
	defer syncActiveConditionMirror(s, c)
	normalizeRunAccounting(run)
	if dt < 0 || math.IsNaN(dt) {
		dt = 0
	}
	ast, _ := FindAsteroid(s, run.AsteroidID)
	if ast == nil {
		s.Run = nil
		return nil, false
	}
	run.TickCount++

	tickEvents(s, c, run, dt, now)

	switch run.Phase {
	case PhaseMining:
		tickMining(s, c, run, ast, dt)
		if run.PirateDistance <= 0 && !run.EMPActive && run.JammerRemaining <= 0 {
			if deployEMPLauncher(s, c, run, now) {
				break
			}
			rollPirateAction(s, c, run, ast, now)
		}
	case PhaseTribute:
		run.TributeSecondsElapsed += dt
		if run.TributeSecondsElapsed >= c.Mining.TributeDecisionSeconds {
			_ = RefuseTribute(s, c, now)
		}
	case PhaseEscaping:
		tickEscape(s, c, run, dt)
	case PhaseCombat:
		if out := tickCombat(s, c, run, dt, now); out != nil {
			return out, true
		}
	}

	if s.Hull <= 0 {
		out := resolveRun(s, c, OutcomeShipLost, now)
		return out, true
	}
	burnRunning := run.Phase == PhaseEscaping ||
		(run.Phase == PhaseCombat && run.Combat != nil && run.Combat.EscapeStarted)
	if burnRunning && run.EscapeSecondsElapsed >= run.EscapeSecondsRequired {
		kind := run.Intent
		switch {
		case run.TributePaid:
			kind = OutcomeTributePaid
		case run.UnderAttack:
			kind = OutcomeEscapedUnderFire
		}
		out := resolveRun(s, c, kind, now)
		return out, true
	}
	return nil, false
}

func tickMining(s *State, c *content.Content, run *ActiveRun, ast *Asteroid, dt float64) {
	if run.EMPActive {
		run.EMPRemaining -= dt
		if run.EMPRemaining <= 0 {
			run.EMPRemaining = 0
			run.EMPActive = false
		}
	}
	outage := run.ActiveEvent != nil && run.ActiveEvent.Kind == EventPowerOutage
	if !outage {
		// Mining stops at the ship's cargo capacity exactly like depletion —
		// RunDepleted treats "hold full" and "rock exhausted" the same way.
		extractionRemaining := math.Max(0, float64(ast.Volume)-run.ExtractedUnits)
		holdRemaining := math.Max(0, RemainingCargoCapacity(s, c)-run.HeldUnits)
		remaining := math.Min(extractionRemaining, holdRemaining)
		if remaining > 0 && s.Fuel > 0 && ast.DrillSec > 0 {
			mineRate := float64(ast.Volume) / ast.DrillSec * MiningSpeedMul(s, c)
			mined := math.Min(remaining, mineRate*dt)
			if mined > 0 {
				run.ExtractedUnits += mined
				run.HeldUnits += mined
				run.MinedUnits = run.HeldUnits // compatibility mirror
			}
			fuelDrain := (c.Mining.FuelDrainBase + float64(ast.Tier)*c.Mining.FuelDrainPerTier) * FuelDrainMul(s, c)
			if run.ActiveEvent != nil && run.ActiveEvent.Kind == EventReactorSurge {
				fuelDrain *= c.Events.ReactorSurgeFuelMul
			}
			consumed := ConsumeFuel(s, c, fuelDrain*dt)
			s.Stats.FuelBurned += consumed
			// Fuel veins replenish fuel only while the drill is actually running;
			// a dry, stalled tank cannot self-start. E exactly offsets the fuel
			// consumed this tick, while higher tiers produce a capped net gain.
			if run.FuelAsteroid && consumed > 0 {
				AddFuel(s, c, consumed*FuelMinerRecoveryMul(s, c))
			}
		}
		if ast.Volume > 0 {
			run.CargoValue = int(math.Round(float64(ast.Value) * run.HeldUnits / float64(ast.Volume)))
		}
		tickSkillCheck(s, c, run, ast, dt)
	}

	if run.LifeSupportBreached {
		applyHullDamageRate(s, c, run, float64(c.Events.LifeSupportBleedPerSecond), dt)
	}

	// A Pirate Jammer charge (consumed at Lock) suspends pirate approach for
	// a fixed duration. If it expires partway through a large tick, only the
	// unsuppressed remainder advances the contact.
	approachDT := dt
	if run.JammerRemaining > 0 {
		suppressed := math.Min(approachDT, run.JammerRemaining)
		run.JammerRemaining = math.Max(0, run.JammerRemaining-suppressed)
		approachDT -= suppressed
	}
	blackout := run.ActiveEvent != nil && run.ActiveEvent.Kind == EventRadarBlackout
	if !blackout && approachDT > 0 {
		rate := pirateApproachRate(s, c, ast)
		run.PirateDistance = math.Max(0, run.PirateDistance-rate*approachDT)
		updatePirateETA(s, c, run, ast)
	}
}

// RunDepleted reports whether an in-progress run has exhausted the asteroid
// and may use the green DEPART action. A full hold pauses extraction but does
// not erase the remaining rock or turn a partial run into a departure.
func RunDepleted(s *State, c *content.Content, ast *Asteroid, run *ActiveRun) bool {
	if ast == nil {
		return true
	}
	normalizeRunAccounting(run)
	return run.ExtractedUnits >= float64(ast.Volume)
}

// RunCargoFull reports whether the current run has filled all cargo space
// available when it locked the asteroid. It is intentionally separate from
// RunDepleted: the remaining asteroid percentage remains truthful.
func RunCargoFull(s *State, c *content.Content, run *ActiveRun) bool {
	if run == nil {
		return false
	}
	normalizeRunAccounting(run)
	return run.HeldUnits >= RemainingCargoCapacity(s, c)
}

// updatePirateETA recomputes the fuzzed pirate arrival estimate shown on the
// mining-screen radar. The true remaining time is never rendered directly —
// only this widened range, narrowed at high Scanner grades.
func updatePirateETA(s *State, c *content.Content, run *ActiveRun, ast *Asteroid) {
	rate := pirateApproachRate(s, c, ast)
	trueRemaining := 0.0
	if rate > 0 {
		trueRemaining = run.PirateDistance / rate
	}
	mc := c.Mining
	uncertainty := clamp(mc.EtaBaseUncertaintyPct-scannerEtaBonus(s, c),
		mc.EtaMinUncertaintyPct, 1.0)
	half := trueRemaining * uncertainty / 2
	run.PirateETAMin = math.Max(0, trueRemaining-half)
	run.PirateETAMax = trueRemaining + half
}

func tickEscape(s *State, c *content.Content, run *ActiveRun, dt float64) {
	if run.UnderAttack {
		applyAttackDamageRate(s, c, run, attackHullDamagePerSecond(s, c), dt)
	}
	tickEscapeBurn(s, c, run, dt)
}

// tickEscapeBurn advances the escape-burn timer, fuel drain, and fuel-out
// penalty shared by tickEscape (PhaseEscaping) and, once
// Combat.EscapeStarted, tickCombat (PhaseCombat) — see combat.go. Incoming
// attack damage is applied by the caller, not here, so PhaseCombat's
// constant pirate DPS is never double-applied.
func tickEscapeBurn(s *State, c *content.Content, run *ActiveRun, dt float64) {
	run.EscapeSecondsElapsed += dt
	fuelDrain := c.Mining.EscapeFuelDrainPerSec * FuelDrainMul(s, c)
	if run.ActiveEvent != nil && run.ActiveEvent.Kind == EventReactorSurge {
		fuelDrain *= c.Events.ReactorSurgeFuelMul
	}
	ConsumeFuel(s, c, fuelDrain*dt)
	if s.Fuel <= 0 {
		run.FuelOutSeconds += dt
	}
	penalty := math.Min(run.FuelOutSeconds*c.Mining.FuelOutEscapePenaltyPerSec, c.Mining.FuelOutEscapePenaltyCapSeconds)
	run.EscapeSecondsRequired = run.BaseEscapeSecondsRequired + penalty
}

func rollPirateAction(s *State, c *content.Content, run *ActiveRun, ast *Asteroid, now int64) {
	if w := c.WorldByIndex(s.WorldIdx); w != nil && w.PiratesAlwaysAttack {
		run.PirateAction = PirateActionAttack
		run.EventLog = append(run.EventLog, RunEventRecord{Kind: "pirate_attack", At: now})
		// Unarmed ships keep today's auto-escape (the burn matrix in
		// docs/gameplay/07); armed ships stand and fight until B/Enter.
		startCombat(s, c, run, ast, !HasWeapon(s), now)
		return
	}
	rng := runRNG(s, run, 9001)
	if rng.Float64() < c.Mining.TributeChance {
		run.PirateAction = PirateActionTribute
		run.Phase = PhaseTribute
		run.TributeSecondsElapsed = 0
		run.EventLog = append(run.EventLog, RunEventRecord{Kind: "tribute_demanded", At: now})
		return
	}
	run.PirateAction = PirateActionAttack
	run.EventLog = append(run.EventLog, RunEventRecord{Kind: "pirate_attack", At: now})
	startCombat(s, c, run, ast, !HasWeapon(s), now)
}

// EmergencyResolve fast-forwards an in-progress run to resolution without
// waiting on real time. It mirrors the same bail/escape path the player
// would take, so cargo survives only if the (simulated) escape succeeds and
// ship loss remains possible — used on disconnect/shutdown.
func EmergencyResolve(s *State, c *content.Content, now int64) *RunOutcome {
	run := s.Run
	if run == nil {
		return nil
	}
	switch run.Phase {
	case PhaseMining:
		if err := BailOrDepart(s, c, now); err != nil || s.Run == nil {
			return nil
		}
	case PhaseTribute:
		if err := RefuseTribute(s, c, now); err != nil || s.Run == nil {
			return nil
		}
	case PhaseCombat:
		// Mirror the player's own choice on disconnect: flee under fire
		// rather than stand and fight blind. A burn already running (Refuse,
		// or an unarmed immediate attack) is left alone.
		if run.Combat != nil && !run.Combat.EscapeStarted {
			if err := CombatEscape(s, c, now); err != nil || s.Run == nil {
				return nil
			}
		}
	}
	run = s.Run
	if run == nil {
		return nil
	}
	remaining := run.EscapeSecondsRequired - run.EscapeSecondsElapsed
	if remaining < 0 {
		remaining = 0
	}
	out, ended := TickRun(s, c, remaining+1, now)
	if !ended {
		// Escape requirement kept growing (e.g. fuel-out penalty loop); force
		// the timer closed rather than loop forever.
		if s.Run != nil {
			s.Run.EscapeSecondsRequired = s.Run.EscapeSecondsElapsed
			out, _ = TickRun(s, c, 0, now)
		}
	}
	return out
}

// TickInterval returns the mining tick duration from content.
func TickInterval(c *content.Content) time.Duration {
	hz := c.Mining.TickHz
	if hz < 1 {
		hz = 4
	}
	return time.Second / time.Duration(hz)
}

func pirateApproachRate(s *State, c *content.Content, ast *Asteroid) float64 {
	return (float64(ast.Risk) / 100.0) * (c.Mining.PirateApproachBase + float64(ast.Tier)*c.Mining.PirateApproachPerTier) * s.Settings.PirateAggression
}

func attackHullDamagePerSecond(s *State, c *content.Content) float64 {
	mul := 1.0
	if w := c.WorldByIndex(s.WorldIdx); w != nil && w.PirateAttackMul > 0 {
		mul = w.PirateAttackMul
	}
	return c.Mining.AttackHullDamagePerSecond * mul
}

// applyHullDamageRate applies continuous, tick-scaled non-combat damage
// (life-support bleed) straight to the hull. Shield/Turret only defend
// against pirate attacks specifically — see applyAttackDamageRate.
func applyHullDamageRate(s *State, c *content.Content, run *ActiveRun, ratePerSecond, dt float64) {
	if ratePerSecond <= 0 || dt <= 0 {
		return
	}
	applyHullDamageCarry(s, c, run, ratePerSecond*dt)
}

// applyHullDamageInstant applies a single lump non-combat hit (e.g. reactor
// surge) straight to the hull, same reasoning as applyHullDamageRate.
func applyHullDamageInstant(s *State, c *content.Content, run *ActiveRun, amount float64) {
	if amount <= 0 {
		return
	}
	applyHullDamageCarry(s, c, run, amount)
}

// applyAttackDamageRate applies continuous pirate-attack damage: the active
// ship's persistent Shield buffer absorbs what it can, and only the
// remainder reaches the hull. Damage mitigation is entirely the Shield's
// job now — the Defense Turret (now "Autocannon Turret") became an active
// weapon under docs/gameplay/07-pirate-combat-and-bounties.md.
func applyAttackDamageRate(s *State, c *content.Content, run *ActiveRun, ratePerSecond, dt float64) {
	if ratePerSecond <= 0 || dt <= 0 {
		return
	}
	amount := ratePerSecond * dt
	inst := ActiveShip(s)
	if inst != nil && inst.ShieldHP > 0 {
		absorbed := math.Min(inst.ShieldHP, amount)
		inst.ShieldHP -= absorbed
		amount -= absorbed
		if inst.ShieldHP <= 0 {
			inst.ShieldHP = 0
			inst.ShieldDamaged = true
		}
	}
	applyHullDamageCarry(s, c, run, amount)
}

func applyHullDamageCarry(s *State, c *content.Content, run *ActiveRun, reduced float64) {
	if reduced <= 0 || s.DevGodMode {
		return
	}
	run.HullDamageCarry += reduced
	whole := int(run.HullDamageCarry)
	if whole > 0 {
		setActiveHull(s, c, max(0, s.Hull-whole))
		run.HullDamageCarry -= float64(whole)
	}
}

func resolveRun(s *State, c *content.Content, kind OutcomeKind, now int64) *RunOutcome {
	run := s.Run
	if run == nil {
		return nil
	}
	normalizeRunAccounting(run)
	ast, _ := FindAsteroid(s, run.AsteroidID)
	astName, astTier, astVolume := "", 0, 0
	if ast != nil {
		astName, astTier, astVolume = ast.Name, ast.Tier, ast.Volume
	}

	recovered, lost := 0, 0
	switch kind {
	case OutcomeShipLost:
		lost = s.CargoValue + run.CargoValue
		s.CargoValue = 0
		s.CargoUnits = 0
	default:
		recovered = run.CargoValue
		s.CargoValue += recovered
		s.CargoUnits += run.HeldUnits
	}

	hullDelta := s.Hull - run.StartHull
	fuelDelta := s.Fuel - run.StartFuel
	depleted := RunDepleted(s, c, ast, run)
	// Only true volume exhaustion removes the asteroid outright on Departed.
	// A full hold always enters the BAILED path and preserves its remnant.
	volumeExhausted := astVolume > 0 && run.ExtractedUnits >= float64(astVolume)

	if ast != nil {
		switch {
		case kind == OutcomeShipLost:
			RemoveAsteroid(s, run.AsteroidID)
		case kind == OutcomeDeparted && volumeExhausted:
			RemoveAsteroid(s, run.AsteroidID)
		default:
			remainRatio := 1.0
			if astVolume > 0 {
				remainRatio = 1 - run.ExtractedUnits/float64(astVolume)
			}
			if remainRatio < c.Mining.RemnantKeepThreshold {
				RemoveAsteroid(s, run.AsteroidID)
			} else {
				applyMinedRemnant(s, run.AsteroidID, run.ExtractedUnits)
			}
		}
	}

	worldName := ""
	if w := c.WorldByIndex(s.WorldIdx); w != nil {
		worldName = w.Name
	}

	events := make([]string, 0, len(run.EventLog))
	for _, e := range run.EventLog {
		events = append(events, string(e.Kind))
	}

	meta := outcomeMeta[kind]
	rec := RunRecord{
		When: now, World: worldName, Asteroid: astName, Tier: astTier,
		Outcome: string(kind), CargoValueRecovered: recovered, CargoValueLost: lost, CargoValueJettisoned: run.TributeDemand,
		HullDelta: hullDelta, FuelDelta: fuelDelta, Depleted: depleted, Events: events,
		PirateDestroyed: run.PirateDestroyed, BountyEarned: run.BountyEarned,
	}
	appendRunLog(s, rec)
	updateStatsOnOutcome(s, kind, astTier)

	if kind == OutcomeShipLost {
		respawnActiveShip(s, c)
	}
	if kind != OutcomeShipLost {
		restoreBurstShieldOnBeltReturn(s, c)
	}

	s.Run = nil
	return &RunOutcome{Kind: kind, Record: rec, Label: meta.Label, Description: meta.Desc}
}

// applyMinedRemnant shrinks a partially-mined asteroid in place so it can be
// reacquired later rather than removing it outright.
func applyMinedRemnant(s *State, id int, minedUnits float64) {
	ast, idx := FindAsteroid(s, id)
	if ast == nil || ast.Volume <= 0 {
		return
	}
	remainFrac := clamp((float64(ast.Volume)-minedUnits)/float64(ast.Volume), 0, 1)
	newVolume := int(math.Round(float64(ast.Volume) * remainFrac))
	if newVolume < 1 {
		newVolume = 1
	}
	ast.Value = int(math.Round(float64(ast.Value) * remainFrac))
	ast.DrillSec = round1(ast.DrillSec * remainFrac)
	if ast.DrillSec < 0.5 {
		ast.DrillSec = 0.5
	}
	ast.Volume = newVolume
	s.Belt[idx] = *ast
}

func appendRunLog(s *State, rec RunRecord) {
	s.RunLog = append([]RunRecord{rec}, s.RunLog...)
	if len(s.RunLog) > 20 {
		s.RunLog = s.RunLog[:20]
	}
}

func updateStatsOnOutcome(s *State, kind OutcomeKind, tier int) {
	s.Stats.RunsTotal++
	switch kind {
	case OutcomeDeparted:
		s.Stats.RunsDeparted++
	case OutcomeBailed:
		s.Stats.RunsBailed++
	case OutcomeTributePaid:
		s.Stats.RunsTributePaid++
	case OutcomeEscapedUnderFire:
		s.Stats.RunsEscapedUnderFire++
	}
	if tier == 3 {
		s.Stats.LegendariesMined++
	}
}
