package sim

import (
	"math"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// tickSkillCheck advances or rolls the mining skill-check prompt. It is only
// called during PhaseMining, guarded by the same "outage" pause as regular
// drilling — pirates and hull damage still tick during a power outage, but
// the calibration minigame does not.
//
// The check is deliberately just a countdown: press the hotkey any time
// before it expires and it's a hit. There is no moving target to line up —
// over a laggy SSH connection, timing a press against a sweeping marker is
// unreliable in a way that has nothing to do with player skill. Letting the
// countdown expire without pressing is a miss, but a miss costs nothing.
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

	run.SkillCheck = &SkillCheck{Window: c.Mining.SkillCheckWindowSeconds}
}

func rollNextSkillCheckIn(s *State, c *content.Content, run *ActiveRun) {
	mc := c.Mining
	rng := runRNG(s, run, 6500+run.TickCount)
	lo, hi := mc.SkillCheckMinIntervalSeconds, mc.SkillCheckMaxIntervalSeconds
	run.NextSkillCheckIn = lo + rng.Float64()*(hi-lo)
}

// AttemptSkillCheck resolves a hotkey press against the currently active
// skill check, if any. Any press while a check is active is a hit — passing
// grants a flat mining-progress bonus (the drill "speeds up"). There is no
// penalty for missing: if there's no active check (already expired, or none
// spawned), the press is simply a no-op.
func AttemptSkillCheck(s *State, c *content.Content, now int64) error {
	run := s.Run
	if run == nil {
		return ErrNoActiveRun
	}
	if run.Phase != PhaseMining || run.SkillCheck == nil {
		return nil
	}
	run.SkillCheck = nil
	rollNextSkillCheckIn(s, c, run)

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
