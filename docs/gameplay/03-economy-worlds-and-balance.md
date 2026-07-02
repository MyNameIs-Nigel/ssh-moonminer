# Gameplay 03 — Economy, Worlds & Balance Data

**Area:** Gameplay · **Phase:** 3 · **Depends on:** gameplay/01 (and /02 for
constants to extract) · **Blocks:** framework/04's validation targets ·
**Parallel-safe with:** tui/02, tui/03, tests/01

## Goal

Give credits somewhere to come from and go to: port services (refuel,
repair), travel costs, the safety-net insurance rule — and move every
tunable number out of Go and into `data/worlds.toml` + `data/balance.toml`
so balance passes never touch code.

## References

| Source | What to take |
| --- | --- |
| `../moon-miner-materials/Web-Prototype/Moon Miner.dc.html` (`refuel`, `repair`, world table) | price constants |
| `../ssh-idlefarmer/data/crops.toml` + `balance.toml` | TOML shape/conventions |
| `../ssh-idlefarmer/internal/content/content.go` | validation approach |
| [../01-concept-and-story.md](../01-concept-and-story.md) | hull rules + insurance divergence |

## Deliverables

- `internal/sim/` — `economy.go` (Refuel, Repair, insurance check, derived
  prices)
- `data/worlds.toml`, `data/balance.toml` (authoritative numbers)
- The `internal/content` struct fields these files map to (coordinate with
  framework/04, which owns the loader plumbing)

## Spec

### Port services (Star Chart screen only)

- **Refuel** — `Refuel(state, content) error`: fills to tank size.
  Cost `round(missing% · 9)` credits (9 cr per fuel point, from prototype).
  Partial refuel when credits can't cover a full tank: buy
  `floor(credits/9)` points (divergence: the prototype refused instead —
  partial is kinder and creates "one more run to afford the tank" moments).
- **Repair** — `Repair(state, content) error`: hull to 100. Cost
  `round(missing · 14)` credits. **Dry-dock surcharge:** if hull is 0 the
  rate is `14 · 1.5 = 21`/point (see hull rules).
- Both no-op with typed errors when already full or credits are zero.

### Hull rules

- Hull damages only via raids (−25) for MVP.
- At `Hull == 0`: `Depart` and `Lock` fail with `ErrHullBreached`; the pilot
  is stuck at the chart until repaired (at surcharge rates).

### Insurance bailout (softlock protection)

A pilot with `credits < cheapest possible departure` (min world travel fuel
· 9 buys enough fuel… compute properly:) — formally, if
`Credits < 50 && Fuel < 6 && WorldIdx == -1` (cannot afford fuel to reach
even Vesta) — may claim a one-time-per-dry-spell **INSURANCE ADVANCE**:
credits set to 300, `Stats.InsuranceClaims++`. Available again only after
banking a clean run. Numbers in `balance.toml`. The TUI (tui/02) surfaces
this as a port-services row that appears only when eligible.

### `data/worlds.toml`

```toml
[[worlds]]
name = "CERES"
sub = "INNER BELT"
ring = "DENSE"
pirate_label = "HIGH"
pirate_mul = 1.55
rarity_bias = 0.18
travel_fuel = 8
rarity_label = "RICH"
desc = "Crowded, lucrative, lawless. Raiders everywhere."
art = "sphere"        # "sphere" | "ringed" — selects ASCII art in tui
# … VESTA, TITAN, IO per the table in gameplay/01
```

### `data/balance.toml`

Every constant used by gameplay/01–04, named and commented:

```toml
[pilot]
start_credits = 1000
start_fuel = 100
start_hull = 100

[belt]
asteroids_per_belt = 7
volume_min = 240
volume_max = 4600
volume_step = 20
drill_sec_min = 2.2
drill_sec_max = 16.0
drill_sec_per_volume = 300   # drill_sec = vol / this
value_per_volume = 1.15
value_step = 25
fuel_cost_min = 5
fuel_cost_max = 36
fuel_cost_tier_bonus = 3
risk_min = 8
risk_max = 96
risk_base = 18
risk_per_tier = 16

[tiers]                       # 4 entries, ascending mult
labels = ["COMMON", "UNCOMMON", "RARE", "LEGENDARY"]
glyphs = ["◇", "◆", "✦", "★"]
mults  = [1.0, 2.3, 4.6, 9.5]

[mining]
tick_hz = 8
pirate_rate_base = 7.0
pirate_rate_per_tier = 3.0
fuel_drain_base = 1.3
fuel_drain_per_tier = 0.35
overdrive_drill_mul = 2.1
overdrive_fuel_mul = 1.9
raided_yield_keep = 0.55
raided_hull_damage = 25
stranded_yield_keep = 0.60

[port]
refuel_per_point = 9
repair_per_point = 14
drydock_surcharge_mul = 1.5
insurance_credits = 300
insurance_fuel_threshold = 6
insurance_credit_threshold = 50
```

(Exact key naming may be adjusted; the **set of tunables** may not shrink.)

### Balance invariants (encode as tests in tests/01)

1. A median common rock on Vesta profits ≥ 3× its total fuel cost in
   credits — the baseline loop must never be a treadmill.
2. Expected value of a legendary on Ceres, accounting for raid probability
   at aggression 1.0, still beats a common on Io — risk must pay on average.
3. A full tank (900 cr) is recoverable from ~2 median Vesta runs.
4. No belt can generate with **every** rock's fuel cost above a full tank.

## Acceptance criteria

- [ ] Refuel/repair/insurance mutate exactly per spec (table-driven tests,
  including partial refuel and the 0-hull surcharge).
- [ ] All constants referenced in `internal/sim` resolve from
  `content.Content` — grep proves no magic balance numbers remain in Go
  (formulas' structure stays in code; coefficients don't).
- [ ] `data/*.toml` round-trips through the content loader with validation
  passing.
- [ ] Invariant tests 1–4 pass against the shipped numbers.

## Out of scope / handoffs

- Loader plumbing/embed → framework/04.
- Upgrade prices → gameplay/04 (adds its own `[upgrades]` tables here).
- Port UI (buttons, eligibility display) → tui/02.
