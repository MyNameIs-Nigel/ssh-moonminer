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
		sim.Dock(st, s.actor.content())
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

// FightPirates chooses [F] FIGHT at the tribute prompt: valid only while
// armed, and — unlike RefuseTribute — does not start the escape burn.
func (s *Session) FightPirates(now int64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.FightPirates(st, s.actor.content(), now)
	})
}

// FireWeapons fires the pilot-triggered weapons in PhaseCombat. If the volley
// destroys the pirate, retain the immediate outcome for the TUI summary.
func (s *Session) FireWeapons(now int64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		out, err := sim.FireWeaponsWithOutcome(st, s.actor.content(), now)
		if out != nil {
			s.lastOutcome = out
		}
		return err
	})
}

// CombatEscape starts the escape burn from PhaseCombat — the [B]/[Enter]
// action, a no-op if the burn is already running.
func (s *Session) CombatEscape(now int64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.CombatEscape(st, s.actor.content(), now)
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

func (s *Session) BuySystemPermit(now int64, systemID string) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.BuySystemPermit(st, s.actor.content(), systemID)
	})
}

func (s *Session) BuyDestinationPermit(now int64, destinationID string) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.BuyDestinationPermit(st, s.actor.content(), destinationID)
	})
}

func (s *Session) SellCargo(now int64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		_, err := sim.SellCargo(st, s.actor.content(), now)
		return err
	})
}

// AcquireShip buys (or buys back) a ship model into the hangar and makes it
// active.
func (s *Session) AcquireShip(now int64, modelID string) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.AcquireShip(st, s.actor.content(), modelID)
	})
}

// SwitchActiveShip makes an already-owned ship the active ship.
func (s *Session) SwitchActiveShip(now int64, modelID string) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.SwitchActiveShip(st, s.actor.content(), modelID)
	})
}

// BuyShipTrack buys the next grade of a stat track for an owned ship.
func (s *Session) BuyShipTrack(now int64, shipID string, track sim.Track) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.BuyShipTrack(st, s.actor.content(), shipID, track)
	})
}

// InstallSlotDevice buys and installs a slot item at grade into an owned
// ship's slot.
func (s *Session) InstallSlotDevice(now int64, shipID string, kind sim.SlotKind, index int, itemID string, grade int) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.InstallSlotDevice(st, s.actor.content(), shipID, kind, index, itemID, grade)
	})
}

// StoreSlotDevice removes an installed slot device into the pilot's
// account-wide inventory for a later free re-equip.
func (s *Session) StoreSlotDevice(now int64, shipID string, kind sim.SlotKind, index int) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.StoreSlotDevice(st, s.actor.content(), shipID, kind, index)
	})
}

// SellSlotDevice removes an installed slot device and refunds 95% of its
// buy price in credits.
func (s *Session) SellSlotDevice(now int64, shipID string, kind sim.SlotKind, index int) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.SellSlotDevice(st, s.actor.content(), shipID, kind, index)
	})
}

// ReplaceSlotDeviceWithPurchase sells an installed module and buys its
// catalog replacement as one validated shipyard action.
func (s *Session) ReplaceSlotDeviceWithPurchase(now int64, shipID string, kind sim.SlotKind, index int, itemID string, grade int) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.ReplaceSlotDeviceWithPurchase(st, s.actor.content(), shipID, kind, index, itemID, grade)
	})
}

// InstallSlotDeviceFromInventory moves a stored device out of inventory and
// into an owned ship's slot for free.
func (s *Session) InstallSlotDeviceFromInventory(now int64, shipID string, kind sim.SlotKind, index, invIndex int) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		return sim.InstallSlotDeviceFromInventory(st, s.actor.content(), shipID, kind, index, invIndex)
	})
}

func (s *Session) UpdateSettings(now int64, fn func(*sim.Settings)) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		sim.UpdateSettings(st, fn)
		return nil
	})
}

// DevSetCredits is a dev-server-only debug action (see internal/sim/dev.go).
func (s *Session) DevSetCredits(now int64, v int) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		sim.DevSetCredits(st, v)
		return nil
	})
}

// DevSetFuel is a dev-server-only debug action (see internal/sim/dev.go).
func (s *Session) DevSetFuel(now int64, v float64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		sim.DevSetFuel(st, s.actor.content(), v)
		return nil
	})
}

// DevSetHull is a dev-server-only debug action (see internal/sim/dev.go).
func (s *Session) DevSetHull(now int64, v int) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		sim.DevSetHull(st, s.actor.content(), v)
		return nil
	})
}

// DevSetGodMode is a dev-server-only debug action (see internal/sim/dev.go).
func (s *Session) DevSetGodMode(now int64, on bool) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		sim.DevSetGodMode(st, on)
		return nil
	})
}

// DevMaxShip is a dev-server-only debug action (see internal/sim/dev.go).
func (s *Session) DevMaxShip(now int64) (Snapshot, error) {
	return s.intent(now, func(st *sim.State) error {
		sim.DevMaxShip(st, s.actor.content())
		return nil
	})
}
