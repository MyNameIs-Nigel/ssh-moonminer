# Framework 03 — Session Lifecycle & Actors

**Area:** Framework · **Phase:** 3 · **Depends on:** framework/01,
framework/02, gameplay/01 (for `sim.State`) · **Blocks:** full end-to-end play

## Goal

Exactly one goroutine owns each active save. The `game.Manager` attaches SSH
sessions to per-save **actors**, enforces the concurrent-login policy
(takeover by default), runs autosave, and guarantees the SIGTERM flush. This
is the concurrency backbone: the TUI and the sim never share memory across
sessions because the actor serializes all access.

## References (mirror these)

| File | What to take |
| --- | --- |
| `../ssh-idlefarmer/internal/game/manager.go` | `Manager`, `Attach`/`detach`, policy, `Shutdown` flush, `randomSeed` |
| `../ssh-idlefarmer/internal/game/actor.go` | actor loop, `do(fn)` closure execution, autosave ticker, dirty flag, `persist` |
| `../ssh-idlefarmer/internal/game/session.go` | `Session` handle the TUI holds, kick delivery |
| `../ssh-idlefarmer/internal/game/game_test.go` | attach/detach/takeover/shutdown test patterns |

## Deliverables

- `internal/game/` — `manager.go`, `actor.go`, `session.go`, tests

## Spec

### Manager

- In-memory registry `map[saveKey]*actor`, `saveKey{fingerprint, slot}`.
  Process-local by design — restart clears stale locks.
- `Attach(ctx, id, publicKey, now, kick)` — `id` is the `identity.SessionIdentity`
  (fingerprint + slot) `identity.Resolver.Resolve` produced (framework/01),
  already the same shape whether the connection was direct or proxied through
  the arcade router; the manager never re-derives identity itself:
  1. refuse if shutting down (`ErrShuttingDown`),
  2. `store.TouchAccount`,
  3. load-or-create the save via `store.LoadOrCreateSave`, decoding with
     `sim.DecodeState`; new saves are `sim.New(content, randomSeed(), now)`,
  4. spin up (or reuse) the actor, then apply the session policy:
     - **takeover** (default): kick the old session with a friendly message,
       attach the new one.
     - **refuse**: return `ErrSaveBusy`; framework/01's middleware turns that
       into a polite `\r\n` message.
- `AttachResult{Session, Created}` — Moon Miner has **no offline catch-up**
  (nothing progresses while docked), so idlefarmer's `Away sim.Events` field
  is dropped. `Created` still drives onboarding.
- `Detach` persists ("disconnect" reason) and stops the actor when no session
  remains.

### Actor

- One goroutine per active save, owning `*sim.State`. All reads/writes go
  through `a.do(func(){ ... })` closures executed on the actor goroutine —
  copy idlefarmer's mechanism.
- **Autosave ticker**: every `cfg.AutosaveInterval`, persist if dirty
  (encode state → `store.SaveState`), clear dirty.
- **Dirty tracking**: any action or tick that mutates state marks dirty.
- **Mining ticks run here.** The TUI sends a tick request (or the actor runs
  its own ticker while a run is active — choose idlefarmer's pattern of
  UI-driven ticks unless it fights the real-time requirement; see
  gameplay/02 "Tick driving" for the decision) and receives a state
  snapshot back. The TUI only ever holds **snapshots** (value copies), never
  the live state pointer.

### Disconnect auto-bail (Moon Miner-specific)

If a session detaches (or is kicked/taken over/shut down) while a mining run
is active, the actor **bails the run first** — banking accrued yield exactly
as if the player pressed B — then persists. A pilot must never reconnect
into a phantom run or lose accrued cargo. This rule lives in the actor's
detach path and calls a `sim.Bail(...)` action (gameplay/02).

### Kick delivery

`Session` carries a buffered kick channel + optional callback so the TUI can
show "Your ship was boarded from another terminal…" before the connection
closes. Mirror idlefarmer's `deliverKick`.

### Shutdown flush

`Manager.Shutdown(ctx)`:
1. set `closing` (new attaches refused while the listener drains),
2. for every actor: kick its session with a maintenance message, auto-bail
   any active run, persist, stop the actor,
3. respect `ctx` cancellation with a wrapped error.

Registered as a shutdown hook from `main.go` (framework/01). Compose file
sets `stop_grace_period: 45s` (framework/04).

## Acceptance criteria

- [ ] Two sequential sessions on one save see continuous state.
- [ ] Concurrent attach under `takeover`: first session receives kick
  message, second gets the save, no data race (`go test -race`).
- [ ] Concurrent attach under `refuse`: second gets `ErrSaveBusy`.
- [ ] Detach mid-mining-run banks the bail yield (assert credits increased
  by accrued amount) and persists.
- [ ] `Shutdown` with N active saves persists all N and returns before a
  generous ctx deadline; attaches during shutdown get `ErrShuttingDown`.
- [ ] Autosave writes only when dirty (spy store or updated_at checks).
- [ ] All of the above as Go tests with a real temp SQLite store; `-race`
  clean.

## Out of scope / handoffs

- What `sim.Bail` computes → gameplay/02.
- SSH-side messaging text → framework/01 owns the middleware copy.
- The TUI's snapshot rendering → tui/01.
