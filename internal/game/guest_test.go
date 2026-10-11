package game

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestGuestActiveRunIsDiscardedWithoutStoreOrEmergencyResolution(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		t.Run(map[bool]string{false: "detach", true: "shutdown"}[shutdown], func(t *testing.T) {
			m := testManager(t)
			m.store = nil
			id := identity.SessionIdentity{Kind: identity.Guest, GuestID: strings.Repeat("a", 32)}
			res, err := m.Attach(t.Context(), id, "", 1000, nil)
			if err != nil {
				t.Fatal(err)
			}
			s := res.Session
			if _, err = s.Depart(1000, 1); err != nil {
				t.Fatal(err)
			}
			rock := scannableRock(t, m, s, 1)
			s.actor.do(func() {
				for i := range s.actor.state.Belt {
					if s.actor.state.Belt[i].ID == rock.ID {
						s.actor.state.Belt[i].Scanned = true
					}
				}
			})
			if _, err = s.Lock(1000, rock.ID); err != nil {
				t.Fatal(err)
			}
			before, _ := s.SnapshotNow()
			deadline := time.After(3 * time.Second)
			for {
				select {
				case snap := <-s.Snapshots():
					if snap.Revision > before.Revision {
						goto ticked
					}
				case <-deadline:
					t.Fatal("guest mining ticker stopped")
				}
			}
		ticked:
			s.actor.do(func() {
				s.actor.dirty = true
				if err := s.actor.persist("test"); err != nil {
					t.Error(err)
				}
			})
			if shutdown {
				if err = m.Shutdown(t.Context()); err != nil {
					t.Fatal(err)
				}
			} else {
				s.Detach()
			}
			<-s.actor.done
			if s.actor.state.Run == nil || s.actor.state.DisconnectNotice != nil {
				t.Fatal("guest ran emergency save resolution")
			}
			if len(m.actors) != 0 {
				t.Fatal("dirty guest retained")
			}
			if !shutdown {
				fresh, err := m.Attach(t.Context(), id, "", 5000, nil)
				if err != nil {
					t.Fatal(err)
				}
				snap, _ := fresh.Session.SnapshotNow()
				if !snap.State.IsDocked() || len(snap.State.RunLog) > 0 {
					t.Fatal("guest restored prior pilot")
				}
				fresh.Session.Detach()
			}
		})
	}
}
func TestGuestsAreIsolatedAndNeverTakenOver(t *testing.T) {
	m := testManager(t)
	m.store = nil
	id := identity.SessionIdentity{Kind: identity.Guest, GuestID: strings.Repeat("a", 32)}
	a, err := m.Attach(t.Context(), id, "", 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Session.Detach()
	if _, err = m.Attach(t.Context(), id, "", 1001, nil); !errors.Is(err, ErrSaveBusy) {
		t.Fatal("guest takeover", err)
	}
	id.GuestID = strings.Repeat("b", 32)
	b, err := m.Attach(t.Context(), id, "", 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Session.Detach()
	a.Session.UpdateSettings(1001, func(s *sim.Settings) { s.ASCIISafe = true })
	bs, _ := b.Session.SnapshotNow()
	if bs.State.Settings.ASCIISafe {
		t.Fatal("guests merged")
	}
	for _, invalid := range []identity.SessionIdentity{{}, {Kind: identity.Guest, GuestID: "bad"}, {Kind: identity.Guest, GuestID: id.GuestID, Slot: "fake"}} {
		if _, err = m.Attach(t.Context(), invalid, "", 1000, nil); err == nil {
			t.Fatal("invalid identity accepted")
		}
	}
}
