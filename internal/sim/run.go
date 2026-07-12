package sim

import (
	"math"
	"time"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// OutcomeKind identifies run resolution type.
type OutcomeKind string

const (
	OutcomeDeparted         OutcomeKind = "departed"
	OutcomeBailed           OutcomeKind = "bailed"
	OutcomeTributePaid      OutcomeKind = "tribute_paid"
	OutcomeEscapedUnderFire OutcomeKind = "escaped_under_fire"
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
	OutcomeShipLost:         {"CONNECTION LOST", "The ship and its hold are gone."},
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
	if RemainingCargoCapacity(s, c) <= 0 {
		return ErrCargoFull
	}
	if s.Fuel < float64(ast.FuelCost) {
		return ErrInsufficientFuel
	}
	s.Fuel -= float64(ast.FuelCost)
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
		s.Run.PirateImmune = true
	}
	s.Run.PirateBearing = runRNG(s, s.Run, 7000).Float64()
	rollNextSkillCheckIn(s, c, s.Run)
	updatePirateETA(s, c, s.Run, ast)
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
	if IsOutOfRange(s, c, ast) {
		return ErrOutOfRange
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
	ast, _ := FindAsteroid(s, run.AsteroidID)
	if ast == nil {
		s.Run = nil
		return nil
	}
	demand := int(math.Round(float64(run.CargoValue) * c.Mining.TributeDemandPct))
	if demand > run.CargoValue {
		demand = run.CargoValue
	}
	if ast.Value > 0 {
		lostUnits := float64(demand) / float64(ast.Value) * float64(ast.Volume)
		run.MinedUnits = math.Max(0, run.MinedUnits-lostUnits)
	}
	run.CargoValue -= demand
	run.TributePaid = true
	run.EventLog = append(run.EventLog, RunEventRecord{Kind: "tribute_paid", At: now})
	startEscape(s, c, ast, false)
	return nil
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
	startEscape(s, c, ast, true)
	return nil
}

func startEscape(s *State, c *content.Content, ast *Asteroid, underAttack bool) {
	run := s.Run
	run.Phase = PhaseEscaping
	if underAttack {
		run.UnderAttack = true
	}
	if run.Intent == "" {
		if RunDepleted(s, c, ast, run) {
			run.Intent = OutcomeDeparted
		} else {
			run.Intent = OutcomeBailed
		}
	}
	cargoLoadRatio := 0.0
	if cap_ := CargoCapacityUnits(s, c); cap_ > 0 {
		cargoLoadRatio = clamp((s.CargoUnits+run.MinedUnits)/cap_, 0, 1)
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
		if run.PirateDistance <= 0 && !run.EMPActive {
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
	}

	if s.Hull <= 0 {
		out := resolveRun(s, c, OutcomeShipLost, now)
		return out, true
	}
	if run.Phase == PhaseEscaping && run.EscapeSecondsElapsed >= run.EscapeSecondsRequired {
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
		cargoCap := MiningRunCapacity(s, c, ast)
		remaining := cargoCap - run.MinedUnits
		if remaining > 0 && s.Fuel > 0 && ast.DrillSec > 0 {
			mineRate := float64(ast.Volume) / ast.DrillSec
			mined := math.Min(remaining, mineRate*dt)
			if mined > 0 {
				run.MinedUnits += mined
			}
			fuelDrain := (c.Mining.FuelDrainBase + float64(ast.Tier)*c.Mining.FuelDrainPerTier) * FuelDrainMul(s, c)
			if run.ActiveEvent != nil && run.ActiveEvent.Kind == EventReactorSurge {
				fuelDrain *= c.Events.ReactorSurgeFuelMul
			}
			burn := fuelDrain * dt
			// Fuel Miner refunds a fraction of a Rare+ asteroid's FuelCost,
			// spread proportionally to how much of it has been mined so far.
			if ast.Tier >= 2 && ast.Volume > 0 {
				if refundPct := FuelMinerRefundPct(s, c); refundPct > 0 {
					refund := refundPct * float64(ast.FuelCost) * (mined / float64(ast.Volume))
					burn = math.Max(0, burn-refund)
				}
			}
			s.Fuel = math.Max(0, s.Fuel-burn)
			s.Stats.FuelBurned += math.Min(burn, s.Fuel+burn)
		}
		if ast.Volume > 0 {
			run.CargoValue = int(math.Round(float64(ast.Value) * run.MinedUnits / float64(ast.Volume)))
		}
		tickSkillCheck(s, c, run, ast, dt)
	}

	if run.LifeSupportBreached {
		applyHullDamageRate(s, run, float64(c.Events.LifeSupportBleedPerSecond), dt)
	}

	// A Pirate Jammer charge (consumed at Lock) keeps pirates from ever
	// approaching this asteroid at all.
	blackout := run.ActiveEvent != nil && run.ActiveEvent.Kind == EventRadarBlackout
	if !blackout && !run.PirateImmune {
		rate := pirateApproachRate(s, c, ast)
		run.PirateDistance = math.Max(0, run.PirateDistance-rate*dt)
		updatePirateETA(s, c, run, ast)
	}
}

// RunDepleted reports whether an in-progress run has reached the green
// DEPART state: the asteroid's own volume is exhausted, or the ship's cargo
// hold filled up first, whichever comes sooner.
func RunDepleted(s *State, c *content.Content, ast *Asteroid, run *ActiveRun) bool {
	if ast == nil {
		return true
	}
	cap_ := MiningRunCapacity(s, c, ast)
	return run.MinedUnits >= cap_
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
	run.EscapeSecondsElapsed += dt
	if run.UnderAttack {
		applyAttackDamageRate(s, c, run, attackHullDamagePerSecond(s, c), dt)
	}
	fuelDrain := c.Mining.EscapeFuelDrainPerSec * FuelDrainMul(s, c)
	if run.ActiveEvent != nil && run.ActiveEvent.Kind == EventReactorSurge {
		fuelDrain *= c.Events.ReactorSurgeFuelMul
	}
	s.Fuel = math.Max(0, s.Fuel-fuelDrain*dt)
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
		startEscape(s, c, ast, true)
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
	startEscape(s, c, ast, true)
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
func applyHullDamageRate(s *State, run *ActiveRun, ratePerSecond, dt float64) {
	if ratePerSecond <= 0 || dt <= 0 {
		return
	}
	applyHullDamageCarry(s, run, ratePerSecond*dt)
}

// applyHullDamageInstant applies a single lump non-combat hit (e.g. reactor
// surge) straight to the hull, same reasoning as applyHullDamageRate.
func applyHullDamageInstant(s *State, run *ActiveRun, amount float64) {
	if amount <= 0 {
		return
	}
	applyHullDamageCarry(s, run, amount)
}

// applyAttackDamageRate applies continuous pirate-attack damage: the
// Defense Turret's percentage mitigation reduces the incoming rate, then
// the active ship's persistent Shield buffer absorbs what's left, and only
// the remainder reaches the hull.
func applyAttackDamageRate(s *State, c *content.Content, run *ActiveRun, ratePerSecond, dt float64) {
	if ratePerSecond <= 0 || dt <= 0 {
		return
	}
	amount := ratePerSecond * AttackDamageMul(s, c) * dt
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
	applyHullDamageCarry(s, run, amount)
}

func applyHullDamageCarry(s *State, run *ActiveRun, reduced float64) {
	if reduced <= 0 || s.DevGodMode {
		return
	}
	run.HullDamageCarry += reduced
	whole := int(run.HullDamageCarry)
	if whole > 0 {
		s.Hull = max(0, s.Hull-whole)
		run.HullDamageCarry -= float64(whole)
	}
}

func resolveRun(s *State, c *content.Content, kind OutcomeKind, now int64) *RunOutcome {
	run := s.Run
	if run == nil {
		return nil
	}
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
		s.CargoUnits += run.MinedUnits
	}

	hullDelta := s.Hull - run.StartHull
	fuelDelta := s.Fuel - run.StartFuel
	depleted := RunDepleted(s, c, ast, run)
	// volumeExhausted (the rock itself has nothing left) is distinct from
	// depleted (which also goes true when the cargo hold fills up first,
	// gameplay/05). Only true volume exhaustion removes the asteroid
	// outright on Departed — a cargo-capped Departed leaves real ore behind
	// and must go through the same remnant-preservation path as a bail.
	volumeExhausted := astVolume > 0 && run.MinedUnits >= float64(astVolume)

	if ast != nil {
		switch {
		case kind == OutcomeShipLost:
			RemoveAsteroid(s, run.AsteroidID)
		case kind == OutcomeDeparted && volumeExhausted:
			RemoveAsteroid(s, run.AsteroidID)
		default:
			remainRatio := 1.0
			if astVolume > 0 {
				remainRatio = 1 - run.MinedUnits/float64(astVolume)
			}
			if remainRatio < c.Mining.RemnantKeepThreshold {
				RemoveAsteroid(s, run.AsteroidID)
			} else {
				applyMinedRemnant(s, run.AsteroidID, run.MinedUnits)
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
		Outcome: string(kind), CargoValueRecovered: recovered, CargoValueLost: lost,
		HullDelta: hullDelta, FuelDelta: fuelDelta, Depleted: depleted, Events: events,
	}
	appendRunLog(s, rec)
	updateStatsOnOutcome(s, kind, astTier)

	if kind == OutcomeShipLost {
		respawnActiveShip(s, c)
	} else if kind == OutcomeDeparted {
		s.Settings.InsuranceUsed = false
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
