# Gameplay 04 — Progression, Upgrades & Ship's Log

**Area:** Gameplay · **Phase:** 4 · **Depends on:** gameplay/01–03 ·
**Parallel-safe with:** framework/04, tests/*, tui tasks

> **Superseded in part:** the "Ships" and "Installed upgrades" sections below
> (the five-ship purchase list and the five-track Drill/Tank/Cargo/Plating/
> Surveyor upgrade system) are replaced by
> [05-fleet-ships-and-shipyard-economy.md](05-fleet-ships-and-shipyard-economy.md) —
> a 4-ship hangar model with per-track stat grades and slot devices. This
> doc's Cosmetics, Lifetime stats, and Ship's log sections are still
> authoritative; only `RunRecord.ShipClass` should eventually reference the
> new model IDs (Skiff/Cicada/Warden/Mule) instead of the old five names —
> a small follow-up, not a blocker.

## Goal

Give the credits a long-term purpose and the pilot a history. Five systems,
all extensions beyond the HTML prototype: **ship purchases** (range/capacity
gates), **ship-installed upgrades** (powerful but lost on death), **cosmetic
configuration** (persistent account identity), **station-aware progression**,
and the **ship's log** (recent run/death history). Together they turn a
5-minute toy into a roguelite economy worth SSHing back into.

## References

| Source | What to take |
| --- | --- |
| [../01-concept-and-story.md](../01-concept-and-story.md) | ships as lives, cosmetics, station goals |
| [../02-danger-economy-and-progression.md](../02-danger-economy-and-progression.md) | progression and death narrative |
| `../ssh-idlefarmer/internal/sim/state.go` | how idlefarmer models upgrade levels + stats in state |
| gameplay/02's `RunRecord` return | the log entry source |

## Deliverables

- `internal/sim/` — `ships.go`, `upgrades.go`, `cosmetics.go`, `stats.go`
- `[ships]`, `[upgrades]`, `[cosmetics]` sections added to `data/balance.toml`
- Extends/finalizes `sim.State` stubs from gameplay/01 (`Ship`, `Cargo`,
  `Stations`, `Cosmetics`, `Stats`, `RunLog`)

## Spec

### Ships

Ships are purchasable active vessels, not permanent unlocks. Buying a ship
replaces the active ship; death replaces it with the free Salvage Skiff.

| Ship | Price target | Gates | Gameplay identity |
| --- | --- | --- | --- |
| **SALVAGE SKIFF** | free | Sol starter | tiny hold, fragile, fast escape |
| **PROSPECTOR** | early | more Sol destinations | better tank/hold; first real goal |
| **CUTTER** | mid | first inter-system permit | jump tier 1, stronger hull |
| **HAULER** | late | high-value cargo loops | huge hold, slow escape when full |
| **SURVEYOR FRIGATE** | endgame | far systems | best sensors and range; costly to lose |

- `BuyShip(state, content, shipID) error` — requires docked, affordable, and
  the ship not already active. It clears installed upgrades from the previous
  ship unless the previous ship is traded in by an explicit future mechanic
  (MVP: no trade-in value).
- `DestroyActiveShip(state, content)` — called by gameplay/02 death resolver:
  active ship becomes Salvage Skiff, upgrades reset, cargo clears, fuel/hull
  reset to starter respawn values, destination clears to Sol dock.
- Ship class determines base fuel tank, cargo capacity, max hull, jump tier,
  base escape time, and cargo escape penalty.

### Installed upgrades

Installed upgrades are attached to the active ship. They are powerful credit
sinks but are destroyed with the ship.

Five tracks, each 0–4 levels, purchasable only while docked. Prices rise
steeply; effects are multiplicative/additive on gameplay/01–02 formulas:

| Track | Flavor | Effect per level (cumulative) |
| --- | --- | --- |
| **DRILL HEAD** | tungsten → plasma | mine rate ×1.12 |
| **FUEL TANK** | drop tanks | tank size +25 or ship-specific coefficient |
| **CARGO RACKS** | braced containers | cargo capacity +20% |
| **HULL PLATING** | whipple shielding | incoming attack hull damage ×0.90 |
| **SURVEYOR** | deep scanner | pirate ETA uncertainty narrows; belt size +1 at high levels |

Price curve: `base · 2.6^level · shipClassMul` with bases (drill 450, tank
500, cargo 550, plating 650, surveyor 800) — all in `balance.toml [upgrades]`,
alongside per-level effect coefficients. Level 0 = stock ship, no effect.

- `BuyUpgrade(state, content, track) error` — requires docked, affordable,
  below max level; typed errors otherwise.
- Effects apply inside existing sim functions (gameplay/01's belt size and
  tank/cargo derivations, gameplay/02's rates/ETA/escape/damage) via small
  `effectiveX(state, content)` helpers in `upgrades.go` so the touch points
  stay greppable.
- Upgrade levels reset to zero on ship death and when buying a new ship.

### Cosmetics

Cosmetics are account-level and survive death. They never affect simulation
math.

```go
type CosmeticUnlocks struct {
    HUDThemes map[string]bool
    ShipPaints map[string]bool
    RadarStyles map[string]bool
    BorderStyles map[string]bool
}
```

- `BuyCosmetic(state, content, kind, id) error` — requires docked and
  affordable; purchased unlock is permanent.
- `SetCosmetic(state, kind, id) error` — requires unlock (or default item).
- MVP cosmetic fields: HUD accent theme, panel border style, radar sweep style,
  ship paint, and ship name/nose-art text. Ship rendering can start abstract;
  the data model should not assume a final renderer.

### Lifetime stats

```go
type Stats struct {
    RunsTotal, RunsDeparted, RunsBailed, RunsTributePaid int
    RunsEscapedUnderFire, ShipsLost int
    CreditsEarned, CreditsSpent, CargoValueSold, CargoValueLost int
    ShipsPurchased, UpgradesPurchased, CosmeticsPurchased int
    SystemsUnlocked, StationsCompleted int
    LegendariesMined int
    FuelBurned, CargoHauled float64
    RandomEventsSeen int
    SalvageAdvances int
    FirstSeen, LastSeen int64 // unix seconds (set by actor attach)
}
```

Updated inside outcome resolution / economy actions (single choke points —
resist scattering `stats.X++` around the codebase).

### Ship's log

`RunLog []RunRecord`, capped at the **20** most recent (drop oldest).
`RunRecord` (finalized here; gameplay/02 introduced it):

```go
type RunRecord struct {
    When      int64
    System    string // "SOL"
    Destination string // "VESTA LOCAL"
    Asteroid  string // "KR-4711"
    Tier      int
    Outcome   string // departed|bailed|tribute_paid|escaped_under_fire|ship_lost
    CargoValueRecovered int
    CargoValueLost int
    HullDelta int
    FuelDelta float64
    Depleted bool
    PirateAction string // none|tribute|attack
    Events []string
    ShipClass string
}
```

Death records render differently: the TUI first shows the dark red
`CONNECTION LOST` screen, then the recap/log entry explains the lost ship,
upgrades, and cargo.

### Consumers (informative)

tui/02 renders stats + log on a Ship's Log screen reachable from the chart
(`L` key), plus shipyard/cosmetic/station panels. The Run Summary screen shows
the newest record. No TUI work in this task — but keep the structs
render-friendly (exported, plain types).

## Acceptance criteria

- [ ] Buying each ship changes tank, cargo, hull, jump tier, and escape profile;
  death resets to Salvage Skiff and clears installed upgrades/cargo.
- [ ] Each upgrade track measurably changes its target formula in a
  table-driven test (e.g. surveyor level 2 narrows ETA uncertainty; cargo rack
  increases capacity; plating reduces attack damage).
- [ ] Price curve and max-level enforcement tested; buying while in a belt or
  active run fails.
- [ ] Cosmetic purchases persist through death and encode/decode; changing a
  cosmetic never changes derived sim values.
- [ ] Stats counters advance correctly across a scripted session touching
  departed, bailed, tribute, escaped-under-fire, ship-lost, a refuel, a ship
  purchase, an upgrade purchase, a cosmetic purchase, and a station completion.
- [ ] RunLog caps at 20 with FIFO eviction; survives encode/decode.
- [ ] Balance sanity: total cost to buy top ship, max its upgrades, unlock every
  system, and complete one late-game station is documented in `balance.toml`
  comments as the "endgame horizon."

## Out of scope / handoffs

- Shipyard/cosmetic/station UI + Ship's Log screen → tui/02 (extend, or a
  follow-up TUI task if it outgrows that doc).
- Global leaderboards / cross-pilot anything — deliberately post-MVP (would
  need store queries across fingerprints; note it in code TODOs only).
