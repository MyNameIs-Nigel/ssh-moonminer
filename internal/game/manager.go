package game

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/store"
)

// Policy controls concurrent session behavior.
type Policy string

const (
	PolicyTakeover Policy = "takeover"
	PolicyRefuse   Policy = "refuse"
)

var (
	ErrSaveBusy      = errors.New("this pilot is already open in another session")
	ErrShuttingDown  = errors.New("moon miner is closing for maintenance")
	ErrSessionClosed = errors.New("session is closed")
)

// Manager owns active save actors.
type Manager struct {
	store    *store.Store
	content  *content.Content
	logger   *slog.Logger
	autosave time.Duration
	policy   Policy

	mu      sync.Mutex
	actors  map[saveKey]*actor
	closing bool
	nextID  atomic.Uint64
}

// NewManager creates a save registry.
func NewManager(st *store.Store, c *content.Content, logger *slog.Logger, autosave time.Duration, policy Policy) *Manager {
	if autosave <= 0 {
		autosave = 30 * time.Second
	}
	if policy != PolicyRefuse {
		policy = PolicyTakeover
	}
	return &Manager{
		store: st, content: c, logger: logger,
		autosave: autosave, policy: policy,
		actors: make(map[saveKey]*actor),
	}
}

func (m *Manager) Content() *content.Content { return m.content }

// AttachResult is returned on successful attach.
type AttachResult struct {
	Session *Session
	Created bool
}

// Attach opens a save for the session.
func (m *Manager) Attach(ctx context.Context, id identity.SessionIdentity, publicKey string, now int64, kick func(reason string)) (AttachResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing {
		return AttachResult{}, ErrShuttingDown
	}
	if err := m.store.TouchAccount(ctx, id.Fingerprint, publicKey, now); err != nil {
		return AttachResult{}, err
	}
	key := saveKey{fingerprint: id.Fingerprint, slot: id.Slot}
	a, ok := m.actors[key]
	created := false
	if !ok {
		row, isNew, err := m.store.LoadOrCreateSave(ctx, id.Fingerprint, id.Slot, now, func() ([]byte, int, error) {
			state := sim.New(m.content, randomSeed(), now)
			payload, err := state.Encode()
			return payload, state.Version, err
		})
		if err != nil {
			return AttachResult{}, err
		}
		state, err := sim.DecodeState(row.State)
		if err != nil {
			return AttachResult{}, fmt.Errorf("load save: %w", err)
		}
		state.Stats.LastSeen = now
		created = isNew
		a = newActor(m, key, state)
		m.actors[key] = a
	}
	sess := &Session{
		id: m.nextID.Add(1), actor: a,
		kicked: make(chan string, 1), kickFn: kick,
		snapCh: make(chan sim.Snapshot, 8),
	}
	var refused bool
	ok = a.do(func() {
		if a.session != nil {
			if m.policy == PolicyRefuse {
				refused = true
				return
			}
			a.session.deliverKick("Your ship was boarded from another terminal.")
		}
		a.session = sess
		a.state.Stats.LastSeen = now
	})
	if !ok {
		delete(m.actors, key)
		return AttachResult{}, errors.New("save closing")
	}
	if refused {
		return AttachResult{}, ErrSaveBusy
	}
	return AttachResult{Session: sess, Created: created}, nil
}

func (m *Manager) detach(s *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a := s.actor
	a.do(func() {
		if a.session == s {
			if a.state.Run != nil {
				sim.EmergencyResolve(a.state, m.content, time.Now().Unix())
				a.dirty = true
			}
			a.session = nil
		}
		a.persist("disconnect")
	})
	if a.session == nil {
		m.stopActorLocked(a)
	}
}

func (m *Manager) stopActorLocked(a *actor) {
	select {
	case <-a.done:
	default:
		close(a.stop)
		<-a.done
	}
	if m.actors[a.key] == a {
		delete(m.actors, a.key)
	}
}

// Shutdown flushes all active saves.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.closing = true
	actors := make([]*actor, 0, len(m.actors))
	for _, a := range m.actors {
		actors = append(actors, a)
	}
	m.mu.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, a := range actors {
			a.do(func() {
				if a.session != nil {
					if a.state.Run != nil {
						sim.EmergencyResolve(a.state, m.content, time.Now().Unix())
						a.dirty = true
					}
					a.session.deliverKick("Maintenance shutdown — progress saved.")
					a.session = nil
				}
			})
			m.mu.Lock()
			m.stopActorLocked(a)
			m.mu.Unlock()
		}
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func randomSeed() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return uint64(time.Now().UnixNano())
	}
	return binary.LittleEndian.Uint64(b[:])
}
