package game

import (
	"context"
	"time"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

type saveKey struct {
	fingerprint string
	slot        string
}

type actor struct {
	key  saveKey
	mgr  *Manager
	reqs chan func()
	stop chan struct{}
	done chan struct{}

	state   *sim.State
	dirty   bool
	session *Session
}

func newActor(m *Manager, key saveKey, state *sim.State) *actor {
	a := &actor{
		key: key, mgr: m,
		reqs: make(chan func()), stop: make(chan struct{}),
		done: make(chan struct{}), state: state,
	}
	go a.run()
	return a
}

func (a *actor) do(fn func()) bool {
	wrapped := make(chan struct{})
	select {
	case a.reqs <- func() { fn(); close(wrapped) }:
		<-wrapped
		return true
	case <-a.done:
		return false
	}
}

func (a *actor) run() {
	defer close(a.done)
	autosave := time.NewTicker(a.mgr.autosave)
	defer autosave.Stop()

	tickDur := sim.TickInterval(a.mgr.content)
	mineTick := time.NewTicker(tickDur)
	mineTick.Stop()
	defer mineTick.Stop()

	for {
		if a.state.WorldIdx >= 0 || a.state.Run != nil || a.state.Scan != nil {
			mineTick.Reset(tickDur)
		} else {
			mineTick.Stop()
		}
		select {
		case fn := <-a.reqs:
			fn()
		case <-autosave.C:
			a.persist("autosave")
		case <-mineTick.C:
			a.tick()
		case <-a.stop:
			if a.state.Run != nil {
				sim.EmergencyResolve(a.state, a.mgr.content, time.Now().Unix())
				a.dirty = true
			}
			a.persist("final flush")
			return
		}
	}
}

func (a *actor) tick() {
	if a.state.WorldIdx < 0 && a.state.Run == nil && a.state.Scan == nil {
		return
	}
	now := time.Now().Unix()
	dt := 1.0 / float64(a.mgr.content.Mining.TickHz)
	var out *sim.RunOutcome
	var ended bool
	if a.state.Run != nil {
		out, ended = sim.TickRun(a.state, a.mgr.content, dt, now)
	}
	if a.state.Scan != nil {
		sim.TickScan(a.state, dt)
	}
	if a.state.Run == nil {
		sim.TickBelt(a.state, a.mgr.content, dt)
	}
	a.dirty = true
	snap := sim.Snapshot{State: *a.state.Clone()}
	if a.session != nil {
		select {
		case a.session.snapCh <- snap:
		default:
		}
		if ended && out != nil {
			a.session.lastOutcome = out
		}
	}
}

func (a *actor) persist(reason string) {
	if !a.dirty {
		return
	}
	payload, err := a.state.Encode()
	if err != nil {
		a.mgr.logger.Error("encode failed", "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	now := time.Now().Unix()
	if err := a.mgr.store.SaveState(ctx, a.key.fingerprint, a.key.slot, payload, a.state.Version, now); err != nil {
		a.mgr.logger.Error("persist failed", "reason", reason, "error", err)
		return
	}
	a.dirty = false
}

func (a *actor) content() *content.Content { return a.mgr.content }
