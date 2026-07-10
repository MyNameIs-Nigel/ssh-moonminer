# Gameplay 05 — Fleet Ships, Slots & Shipyard Economy

**Area:** Gameplay · **Phase:** 5 · **Depends on:** gameplay/01–04 (retires and
replaces most of gameplay/04's `[ships]`/`[upgrades]` design and the shipped
single-track `internal/sim/upgrades.go`) · **Parallel-safe with:**
tui/05-shipyard-screen.md (owns the screen this doc's actions are driven from)

## Goal

Replace the single always-the-same-ship model (one flat `Upgrades` struct, no
purchasable ships) with a small owned **fleet**: 4 starter ships across 3
brands and 3 classes, each with its own persistent stat grades and a physical
loadout of swappable slot devices gated by a per-ship power budget. Losing your
active ship no longer wipes your account back to a single default — it
destroys *that ship* (and whatever was bolted to it), and you either fly
another ship already sitting in your hangar or **buy back** the lost hull at a
steep discount. This is the credit sink and the decision space that makes the
mid-game interesting: which ship do I fly today, and what do I bolt onto it
given a fixed power and mass budget?

This doc is sim-only (`internal/sim`, `internal/content`, `data/*.toml`). The
standalone cyber-futuristic Shipyard screen that drives these actions is
[../tui/05-shipyard-screen.md](../tui/05-shipyard-screen.md).

## References

| Source | What to take |
| --- | --- |
| [../01-concept-and-story.md](../01-concept-and-story.md) | "ships are lives" narrative — this doc updates the ship table there |
| [../02-danger-economy-and-progression.md](../02-danger-economy-and-progression.md) | death/respawn narrative — this doc updates the buyback rule there |
| [../gameplay/04-progression-and-ship-log.md](../gameplay/04-progression-and-ship-log.md) | superseded ship/upgrade sections — see Migration below |
| `internal/sim/upgrades.go`, `internal/sim/state.go` (`Upgrades` struct) | the single-track system this doc retires |
| `internal/sim/belt.go` (`GenerateBelt`, `rollTier`) | where the distance→rarity nudge and Scanner distance lock plug in |
| `internal/sim/events.go` (`tickEvents`) | where the hull-gated event-chance change plugs in |
| `data/balance.toml` `[upgrades]` | the section this doc's `[ships]`/`[slots]`/`[power]` tables replace |

## Migration: what's retired

The shipped game has one implicit ship with five upgrade tracks (Drill, Tank,
Plating, Damper, Surveyor — `internal/sim/upgrades.go`). This doc retires all
five in favor of the fleet/slot model below. Nothing is silently dropped:

| Old track | Disposition |
| --- | --- |
| **Tank** | Folded into the new **Fuel Efficiency** ship upgrade (drain-rate multiplier) plus the **Extra Fuel Tank** utility slot device (capacity). |
| **Plating** | Folded into the new **Hull** ship upgrade (max-hull pool) plus the **Defense Turret** weapon device and **Shield** utility device (damage mitigation). |
| **Surveyor** | Folded into the new **Scanner** ship upgrade (distance lock + ETA narrowing) plus the **Seismic Sensors** internal module (free pre-scans). Its old "belt size +1 per level" effect is retired — belt asteroid count is world-controlled only from this doc forward. |
| **Drill (mine rate)** | Retired with no direct replacement. The requested set of ship upgrades (Thrusters/Hull/Fuel Efficiency/Power Generator/Scanner) has no mining-speed track; mining speed is now influenced only by asteroid choice and the existing skill-check bonus. A future internal-slot module could reintroduce it — out of scope here. |
| **Damper** (pirate-ETA/signature) | Retired as a standalone track. ETA narrowing lives on Scanner (A/S grades); there is no direct replacement for its old effect on `PirateApproachBase`. |

`data/balance.toml`'s `[upgrades]` section and `internal/sim/upgrades.go`'s
five `UpgradeTrack` constants are deleted by this work, not deprecated
alongside the new system — there is exactly one upgrade model going forward.

## Deliverables

- `internal/sim/ships.go` — ship model catalog, `ShipInstance`, hangar
  (buy/sell/switch/buyback), track-grade purchases, derived-stat helpers
- `internal/sim/slots.go` — slot device catalog, install/uninstall, power and
  mass accounting
- Rewrites `internal/sim/upgrades.go` (or deletes it, folding what remains into
  `ships.go`) and updates `internal/sim/state.go` (`State.Ships`,
  `State.ActiveShipID`, drops `State.Upgrades`)
- Updates `internal/sim/belt.go` (distance→rarity nudge, ordering fix so
  distance is rolled before tier) and `internal/sim/events.go` (hull-gated
  event chance)
- `[ships]`, `[slots]`, `[power]` tables in `data/balance.toml`; `internal/content`
  struct fields + validation for all three

## Spec

### Fleet ownership model (the hangar)

The pilot owns a **hangar**: a set of ship *models* they've bought, each
tracked as one persistent `ShipInstance` (its own track grades, its own
installed slot devices, its own Pirate Jammer arm-state). The hangar holds **at
most one instance per model** — buying a model you already own is rejected
(no fleet of duplicate Wardens). One instance is the **active ship**; only the
active ship can depart into a belt. Switching the active ship is a docked-only
action.

```go
type ShipInstance struct {
    ModelID string              `json:"model_id"`
    Grades  TrackGrades         `json:"grades"`
    Utility []SlotDevice        `json:"utility,omitempty"`
    Weapon  []SlotDevice        `json:"weapon,omitempty"`
    Internal *SlotDevice        `json:"internal,omitempty"`
    JammerArmed bool            `json:"jammer_armed"`
}

type TrackGrades struct {
    Thrusters, Hull, FuelEff, PowerGen, Scanner int // 0..5 (E..S)
}

type SlotDevice struct {
    ItemID string `json:"item_id"`
    Grade  int    `json:"grade"` // 0..5 (E..S)
}
```

`State.Ships map[string]ShipInstance` (keyed by `ModelID`) replaces
`State.Upgrades`; `State.ActiveShipID string` replaces the implicit single
ship. New pilots start owning exactly one instance: the free Skiff, grades all
at their listed starting grade (see table below), no installed devices,
active.

Losing the active ship (hull reaches 0, gameplay/02's death resolver) deletes
its `ShipInstance` from the hangar entirely — grades and installed devices are
destroyed, matching the existing "ships are lives" rule. It does **not** touch
any other ship sitting in the hangar. If the destroyed ship was the pilot's
only ship, they fall back to owning nothing and must either fly a hangar ship
they still have or **buy back** the lost model (see below); the free Skiff's
buyback price is always 0, so a pilot can never be permanently shipless.

### Grades (E–S, 0–5 circles)

Every ship track and every slot device has a **grade** from 0 to 5, rendered
as the existing filled/empty circle indicator (`●●●○○` style, already in
`internal/tui/chart.go`) at a fixed width of 5 circles:

| Grade | E | D | C | B | A | S |
| --- | --- | --- | --- | --- | --- | --- |
| Circles filled | 0 | 1 | 2 | 3 | 4 | 5 |

A ship's **cap** per track is the highest grade its model can ever reach —
different classes/models cap different tracks at different grades on purpose
(see the ship table), so no ship can be maxed out everywhere. Slot devices are
not ship-capped; any device up to grade 5 can be bought and installed if power
and mass allow it (see below) — the constraint on a device's grade is the
player's power/mass/credit budget, not the hull.

### Brands

Three publishers, used for flavor and (as more ships are added later) a
consistent price/lean identity:

| Brand | Identity | Tendency |
| --- | --- | --- |
| **Federation** | Standardized, bureaucratic, well-funded | Reliable all-around caps, priced ~mid, best Scanner/Power Generator ceilings |
| **Alliance** | Military-industrial | Best Hull/weapon caps, worst Fuel Efficiency ceilings, priced highest |
| **Independent** | Scrappy, jury-rigged, cheap | Lowest prices, uneven caps, but punches above its price in at least one track |

Only 4 ships exist today (Independent ×2, Federation ×1, Alliance ×1), so this
table is mostly a promise for ships added later — the starter Skiff and the
Mule both being Independent is why they read as "cheap and scrappy" rather
than "balanced."

### Classes

| Class | Role | Weapon slot? |
| --- | --- | --- |
| **Miner** | Lightweight, mine-and-leave, best escape | Never |
| **Fighter** | Balanced, built to survive a pirate encounter | Yes (2) |
| **Freighter** | Heavy, slow, cavernous hold | Yes, but only 1 |

### The 4 starter ships

Price is credits to buy from empty-handed; Buyback is 25% of Price, rounded to
the nearest 10, charged instead of Price when re-buying a model whose
`ShipInstance` was just destroyed. Grades are `start`→`cap` per track (letter
+ circle count). Base Mass is the unladen hull mass used by the mass formula
below (before any slot devices are installed).

| Ship | Brand | Class | Price | Buyback | Slots (Util/Weap/Int) | Base Mass | Thrusters | Hull | Fuel Eff | Power Gen | Scanner |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| **SKIFF** (starter) | Independent | Miner | free | free | 2/0/1 | 100 | D→B | E→D | D→C | E→D | E→C |
| **CICADA** | Federation | Miner | 9,000 | 2,250 | 3/0/1 | 120 | C→B | D→C | C→A | D→C | C→S |
| **WARDEN** | Alliance | Fighter | 18,000 | 4,500 | 2/2/1 | 220 | C→B | C→A | D→C | C→B | D→C |
| **MULE** | Independent | Freighter | 26,000 | 6,500 | 4/1/1 | 380 | E→C | C→S | E→C | C→A | E→C |

Reading the table as intended tradeoffs:

- **SKIFF** is the free fallback: fast enough to escape, but so hull- and
  power-starved it can barely run one device. It is the only ship every pilot
  always has access to.
- **CICADA** is the survey specialist: best Scanner in the game (only ship
  that can reach S — full 10km unlock plus the A/S ETA-narrowing bonus) and
  the best Fuel Efficiency ceiling, at the cost of no weapon slot at all and a
  still-fragile hull.
- **WARDEN** is the only real combat ship: highest Hull ceiling among
  non-Freighters, two weapon slots, and enough Power Generator headroom to
  run a Turret and something else — but its Fuel Efficiency cap is the
  worst in the fleet, so combat loadouts drink fuel.
- **MULE** is the tank: best Hull ceiling in the game and four utility slots
  (room for cargo *and* a fuel tank *and* a shield), backed by the best Power
  Generator ceiling to actually run all of it — but Thrusters and Scanner are
  both capped at C, so it escapes slowly and can't see past 8km without a
  Scanner upgrade it's not built to carry far.

Exact prices/grades above are the balance-pass starting point, not gospel —
tune them against the invariants in "Balance worked examples" below before
shipping, the same way `balance.toml`'s other sections get tuned. The
*shape* of the tradeoffs (SKIFF fast/fragile, CICADA blind-spot-free but
unarmed, WARDEN the only real fighter, MULE the slow tank) is load-bearing.

### Ship upgrade tracks (stat grades — never cost power)

Purchasable only while docked, one grade at a time, up to the active ship's
cap for that track. Price scales per ship (heavier/pricier hulls cost more to
upgrade) and per grade:

```
price(track, ship, grade) = round(trackBaseUnit[track] * shipTierMul[ship] * 2.2^grade)
```

| Track | trackBaseUnit |
| --- | --- |
| Thrusters | 500 |
| Hull | 450 |
| Fuel Efficiency | 550 |
| Power Generator | 650 |
| Scanner | 700 |

| Ship | shipTierMul |
| --- | --- |
| SKIFF | 1.0 |
| CICADA | 1.8 |
| WARDEN | 2.6 |
| MULE | 3.4 |

Effects, applied at the ship's **current absolute grade** (not a per-level
delta — buying grade 3 sets the multiplier as if grade 3 had always been
installed):

| Track | Effect at grade *g* (0..5) |
| --- | --- |
| **Thrusters** | Escape-time multiplier `0.94^g` (S = ~27% faster escape) |
| **Hull** | Max hull pool `100 + 20*g` (S = 200, double the starter pool) |
| **Fuel Efficiency** | Fuel-drain multiplier `0.92^g` (S = ~34% less fuel burn) |
| **Power Generator** | Power capacity `10 + 8*g` (E=10 … S=50) |
| **Scanner** | Distance lock `min(10, 4 + 2*g)` km (B already reaches the 10km belt max); at grade **A/S** additionally narrows pirate ETA uncertainty by an extra 5 percentage points per grade beyond B, mirroring the retired Surveyor's ETA effect |

### Slots, power, and mass

Every ship has three slot *kinds*. Utility and Weapon slot **counts** are
per-ship (table above); Internal is always exactly **one** slot on every ship
— it holds a single swappable core module.

- **Utility slot** — Extra Cargo, Extra Fuel Tank, Shield, Chaff Launcher.
- **Weapon slot** — Defense Turret (only item today; not every ship has this
  slot at all).
- **Internal slot** — exactly one of: Seismic Sensors, Jump Drive (locked),
  Fuel Miner, Pirate Jammer. Swapping the installed module is a docked action
  like any other slot change.

**Power.** A ship's Power Generator grade sets its power *capacity*
(`10 + 8*g`, above). Every installed device draws power **except** Extra Cargo
and Extra Fuel Tank, which are pure structural additions with zero power draw
— this is the one explicit exception in "every slot costs power." The sum of
all installed devices' power draw must never exceed the ship's capacity;
installing a device that would exceed capacity is rejected outright (no
brownout mechanic — the shipyard simply won't let you equip it until you free
up power or upgrade the generator).

```
powerCost(deviceType, grade) = round(k[deviceType] * (grade+1)^1.5)
```

| Device | k | Power cost by grade (E,D,C,B,A,S) |
| --- | --- | --- |
| Shield | 2.7 | 3, 8, 14, 22, 30, 40 |
| Chaff Launcher | 0.6 | 1, 2, 3, 5, 7, 9 |
| Defense Turret | 1.6 | 2, 5, 8, 13, 18, 24 |
| Seismic Sensors / Fuel Miner / Pirate Jammer | 1.0 | 1, 3, 5, 8, 11, 15 |
| Extra Cargo / Extra Fuel Tank | — | 0 at every grade |

A fully-upgraded MULE (Power Generator A = 42 capacity) can run an S-grade
Shield (40) alone, or comfortably mix a B-grade Shield (22) with a B-grade
Turret (13) and an E-grade internal module (1) at 36/42 — but never an S
Shield *and* anything else. This is the intended "opt for more cargo instead
of a better weapon" tension: Extra Cargo costs mass but no power, so it's
always available headroom once the power-hungry choice is made.

**Mass.** Slot devices add hull mass by grade; heavier devices cost more mass
per grade than cheap ones:

```
mass(deviceType, grade) = massPerGrade[deviceType] * (grade + 1)
```

| Device | massPerGrade |
| --- | --- |
| Shield | 8 |
| Extra Cargo | 6 |
| Extra Fuel Tank | 5 |
| Defense Turret | 7 |
| Internal modules | 4 |
| Chaff Launcher | 2 |

Total ship mass = Base Mass (ship table) + sum of installed device mass. Let
`r = totalMass / baseMass` (always ≥ 1). Mass increases fuel burn and escape
time on top of — not instead of — the Thrusters/Fuel Efficiency track
multipliers:

```
fuelMul   = fuelEffMul(grade)   * (1 + 0.5 * (r - 1))
escapeMul = thrusterMul(grade)  * (1 + 0.6 * (r - 1))
```

So a MULE loaded with Shield+Cargo+Fuel-Tank at high grades pays for that
capacity in both slower escapes and thirstier tanks, which its Thrusters cap
(C) and Fuel Efficiency cap (C) only partially offset — matching its "slow
tank" identity rather than making it a strictly-better ship once maxed.

### Slot device catalog

Credit price follows the same shape as track upgrades: `price(item, grade) =
round(itemBase * 2.2^grade)`.

| Item | Slot | itemBase | Effect at grade *g* (0..5) |
| --- | --- | --- | --- |
| **Extra Cargo** | Utility | 300 | Cargo capacity `+15*(g+1)` |
| **Extra Fuel Tank** | Utility | 350 | Fuel capacity `+10*(g+1)` |
| **Shield** | Utility | 900 | Absorbs `20*(g+1)` hull damage per encounter before hull itself takes damage; recharges to full every time the ship departs |
| **Chaff Launcher** | Utility | 250 | Auto-fires on the first attack tick each run, suppressing pirate fire for `2+g` seconds; one use per run, resets on next departure |
| **Defense Turret** | Weapon | 700 | Reduces `AttackHullDamagePerSecond` by `8%*(g+1)` (cumulative multiplier `1 - 0.08*(g+1)`) |
| **Seismic Sensors** | Internal | 600 | 3 random belt asteroids arrive pre-scanned at zero fuel cost; at grade C+ at least one of the 3 is guaranteed Uncommon+, at grade A+ at least one is guaranteed Rare+ |
| **Fuel Miner** | Internal | 750 | Mining a Rare+ asteroid refunds fuel equal to `(0.25 + 0.15*g)` of that asteroid's `FuelCost` |
| **Pirate Jammer** | Internal | 850 | While armed, the asteroid currently being mined draws no pirates at all; consumes the arm on use, must rearm (free, instant) while docked; grade C+ grants `1 + floor(g/3)` uses per trip before a dock visit is required |
| **Jump Drive** | Internal | 5,000 | **Locked.** Visible in the shipyard as a teased, un-buyable item ("REQUIRES NEXT-SYSTEM ACCESS") until the future system-travel content lands. No power cost or effect yet. |

### Buyback

```
BuybackPrice(model) = round(0.25 * Price(model) / 10) * 10
```

Buying back a destroyed model returns a **fresh** `ShipInstance` — all track
grades reset to that model's starting grades, no slot devices installed. It is
not the same upgraded ship that died; re-equipping it costs credits again
exactly like a first-time purchase. This is what makes buyback a discount on
*re-entering the model*, not a death-insurance policy that protects your
build.

### Scanner distance lock

Asteroids at or beyond the active ship's Scanner lock distance
(`min(10, 4 + 2*grade)` km, see track table) cannot be scanned or targeted —
the belt view renders them as an `OUT OF RANGE` contact instead of a normal
one. This is a hard gate, not a fuel/time surcharge (which already exists via
`scan_fuel_cost`/`scan_sec_per_km` for in-range contacts). A stock ship
(Scanner grade E, 4km lock) cannot see roughly the far half of a belt
(`distance_max` is 10km); grade B (8 or 10km, ship-dependent cap) opens
essentially the whole belt.

### Distance-biased rarity

Farther contacts should be modestly more likely to roll a higher tier, on top
of the existing per-world `RarityBias`. Small nudge, not a guarantee:

```
effectiveBias(asteroid) = world.RarityBias + (distance / distance_max) * distance_rarity_bonus_max
```

`distance_rarity_bonus_max = 0.08` (a tunable in `[belt]`) — at the 10km max
distance this adds at most the same swing as going from a MED-bias world to a
slightly-richer one; it should never make far rocks reliably Legendary.
**Implementation note:** `belt.go`'s `GenerateBelt` currently rolls `tier`
(line ~56) before `distance` (line ~61) for each asteroid — this change
requires rolling distance first, then feeding it into `rollTier`'s weights.

### Random events: hull-gated, not hull-scaled

Today (`internal/sim/events.go`) event chance scales continuously with
`(1 - hullPct)^2` at every hull level — a full-health ship still has a small
event chance. New rule: **events can only roll at all below 20% hull.** Above
that threshold, chance is exactly zero, full stop.

```
if hullPct >= 0.20 {
    return // no roll, ever
}
chancePerMinute = base_chance_per_minute +
    ((0.20 - hullPct) / 0.20) * low_hull_chance_bonus
badWeight = base_bad_weight + ((0.20 - hullPct) / 0.20) * low_hull_bad_weight
```

Same `badEvents`/`nuisanceEvents` split and cooldown as today; only the gating
condition and the rescale of the falloff curve from a 0–100% hull band to a
0–20% band change. `[events]` keeps its existing key names in
`balance.toml` — only the *meaning* of the 0..1 input to the formula changes
(it's now `(0.20-hullPct)/0.20` instead of raw `1-hullPct`), so this is a
one-function change in `tickEvents`, not a new content table.

## Balance worked examples

These are sanity checks to encode as tests once this lands (mirrors
gameplay/03's "Balance invariants" pattern):

1. A pilot who never buys anything can still profitably run the SKIFF
   indefinitely — its 2 utility slots plus 1 internal slot are enough to
   install at least a low-grade Fuel Tank or Cargo pod within its E-grade
   Power Generator's 10-power budget (Cargo/Fuel Tank cost 0 power by design,
   so this always holds regardless of Power Generator grade).
2. CICADA's Price (9,000) is reachable from a small number of successful
   starter-belt sales, matching gameplay/03 invariant 3's shape for "first
   real upgrade."
3. No single starter ship can reach grade S on every track — verify
   programmatically from the ship table (every row has at least two tracks
   capped below S).
4. WARDEN with a maxed Power Generator (B = 34) cannot run two S-grade weapon
   devices simultaneously even if it had two Turrets to install (24+24 > 34) —
   the power system meaningfully constrains double-stacking even the ship
   built for combat.
5. Buyback price for every non-free ship is strictly less than 30% of its
   sell price (guards against a future balance pass accidentally making
   buyback as expensive as buying fresh).
6. Death never leaves a pilot unable to fly: buyback price for SKIFF is always
   0.

## Acceptance criteria

- [ ] `State.Ships`/`State.ActiveShipID` fully replace `State.Upgrades`;
  `DecodeState` migrates or rejects pre-existing saves (single-save prototype
  — a hard reset of `var/moonminer.db` in dev is acceptable, call it out in
  the PR).
- [ ] Buying, switching, and losing ships behave exactly per the hangar model
  above, including the "at most one instance per model" rule and buyback
  restoring a fresh (not previously-equipped) instance.
- [ ] Every ship/device price and power/mass formula above resolves from
  `content.Content` — no magic numbers in Go.
- [ ] Track-grade purchases respect each ship's per-track cap; buying past cap
  and buying while in a belt both fail with typed errors.
- [ ] Installing a slot device that would exceed the ship's power capacity is
  rejected before any credits are spent; Extra Cargo/Extra Fuel Tank never
  consume power at any grade.
- [ ] Mass from installed devices measurably changes effective fuel drain and
  escape time on top of the Thrusters/Fuel Efficiency track multipliers
  (table-driven test with at least one heavy vs. light loadout).
- [ ] Scanner lock hides out-of-range asteroids from targeting; a grade-B+
  Scanner (whichever ship reaches it) exposes the full 10km belt.
- [ ] Distance-biased rarity nudge is measurable but bounded — a distribution
  test over many generated belts shows a small, not dominant, tier shift at
  high distance.
- [ ] Random events never roll at hull ≥ 20%; a scripted test holds hull just
  above and just below the threshold and asserts zero vs. nonzero event rolls
  over a fixed simulated duration.
- [ ] Balance worked examples 1–6 above pass as tests.

## Out of scope / handoffs

- The standalone Shipyard screen (layout, navigation, cyber-futuristic
  presentation) → [../tui/05-shipyard-screen.md](../tui/05-shipyard-screen.md).
- Jump Drive's actual system-travel behavior is intentionally unimplemented —
  this doc only requires it appear locked/teased in the catalog.
- Cosmetics, stations, permits, and the ship's-log/stats extensions in
  gameplay/04 are unaffected by this doc except where `ShipClass`-shaped
  fields need to reference the new model IDs instead of the old aspirational
  five-ship list — a small follow-up, not blocking.
