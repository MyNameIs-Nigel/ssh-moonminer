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

func (s *Session) SetOverdrive(on bool) (Snapshot, error) {
	var snap Snapshot
	ok := s.actor.do(func() {
		sim.SetOverdrive(s.actor.state, on)
		s.actor.dirty = true
		snap = Snapshot{State: *s.actor.state.Clone()}
	})
	if !ok {
		return Snapshot{}, ErrSessionClosed
	}
	return snap, nil
}

func (s *Session) Bail(now int64) (Snapshot, *sim.RunOutcome, error) {
	var snap Snapshot
	var out *sim.RunOutcome
	ok := s.actor.do(func() {
		out = sim.Bail(s.actor.state, s.actor.content(), now)
		if out != nil {
			s.actor.dirty = true
			s.lastOutcome = out
		}
		snap = Snapshot{State: *s.actor.state.Clone()}
	})
	if !ok {
		return Snapshot{}, nil, ErrSessionClosed
	}
	return snap, out, nil
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
