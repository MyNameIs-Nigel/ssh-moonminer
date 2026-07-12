package sim

import (
	"math"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// badEvents stack pressure fast (hull bleed, escape penalty, fuel burn) and
// become more common as hull falls. nuisanceEvents are merely disruptive.
var badEvents = []EventKind{EventLifeSupportFailure, EventCargoShift, EventReactorSurge}
var nuisanceEvents = []EventKind{EventPowerOutage, EventRadarBlackout}

// tickEvents advances the active event (if any) and, once its cooldown has
// elapsed, rolls for a new one. Events only fire during mining and escaping
// — the tribute phase is a short, deliberately quiet decision beat.
func tickEvents(s *State, c *content.Content, run *ActiveRun, dt float64, now int64) {
	if run.Phase == PhaseTribute {
		return
	}
	if run.ActiveEvent != nil {
		run.ActiveEvent.Remaining -= dt
		if run.ActiveEvent.Kind == EventLifeSupportFailure && run.ActiveEvent.Remaining <= 0 && !run.LifeSupportBreached {
			if run.Phase == PhaseMining {
				run.LifeSupportBreached = true
			}
		}
		if run.ActiveEvent.Remaining <= 0 {
			run.ActiveEvent = nil
			run.EventCooldown = c.Events.CooldownSeconds
		}
		return
	}
	if run.EventCooldown > 0 {
		run.EventCooldown = math.Max(0, run.EventCooldown-dt)
		return
	}

	// Random events only roll below the hull gate (20% by default) — above
	// it, chance is exactly zero, not merely low. See gameplay/05-fleet-
	// ships-and-shipyard-economy.md "Random events: hull-gated, not
	// hull-scaled".
	gate := c.Fleet.EventHullGatePct
	maxHull := MaxHull(s, c)
	if maxHull <= 0 {
		return
	}
	hullPct := clamp(float64(s.Hull)/float64(maxHull), 0, 1)
	if hullPct >= gate {
		return
	}
	seriousness := (gate - hullPct) / gate // 0 at the gate, 1 at hull=0

	chancePerMinute := c.Events.BaseChancePerMinute + seriousness*c.Events.LowHullChanceBonus
	chanceThisTick := chancePerMinute / 60.0 * dt
	rng := runRNG(s, run, 4000+run.TickCount)
	if rng.Float64() >= chanceThisTick {
		return
	}

	badWeight := c.Events.BaseBadWeight + seriousness*c.Events.LowHullBadWeight
	isBad := rng.Float64() < badWeight/(badWeight+1)
	var kind EventKind
	if isBad {
		kind = badEvents[rng.Intn(len(badEvents))]
	} else {
		kind = nuisanceEvents[rng.Intn(len(nuisanceEvents))]
	}
	startEvent(s, c, run, kind, now)
}

func startEvent(s *State, c *content.Content, run *ActiveRun, kind EventKind, now int64) {
	ec := c.Events
	ev := &RunEvent{Kind: kind}
	switch kind {
	case EventPowerOutage:
		ev.Remaining = ec.PowerOutageSeconds
		ev.HUDTreatment = HUDBlackout
	case EventRadarBlackout:
		ev.Remaining = ec.RadarBlackoutSeconds
		ev.HUDTreatment = HUDStatic
	case EventLifeSupportFailure:
		ev.Remaining = ec.LifeSupportCountdownSeconds
		ev.HUDTreatment = HUDRedPulse
	case EventCargoShift:
		ev.Remaining = ec.CargoShiftEffectSeconds
		ev.HUDTreatment = HUDJitter
		factor := 1 + ec.CargoShiftEscapePct
		run.CargoShiftPenaltyMul *= factor
		if run.Phase == PhaseEscaping {
			run.BaseEscapeSecondsRequired *= factor
		}
	case EventReactorSurge:
		ev.Remaining = ec.ReactorSurgeSeconds
		ev.HUDTreatment = HUDAmberGlow
		rng := runRNG(s, run, 5000+run.TickCount)
		if rng.Float64() < ec.ReactorSurgeHullHitChance {
			applyHullDamageInstant(s, c, run, float64(ec.ReactorSurgeHullHitAmount))
		}
	}
	run.ActiveEvent = ev
	run.EventLog = append(run.EventLog, RunEventRecord{Kind: kind, At: now})
}
