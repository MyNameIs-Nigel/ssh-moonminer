package sim

import (
	"math"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// SkillCheckPosition returns the current 0..1 marker position for an active
// skill check, for the TUI to render. It uses the same formula as the sim's
// own hit test, so the rendered marker and the hit test never disagree.
func SkillCheckPosition(sc *SkillCheck) float64 {
	if sc == nil {
		return 0
	}
	return sweepPosition(sc.Elapsed, sc.Period)
}

// sweepPosition is the pure triangle-wave function driving the skill-check
// marker. It is shared by the tick (for rendering) and AttemptSkillCheck
// (for the hit test) so the two can never disagree about where the marker
// currently sits.
func sweepPosition(elapsed, period float64) float64 {
	if period <= 0 {
		return 0
	}
	t := math.Mod(elapsed, period) / period
	if t < 0.5 {
		return t * 2
	}
	return 2 - t*2
}

// tickSkillCheck advances or rolls the mining skill-check prompt. It is only
// called during PhaseMining, guarded by the same "outage" pause as regular
// drilling — pirates and hull damage still tick during a power outage, but
// the calibration minigame does not.
func tickSkillCheck(s *State, c *content.Content, run *ActiveRun, ast *Asteroid, dt float64) {
	if run.SkillCheck != nil {
		run.SkillCheck.Elapsed += dt
		if run.SkillCheck.Elapsed >= run.SkillCheck.Window {
			run.SkillCheck = nil
			rollNextSkillCheckIn(s, c, run)
		}
		return
	}

	run.NextSkillCheckIn -= dt
	if run.NextSkillCheckIn > 0 {
		return
	}
	if ast == nil || ast.Volume <= 0 {
		return
	}
	// Don't spawn a check once the rock is nearly depleted — BAIL/DEPART is
	// about to be the only thing that matters.
	remainRatio := 1 - run.MinedUnits/float64(ast.Volume)
	if remainRatio < 0.05 {
		return
	}

	rng := runRNG(s, run, 6000+run.TickCount)
	mc := c.Mining
	width := mc.SkillCheckZoneWidthPct
	run.SkillCheck = &SkillCheck{
		ZoneStart: rng.Float64() * (1 - width),
		ZoneWidth: width,
		Period:    mc.SkillCheckSweepPeriodSeconds,
		Window:    mc.SkillCheckWindowSeconds,
	}
}

func rollNextSkillCheckIn(s *State, c *content.Content, run *ActiveRun) {
	mc := c.Mining
	rng := runRNG(s, run, 6500+run.TickCount)
	lo, hi := mc.SkillCheckMinIntervalSeconds, mc.SkillCheckMaxIntervalSeconds
	run.NextSkillCheckIn = lo + rng.Float64()*(hi-lo)
}

// AttemptSkillCheck resolves a press/click against the currently active
// skill check, if any. A hit grants a flat mining-progress bonus; a miss
// (or no active check) costs nothing. Either way, resolving an attempt
// consumes the current check and re-rolls the countdown to the next one.
func AttemptSkillCheck(s *State, c *content.Content, now int64) error {
	run := s.Run
	if run == nil {
		return ErrNoActiveRun
	}
	if run.Phase != PhaseMining || run.SkillCheck == nil {
		return nil
	}
	sc := run.SkillCheck
	pos := sweepPosition(sc.Elapsed, sc.Period)
	hit := s.Settings.ReducedMotion || (pos >= sc.ZoneStart && pos <= sc.ZoneStart+sc.ZoneWidth)
	run.SkillCheck = nil
	rollNextSkillCheckIn(s, c, run)
	if !hit {
		return nil
	}

	ast, _ := FindAsteroid(s, run.AsteroidID)
	if ast == nil || ast.Volume <= 0 {
		return nil
	}
	remaining := float64(ast.Volume) - run.MinedUnits
	bonus := remaining * c.Mining.SkillCheckBonusPct
	if bonus > 0 {
		run.MinedUnits += bonus
		if ast.Volume > 0 {
			run.CargoValue = int(math.Round(float64(ast.Value) * run.MinedUnits / float64(ast.Volume)))
		}
		run.EventLog = append(run.EventLog, RunEventRecord{Kind: "skill_check_hit", At: now})
	}
	return nil
}
