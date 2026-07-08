# Gameplay 01 — Simulation Engine & Belt Generation

**Area:** Gameplay · **Phase:** 1 · **Blocks:** gameplay/02–04, tui/02,
tui/03, framework/03 · **Parallel-safe with:** framework/01, tui/01, tui/04

## Goal

Create `internal/sim`: the headless, deterministic engine that owns all game
state and rules. Everything else (TUI, actors, tests) is a consumer. This task
lands the state model, encoding, RNG discipline, system/destination/tier/ship
data types, cargo manifest primitives, and procedural belt generation. The
real-time mining, pirate, escape, and death tick is gameplay/02.

## References

| Source | What to take |
| --- | --- |
| `../ssh-idlefarmer/internal/sim/state.go` | state struct + versioned JSON encode/decode pattern |
| `../ssh-idlefarmer/internal/sim/derive.go` | derived-values pattern (compute, don't store) |
| `../moon-miner-materials/Web-Prototype/Moon Miner.dc.html` lines ~326–424 | historical palette/layout reference only; formulas here supersede it |
| [../01-concept-and-story.md](../01-concept-and-story.md) | manual mining, ship death, locked systems |
| [../02-danger-economy-and-progression.md](../02-danger-economy-and-progression.md) | progression narrative and open design questions |

## Deliverables

- `internal/sim/` — `state.go`, `belt.go`, `rng.go` (+ any small helpers)
- Type definitions consumed by `internal/content` (system/destination/tier structs may
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
    Version   int    // bump on breaking schema change
    Seed      uint64 // RNG root
    BeltCount uint64 // belts generated so far (RNG stream discriminator)

    Credits int
    Fuel    float64 // 0..TankSize
    Hull    int     // 0..100; 0 destroys the active ship

    SystemID      string      // current solar system, e.g. "sol"
    DestinationID string      // current planet/belt, empty = docked at chart/station
    Belt          []Asteroid  // current belt contacts (persisted)
    Cargo         CargoHold   // unsold cargo; sold by gameplay/03
    Ship          ShipState   // active ship class + installed upgrades; lost on death
    Permits       PermitState // unlocked system routes/destinations; survives death
    Stations      StationSet  // owned/building stations; survives death
    Cosmetics     Cosmetics   // account-level cosmetic choices/unlocks
    Settings      Settings    // tweaks: belt view, aggression, contrast (tui/04)
    Stats         Stats       // lifetime counters (gameplay/04)
    RunLog        []RunRecord // last N run summaries (gameplay/04)

    Run *ActiveRun // non-nil only while mining/escaping; never persisted non-nil
}
```

`Encode() ([]byte, error)` / `DecodeState([]byte) (*State, error)` — JSON,
version-checked, mirroring idlefarmer. `sim.New(content, seed, now)` builds
a fresh pilot: **500 credits, starter Salvage Skiff, starter fuel, 100 hull,
empty cargo, Sol access only**, docked, empty belt.

### Ship, cargo, permits, stations, cosmetics stubs

Gameplay/04 finalizes these types, but gameplay/01 owns their placement in the
state so persistence and TUI work can start:

```go
type ShipState struct {
    ClassID  string   // "salvage_skiff", "prospector", ...
    Upgrades Upgrades // installed on this ship; reset on death
}

type CargoHold struct {
    Used  float64
    Items []CargoStack
}

type CargoStack struct {
    SystemID, DestinationID string
    Tier                    int
    Units                   float64
    BaseValue               int // value before dock/station sale multipliers
}

type PermitState struct {
    Systems      map[string]bool // purchased one-time route permits
    Destinations map[string]bool // optional local permits/nav beacons
}

type StationSet map[string]StationState // keyed by system id

type Cosmetics struct {
    HUDThemeID string
    ShipPaintID string
    ShipName string
}
```

Derived helpers compute tank size, cargo capacity, jump rating, destination
eligibility, and station sale/refuel modifiers from `State + content`.

### Tiers, systems, destinations, and ships (data, loaded from TOML by framework/04)

Tiers (fixed four):

| Tier | Glyph | Mult | Color role |
| --- | --- | --- | --- |
| COMMON | ◇ | 1.0 | white |
| UNCOMMON | ◆ | 2.3 | cyan |
| RARE | ✦ | 4.6 | violet |
| LEGENDARY | ★ | 9.5 | gold |

Systems:

| Field | SOL | ERIDANI DRIFT | KEPLER REACH | REDLINE EXPANSE |
| --- | --- | --- | --- | --- |
| id | `sol` | `eridani` | `kepler` | `redline` |
| starts_unlocked | true | false | false | false |
| transfer_fee | 0 | 75,000 | 350,000 | 1,200,000 |
| required_ship_class | salvage_skiff | cutter | hauler | surveyor_frigate |
| pirate_mul | 1.0 | 1.35 | 1.7 | 2.2 |
| station_build_cost | 0 | 500,000 | 1,750,000 | 5,000,000 |

Initial destinations:

| Destination | System | Starts unlocked | Lock requirement | Travel fuel | Risk role |
| --- | --- | --- | --- | --- | --- |
| **VESTA LOCAL** | SOL | yes | none | 6 | low tutorial value |
| **CERES CLAIMS** | SOL | no | fuel tank I | 18 | high pirates, good ore |
| **IO SHADOW** | SOL | no | local permit 25,000 + fuel tank II | 34 | quieter, longer haul |
| **TITAN ICE RINGS** | SOL | no | Prospector ship + fuel tank III | 55 | rare ice, long escape |
| **ERIDANI-3** | ERIDANI | no | system permit + Cutter | 70 | first inter-system spike |
| **KEPLER BELT** | KEPLER | no | system permit + Hauler | 95 | station-focused late game |
| **REDLINE GRAVEYARD** | REDLINE | no | system permit + Surveyor Frigate | 130 | endgame lethal value |

Ship classes:

| Ship | Free on respawn | Jump tier | Base tank | Base hold | Base hull | Flee profile |
| --- | --- | --- | --- | --- | --- | --- |
| Salvage Skiff | yes | 0 | 60 | 40 | 100 | fast when empty, fragile |
| Prospector | no | 0 | 95 | 85 | 115 | balanced |
| Cutter | no | 1 | 130 | 145 | 135 | medium |
| Hauler | no | 2 | 175 | 260 | 150 | very slow when full |
| Surveyor Frigate | no | 3 | 220 | 210 | 170 | strong sensors, expensive |

### Asteroid

```go
type Asteroid struct {
    ID       int
    Name     string  // e.g. "KR-4711"
    Tier     int     // index into tiers
    Units    int     // mineable cargo units, 20..260, multiples of 5
    MineSec  float64 // 45..210, one decimal; full depletion time before upgrades/events
    Value    int     // base credits if all units are sold at public dock
    FuelCost int     // in-belt approach fuel to reach it, 3..24 by default
    Risk     int     // 8..96, pirate ETA/event pressure input
    Dots     int     // 1..5 threat pips (derived: ceil(Risk/20))
    Size     string  // "sm" | "md" | "lg" (render hint)
    X, Y     int     // 10..84, 12..80 (percent coords for radar/blob views)
}
```

### Belt generation (`GenerateBelt(state, content, destinationID)`)

For `n = destination.base_asteroids + surveyor_bonus` asteroids (default 6–9
depending destination; make values balance constants):

1. **Tier roll**: weights
   `[0.52 - bias*0.45, 0.30, 0.13 + bias*0.35, 0.05 + bias*0.10]`
   (bias = destination rarityBias); normalize and walk cumulative with one
   uniform roll.
2. **Units**: `round(uniform(units_min, units_max) / 5) * 5`, with tier and
   destination multipliers allowed to push late-game rocks larger.
3. **Mine time (sec)**: `round1(clamp(units * mine_sec_per_unit * tier_time_mul,
   mine_sec_min, mine_sec_max))`. Default target: common starter rocks take
   around 45–75 seconds; rare late-game rocks can take several minutes.
4. **Value**: `round(units * value_per_unit * tierMult * destination.value_mul / value_step) * value_step`.
5. **Fuel cost**: `round(clamp(destination.travel_fuel + uniform(-4,10) + tier*5,
   fuel_cost_min, fuel_cost_max))`.
6. **Risk**: `round(clamp(system.pirate_mul * destination.pirate_mul *
   (risk_base + tier*risk_per_tier) + uniform(0, risk_noise), risk_min, risk_max))`.
7. **Dots**: `clamp(ceil(risk/20), 1, 5)`.
8. **Size**: `lg` if units > 170, `md` if units > 70, else `sm`.
9. **Name**: prefix from `{AX,KR,VL,ND,TH,ZE,QU,RX,OB,MX}` + `-` +
   integer 1000–9999.
10. **Position**: `x = round(uniform(10,84))`, `y = round(uniform(12,80))`.

Sort ascending by fuel cost. Increment `BeltCount` (RNG stream advances so a
rescan differs).

### Chart-level actions (this task)

- `Depart(state, content, systemID, destinationID) error` — requires
  destination unlocked, active ship jump rating sufficient, `Fuel ≥ travelFuel`,
  `Hull > 0`, and no unsatisfied station-only restriction; deducts travel fuel,
  sets `SystemID`/`DestinationID`, generates the belt.
- `BuySystemPermit(state, content, systemID) error` — one-time large credit
  sink; requires active ship class can reach the system. The permit survives
  death. Repeat transfers still pay ordinary fuel via `Depart`, not the large
  permit fee.
- `Rescan(state, content)` — regenerates the current belt (free; V2 could
  price it).
- `Dock(state)` — clears `DestinationID`, keeps the current `SystemID`, discards
  the belt list, and leaves cargo untouched for selling.
  Docking with an active run is illegal (callers must bail first).
- Errors are typed (`ErrInsufficientFuel`, `ErrHullBreached`, …) so the TUI
  maps them to flash messages.

### Derived values

Follow idlefarmer's `derive.go` pattern: refuel/repair prices, whether the
pilot can afford/depart, tank size, hold capacity, cargo load ratio, jump
rating, station modifiers, permit lock messages, and death-loss preview are
computed from state + content, never stored.

## Acceptance criteria

- [ ] `go test ./internal/sim` passes; package imports nothing but stdlib +
  `internal/content`.
- [ ] Encode → Decode round-trips every field (reflect-based or golden
  test).
- [ ] Same seed ⇒ `GenerateBelt` produces byte-identical belts across runs
  and platforms; different `BeltCount` ⇒ different belts.
- [ ] 10k-asteroid statistical test: all values within spec ranges; late-game
  systems produce measurably more rare+legendary than Vesta Local (see tests/01
  for exact bounds).
- [ ] `Depart` on 5 fuel to Vesta Local (needs 6) fails with
  `ErrInsufficientFuel` and mutates nothing.
- [ ] Locked destination/system attempts fail with a typed error that includes
  a renderable lock reason.
- [ ] Encode/decode preserves cargo, ship class, permits, station states, and
  cosmetics.

## Out of scope / handoffs

- Mining tick, manual bail/depart, pirate actions, random events, death outcomes
  → gameplay/02.
- Refuel/repair/selling/station/passive economics → gameplay/03.
- Ship purchase/death reset/upgrades/cosmetics/Stats/RunLog field internals →
  gameplay/04 (leave typed stubs).
- TOML schema/loader → gameplay/03 (numbers) + framework/04 (plumbing).
