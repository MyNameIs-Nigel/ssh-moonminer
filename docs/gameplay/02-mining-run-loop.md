# Gameplay 02 — The Mining Run Loop

**Area:** Gameplay · **Phase:** 2 · **Depends on:** gameplay/01 ·
**Blocks:** tui/03, framework/03's emergency escape · **Parallel-safe with:**
framework/02, tui/01, tui/04

## Goal

Implement the heart of the game in `internal/sim`: locking a target, the long
manual mining operation, cargo transfer, rough pirate ETA/radar, random events,
pirate tribute/attack behavior, flee timing, hull loss, and ship death. Everything
is pure functions over `sim.State` — the actor drives ticks with timestamps and
RNG inputs, the TUI renders snapshots.

## References

| Source | What to take |
| --- | --- |
| `../moon-miner-materials/Web-Prototype/Moon Miner.dc.html` lines ~449–487 (`lock`, `startMine`, `endRun`, `bail`) | historical interaction reference only; this doc supersedes the rates/outcomes |
| `../ssh-idlefarmer/internal/sim/advance.go` + `actions.go` | pure tick/action function style |
| [../01-concept-and-story.md](../01-concept-and-story.md) | manual mining loop and death presentation |
| [../02-danger-economy-and-progression.md](../02-danger-economy-and-progression.md) | pirate behavior, random events, ship loss |

## Deliverables

- `internal/sim/` — `run.go` (ActiveRun, Lock, TickRun, BailOrDepart,
  AcceptTribute, RefuseTribute, outcome/death resolution), `events.go` (random
  event rolls/effects), typed errors, `RunOutcome` type

## Spec

### ActiveRun

```go
type ActiveRun struct {
    AsteroidID int
    Phase      RunPhase // mining | tribute | escaping | resolved

    MinedUnits     float64 // units moved from asteroid into cargo
    RemainingUnits float64 // asteroid units still available
    CargoValue     int     // base value currently added from this asteroid

    PirateDistance float64 // 0..100; 100 = far, 0 = arrived
    PirateETA      ETAEstimate
    PirateAction   PirateAction // none | tribute | attack

    EscapeSecondsRequired float64
    EscapeSecondsElapsed  float64
    UnderAttack           bool

    ActiveEvent *RunEvent
    EventLog    []RunEventRecord
    StartedAt   int64 // unix millis (for stats/log)
}
```

`RunPhase`:

| Phase | Meaning |
| --- | --- |
| `mining` | Drill is transferring asteroid units into cargo. |
| `tribute` | Pirates arrived and are waiting for the player to accept/drop cargo or refuse. |
| `escaping` | Ship is trying to leave; may be under attack. |
| `resolved` | Internal transient used while producing the RunRecord. |

`ETAEstimate` stores a deliberately rough range (`MinSeconds`, `MaxSeconds`,
`Confidence`) plus the true arrival time/distance used by the sim. The TUI must
render only the rough range and radar distance. Surveyor upgrades narrow the
range; damaged sensors and events widen or freeze it.

### Locking a target — `Lock(state, content, asteroidID) error`

- Requires: currently in a destination belt (`DestinationID != ""`), no active
  run, asteroid exists in `state.Belt`, `Fuel ≥ asteroid.FuelCost`, `Hull > 0`,
  cargo hold not full.
- Deducts flight fuel immediately, initializes `ActiveRun` with:
  - `RemainingUnits = asteroid.Units`;
  - `PirateDistance = 100`;
  - a true pirate arrival time derived from asteroid risk/system risk;
  - a rough ETA range whose width depends on surveyor/sensor upgrades.
- Failure returns `ErrInsufficientFuel` etc. without mutation.

### The tick — `TickRun(state, content, dtSeconds float64) (RunOutcome, bool)`

Rates (constants into `balance.toml` via gameplay/03):

```
mineRate = asteroid.Units / asteroid.MineSec
mineRate *= shipDrillMultiplier(state)

pirateArrivalSeconds =
    baseArrivalSeconds
    / riskMultiplier(asteroid, destination, system, settings)

escapeSecondsRequired =
    baseEscapeSeconds(ship)
    + cargoLoadRatio(state)^escapeCargoExponent * cargoEscapePenalty(ship)

attackHullDamagePerSecond =
    basePirateDamagePerSecond
    * pirateDamageMultiplier(system, destination, asteroid)
    * eventDamageMultiplier(state.Run.ActiveEvent)
```

Per tick with `dt` seconds (the actor calls this at **4 Hz — dt = 0.25**; long
operations do not need 8 Hz precision):

#### Mining phase

- Move `mineRate * dt` units from asteroid to cargo, clamped by remaining
  asteroid units and remaining cargo capacity.
- Update `CargoValue = round(asteroid.Value * MinedUnits / asteroid.Units)`.
- Drain normal mining fuel.
- Advance pirate radar distance toward 0 based on the true arrival timer.
- Roll at most one random event when the event cooldown allows it.
- Do **not** auto-resolve when the asteroid is depleted. Depletion only changes
  the available player action from flashing yellow `BAIL` to green `DEPART`.
- If cargo is full, mining pauses but pirates continue approaching.
- If pirates reach distance 0, roll `PirateAction`.

#### Tribute phase

`PirateAction == tribute` pauses mining and presents a demand:

- `AcceptTribute` jettisons `round(CargoValue * tributeDemandPct)` worth of
  cargo from this asteroid, logs `tribute_paid`, and starts escaping without
  immediate attack.
- `RefuseTribute` logs `tribute_refused`, sets `UnderAttack=true`, and starts
  escaping.
- A TUI timeout may call `RefuseTribute` after `tributeDecisionSeconds` to keep
  the game moving, but this timeout must be actor/sim driven, not wall-clock in
  the view.

#### Escape phase

- `BailOrDepart` starts escape. If `RemainingUnits > 0`, outcome intent is
  `bailed`; if `RemainingUnits == 0`, intent is `departed`.
- Escape duration is derived from ship class and cargo load. Full ships should
  feel dangerous: a full Hauler may take several times longer to escape than an
  empty Skiff.
- If `UnderAttack`, subtract hull every tick. Hull damage can trigger bad
  random events more often.
- When `EscapeSecondsElapsed >= EscapeSecondsRequired`, resolve as escaped.
- If `Hull <= 0`, resolve as `ship_lost`.
- Fuel reaching zero during escape does not call an emergency tow. Instead it
  increases escape time and event chance; if the ship cannot flee before hull
  reaches zero, it dies.

### Random events

Events roll only during mining and escape. Probability increases as hull falls:

```
eventChancePerMinute = baseEventChance
    + (1 - HullPct)^2 * lowHullEventBonus
    + activeSystem.eventMul

badEventWeight = baseBadWeight + (1 - HullPct) * lowHullBadWeight
```

Event effects are mechanical and renderable:

| Event | Sim effect |
| --- | --- |
| `power_outage` | mining pauses for N seconds; escape continues but input limited to bail/depart |
| `radar_blackout` | ETA/radar estimate freezes and widens after recovery |
| `life_support_failure` | starts a countdown; if still mining when it expires, hull bleeds until escape starts |
| `cargo_shift` | increases current escape requirement by a percentage |
| `reactor_surge` | increases fuel burn for N seconds; small immediate hull hit chance |

The event record includes a `HUDTreatment` enum so tui/03 can give every event
its unique visual effect without re-deriving event meaning.

### Outcomes — `resolveRun(state, kind)`

| Kind | Credits now? | Side effects | Log color role |
| --- | --- | --- | --- |
| `departed` | none; cargo remains unsold | asteroid removed; cargo kept | green |
| `bailed` | none; cargo remains unsold | asteroid remains or is depleted by mined units (see below); cargo kept | amber |
| `tribute_paid` | none | demanded cargo removed; escape logged | amber |
| `escaped_under_fire` | none | hull damage already applied; cargo kept | red |
| `ship_lost` | none | active ship reset to Salvage Skiff; ship upgrades/cargo/run asteroid lost; respawn at Sol dock | dark red |

Cargo is not converted to credits here. Selling is gameplay/03. The `RunRecord`
must still include `CargoValueRecovered`, `CargoValueLost`, `HullDelta`,
`FuelDelta`, event list, and whether the asteroid was depleted.

Asteroid removal:

- `departed` after depletion removes the asteroid from `state.Belt`.
- `bailed` before depletion keeps the asteroid only if
  `remainingUnits / originalUnits >= remnantKeepThreshold`; otherwise remove it
  as "too scattered to reacquire." This avoids clutter from tiny leftovers.
- `ship_lost` removes the asteroid claim and regenerates no cargo.

Outcome labels/descriptions:

- departed — `DEPARTED` / "Asteroid depleted. Cargo sealed. No clean run is safe until the dock buys it."
- bailed — `BAILED` / "Cut the drill and ran with a partial hold."
- tribute_paid — `TRIBUTE PAID` / "Cargo jettisoned. Pirates took the easy money and let the ship run."
- escaped_under_fire — `ESCAPED UNDER FIRE` / "Hull torn open, engines screaming, cargo still aboard."
- ship_lost — `CONNECTION LOST` / "The ship and its hold are gone."

### Player controls/actions

- `BailOrDepart(state)` — starts escape. It is yellow `BAIL` while resources
  remain and green `DEPART` after depletion. Safe to call with `state.Run ==
  nil` as a no-op because disconnect races a just-finished run.
- `AcceptTribute(state)` — valid only in tribute phase.
- `RefuseTribute(state)` — valid only in tribute phase; starts attack/escape.
- There is no overdrive in this spec. Mining tension comes from time, radar,
  cargo load, events, and escape risk.

### Tick driving (decision)

The **actor** owns a real-time ticker that runs only while `state.Run != nil`
(4 Hz), calling `TickRun` and pushing snapshots to the attached TUI session.
The sim itself never sleeps and never owns a goroutine. Actor-driven ticking is
required: a player must not pause pirates, tribute timers, oxygen failures, or
escape damage by suspending terminal output.

## Acceptance criteria

- [ ] Deterministic: scripted tick sequences produce identical outcomes
  across runs/platforms (no wall clock anywhere).
- [ ] Table-driven outcome tests: departed, bailed, tribute paid,
  escaped-under-fire, and ship-lost; assert cargo deltas, hull deltas, belt
  changes, run-log append, `Run == nil` after.
- [ ] Mining never auto-resolves on depletion; `BailOrDepart` is required.
- [ ] Full cargo materially increases escape time versus empty cargo for every
  ship class; Hauler full-load escape is the slowest baseline case.
- [ ] Pirate arrival rolls either tribute or immediate attack with deterministic
  seeded RNG.
- [ ] Accepting tribute removes the configured cargo percentage and avoids
  immediate attack; refusing starts attack.
- [ ] Hull 0 triggers ship loss: active ship resets, ship upgrades and cargo are
  cleared, banked credits/permits/stations/cosmetics survive.
- [ ] Low-hull event probability is measurably higher than high-hull event
  probability in a deterministic distribution test.
- [ ] `BailOrDepart` with no active run is a no-op.
- [ ] Numeric floors: cargo, hull, fuel, timers, and values never become
  negative or NaN at dt edge cases (dt=0, huge dt).

## Out of scope / handoffs

- The 4 Hz ticker goroutine itself → framework/03 (actor).
- Balance constant extraction to TOML → gameplay/03.
- Rendering mining site, radar, event HUD effects, death screen → tui/03.
- Stats/RunRecord struct details → gameplay/04 (call its append helper).
