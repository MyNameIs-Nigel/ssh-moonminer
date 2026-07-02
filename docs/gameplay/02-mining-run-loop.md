# Gameplay 02 — The Mining Run Loop

**Area:** Gameplay · **Phase:** 2 · **Depends on:** gameplay/01 ·
**Blocks:** tui/03, framework/03's auto-bail · **Parallel-safe with:**
framework/02, tui/01, tui/04

## Goal

Implement the heart of the game in `internal/sim`: locking a target, the
real-time drill tick (drill ↑, fuel ↓, pirate proximity ↑), the overdrive
risk dial, and the four run outcomes. Everything is pure functions over
`sim.State` — the actor drives ticks with timestamps, the TUI renders
snapshots.

## References

| Source | What to take |
| --- | --- |
| `../moon-miner-materials/Web-Prototype/Moon Miner.dc.html` lines ~449–487 (`lock`, `startMine`, `endRun`, `bail`) | exact rates and outcome math, transcribed below |
| `../ssh-idlefarmer/internal/sim/advance.go` + `actions.go` | pure tick/action function style |
| [../01-concept-and-story.md](../01-concept-and-story.md) | outcome table, disconnect auto-bail rule |

## Deliverables

- `internal/sim/` — `run.go` (ActiveRun, Lock, TickRun, SetOverdrive, Bail,
  outcome resolution), typed errors, `RunOutcome` type

## Spec

### ActiveRun

```go
type ActiveRun struct {
    AsteroidID int
    Drill      float64 // 0..100 (%)
    Pirate     float64 // 0..100 (%)
    Yield      int     // credits banked-so-far = Value·Drill/100 (derived each tick, stored for cheap reads)
    Overdrive  bool
    StartedAt  int64   // unix millis (for stats/log)
}
```

### Locking a target — `Lock(state, content, asteroidID) error`

- Requires: docked-in-belt (`WorldIdx ≥ 0`), no active run, asteroid exists
  in `state.Belt`, `Fuel ≥ asteroid.FuelCost`, `Hull > 0`.
- Deducts flight fuel immediately, sets `state.Run` with zeroed gauges.
- Failure returns `ErrInsufficientFuel` etc. without mutation.

### The tick — `TickRun(state, content, dtSeconds float64) (RunOutcome, bool)`

Rates (from the prototype; constants into `balance.toml` via gameplay/03):

```
drillRate  = 100 / asteroid.DrillSec                      // %/sec
pirateRate = (asteroid.Risk/100) · (7 + tier·3) · aggression   // %/sec
fuelDrain  = 1.3 + tier·0.35                              // %/sec

overdrive: drill ×2.1, fuelDrain ×1.9  (pirate rate unaffected)
aggression = state.Settings.PirateAggression (default 1.0, a Tweak)
```

Per tick with `dt` seconds (the actor calls this at **8 Hz — dt = 0.125**,
close to the prototype's 120 ms interval; make the Hz a constant):

```
drill  = min(100, drill + drillRate·dt·(over ? 2.1 : 1))
fuel   = max(0,   fuel  − fuelDrain·dt·(over ? 1.9 : 1))
pirate = min(100, pirate + pirateRate·dt)
yield  = round(value · drill/100)
```

Then resolve, first match wins (order matters and is prototype-faithful:
raided before stranded before clean):

| Check | Outcome |
| --- | --- |
| `pirate ≥ 100` | **Raided** |
| `fuel ≤ 0` | **Stranded** |
| `drill ≥ 100` | **Clean** |
| else | run continues |

### Outcomes — `resolveRun(state, kind)`

| Kind | Banked | Side effects | Log color role |
| --- | --- | --- | --- |
| `clean` | full `Value` | — | gold |
| `bail` | `Yield` | — | cyan |
| `raided` | `round(Yield · 0.55)` | `Hull −25` (floor 0) | red |
| `stranded` | `round(Yield · 0.60)` | fuel stays 0 (tow gets you back; the 40% cut is the fee) | amber |

All outcomes: add banked credits, remove the asteroid from `state.Belt`,
append a `RunRecord` (gameplay/04), update `Stats`, clear `state.Run`. The
resolved `RunRecord` (kind, label, description, asteroid name, banked,
fuel/hull after) is returned so the TUI can render the summary without
re-deriving.

Outcome labels/descriptions (prototype copy, keep):

- clean — `CLEAN EXTRACTION` / "Vein drilled to the core. Full cargo secured and clear of contacts."
- bail — `CARGO SECURED` / "Disengaged early and ran. You keep every credit already mined."
- raided — `RAIDED` / "Pirates boarded mid-drill. Lost cargo and took hull damage breaking away."
- stranded — `STRANDED` / "Tanks dry. Emergency tow sold your hold at a loss to cover the fee."

### Player controls

- `SetOverdrive(state, on bool)` — the TUI maps key-down/key-up of Space.
  Terminals don't deliver key-up: tui/03 implements "hold" as
  press-to-toggle-on + auto-off after a short no-repeat window; the sim just
  exposes the boolean.
- `Bail(state) RunRecord` — resolves as `bail` immediately. Also called by
  the actor on disconnect/kick/shutdown (framework/03's auto-bail) — it must
  be safe to call with `state.Run == nil` (no-op) because disconnect races
  a just-finished run.

### Tick driving (decision)

The **actor** owns a real-time ticker that runs only while `state.Run != nil`
(8 Hz), calling `TickRun` and pushing snapshots to the attached TUI session
(the same channel used for kicks can carry snapshot notifications, or the
TUI polls each Bubble Tea tick — pick whichever matches framework/03's
session API; document the choice in code). The sim itself never sleeps and
never owns a goroutine. TUI-driven ticking is acceptable if snapshot
latency stays under ~150 ms; actor-driven is safer against a stalled client
freezing their own pirates — **actor-driven is the default choice** because
a player must not pause the game by suspending output.

## Acceptance criteria

- [ ] Deterministic: scripted tick sequences produce identical outcomes
  across runs/platforms (no wall clock anywhere).
- [ ] Table-driven outcome tests: craft asteroids/states that end in each of
  the four outcomes; assert banked credits, hull, belt shrinkage, run-log
  append, `Run == nil` after.
- [ ] Overdrive math: with overdrive held from t=0, a rock with
  `DrillSec=10` finishes in ~4.76s of simulated time and burns ~1.9× fuel
  vs the same run without.
- [ ] Priority test: a tick where pirate and drill both cross 100 resolves
  **raided**.
- [ ] `Bail` with no active run is a no-op.
- [ ] Yield floors: banking never produces negative or NaN credits at
  dt edge cases (dt=0, huge dt).

## Out of scope / handoffs

- The 8 Hz ticker goroutine itself → framework/03 (actor).
- Balance constant extraction to TOML → gameplay/03.
- Rendering gauges → tui/03.
- Stats/RunRecord struct details → gameplay/04 (call its append helper).
