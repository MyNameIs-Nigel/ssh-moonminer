package game

import (
	"sync"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

// Session is one terminal's handle onto a save actor.
type Session struct {
	id       uint64
	actor    *actor
	kicked   chan string
	kickFn   func(reason string)
	kickOnce sync.Once

	snapCh      chan sim.Snapshot
	lastOutcome *sim.RunOutcome
}

// Snapshot returns the latest state copy.
type Snapshot = sim.Snapshot

// Kicked yields takeover/shutdown notices.
func (s *Session) Kicked() <-chan string { return s.kicked }

// Snapshots yields mining tick updates.
func (s *Session) Snapshots() <-chan sim.Snapshot { return s.snapCh }

// LastOutcome returns the most recent run outcome (if any).
func (s *Session) LastOutcome() *sim.RunOutcome { return s.lastOutcome }

// ClearOutcome resets the stored outcome after the TUI consumes it.
func (s *Session) ClearOutcome() { s.lastOutcome = nil }

func (s *Session) deliverKick(reason string) {
	s.kickOnce.Do(func() {
		if reason != "" {
			s.kicked <- reason
			if s.kickFn != nil {
				s.kickFn(reason)
			}
		}
		close(s.kicked)
	})
}

// Detach persists and releases the session.
func (s *Session) Detach() {
	s.actor.mgr.detach(s)
	s.deliverKick("")
}

func (s *Session) intent(now int64, apply func(st *sim.State) error) (Snapshot, error) {
	var snap Snapshot
	var actErr error
	ok := s.actor.do(func() {
		actErr = apply(s.actor.state)
		if actErr == nil {
			s.actor.dirty = true
		}
		snap = Snapshot{State: *s.actor.state.Clone()}
	})
	if !ok {
		return Snapshot{}, ErrSessionClosed
	}
	return snap, actErr
}

// SnapshotNow returns current state without mutation.
func (s *Session) SnapshotNow() (Snapshot, error) {
	var snap Snapshot
	ok := s.actor.do(func() {
		snap = Snapshot{State: *s.actor.state.Clone()}
	})
	if !ok {
		return Snapshot{}, ErrSessionClosed
	}
	return snap, nil
}

func (s *Session) Depart(now int64, worldIdx int) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.Depart(st, s.actor.content(), worldIdx)
	})
}

func (s *Session) Scan(now int64, asteroidID int) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.Scan(st, s.actor.content(), asteroidID, now)
	})
}

func (s *Session) Dock(now int64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		sim.Dock(st)
		return nil
	})
}

func (s *Session) Lock(now int64, asteroidID int) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.Lock(st, s.actor.content(), asteroidID, now)
	})
}

// BailOrDepart starts the escape sequence: yellow BAIL while resources
// remain, green DEPART once the asteroid is depleted. The outcome (if any)
// arrives asynchronously via Snapshots()/LastOutcome() once the escape
// sequence resolves.
func (s *Session) BailOrDepart(now int64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.BailOrDepart(st, s.actor.content(), now)
	})
}

// AcceptTribute jettisons the demanded cargo and starts escaping unarmed.
func (s *Session) AcceptTribute(now int64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.AcceptTribute(st, s.actor.content(), now)
	})
}

// RefuseTribute rejects the pirates' demand and starts fleeing under fire.
func (s *Session) RefuseTribute(now int64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.RefuseTribute(st, s.actor.content(), now)
	})
}

// AttemptSkillCheck resolves a press/click against the active mining
// "drill calibration" prompt, if any. Misses cost nothing.
func (s *Session) AttemptSkillCheck(now int64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.AttemptSkillCheck(st, s.actor.content(), now)
	})
}

func (s *Session) Refuel(now int64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.Refuel(st, s.actor.content())
	})
}

func (s *Session) Repair(now int64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.Repair(st, s.actor.content())
	})
}

func (s *Session) Insurance(now int64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.InsuranceAdvance(st, s.actor.content())
	})
}

func (s *Session) BuyUpgrade(now int64, track sim.UpgradeTrack) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.BuyUpgrade(st, s.actor.content(), track)
	})
}

func (s *Session) UpdateSettings(now int64, fn func(*sim.Settings)) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		sim.UpdateSettings(st, fn)
		return nil
	})
}
