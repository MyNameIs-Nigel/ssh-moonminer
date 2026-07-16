package sim

import (
	"math"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// rollPressurePoints lays out one to three deterministic fracture points on
// the asteroid surface. The locations deliberately come from a distinct RNG
// stream so changing skill-check cadence cannot change which rocks have fuel
// or how pirates behave.
func rollPressurePoints(s *State, c *content.Content, run *ActiveRun) {
	min, max := c.Mining.PressurePointsMin, c.Mining.PressurePointsMax
	if min < 1 {
		min = 1
	}
	if max < min {
		max = min
	}
	rng := runRNG(s, run, 6400)
	count := min
	if max > min {
		count += rng.Intn(max - min + 1)
	}
	// Eight evenly spaced perimeter anchors give the renderer enough variety
	// without allowing two points to occupy the same visibly confusing spot.
	positions := rng.Perm(8)
	run.PressurePoints = make([]PressurePoint, count)
	for i := range run.PressurePoints {
		run.PressurePoints[i] = PressurePoint{Position: positions[i], Status: PressurePointDormant}
	}
}

// tickSkillCheck advances the currently lit pressure point, or reveals the
// next untouched point after its cooldown. A run that cannot mine any more
// (rock exhausted or cargo full) never presents a stale interaction.
func tickSkillCheck(s *State, c *content.Content, run *ActiveRun, ast *Asteroid, dt float64) {
	if ast == nil || ast.Volume <= 0 || RunDepleted(s, c, ast, run) || RunCargoFull(s, c, run) {
		if run.SkillCheck != nil {
			setPressurePointStatus(run, run.SkillCheck.PressurePoint, PressurePointDormant)
			run.SkillCheck = nil
		}
		return
	}

	if run.SkillCheck != nil {
		run.SkillCheck.Elapsed += dt
		if run.SkillCheck.Elapsed >= run.SkillCheck.Window {
			setPressurePointStatus(run, run.SkillCheck.PressurePoint, PressurePointMissed)
			run.SkillCheck = nil
			rollNextSkillCheckIn(s, c, run)
		}
		return
	}

	// Legacy/dev fixtures with no pressure-point list keep the original
	// one-off calibration behavior. Real Lock-created runs always have points.
	if len(run.PressurePoints) > 0 && nextPressurePoint(run) < 0 {
		return
	}
	run.NextSkillCheckIn -= dt
	if run.NextSkillCheckIn > 0 {
		return
	}

	point := nextPressurePoint(run)
	if len(run.PressurePoints) > 0 && point < 0 {
		return
	}
	if point >= 0 {
		setPressurePointStatus(run, point, PressurePointActive)
	}
	run.SkillCheck = &SkillCheck{Window: c.Mining.SkillCheckWindowSeconds, PressurePoint: point}
}

func nextPressurePoint(run *ActiveRun) int {
	for i := range run.PressurePoints {
		if run.PressurePoints[i].Status == PressurePointDormant {
			return i
		}
	}
	return -1
}

func setPressurePointStatus(run *ActiveRun, index int, status PressurePointStatus) {
	if index >= 0 && index < len(run.PressurePoints) {
		run.PressurePoints[index].Status = status
	}
}

func rollNextSkillCheckIn(s *State, c *content.Content, run *ActiveRun) {
	mc := c.Mining
	rng := runRNG(s, run, 6500+run.TickCount)
	lo, hi := mc.SkillCheckMinIntervalSeconds, mc.SkillCheckMaxIntervalSeconds
	run.NextSkillCheckIn = lo + rng.Float64()*(hi-lo)
}

// AttemptSkillCheck resolves a hotkey press against the currently active
// pressure point. A hit turns it green and fractures a configured fraction of
// the asteroid's remaining ore; missing it turns it red on expiry.
func AttemptSkillCheck(s *State, c *content.Content, now int64) error {
	run := s.Run
	if run == nil {
		return ErrNoActiveRun
	}
	if run.Phase != PhaseMining || run.SkillCheck == nil {
		return nil
	}
	normalizeRunAccounting(run)
	point := run.SkillCheck.PressurePoint
	run.SkillCheck = nil
	setPressurePointStatus(run, point, PressurePointHit)
	rollNextSkillCheckIn(s, c, run)

	ast, _ := FindAsteroid(s, run.AsteroidID)
	if ast == nil || ast.Volume <= 0 || RunDepleted(s, c, ast, run) || RunCargoFull(s, c, run) {
		return nil
	}
	capacityRemaining := math.Max(0, RemainingCargoCapacity(s, c)-run.HeldUnits)
	remaining := math.Min(float64(ast.Volume)-run.ExtractedUnits, capacityRemaining)
	bonus := remaining * c.Mining.SkillCheckBonusPct
	if bonus > 0 {
		run.ExtractedUnits += bonus
		run.HeldUnits += bonus
		run.MinedUnits = run.HeldUnits // compatibility mirror
		if ast.Volume > 0 {
			run.CargoValue = int(math.Round(float64(ast.Value) * run.HeldUnits / float64(ast.Volume)))
		}
		run.EventLog = append(run.EventLog, RunEventRecord{Kind: "pressure_point_hit", At: now})
	}
	return nil
}
