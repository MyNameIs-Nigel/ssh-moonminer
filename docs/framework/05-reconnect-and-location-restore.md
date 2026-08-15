# Framework 05 — Reconnect and Location Restore

> **Status: complete (v1.6.2).** Rules 1–4 shipped; rule 5 (restoring the
> highlighted contact) was left out as explicitly optional. Two extra bugs
> surfaced while building the tests and were fixed in the same change — see
> § "Found while building this" at the end.

## Goal

A pilot who loses their connection at a belt must come back **at that belt**,
not at the star chart. Today the save already remembers where they were —
`sim.State.WorldIdx`, `SystemID` and `Belt` are all persisted, exactly as
[framework/02](02-persistence-and-save-model.md) specifies — but the TUI opens
every session on the Star Chart regardless. The result is a pilot who *is* out
in a belt while looking at a dock screen where every service refuses with
`must be docked to use port services`, and whose only working action costs a
second tank of travel fuel and rolls a brand-new belt. With a full hold it is
not even that: it is an unrecoverable save.

This task makes the reconnect screen a function of the persisted state, adds
the missing dock affordance, and closes the double-charge hole in `Depart`.
It changes no gameplay rule: an interrupted mining run still resolves through
the emergency bail/escape path of
[framework/03](03-session-lifecycle-and-actors.md) — **a disconnect is not a
pause and never becomes one.** What is restored is the pilot's *location*, not
their run.

## The bug, precisely

Verified against `main` @ `c24bd28` (v1.6.1), 2026-08-15.

1. `internal/tui/game.go`'s `NewGame` hardcodes `scr: scrChart` for every
   session — new pilot, returning pilot, mid-belt reconnect alike. Nothing
   downstream ever corrects it: `scr` only changes on a key press or when a
   snapshot arrives carrying `Run != nil`, and `Run` is never persisted.
2. `sim.State.Encode`/`DecodeState` clear only `Run` and `Scan`. `WorldIdx`,
   `SystemID` and `Belt` survive, so `IsDocked()` (`WorldIdx < 0`) is **false**
   for a pilot restored onto the chart screen.
3. Every port and shipyard action guards on `IsDocked()` and returns
   `ErrInBelt` — `SellCargo`, `Refuel`, `Repair`, `InsuranceAdvance`,
   `BuySystemPermit`/`BuyDestinationPermit` (`internal/sim/economy.go`),
   `AcquireShip`, `SwitchActiveShip`, `BuyShipTrack` (`internal/sim/ships.go`),
   and every slot install/store/sell/replace (`internal/sim/slots.go`). On the
   chart screen those are `F`, `R`, `C`, `I` and the whole `S` shipyard: all of
   them flash "must be docked to use port services" at a pilot who is staring
   at the dock.
4. `sim.Dock` — the only thing that sets `WorldIdx = -1` outside ship loss — is
   reachable from exactly one place, `keyBelt("q")` on the belt screen. The
   chart screen has no dock path, by key or by mouse.
5. So the only action that works is `Enter` → `sim.Depart`, which does **not**
   check `IsDocked()`. Departing to the world you are already at charges
   `TravelFuel` a second time and calls `GenerateBelt`, which increments
   `s.BeltCount` and therefore rolls a *different* belt. The scanned contacts
   the save carefully preserved are destroyed by the only move available.

**The unrecoverable case.** `Depart` also requires
`RemainingCargoCapacity(s, c) > 0`. A pilot who disconnects with a full hold —
the emergency escape banks the cargo, `WorldIdx` stays `>= 0` — reconnects to a
chart where `C` (sell) fails with `ErrInBelt`, `Enter` (depart) fails with
`ErrCargoFull`, and `I` (insurance) is ineligible because
`InsuranceEligible` itself requires `IsDocked()`. There is no key that
advances the game. Reconnecting does not help; the state is on disk. The
softlock safety net cannot fire, because the safety net is docked-gated too.

## References

- [framework/02-persistence-and-save-model.md](02-persistence-and-save-model.md)
  § "The save blob" — already specifies "the current belt (system/destination +
  remaining asteroids) **so a reconnecting pilot finds the belt they left**".
  The store half of that sentence shipped; the UI half never did.
- [framework/03-session-lifecycle-and-actors.md](03-session-lifecycle-and-actors.md)
  § "Disconnect emergency bail" — the rule this task must not weaken.
- [tui/01-app-shell-input-and-mouse.md](../tui/01-app-shell-input-and-mouse.md) —
  owns the root model and screen router being changed here.
- `internal/tui/game.go` (`NewGame`), `internal/tui/chart.go` (`keyChart`),
  `internal/tui/belt.go` (`keyBelt`), `internal/sim/belt.go` (`Depart`, `Dock`).

## Deliverables

- `internal/tui/game.go` — initial screen derived from the restored snapshot.
- `internal/tui/chart.go` — a guard path so the chart can never be a dead end.
- `internal/sim/belt.go` — `Depart` refuses when not docked.
- `internal/tui/summary.go` (or equivalent) — the "while you were away" notice.
- Tests: `internal/tui` for the routing, `internal/sim` for the `Depart`
  guard, and the first `internal/game` test (see tests/02) covering
  disconnect → reattach location continuity.

## Spec

### 1. Restore the screen from the state (the fix)

`NewGame` picks the opening screen from the snapshot instead of hardcoding it:

| Restored state | Opening screen |
| --- | --- |
| `WorldIdx < 0` (docked) | `scrChart` — unchanged |
| `WorldIdx >= 0` | `scrBelt`, `rockSel = 0` |

`WorldIdx >= 0` with an **empty** `Belt` is a normal, reachable state (the
pilot mined every contact out; `removeAsteroid` deletes depleted rocks). The
belt screen already handles `len(Belt) == 0` — its selection keys no-op and
`Q` still docks — so the rule needs no `len(Belt) > 0` qualifier, and must not
have one: an empty-belt pilot routed to the chart is the same softlock.

`Run` is never persisted, so no session can open on `scrMining`. Leave the
existing "`Run != nil` in a snapshot ⇒ `scrMining`" behaviour alone; it is how
a *live* run takes over the screen, not a restore path.

New pilots (`AttachResult.Created`) are always docked, so onboarding is
unaffected.

### 2. Make the chart screen impossible to strand on

Belt-and-suspenders, because the state that reaches the chart screen is not
fully controlled by this repo (a future migration, a hand-edited save, a
Litestream restore mid-write):

- When `keyChart` runs with `!IsDocked()`, `Q`-to-chart semantics invert: the
  screen offers **`Enter` RETURN TO BELT**, which sets `scr = scrBelt` without
  touching sim state at all. No fuel, no belt roll.
- The chart's port-service block renders disabled (dim, with a
  `◇ IN BELT — RETURN TO BELT TO DOCK` notch) rather than offering five keys
  that all error.

Cheaper alternative if the render work is unwanted: have `NewGame`'s router
call `sim.Dock` when it finds `WorldIdx >= 0` but cannot route (it never can,
after rule 1) — rejected here because docking silently rearms jammer/EMP/
missiles and restores shields, which is a free service the pilot did not
travel back to buy. **Restoring the pilot's position must never hand out the
dock rearm.**

### 3. `Depart` requires being docked

`sim.Depart` gains the same `!s.IsDocked() → ErrInBelt` guard every other
travel-adjacent action already has, checked before fuel is consumed and before
`GenerateBelt` is called, so a rejected departure mutates nothing (the
gameplay/01 acceptance rule).

This closes the double-charge: with the guard, the only route from one belt to
another is belt → `Q` (dock, which is where the rearm belongs) → chart →
`Enter`. That is already the flow every player follows; the guard makes it the
only one.

**Migration risk:** existing `internal/sim` tests that call `Depart` twice
without an intervening `Dock` will now fail. That is the guard working — insert
`sim.Dock(...)` in those fixtures rather than weakening the guard.

### 4. Tell the pilot what happened while they were gone

A pilot whose run was emergency-resolved currently learns nothing: the
`RunOutcome` lives on the `Session`, which is destroyed with the actor, and
the only surviving trace is a `RunRecord` in the ship's log that looks
identical to a run they flew themselves.

- `sim.RunRecord` gains `Disconnected bool` (JSON `disconnected,omitempty`;
  no `StateVersion` bump needed — old records decode as `false`). Set it in
  `sim.EmergencyResolve`'s resolution path only.
- On restore, if the newest `RunLog` entry has `Disconnected` set and its
  `When` is at or after `Stats.LastSeen` at detach, the belt screen opens with
  a one-shot notice: outcome label, cargo recovered or lost, hull delta —
  the Run Summary's content, framed as *"SIGNAL LOST — AUTOPILOT LOGGED"*.
  Dismissed by any key, never shown twice.

### 5. Optional: restore the selected contact

`rockSel = 0` is a correct restore, not a perfect one. If "drop you right where
you were" should include the target the pilot had highlighted, persist the
selected asteroid **ID** (not index — the belt list is re-sorted by distance
and shrinks as rocks deplete) as an optional `sim.State` field, and have the
router resolve ID → index on restore, falling back to `0` when the rock is
gone. Cosmetic; ship rules 1–3 first.

## Acceptance criteria

- [x] Attach a save with `WorldIdx >= 0` and a populated belt: the session
  opens on the belt screen showing that belt's contacts, and the persisted
  `Belt` is byte-identical to what was saved (no regeneration, `BeltCount`
  unchanged). — `TestInitialScreenFollowsSavedLocation`,
  `TestReattachRestoresBeltLocation`
- [x] Attach a save with `WorldIdx >= 0` and an **empty** belt: opens on the
  belt screen, `Q` docks normally. — `TestInitialScreenFollowsSavedLocation`
- [x] Attach a docked save: opens on the star chart, unchanged. A newly
  created pilot opens on the chart with onboarding. — same test; onboarding
  wins the overlay slot, so the reconnect notice never covers it.
- [x] End-to-end through `internal/game`: attach → `Depart` → `Detach` →
  re-`Attach` returns a state with the same `WorldIdx`, `SystemID`, `BeltCount`
  and `Belt`. — `TestReattachRestoresBeltLocation`, the package's first test.
- [x] Mid-run disconnect still resolves the emergency bail/escape: cargo kept
  only on escape, ship death still possible, `Run == nil` on reattach, and the
  pilot reattaches **at the belt** with that outcome visible once. —
  `TestDetachMidRunResolvesAndLeavesNotice`,
  `TestEmergencyResolveShipLostRespawnsDocked`
- [x] `Depart` while `!IsDocked()` returns `ErrInBelt` and mutates nothing —
  fuel, `BeltCount` and `Belt` all unchanged. — `TestDepartRequiresDocked`
- [x] The full-hold case is recoverable: reconnect with a full hold at a belt,
  `Q` to dock, `C` to sell. No key sequence from a restored session leaves the
  pilot with no legal action. —
  `TestDepartInBeltWithFullHoldIsRefusedNotCargoFull`
- [x] `go build ./... && go vet ./... && go test ./...` green (175 tests, up
  from 157); CI `-race` clean.

## Found while building this

Two live bugs surfaced that this doc did not predict. Both are in
`internal/game`, which had **zero tests** before this task — that is why they
survived.

1. **The actor's tick ticker was starved by its own clients.** `actor.run()`
   called `mineTick.Reset(tickDur)` at the top of every loop iteration, and
   `Reset` restarts a ticker's period from zero. Any client sending requests
   faster than the 250 ms tick therefore prevented the tick from **ever**
   firing: a player mashing mining pressure points above 4 Hz froze their own
   drill, the pirate approach and the escape timer, and an in-flight scan never
   completed. The ticker is now started and stopped only on an edge. Pinned by
   `TestFrequentRequestsDoNotStarveTheTick`, which is also how it was found —
   the reconnect test's scan never finished.
2. **`Session.lastOutcome` was raced.** The actor goroutine writes it in
   `tick()`; the TUI goroutine reads it via `LastOutcome()` on its own 1 Hz
   timer, which has no happens-before edge to that write. (The `snapMsg` path
   is fine — the channel provides the edge.) It now has its own mutex. This
   was latent rather than observed: with no tests in `internal/game`, CI's
   `-race` job had never exercised the package.

## Out of scope / handoffs

- **Resuming an interrupted mining run.** Explicitly rejected: framework/03's
  rule that a disconnect must not be a free pause is a balance decision, and
  suspending a run would make dropping the connection the correct play against
  a losing pirate fight. The run resolves; the location restores.
- Persisting `ActiveScan` — same reasoning, and a scan is seconds long.
- Any change to `sim.Dock`'s rearm/shield-restore behaviour → gameplay/05.
- The Run Summary screen's own layout → tui/02; this task only reuses it.
