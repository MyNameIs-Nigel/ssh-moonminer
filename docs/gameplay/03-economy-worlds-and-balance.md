# Gameplay 03 — Economy, Worlds & Balance Data

**Area:** Gameplay · **Phase:** 3 · **Depends on:** gameplay/01 (and /02 for
constants to extract) · **Blocks:** framework/04's validation targets ·
**Parallel-safe with:** tui/02, tui/03, tests/01

## Goal

Give credits somewhere to come from and go to: cargo sales, port services
(refuel, repair), route permits, system transfer costs, ship purchases, station
construction, local station bonuses, passive income, and the safety-net
respawn rule — and move every tunable number out of Go and into
`data/worlds.toml` + `data/balance.toml` so balance passes never touch code.

## References

| Source | What to take |
| --- | --- |
| `../moon-miner-materials/Web-Prototype/Moon Miner.dc.html` (`refuel`, `repair`, world table) | historical UI/pricing reference only; this doc owns final numbers |
| `../ssh-idlefarmer/data/crops.toml` + `balance.toml` | TOML shape/conventions |
| `../ssh-idlefarmer/internal/content/content.go` | validation approach |
| [../01-concept-and-story.md](../01-concept-and-story.md) | locked systems, cargo, station goals |
| [../02-danger-economy-and-progression.md](../02-danger-economy-and-progression.md) | station/passive/death economy narrative |

## Deliverables

- `internal/sim/` — `economy.go` (SellCargo, Refuel, Repair, system permits,
  station funding/claiming, safety-net respawn helpers, derived prices)
- `data/worlds.toml`, `data/balance.toml` (authoritative numbers)
- The `internal/content` struct fields these files map to (coordinate with
  framework/04, which owns the loader plumbing)

## Spec

### Cargo sales

Cargo in `state.Cargo` is unsold inventory, not credits. Selling is available
only while docked at a public dock or an owned station in the current system.

- `SellCargo(state, content, venue) (SaleRecord, error)`
  - `venue=public_dock`: pays `sum(BaseValue) * destination/system market mul`.
  - `venue=owned_station`: requires completed station in current system; pays
    public value * `station_sale_bonus_mul`.
  - clears sold cargo stacks and updates stats.
- Cargo from other systems can be sold anywhere, but station bonuses apply only
  when selling at a station the pilot owns in the current system.
- Death clears unsold cargo before it can be sold.

### Port services (Star Chart / station screen only)

- **Refuel** — `Refuel(state, content) error`: fills to tank size.
  Cost `round(missingFuel * refuel_per_point * localRefuelMultiplier)`.
  Partial refuel when credits can't cover a full tank: buy
  `floor(credits / effectiveRate)` points.
  - Public dock multiplier: `1.0`.
  - Owned completed station in current system: default `0.65` multiplier.
- **Repair** — `Repair(state, content) error`: hull to ship max hull. Cost
  `round(missingHull * repair_per_point)`.
- Both no-op with typed errors when already full or credits are zero.

### Route permits and locked systems

> **Superseded in part by** [08-jump-network-and-frontier-progression.md](08-jump-network-and-frontier-progression.md):
> the system-level `transfer_fee` / `BuySystemPermit` model below is retired.
> Inter-system access is a lifetime pilot **Jump Rating** priced in
> `[[ratings]]` and gated on a frontier qualification, and each crossing
> charges its own mass-scaled fuel plus hull transit stress. **Destination**
> permits and nav beacons (`permit_fee`, e.g. CERES) are unaffected and the
> rest of this section stands.

- `BuySystemPermit(state, content, systemID) error` spends the system's
  `transfer_fee`, marks the route unlocked, and survives ship death.
- Buying a permit requires the active ship's class to meet the system's minimum
  ship requirement. The player may need to buy a superior ship first.
- Repeat travel to an unlocked system does **not** charge the large transfer fee
  again, but still requires a ship that can jump there and enough fuel for the
  selected destination.
- Local destination permits/nav beacons use the same pattern with smaller costs.

### Stations and passive income

Stations are late-game local investments keyed by system id:

- `StartOrFundStation(state, content, systemID, credits) error`
  - requires the system permit and active presence in that system;
  - applies credits to staged construction;
  - marks the station complete when funded through all stages.
- Completed stations:
  - generate claimable passive credits at `income_per_hour`, capped by
    `offline_income_cap_hours`;
  - reduce refuel costs in that system only;
  - unlock station sale venue and sale bonus in that system only.
- `ClaimStationIncome(state, content, now) (int, error)` computes capped
  elapsed income from each completed station, adds credits, and updates
  `LastIncomeClaimAt`.
- Station ownership/build progress survives ship death.

### Death and safety-net respawn

At hull 0, gameplay/02 calls the death resolver:

- active ship resets to the free Salvage Skiff;
- ship-installed upgrades reset to zero;
- fuel resets to starter skiff fuel minimum;
- cargo clears;
- current run and destination clear; pilot respawns at Sol dock;
- credits, permits, stations, cosmetics, settings, and stats survive.

Softlock protection: after death or any docked state, if the player has the
starter ship, no cargo, fuel below cheapest departure, and credits below the
minimum starter refuel, they may claim a **SALVAGE ADVANCE**. It grants enough
credits/fuel for one Vesta Local run and increments `Stats.SalvageAdvances`.
Available again only after selling cargo or buying any non-free ship.

### `data/worlds.toml`

```toml
[[systems]]
id = "sol"
name = "SOL"
starts_unlocked = true
transfer_fee = 0
required_ship = "salvage_skiff"
pirate_mul = 1.0
station_build_cost = 0

[[systems]]
id = "eridani"
name = "ERIDANI DRIFT"
starts_unlocked = false
transfer_fee = 75000
required_ship = "cutter"
pirate_mul = 1.35
station_build_cost = 500000

[[destinations]]
id = "vesta_local"
system_id = "sol"
name = "VESTA LOCAL"
sub = "STARTER BELT"
starts_unlocked = true
required_ship = "salvage_skiff"
required_fuel_tank_level = 0
permit_fee = 0
travel_fuel = 6
pirate_mul = 0.8
rarity_bias = 0.04
value_mul = 0.85
base_asteroids = 6
desc = "Thin, legal, and picked over. Good enough to buy your next tank."
art = "sphere"        # "sphere" | "ringed" | "station"

# Add the remaining destinations from gameplay/01.
```

### `data/balance.toml`

Every constant used by gameplay/01–04, named and commented:

```toml
[pilot]
start_credits = 500
start_ship = "salvage_skiff"
start_fuel = 45
start_hull = 100

[belt]
units_min = 20
units_max = 260
units_step = 5
mine_sec_min = 45.0
mine_sec_max = 210.0
mine_sec_per_unit = 0.9
value_per_unit = 38
value_step = 25
risk_min = 8
risk_max = 96
risk_base = 14
risk_per_tier = 18
risk_noise = 18

[tiers]                       # 4 entries, ascending mult
labels = ["COMMON", "UNCOMMON", "RARE", "LEGENDARY"]
glyphs = ["◇", "◆", "✦", "★"]
mults  = [1.0, 2.3, 4.6, 9.5]

[mining]
tick_hz = 4
base_arrival_seconds = 130
arrival_risk_scale = 1.4
eta_base_uncertainty_pct = 0.45
tribute_chance = 0.45
tribute_demand_pct_min = 0.45
tribute_demand_pct_max = 0.75
tribute_decision_seconds = 12
base_escape_seconds = 8
escape_cargo_exponent = 1.7
base_pirate_damage_per_second = 4.5
remnant_keep_threshold = 0.20
mining_fuel_per_second = 0.08

[events]
base_event_chance_per_minute = 0.08
low_hull_event_bonus = 0.45
base_bad_weight = 0.35
low_hull_bad_weight = 0.55
event_cooldown_seconds = 18

[port]
refuel_per_point = 9
repair_per_point = 14
station_refuel_mul = 0.65
public_sale_mul = 1.0
station_sale_bonus_mul = 1.18
salvage_advance_credits = 180
salvage_advance_fuel = 35
salvage_credit_threshold = 75
salvage_fuel_threshold = 6

[stations]
offline_income_cap_hours = 24
income_per_hour_base = 250
build_stage_count = 4
```

(Exact key naming may be adjusted; the **set of tunables** may not shrink.)

### Balance invariants (encode as tests in tests/01)

1. A median common rock on Vesta Local, sold at a public dock after a successful
   bail/depart, nets enough to afford another starter run plus visible progress
   toward fuel tank I.
2. Expected value of a legendary in a locked Sol destination, accounting for
   tribute/attack/death probability at aggression 1.0, beats a common on Vesta
   Local by at least 2x.
3. A first fuel-tank upgrade is reachable from a small number of successful
   starter cargo sales; a first superior ship requires materially more risk.
4. No generated starter belt can have every asteroid outside the starter
   Scanner lock.
5. Station refuel discounts and sale bonuses are local: owning a station in
   Eridani changes Eridani prices and does not change Sol prices.
6. Death never removes credits, permits, cosmetics, or stations.

## Acceptance criteria

- [ ] SellCargo/refuel/repair/permit/station/salvage-advance actions mutate
  exactly per spec (table-driven tests, including partial refuel and station
  local-only modifiers).
- [ ] All constants referenced in `internal/sim` resolve from
  `content.Content` — grep proves no magic balance numbers remain in Go
  (formulas' structure stays in code; coefficients don't).
- [ ] `data/*.toml` round-trips through the content loader with validation
  passing.
- [ ] Invariant tests 1–6 pass against the shipped numbers.

## Out of scope / handoffs

- Loader plumbing/embed → framework/04.
- Ship prices, upgrade prices → **gameplay/05**
  (`05-fleet-ships-and-shipyard-economy.md`), which supersedes the
  `[ships]`/`[upgrades]` pointer this doc originally made to gameplay/04.
  Cosmetic unlock prices remain gameplay/04's, unaffected.
- Port UI (buttons, eligibility display) → tui/02.
