# Gameplay 01 — Simulation Engine & Belt Generation

**Area:** Gameplay · **Phase:** 1 · **Blocks:** gameplay/02–04, tui/02,
tui/03, framework/03 · **Parallel-safe with:** framework/01, tui/01, tui/04

## Goal

Create `internal/sim`: the headless, deterministic engine that owns all game
state and rules. Everything else (TUI, actors, tests) is a consumer. This
task lands the state model, encoding, RNG discipline, world/tier data types,
and procedural belt generation. The real-time mining tick is gameplay/02.

## References

| Source | What to take |
| --- | --- |
| `../ssh-idlefarmer/internal/sim/state.go` | state struct + versioned JSON encode/decode pattern |
| `../ssh-idlefarmer/internal/sim/derive.go` | derived-values pattern (compute, don't store) |
| `../moon-miner-materials/Web-Prototype/Moon Miner.dc.html` lines ~326–424 | tiers, worlds, `genBelt` formulas (transcribed below — the HTML is the tie-breaker) |
| [../01-concept-and-story.md](../01-concept-and-story.md) | divergence list (travel fuel, starting state, hull rules) |

## Deliverables

- `internal/sim/` — `state.go`, `belt.go`, `rng.go` (+ any small helpers)
- Type definitions consumed by `internal/content` (world/tier structs may
  live in `internal/content` with sim importing them — follow idlefarmer's
  split where `content` owns loaded data and `sim` owns runtime state)

## Spec

### Purity rules (non-negotiable)

- No I/O, no logging, no `time.Now()`, no global `math/rand`. Timestamps and
  randomness come in as arguments or live in the state.
- The RNG is a seeded `*rand.Rand` (or PCG state) **stored in the save**
  (seed + stream position or just re-seeded per belt from a persisted
  counter). Same seed + same action sequence ⇒ identical state forever.
  This is what makes tests/01 possible.

### State

```go
type State struct {
    Version   int      // bump on breaking schema change
    Seed      uint64   // RNG root
    BeltCount uint64   // belts generated so far (RNG stream discriminator)

    Credits int
    Fuel    float64  // 0..TankSize (base 100)
    Hull    int      // 0..100

    WorldIdx  int         // current world, -1 = docked at chart
    Belt      []Asteroid  // current belt contacts (persisted)
    Upgrades  Upgrades    // gameplay/04
    Settings  Settings    // tweaks: belt view, aggression, contrast (tui/04)
    Stats     Stats       // lifetime counters (gameplay/04)
    RunLog    []RunRecord // last N run summaries (gameplay/04)

    Run *ActiveRun // non-nil only while drilling; never persisted non-nil
}
```

`Encode() ([]byte, error)` / `DecodeState([]byte) (*State, error)` — JSON,
version-checked, mirroring idlefarmer. `sim.New(content, seed, now)` builds
a fresh pilot: **1,000 credits, 100 fuel, 100 hull**, docked, empty belt.

### Tiers and worlds (data, loaded from TOML by framework/04)

Tiers (fixed four):

| Tier | Glyph | Mult | Color role |
| --- | --- | --- | --- |
| COMMON | ◇ | 1.0 | white |
| UNCOMMON | ◆ | 2.3 | cyan |
| RARE | ✦ | 4.6 | violet |
| LEGENDARY | ★ | 9.5 | gold |

Worlds:

| Field | CERES | VESTA | TITAN | IO |
| --- | --- | --- | --- | --- |
| sub | INNER BELT | MID BELT | SATURN RINGS | JOVIAN ORBIT |
| ring label | DENSE | MODERATE | RINGED | SPARSE |
| pirate label | HIGH | MED | MED | LOW |
| pirateMul | 1.55 | 1.05 | 0.95 | 0.70 |
| rarityBias | 0.18 | 0.10 | 0.30 | 0.05 |
| travelFuel | 8 | 6 | 20 | 14 |
| rarity label | RICH | BALANCED | RARE-HEAVY | THIN |
| desc | "Crowded, lucrative, lawless. Raiders everywhere." | "Close to port. Steady, honest pickings." | "Rare ices in the rings. A long, cold haul." | "Quiet and safe. Slim takings for the patient." |

### Asteroid

```go
type Asteroid struct {
    ID       int
    Name     string  // e.g. "KR-4711"
    Tier     int     // index into tiers
    Volume   int     // 240..4600, multiples of 20
    DrillSec float64 // 2.2..16.0, one decimal
    Value    int     // credits at 100% drill
    FuelCost int     // flight cost to reach it, 5..36
    Risk     int     // 8..96, pirate pressure input
    Dots     int     // 1..5 threat pips (derived: ceil(Risk/20))
    Size     string  // "sm" | "md" | "lg" (render hint)
    X, Y     int     // 10..84, 12..80 (percent coords for radar/blob views)
}
```

### Belt generation (`GenerateBelt(state, content, worldIdx)`) — transcribed from the prototype

For `n = 7` asteroids (make `n` a balance.toml constant):

1. **Tier roll**: weights
   `[0.46 − bias·0.5, 0.30, 0.16 + bias·0.6, 0.08 + bias·0.4]`
   (bias = world rarityBias); walk cumulative with one uniform roll.
2. **Volume**: `round(uniform(240, 4600) / 20) · 20`.
3. **Drill time (sec)**: `round1(clamp(vol/300, 2.2, 16))`.
4. **Value**: `round(vol · 1.15 · tierMult / 25) · 25`.
5. **Fuel cost**: `round(clamp(5 + uniform(0,14) + tier·3, 5, 36))`.
6. **Risk**: `round(clamp(pirateMul · (18 + tier·16) + uniform(0,16), 8, 96))`.
7. **Dots**: `clamp(ceil(risk/20), 1, 5)`.
8. **Size**: `lg` if vol > 3000, `md` if vol > 1300, else `sm`.
9. **Name**: prefix from `{AX,KR,VL,ND,TH,ZE,QU,RX,OB,MX}` + `-` +
   integer 1000–9999.
10. **Position**: `x = round(uniform(10,84))`, `y = round(uniform(12,80))`.

Sort ascending by fuel cost. Increment `BeltCount` (RNG stream advances so a
rescan differs).

### Chart-level actions (this task)

- `Depart(state, content, worldIdx) error` — requires
  `Fuel ≥ travelFuel` and `Hull > 0`; deducts travel fuel, sets `WorldIdx`,
  generates the belt. (Divergence from prototype: travel fuel is charged.)
- `Rescan(state, content)` — regenerates the current belt (free; V2 could
  price it).
- `Dock(state)` — clears `WorldIdx` to −1, keeps the belt list discarded.
  Docking with an active run is illegal (callers must bail first).
- Errors are typed (`ErrInsufficientFuel`, `ErrHullBreached`, …) so the TUI
  maps them to flash messages.

### Derived values

Follow idlefarmer's `derive.go` pattern: refuel/repair prices, whether the
pilot can afford/depart, tank size after upgrades — computed from state +
content, never stored.

## Acceptance criteria

- [ ] `go test ./internal/sim` passes; package imports nothing but stdlib +
  `internal/content`.
- [ ] Encode → Decode round-trips every field (reflect-based or golden
  test).
- [ ] Same seed ⇒ `GenerateBelt` produces byte-identical belts across runs
  and platforms; different `BeltCount` ⇒ different belts.
- [ ] 10k-asteroid statistical test: all values within spec ranges; Titan
  produces measurably more rare+legendary than Io (see tests/01 for exact
  bounds).
- [ ] `Depart` on 5 fuel to Ceres (needs 8) fails with
  `ErrInsufficientFuel` and mutates nothing.

## Out of scope / handoffs

- Mining tick, overdrive, outcomes → gameplay/02.
- Refuel/repair/insurance economics → gameplay/03.
- Upgrades/Stats/RunLog field internals → gameplay/04 (leave typed stubs).
- TOML schema/loader → gameplay/03 (numbers) + framework/04 (plumbing).
