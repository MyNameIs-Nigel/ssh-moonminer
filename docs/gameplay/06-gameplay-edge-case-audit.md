# Gameplay 06 — Full-Loop Edge-Case Audit & Resolution Plan

**Area:** Gameplay / economy / shipyard / TUI  
**Status:** Design-review task; no gameplay behavior is changed by this document.  
**Scope:** The shipped loop from dock preparation through travel, mining, escape,
docking, ship loss, and recovery. This is intentionally broader than fuel.

## Goal

Make the hard-roguelite loop legible and fair at its boundaries. A player should
lose resources because they accepted a meaningful risk, not because the game
silently changed what an equipped module, ship, cargo hold, or pirate demand
means.

The current implementation has a sound deterministic simulation foundation, but
several resources are represented at account scope while their gameplay meaning
is physical and ship-scoped. That mismatch produces free repairs, capacity
overflows, protected cargo, and purchased equipment with no effect.

## Recommendation in one view

Keep fuel as an **absolute, continuous amount**. Mining drains fractional fuel
per tick, so changing it to an integer would create rounding artifacts. A
percentage is useful only as a progress bar; it must not be the only displayed
value or the source of truth.

Adopt these ownership rules before implementing individual fixes:

| Resource | Authoritative owner | Rule at a dock |
| --- | --- | --- |
| Hull and base fuel | `ShipInstance` | They persist for that ship; switching never repairs or refuels it. |
| Fuel in an Extra Fuel Tank | that `SlotDevice` | A stored tank retains its own fuel; selling it discards/sells the fuel explicitly. |
| Cargo | the active ship's hold | A switch is allowed only when the destination hold can carry it; reducing capacity below loaded cargo is rejected. |
| Credits, permits, stored modules | pilot account | These survive a ship change and ship loss. |
| Ore extracted from an asteroid | `ActiveRun` | It never returns to the asteroid, even if pirates take it. |

For fuel flow, use an intuitive auxiliary-reserve rule: refuelling fills and
burning drains detachable tanks first, leaving the hull's base tank as the
emergency reserve. A newly bought tank starts empty. Storing and reinstalling a
tank restores the exact amount it held; selling one shows a confirmation such as
`SELL TANK — 18 / 30 FUEL WILL BE LOST`.

This is more logical than silently clamping fuel on removal: a physical tank
keeps its contents, storage is meaningful, and a player can predict every
outcome. The HUD should show both `FUEL 86 / 120` and a percentage bar.

## Findings

Priority means player-impact priority, not implementation order. Estimates are
for one engineer familiar with this repository and include focused automated
tests, but exclude balance playtesting.

### 1. Removing a fuel tank creates fuel above the remaining capacity

**Priority:** P0 — economy exploit and state invariant violation  
**Current behavior:** `State.Fuel` is a single amount. `FuelCapacity` adds
installed tank capacity, but `StoreSlotDevice` and `SellSlotDevice` remove a
tank without changing fuel. A ship refuelled to 120/120 can sell a +20 tank and
continue with 120/100 fuel. `Refuel` then reports that it is already full, and
the player receives both the sale refund and the extra fuel.

**Why it hurts the loop:** Capacity upgrades become a way to borrow fuel, not a
loadout choice. Percentage-only HUDs hide the over-cap state particularly well.
The same ambiguity means "store then reattach" has no explainable answer.

**Fix:** Use the ownership model above. Add a fuel field only to fuel-tank
devices and move base fuel/hull into `ShipInstance`; provide helpers for total
fuel, total capacity, refuelling, and deterministic reserve-first consumption.
Do not permit a generic `State.Fuel` to exceed the active capacity. A smaller
fallback fix—clamp fuel on removal—is safe but makes storage feel destructive
and should be avoided unless the larger condition-state migration is deferred.

**Tests:**

- Fill a base tank and a grade-C extra tank; store it, verify the active total
  falls by exactly the tank's contained fuel and the stored device retains it.
- Reinstall the same tank on another eligible ship and verify its fuel returns
  unchanged, with no credit or fuel creation.
- Sell a partly full tank and verify the confirmation/recorded loss is exact;
  total fuel never exceeds total capacity before or after every action.
- Round-trip the state through `Encode`/`DecodeState` and exercise a migration
  from the old single-fuel save shape.

**Difficulty:** XL — 4–6 days; it shares the condition-state work in finding 2.

### 2. Switching ships is an unlimited free refuel, repair, shield service, and rearm

**Priority:** P0 — bypasses two credit sinks and risk recovery  
**Current behavior:** `activateShip` sets fuel to capacity, hull to maximum,
restores the shield, and rearms jammer/EMP devices. `SwitchActiveShip` calls it
for free. A player can fly a damaged, empty ship to dock, switch to a spare, and
switch back to erase all damage and fuel cost. Purchasing a ship also immediately
activates it, making the effect easy to trigger accidentally.

**Why it hurts the loop:** Refuel and repair prices stop regulating run length.
A second cheap hull becomes stronger insurance than the intended port economy,
and recovery after a risky escape feels arbitrary rather than earned.

**Fix:** Persist hull and base fuel per `ShipInstance`. Switching transfers the
pilot to the selected ship's existing condition; it does not service either
ship. Keep the free dock service only for the explicitly designed consumables
(EMP/jammer/shield) or price those services deliberately. A newly acquired or
bought-back hull may start fully serviced as part of its purchase contract, but
the UI should say so and purchase should ask whether to activate it.

**Tests:**

- Damage and drain Ship A, switch A → B → A, and assert A's hull/fuel are
  unchanged while B retains its own condition.
- Verify refuelling and repairing only affect the active ship and charge once.
- Verify a new hull/buyback follows the chosen full-or-empty purchase rule;
  switching does not invoke that rule.
- TUI test: the hangar status shows each parked ship's condition and purchase
  does not silently change the active ship without confirmation.

**Difficulty:** XL — 4–6 days including migration and TUI status work.

### 3. A full hold can still start a run and spend fuel for zero ore

**Priority:** P0 — avoidable loss and pirate exposure  
**Current behavior:** `Lock` does not check remaining cargo space. If the hold
is already full, it deducts the asteroid flight fuel and starts a run whose
depletion condition is already true. `Depart` likewise allows travel to a belt
with no available cargo space.

**Why it hurts the loop:** A player can pay travel, scan, and lock costs only to
discover that the green DEPART state was true from the first frame. Pirates can
still arrive, making the error much more expensive than the information given.

**Fix:** Introduce `RemainingCargoCapacity(s, c)`. Reject `Lock` with a typed
`ErrCargoFull` before spending fuel. Also block normal `Depart` while cargo is
full; the only player-facing alternatives should be SELL CARGO or switch to a
ship with a larger compatible hold. If revisiting a belt with no mining intent
is valuable later, expose it as a separate travel action rather than silently
permitting a dead mining trip.

**Tests:**

- Set cargo equal to capacity; assert `Lock` and `Depart` return `ErrCargoFull`
  without changing fuel, belt, run, or RNG state.
- Set cargo to capacity minus one fractional mining unit; assert one unit can
  be mined and then normal full-hold behavior occurs.
- TUI test: chart departure and target-lock controls show the specific reason,
  rather than a generic unavailable action.

**Difficulty:** S — 0.5–1 day.

### 4. Removing cargo capacity or switching to a smaller ship leaves impossible cargo

**Priority:** P0 — broken physical model and follow-on dead runs  
**Current behavior:** Extra Cargo may be stored or sold while the hold contains
more units than the remaining capacity. The pilot may also switch from a full
MULE to a smaller ship. `CargoUnits` remains global, so the port can render
values such as `4700/1400`; subsequent mining has zero room.

**Why it hurts the loop:** The game neither transfers nor jettisons the cargo,
so it asks the player to infer an invisible exception to cargo capacity. It also
makes a useful fleet feel dangerous to manage for administrative reasons.

**Fix:** Before any capacity-reducing module removal, ship switch, or automatic
purchase activation, calculate the destination capacity. Reject the action with
`ErrCargoDoesNotFit` if current cargo exceeds it. Treat a permitted switch as a
docked crew transfer of physical cargo. Selling cargo remains the simple way to
make every ship compatible.

**Tests:**

- Fill a ship using an Extra Cargo device; storing/selling it must fail until
  enough cargo is sold.
- A MULE full above a Skiff's capacity cannot switch to Skiff, but can switch
  to a ship whose capacity fits exactly.
- Property test: after every supported dock action,
  `0 <= CargoUnits <= CargoCapacityUnits(active ship)`.

**Difficulty:** M — 1–2 days; coordinate with finding 3.

### 5. Pirate tribute protects all cargo mined before the current asteroid

**Priority:** P0 — high-value cargo is unintentionally insulated  
**Current behavior:** `AcceptTribute` computes demand from `run.CargoValue`,
which contains only ore from the asteroid currently being drilled. Cargo carried
from earlier asteroid runs lives in `State.CargoValue` and is untouched.

**Why it hurts the loop:** The intended tension is that a loaded hold escapes
slowly and pirates can take the easy money. A player can fill most of the hold
with valuable ore, start a tiny final rock, and pay tribute only from that tiny
rock—sometimes effectively zero—while keeping the valuable load.

**Fix:** Define the demand against total cargo aboard:
`State.CargoValue + ActiveRun.CargoValue`. Deduct it proportionally from both
the already-held and current-run cargo (and their unit counts). Keep a concise
combat log that states total cargo, tribute demanded, and cargo retained.

**Tests:**

- Seed prior cargo and a small current run, trigger tribute, and assert the
  loss equals the configured percentage of the combined value.
- Verify zero current-run cargo cannot turn a tribute into a free escape while
  stored cargo exists.
- Check values/units remain non-negative and recovered cargo at resolution is
  the exact post-tribute total.

**Difficulty:** M — 1–2 days; depends on the accounting change in finding 6.

### 6. Tribute cargo is returned to the asteroid as re-mineable ore

**Priority:** P0 — repeatable ore and an incorrect narrative consequence  
**Current behavior:** To represent dropped tribute, `AcceptTribute` reduces
`ActiveRun.MinedUnits`. Remnant calculation later uses that same field to decide
how much ore remains on the asteroid. Jettisoned ore is therefore treated as
never extracted and can be mined again.

**Why it hurts the loop:** Pirates appear to take cargo and then magically put
it back into the rock. It rewards repeating a low-risk tribute sequence and
makes asteroid depletion impossible to reason about.

**Fix:** Split extraction from held cargo. For example, preserve
`ExtractedUnits` for asteroid/remnant accounting and track `HeldUnits` (plus
`JettisonedUnits` for records) for cargo and tribute. Remnants must use all ore
ever extracted, while outcome cargo uses only what remains in the hold.

**Tests:**

- Mine part of an asteroid, accept tribute, resolve, and assert its remaining
  volume equals original volume minus total extracted volume—not minus the
  cargo the player retained.
- Reacquire the remnant and verify total value mined across both visits cannot
  exceed the asteroid's original value, subject only to documented rounding.
- Deterministic replay test: the same ticks and choice produce identical
  extraction, jettison, remnant, and run-log values.

**Difficulty:** M — 1–2 days; this is the prerequisite for finding 5.

### 7. Duplicate Turrets and Shields consume resources but only the first works

**Priority:** P1 — expensive dead purchases  
**Current behavior:** the WARDEN can install two Turrets, and ships can install
multiple Shields. `AttackDamageMul` and `ShieldMaxHPFor` use `findDevice`, which
returns only the first matching device. A second item consumes credits, power,
and mass but adds no defense. The picker gives no warning.

**Why it hurts the loop:** The only current weapon catalogue entry makes an
entire WARDEN slot a trap. Players who optimise from the visible power/mass
numbers can still buy a nonfunctional item, which feels like a bug rather than
a difficult trade-off.

**Fix:** Choose and document one rule per item category:

- Extra Cargo, Extra Fuel Tank, and EMP Launcher stack; each has a distinct
  physical purpose.
- Shield should be unique unless its capacity/charge is deliberately summed.
- Turrets should either stack using a capped multiplicative mitigation formula
  (recommended while it is the sole weapon) or be unique until more weapon
  choices exist.

The shipyard must disable/relabel an item that cannot provide another effect.

**Tests:**

- Install two Turrets and assert the selected stacking formula is applied, or
  assert the second install returns a typed duplicate error before credits
  change.
- Repeat for Shields and verify shield charge, power, mass, and HUD all agree.
- TUI picker test: duplicate-unique items have an explicit reason instead of a
  selectable price.

**Difficulty:** S–M — 1 day for uniqueness; 1–2 days for stacking/charge UI.

### 8. Progression-gating hardware can be rented for one-way access

**Priority:** P1 — progression gates do not mean what they appear to mean  
**Current behavior:** route gates inspect only the active ship's equipment when
departing *to* a destination. Once docked in Eridani, the player can store or
sell the Jump Drive and depart to a Sol destination because no origin-side route
rule exists. Likewise, a player may buy an Extra Fuel Tank, enter a capacity-
gated belt, then sell it at the next dock for 95% of its price. The fuel-overcap
bug in finding 1 makes this even stronger.

**Why it hurts the loop:** A costly ship configuration becomes a short rental
instead of an earned capability. The star map says systems are places the pilot
occupies (`SystemID`), while travel behaves like a one-way menu with no source
constraints. Both readings cannot be true at once.

**Fix:** Make an explicit design choice and enforce it consistently:

1. **Recommended:** systems are physical locations. Model legal outbound links
   and require the relevant route key both to enter and to leave a gated system.
   Prevent selling/storing the last required key while docked there.
2. **Simpler arcade alternative:** destinations are contracts launched from a
   universal port. Remove `SystemID` as a location claim, describe hardware as
   a per-contract requirement, and intentionally allow resale after a run.

The physical-location option better fits the SSH roguelite's world and makes
the Jump Drive and fuel-capacity investments real progression.

**Tests:**

- Enter a gated system; attempting to remove the final required key fails with
  a clear reason and leaves the loadout unchanged.
- With source/destination links enabled, every reachable dock has at least one
  valid outbound route (property test over all content systems/loadouts).
- Verify a capacity upgrade cannot be used to retain fuel, pass a gate, then
  disappear without the defined exit cost.

**Difficulty:** L — 2–4 days; larger if world-map topology is added to TOML.

### 9. The insurance advance can be claimed by pilots who are not softlocked

**Priority:** P1 — free credits and an unclear recovery contract  
**Current behavior:** `InsuranceEligible` checks only docked state, the
one-claim flag, credits, and fuel. It does not require the starter ship or an
empty hold. A pilot carrying sellable cargo, or one who owns an expensive fleet,
can empty fuel/credits and claim the 300-credit advance. The flag resets only
on `OutcomeDeparted`, whereas the design says recovery should return after
selling cargo or buying a non-free ship.

**Why it hurts the loop:** The safety net becomes a farmable cash subsidy for
players who already have an escape route. Conversely, a genuinely recovering
pilot can complete a cautious bail/escape and still be denied the documented
next advance.

**Fix:** State the safety-net contract in one predicate: docked, starter ship,
no sellable cargo, no viable departure fuel, credits below the minimum refuel,
and no active claim. Reset the claim on the documented recovery milestones
(cargo sale or non-free ship acquisition), not on one particular run outcome.

**Tests:**

- Table-drive eligibility for cargo, non-starter ships, fuel at/just below the
  threshold, credits at/just below the threshold, and prior claim use.
- Claim, then sell cargo / buy a paid ship / bail / depart in separate cases;
  assert exactly the intended transitions reset it.
- Balance invariant: an eligible pilot can buy the cheapest departure fuel and
  start one starter run after receiving the advance.

**Difficulty:** M — 1–2 days including content/invariant tests.

### 10. Pirate Aggression is a no-cost difficulty/reward lever

**Priority:** P1 — unresolved game-design choice  
**Current behavior:** the Tweaks overlay allows 0.5, 1.0, 1.5, and 2.0 Pirate
Aggression. It directly changes pirate approach speed while asteroid values,
permits, ship prices, and stats remain unchanged.

**Why it hurts the loop:** If it is meant to be an optimisation control, every
rational pilot uses 0.5 for the same income. If it is an accessibility setting,
calling it aggression without explaining its consequence makes players feel
that they are cheating or missing rewards. A hard roguelite needs a clear
answer, not an accidental one.

**Fix:** Choose one of these valid models:

- **Accessibility (recommended):** rename it to a threat-assist setting,
  explain that rewards are unchanged, and keep it freely selectable.
- **Challenge contract:** lock a chosen difficulty for a pilot/run and apply a
  transparent reward/score multiplier, with 1.0 as the intended balance.

Do not leave a mechanical risk knob disguised as a neutral visual tweak.

**Tests:**

- For accessibility, test that changing the setting changes only stated threat
  parameters and is rendered with the explanatory label.
- For challenge mode, distribution-test pirate arrival and the exact published
  reward multiplier at every level; settings cannot change mid-run.

**Difficulty:** S for accessibility copy/UI; M–L for a reward/score economy.

### 11. Fuel information and refuelling price semantics disagree

**Priority:** P2 — avoidable confusion in a core resource  
**Current behavior:** normal HUDs show only a percentage, even though tank size
varies. `Refuel` charges a rounded full-tank price but, when credits are short,
turns every remaining credit into fractional fuel. The design document instead
describes buying whole fuel points with `floor(credits / rate)`.

**Why it hurts the loop:** At 50%, a Skiff and a tank-equipped MULE look equally
safe despite very different range. A player with eight credits can buy 8/9 of a
fuel point despite a 9-credit per-point price, while the visible full price is
rounded differently. This makes planning one more run needlessly opaque.

**Fix:** Render `FUEL current / capacity (percent)` everywhere, including the
shipyard and mining HUD. Keep fractional internal fuel, but make port purchases
whole fuel points at a whole-credit rate: buy `min(floor(credits/rate),
floor(missing))` points and charge exactly points × rate. If sub-point topping
off is desired, publish a continuous rate and eliminate all rounding instead.
The whole-point option is easier to explain and matches the existing design
specification.

**Tests:**

- Golden/render tests show absolute fuel and capacity at dock and in a run.
- Table-test credits 0, 1, 8, 9, full-cost minus one, exact full cost, and
  near-full tanks; assert exact credits spent and fuel gained.
- Invariant: port refuelling never exceeds capacity and never creates value
  through mixed rounding.

**Difficulty:** S–M — 1–2 days; larger only when combined with finding 1.

### 12. The mining resource meter ignores cargo already aboard

**Priority:** P2 — the screen lies at the important decision point  
**Current behavior:** `renderDrilling` calculates its resource cap from total
ship capacity, not remaining capacity after `State.CargoUnits`. The simulation
ends mining at remaining capacity, but the UI bar can still show 50% resource
left while DEPART is already available.

**Why it hurts the loop:** The player uses this meter to judge whether to keep
mining or flee. A progress display that contradicts the action state turns a
deliberate cargo-risk decision into a perceived bug.

**Fix:** Use the same shared helper as the simulation:
`min(asteroid volume, RemainingCargoCapacity + ActiveRun.HeldUnits)`. Avoid
duplicating capacity math in the TUI; expose a named sim-derived value if that
makes the relationship clearer.

**Tests:**

- Preload half a hold, enter a run, advance until it is full, and assert the
  rendered resource value reaches zero exactly when `RunDepleted` becomes true.
- Cover empty hold, rock-limited run, and cargo-limited run.

**Difficulty:** S — less than a day; depends on finding 3's helper.

### 13. Replacing a module cannot use the old module's sale proceeds

**Priority:** P2 — the 95% resale promise fails at the moment players need it  
**Current behavior:** the picker marks a new catalog module unaffordable unless
current credits cover its full price. The player cannot select it, choose SELL
for the installed module, and use that refund toward the replacement—even when
the refund would make the purchase affordable. They must sell first, reopen the
picker, and remember what they intended to buy.

**Why it hurts the loop:** The shipyard advertises module resale as a flexible
build tool but makes an ordinary upgrade path awkward. It especially punishes
cash-poor players trying to change a loadout after learning from a loss.

**Fix:** For an occupied slot, let the picker show both `BUY WITH CASH` and
`SELL + BUY` affordability. Once the player chooses sale, execute removal and
installation atomically in one sim action after all validation passes. Stored
replacement remains free and must preserve its own state (notably tank fuel).

**Tests:**

- With credits below the new price but above `new price − sale refund`, verify
  the replacement option is selectable and completes with exact balances.
- Force a validation failure (power, bad inventory item, capacity conflict) and
  assert the existing module and all credits remain unchanged.
- TUI test covers the prospective sale price and an explicit confirmation.

**Difficulty:** M — 1–2 days.

### 14. Ship loss auto-selects an expensive backup while the summary promises a Skiff

**Priority:** P2 — player agency and narrative state diverge  
**Current behavior:** on ship loss, the simulation picks the highest-price
remaining ship as active. The death summary always says that a starter Skiff is
waiting. With a backup WARDEN or MULE, both statements cannot be true.

**Why it hurts the loop:** A ship is a major build decision. Automatically
launching the most valuable surviving hull can feel like the game has spent the
player's next risk decision, and the summary conceals the actual recovery state.

**Fix:** After respawning at Sol, leave the player at a hangar-selection state
or choose a documented default (for example the Skiff if owned). Render the
actual active replacement and its condition. Do not silently service a backup;
finding 2's per-ship condition rule applies here too.

**Tests:**

- Lose an active ship with multiple backups; assert the documented selection
  policy, screen copy, active ID, hull/fuel, and cargo loss are consistent.
- Lose the only ship; assert a fresh free Skiff is available and no permanent
  softlock is possible.

**Difficulty:** M — 1–2 days, mostly state/UI flow after finding 2.

### 15. The scanner range boundary is excluded

**Priority:** P3 — small but crisp rule-boundary bug  
**Current behavior:** `IsOutOfRange` returns true when distance is greater than
or equal to the Scanner lock. A target exactly at a displayed 4.0 km lock is
therefore unavailable, although player language and the design describe the
lock as the farthest distance the scanner can target.

**Why it hurts the loop:** Boundary rules are where deterministic games gain or
lose player trust. A distance meter saying 4.0 km against a 4.0 km lock should
not create an unexplained denial.

**Fix:** Treat the lock as inclusive (`distance > lock` is out of range) and
render the same precision used by the comparison. If a deliberate safety margin
is desired, show that smaller effective limit rather than a rounded larger one.

**Tests:**

- Table-test distance just below, exactly equal to, and just above every
  scanner grade's limit; scan/lock availability must match the published rule.

**Difficulty:** S — less than a day.

## Recommended delivery plan

Do not implement all fifteen findings as isolated `if` statements. The order
below creates stable invariants first and makes the smaller fixes straightforward.

1. **Protect player state now (2–4 days):** findings 3, 4, 7, 12, 13, and 15.
   These are guards, shared capacity helpers, and UI corrections with little
   persistence impact.
2. **Repair cargo/pirate accounting (2–3 days):** findings 5 and 6. Introduce
   explicit extracted/held/jettisoned run quantities, then write the value
   conservation properties before rebalancing tribute.
3. **Make ship condition physical (4–6 days):** findings 1 and 2, plus the
   fuel portion of 11 and the recovery portion of 14. This is the only save
   migration-heavy change and should be one reviewed slice, not a patch series.
4. **Resolve progression/recovery policy (3–5 days):** findings 8–10 and the
   remaining death flow. These need a product decision before implementation.

**Overall difficulty:** L/XL — approximately 11–18 focused engineering days,
plus balance sessions for tribute, threat settings, route topology, and recovery
economy. The first two stages are independently shippable and remove the most
exploit-prone behaviors in roughly one work week.

## Test strategy and acceptance criteria

Simulation tests belong in `internal/sim` and remain pure/deterministic. TUI
tests belong beside the existing shipyard/chart/mining tests. Add migration
fixtures to `state_test.go`; no test needs wall clock or I/O.

### Cross-cutting invariants

- Active total fuel is never negative or above active capacity; stored tank fuel
  is non-negative and no greater than that tank's capacity.
- Active cargo is never negative or above the active ship's capacity after any
  dock, loadout, fleet, or outcome action.
- Switching ships cannot create credits, fuel, hull, shield charge, EMP charges,
  or jammer charges except through an explicitly documented service/purchase.
- Across an asteroid's lifetime, extracted plus unmined volume equals original
  volume, and cargo jettison does not become asteroid ore.
- Pirate tribute is calculated from all cargo physically aboard, not merely the
  last asteroid's contribution.
- Every enabled TUI action has a sim action that succeeds under the displayed
  conditions; every rejected sim action leaves state unchanged.

### Scenario suite

Create table-driven scenarios that run complete loops rather than unit-testing
only helper math:

1. Starter Skiff: refuel, travel, scan, mine, bail, dock, sell, repeat.
2. Tank lifecycle: fill, store, reinstall, sell, buy a replacement, and travel
   through a capacity gate.
3. Fleet handoff: cargo-full MULE → Skiff rejection; damaged Ship A → Ship B →
   Ship A preserves both conditions.
4. Pirate accounting: valuable prior cargo plus a cheap last rock; accept
   tribute and reacquire the remnant.
5. Duplicate combat loadout: one versus two Turrets/Shields follows the selected
   catalogue rule and shows the same effect in HUD and sim.
6. Recovery: eligible and ineligible insurance claims; loss with and without a
   backup ship; actual death summary copy.
7. Difficulty: every pirate-aggression level follows the stated accessibility or
   reward contract.

Before an implementation PR is complete, run:

```bash
go build ./...
go vet ./...
go test ./...
```

## Out of scope

This audit deliberately does not prescribe final numerical balance values,
station-market bonuses, new ship/device content, or a leaderboard. Those should
be tuned only after the ownership and conservation rules above are true; changing
prices before then would balance around loopholes rather than intended decisions.
