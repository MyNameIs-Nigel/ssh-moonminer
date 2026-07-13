package sim

import (
	"fmt"
	"math"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// rollPirateID weighted-picks a named roster entry for this run's pirate,
// deterministically, from asteroid/world threat — docs/gameplay/07-pirate-
// combat-and-bounties.md. Called once, from Lock, so the pirate foretold by
// the whole radar approach is the one combat starts against.
func rollPirateID(s *State, c *content.Content, run *ActiveRun, ast *Asteroid) string {
	if len(c.Pirates) == 0 {
		return ""
	}
	mul := 1.0
	if w := c.WorldByIndex(s.WorldIdx); w != nil && w.PirateMul > 0 {
		mul = w.PirateMul
	}
	threat := 0.0
	if ast != nil {
		threat = float64(ast.Risk) / 100.0 * mul
	}
	var eligible []content.Pirate
	totalWeight := 0.0
	for _, p := range c.Pirates {
		if threat >= p.MinThreat && threat <= p.MaxThreat {
			eligible = append(eligible, p)
			totalWeight += math.Max(p.Weight, 0)
		}
	}
	if len(eligible) == 0 || totalWeight <= 0 {
		// No band covers this threat (or all matching weights are zero) —
		// fall back to whichever roster entry's band is numerically closest.
		best := c.Pirates[0]
		bestDist := pirateThreatBandDist(best, threat)
		for _, p := range c.Pirates[1:] {
			if d := pirateThreatBandDist(p, threat); d < bestDist {
				best, bestDist = p, d
			}
		}
		return best.ID
	}
	rng := runRNG(s, run, 7100)
	roll := rng.Float64() * totalWeight
	for _, p := range eligible {
		roll -= math.Max(p.Weight, 0)
		if roll <= 0 {
			return p.ID
		}
	}
	return eligible[len(eligible)-1].ID
}

func pirateThreatBandDist(p content.Pirate, threat float64) float64 {
	if threat < p.MinThreat {
		return p.MinThreat - threat
	}
	if threat > p.MaxThreat {
		return threat - p.MaxThreat
	}
	return 0
}

// worldPirateAttackMul returns the current world's incoming pirate-damage
// multiplier (1 if unset/invalid), shared by the legacy escape-damage path
// and combat.
func worldPirateAttackMul(s *State, c *content.Content) float64 {
	if w := c.WorldByIndex(s.WorldIdx); w != nil && w.PirateAttackMul > 0 {
		return w.PirateAttackMul
	}
	return 1
}

// startCombat begins a tactical-scope engagement against the run's rolled
// pirate (run.PirateID, set at Lock). withEscape pre-starts the escape burn
// alongside combat — the "burn matrix" from docs/gameplay/07: true for
// RefuseTribute and an unarmed immediate attack, false for FightPirates and
// an armed immediate attack (the player must press B/Enter to start
// running).
func startCombat(s *State, c *content.Content, run *ActiveRun, ast *Asteroid, withEscape bool, now int64) {
	normalizeRunAccounting(run)
	p := c.PirateByID(run.PirateID)
	if p == nil && len(c.Pirates) > 0 {
		p = &c.Pirates[0]
	}
	if p == nil {
		// No pirate content at all — nothing to fight; leave the run
		// mining rather than crash on missing/invalid content.
		return
	}
	rng := runRNG(s, run, 9100)
	run.Phase = PhaseCombat
	run.UnderAttack = true
	run.Combat = &CombatState{
		PirateName:    p.Name,
		PirateHull:    p.Hull,
		PirateMaxHull: p.Hull,
		PirateDPS:     p.DamagePerSecond * worldPirateAttackMul(s, c),
		Bounty:        p.Bounty,
		Maneuver:      math.Max(p.Maneuver, 0.01),
		Phase1:        rng.Float64() * 2 * math.Pi,
		Phase2:        rng.Float64() * 2 * math.Pi,
	}
	run.Combat.OddsPct = int(math.Round(EstimateOdds(s, c, p) * 100))
	if withEscape {
		configureEscape(s, c, run, ast)
		run.Combat.EscapeStarted = true
	}
	run.EventLog = append(run.EventLog, RunEventRecord{Kind: EventKind("combat_started:" + p.ID), At: now})
}

// FightPirates is the [F] FIGHT tribute-prompt action: valid only during
// tribute, requires an installed weapon, and — unlike RefuseTribute — does
// not start the escape burn; the player stands their ground until
// CombatEscape (B/Enter).
func FightPirates(s *State, c *content.Content, now int64) error {
	run := s.Run
	if run == nil {
		return ErrNoActiveRun
	}
	if run.Phase != PhaseTribute {
		return ErrInvalidRunPhase
	}
	if !HasWeapon(s) {
		return ErrNoWeaponInstalled
	}
	ast, _ := FindAsteroid(s, run.AsteroidID)
	if ast == nil {
		s.Run = nil
		return nil
	}
	run.EventLog = append(run.EventLog, RunEventRecord{Kind: "tribute_fought", At: now})
	startCombat(s, c, run, ast, false, now)
	return nil
}

// CombatEscape starts the escape burn from PhaseCombat — the [B]/[Enter]
// action mirroring BailOrDepart's bail-vs-depart intent selection. A no-op
// if the burn is already running (Refuse/unarmed-attack pre-started it).
func CombatEscape(s *State, c *content.Content, now int64) error {
	run := s.Run
	if run == nil {
		return ErrNoActiveRun
	}
	if run.Phase != PhaseCombat || run.Combat == nil {
		return ErrInvalidRunPhase
	}
	if run.Combat.EscapeStarted {
		return nil
	}
	ast, _ := FindAsteroid(s, run.AsteroidID)
	if ast == nil {
		s.Run = nil
		return nil
	}
	normalizeRunAccounting(run)
	configureEscape(s, c, run, ast)
	run.Combat.EscapeStarted = true
	return nil
}

// FireWeapons fires every installed weapon as one volley against the
// current firing solution. Requires PhaseCombat, at least one installed
// weapon, and an un-overheated capacitor.
func FireWeapons(s *State, c *content.Content, now int64) error {
	run := s.Run
	if run == nil {
		return ErrNoActiveRun
	}
	if run.Phase != PhaseCombat || run.Combat == nil {
		return ErrInvalidRunPhase
	}
	if !HasWeapon(s) {
		return ErrNoWeaponInstalled
	}
	cs := run.Combat
	if cs.LockRemaining > 0 {
		return ErrWeaponsLocked
	}
	damage, heat, _ := InstalledWeaponVolley(s, c)
	cs.ShotsFired++
	rng := runRNG(s, run, 10000+run.TickCount)
	hit := rng.Float64() < cs.Solution
	cs.Heat += heat
	if cs.Heat >= c.Combat.HeatCapacity {
		cs.Heat = c.Combat.HeatCapacity
		cs.LockRemaining = c.Combat.OverheatLockSeconds
		cs.Log = prependCombatLog(cs.Log, "CAPACITORS OVERHEATED")
	}
	if hit {
		cs.ShotsHit++
		cs.PirateHull = math.Max(0, cs.PirateHull-damage)
		cs.Log = prependCombatLog(cs.Log, fmt.Sprintf("Direct hit! %s hull -%.0f", cs.PirateName, damage))
		if cs.PirateHull <= 0 {
			winCombat(s, c, run, now)
		}
	} else {
		cs.Log = prependCombatLog(cs.Log, "Shot missed — solution too weak")
	}
	return nil
}

const combatLogMaxLines = 6

func prependCombatLog(log []string, line string) []string {
	log = append([]string{line}, log...)
	if len(log) > combatLogMaxLines {
		log = log[:combatLogMaxLines]
	}
	return log
}

// winCombat resolves a destroyed pirate: the bounty becomes a voucher
// immediately (it survives ship loss, unlike cargo), and combat ends either
// back into mining (burn never started — the radar stays clear for the
// rest of this run) or by dropping UnderAttack while an already-running
// burn continues uninterrupted.
func winCombat(s *State, c *content.Content, run *ActiveRun, now int64) {
	cs := run.Combat
	if cs == nil {
		return
	}
	s.BountyVouchers += cs.Bounty
	s.Stats.PiratesDestroyed++
	s.Stats.BountyCreditsEarned += cs.Bounty
	run.PirateDestroyed = cs.PirateName
	run.BountyEarned = cs.Bounty
	run.EventLog = append(run.EventLog, RunEventRecord{Kind: "pirate_destroyed", At: now})
	escapeStarted := cs.EscapeStarted
	run.Combat = nil
	run.UnderAttack = false
	if escapeStarted {
		run.Phase = PhaseEscaping
	} else {
		run.Phase = PhaseMining
		run.PirateImmune = true
	}
}

// tickCombat advances one PhaseCombat tick: the pirate's maneuver (a pure,
// deterministic function of TickCount so replay stays exact — no per-tick
// RNG here), the firing solution, heat decay/lockout, incoming constant-
// rate pirate damage, and — once Combat.EscapeStarted — the shared escape
// burn bookkeeping from run.go's tickEscapeBurn.
func tickCombat(s *State, c *content.Content, run *ActiveRun, dt float64) {
	cs := run.Combat
	if cs == nil {
		return
	}
	cc := c.Combat
	hz := float64(c.Mining.TickHz)
	if hz < 1 {
		hz = 4
	}
	t := float64(run.TickCount) / hz
	m := cs.Maneuver
	cs.Bearing = wrap01(cc.ArcCenter +
		cc.ManeuverAmp1*math.Sin(2*math.Pi*cc.ManeuverW1*m*t+cs.Phase1) +
		cc.ManeuverAmp2*math.Sin(2*math.Pi*cc.ManeuverW2*m*t+cs.Phase2))
	cs.Range = cc.RangeMin + (1-cc.RangeMin)*math.Abs(math.Sin(2*math.Pi*cc.ManeuverW3*m*t+cs.Phase1))

	arcHalf := math.Max(cc.ArcHalfWidth, 0.0001)
	arcFactor := clamp(1-angDist01(cs.Bearing, cc.ArcCenter)/arcHalf, 0, 1)
	rangeFactor := cc.SolutionRangeFloor + (1-cc.SolutionRangeFloor)*(1-cs.Range)
	_, _, solutionBonus := InstalledWeaponVolley(s, c)
	cs.Solution = clamp(arcFactor*rangeFactor+solutionBonus, 0, 1)

	if dt > 0 {
		cs.Heat = math.Max(0, cs.Heat-cc.HeatDecayPerSecond*dt)
		cs.LockRemaining = math.Max(0, cs.LockRemaining-dt)
	}

	applyAttackDamageRate(s, c, run, cs.PirateDPS, dt)

	if cs.EscapeStarted {
		tickEscapeBurn(s, c, run, dt)
	}
}

func wrap01(v float64) float64 {
	v = math.Mod(v, 1)
	if v < 0 {
		v += 1
	}
	return v
}

// angDist01 returns the shortest circular distance between two 0..1
// values.
func angDist01(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > 0.5 {
		d = 1 - d
	}
	return d
}

// EstimateOdds is a deterministic, content-only (no RNG) win estimate for
// the tribute prompt's [F] FIGHT button and the combat header — a
// time-to-kill ratio between the player's installed weapons and the
// pirate's hull/DPS. Zero when unarmed.
func EstimateOdds(s *State, c *content.Content, p *content.Pirate) float64 {
	if p == nil || !HasWeapon(s) {
		return 0
	}
	cc := c.Combat
	maneuver := math.Max(p.Maneuver, 0.01)
	avgSol := clamp(cc.OddsAvgSolutionBase/maneuver, 0.15, 0.90)
	volleyDmg, volleyHeat, _ := InstalledWeaponVolley(s, c)
	if volleyDmg <= 0 || volleyHeat <= 0 || cc.HeatDecayPerSecond <= 0 {
		return cc.OddsFloor
	}
	sustained := cc.HeatDecayPerSecond / volleyHeat
	pDPS := volleyDmg * sustained * avgSol
	if pDPS <= 0 {
		return cc.OddsFloor
	}
	ttkPirate := p.Hull / pDPS

	ehp := float64(MaxHull(s, c)) + ShieldMaxHP(s, c)
	pirateDPS := p.DamagePerSecond * worldPirateAttackMul(s, c)
	if pirateDPS <= 0 {
		return cc.OddsCeiling
	}
	ttkSelf := ehp / pirateDPS

	odds := ttkSelf / (ttkSelf + ttkPirate)
	return clamp(odds, cc.OddsFloor, cc.OddsCeiling)
}

// EstimateOddsForRun is EstimateOdds against the run's already-rolled
// pirate (run.PirateID) — the convenience form the tribute-prompt and
// combat-header TUI code calls.
func EstimateOddsForRun(s *State, c *content.Content, run *ActiveRun) float64 {
	if run == nil {
		return 0
	}
	return EstimateOdds(s, c, c.PirateByID(run.PirateID))
}
