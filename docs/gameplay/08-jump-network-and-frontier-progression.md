# Gameplay 08 — The Jump Network, Pilot Ratings & Frontier Progression

**Area:** Gameplay + TUI · **Phase:** 8 (beta) · **Depends on:** gameplay/02
(run loop), gameplay/03 (permits, travel, balance TOML), gameplay/05
(hangar, slots, power/mass), gameplay/07 (bounties, named pirates) ·
**Supersedes:** the ship-mounted Jump Drive in gameplay/05 and gameplay/07,
the "Locked destinations" gate list in `../02-danger-economy-and-progression.md`,
and the system-level `transfer_fee` model in gameplay/03.

Like gameplay/05 and gameplay/07, this doc owns both sim and TUI work — the
jump sequence and the sim's transit resolution are too coupled to spec apart.

## Goal

Turn crossing a system boundary from a menu selection into the game's second
real ritual (the first being the extraction decision). Today the entire
inter-system wall is one 25,000cr module purchase; past it, all of Eridani is
open, permit-free, and costs fuel comparable to an in-system hop to Titan. A
jump is currently indistinguishable from a flight.

This doc replaces that with four coupled changes:

1. **The Jump Drive leaves the ship and becomes the pilot.** Route access is a
   lifetime **Jump Rating** (Class E → D → C), not a device bolted to one hull.
2. **A rating is earned, not only bought.** Each class costs credits *and* a
   qualification that can only be completed at the current frontier.
3. **A jump costs something every single time** — fuel scaled by hull mass,
   guaranteed transit stress on the hull, and a chance of a bad translation.
4. **Ships are physical property parked at a dock.** Your hangar does not
   teleport. You fly a hull to the frontier or you pay to have it hauled.

The intended feeling: arriving in a new system for the first time should be
the most expensive and most memorable thing a pilot has done, and deciding
*which* ship gets to live out there should be a standing strategic problem.

## References

| Source | What to take |
| --- | --- |
| [../01-concept-and-story.md](../01-concept-and-story.md) | § "Systems, worlds, and ships" — the KEPLER REACH / REDLINE EXPANSE promise and its existing "higher jump rating" wording, which this doc cashes in |
| [../02-danger-economy-and-progression.md](../02-danger-economy-and-progression.md) | locked-destination narrative and the `LOCKED — NEED JUMP DRIVE` vocabulary this doc rewrites |
| [03-economy-worlds-and-balance.md](03-economy-worlds-and-balance.md) | permit/transfer machinery, port services, `worlds.toml`/`balance.toml` conventions |
| [05-fleet-ships-and-shipyard-economy.md](05-fleet-ships-and-shipyard-economy.md) | hangar model, grades, power/mass formulas, buyback — the Jump Drive slot this doc deletes |
| [06-gameplay-edge-case-audit.md](06-gameplay-edge-case-audit.md) | **finding 8** (route hardware rented for one-way access) — this doc resolves its system half outright |
| [07-pirate-combat-and-bounties.md](07-pirate-combat-and-bounties.md) | bounty vouchers and `Stats.PiratesDestroyed`, the source of combat qualifications |
| `internal/tui/death.go` | the CRT-failure flash the jump sequence deliberately mirrors in blue |
| `internal/tui/permit.go` | the existing confirm-overlay pattern the jump confirm reuses |
| `internal/sim/derive.go` (`RouteLockReason`, `systemsLinked`) | the gate evaluation this doc extends |
| `internal/sim/events.go` (`tickEvents`) | the 20% hull event gate that jump damage can push a pilot under |

## Deliverables

- `internal/sim/jump.go` — Jump Rating state, gate evaluation, jump cost,
  transit resolution (stress + drift roll), `Jump()` action
- `internal/sim/frontier.go` — per-system pilot record and qualification
  predicates
- `internal/sim/ferry.go` — the hauler service (priced relocation of a
  non-active hull)
- Extends `internal/sim/ships.go` (ship location, location-scoped purchase),
  `internal/sim/state.go` (`JumpClass`, `Frontier`, `ShipInstance.SystemID`),
  `internal/sim/derive.go` (`RouteLockReason` rewrite)
- **Deletes** `SlotJumpDrive`, `ItemJumpDrive`, `ErrRouteKeyRequired` and the
  store/sell route-key guard in `internal/sim/slots.go`
- `internal/tui/jump.go` — the jump sequence screen; `internal/tui/certify.go`
  — the certification overlay
- Extends `internal/tui/chart.go` (rating readout, gate rows, ferry panel) and
  `internal/tui/shipyard.go` (drops the JUMP DRIVE row, adds hull location)
- `[jump]`, `[ferry]`, `[[ratings]]` in `data/balance.toml`; two new systems
  and their destinations in `data/worlds.toml`; three new `[[ships]]`
- Save schema **v8** with the migration in "Migration" below

## Migration: what's retired

| Retired | Disposition |
| --- | --- |
| **`SlotJumpDrive` / `ItemJumpDrive`** | Deleted. Route access moves to the pilot's Jump Rating. The Shipyard's one-item JUMP DRIVE picker row disappears with it (tui/05). |
| **`ErrRouteKeyRequired`** and the "can't sell the drive while docked in a gated system" guard | Deleted. A lifetime pilot rating cannot be sold, so the exploit it patched no longer exists — this is the clean fix gameplay/06 finding 8 asked for, replacing the band-aid. |
| **System-level `transfer_fee`** | Retired as a gate. `[[ratings]]` prices replace it. Destination-level `permit_fee` (Ceres) is **unaffected** and stays. |
| **`System.required_item_id`** | Retired. Replaced by `System.required_rating`. |
| **`State.SystemPermits`** | Retired. Superseded by `State.JumpClass`. `DestinationPermits` stays. |

**Save migration (v7 → v8).** Any pilot holding a Jump Drive — installed on
any hull or sitting in `Inventory` — is granted **Class E** and the device is
removed. The **first** drive is *not* refunded: Class E costs exactly what the
drive cost, so the grant is the value they already paid for and a refund on top
would hand every existing pilot a ~24,000cr windfall. Only *additional* copies
(a pilot who equipped two hulls) refund at
`jump_drive_base_price * slots.sell_value_pct`. A pilot with `SystemPermits["eridani"]` is likewise granted Class E.
Every existing `ShipInstance` is stamped with the pilot's current `SystemID`
(they were all effectively co-located under the old model), and a
`FrontierRecord` is created for Sol and for any system the pilot has visited,
seeded from `RunLog` where the system is recoverable and zeroed otherwise.
Seeding from an incomplete log is acceptable: it can only make the next
qualification slower, never skip one.

## Spec

### 1. The Jump Rating

`State.JumpClass int` — `-1` uncertified, `0` = Class E, `1` = D, `2` = C.
It is a pilot credential: it survives ship death, cannot be sold, stored,
or transferred, and is never attached to a hull.

Classes use the game's existing E–S grade lettering (`sim.GradeLetter`) so the
readout is consistent with ship tracks and slot devices. **Beta ships three
classes.** B, A and S are deliberately absent rather than stubbed — the star
chart shows the far edge of the network as `NO CHARTED ROUTE`, which reads as
worldbuilding rather than as an unfinished ladder.

> **Why the rating is pilot-side even though it costs a death stake.** Under
> the shipped model, dying in Eridani destroyed the 25,000cr drive with the
> hull — a real stake this change gives up. It is given up on purpose: a
> per-`ShipInstance` route key taxes the hangar model gameplay/05 just
> introduced (owning a Cicada *and* a Warden meant buying the drive twice or
> shuffling it every time you switched hulls), and it forced the
> `ErrRouteKeyRequired` guard to stop players renting one-way access. The
> stake is repaid, with interest, by the per-jump cost in § 4 — which charges
> on **every** crossing rather than once at purchase.

### 2. Earning a class: credits plus frontier proof-of-work

A class is bought at a dock **in the system on the near side of the gate it
opens** — you certify at the frontier, not back home. Two requirements, both
hard:

- **Credits**, a large escalating sink.
- **A qualification** whose counters only advance in the system you are
  currently certified for. This is the load-bearing half: a pure credit price
  is farmable at Vesta, which would make every wall in the game fall to
  patience. Requiring frontier work means you cannot buy your way past content
  you have not played.

`State.Frontier map[string]*FrontierRecord`, keyed by system id:

```go
type FrontierRecord struct {
    Visited           bool           `json:"visited"`
    FirstArrivalAt    int64          `json:"first_arrival_at"`
    RunsSurvived      int            `json:"runs_survived"`
    RunsByDestination map[string]int `json:"runs_by_destination,omitempty"`
    CargoValueSold    int            `json:"cargo_value_sold"`
    PiratesDestroyed  int            `json:"pirates_destroyed"`
    LegendariesMined  int            `json:"legendaries_mined"`
    ShipsLost         int            `json:"ships_lost"`
}
```

Counters advance at the same single choke points that already update `Stats`
(outcome resolution and the economy actions) — do not scatter increments.
`CargoValueSold` is credited to the system the cargo was **mined** in, not the
system it was sold in, so hauling Eridani ore home to Sol still counts toward
Eridani's qualification. A run closed by framework/05's disconnect autopilot
counts exactly as its outcome does for `Stats` — if the autopilot escaped, the
run survived. Qualifications must never punish a dropped connection.

**The beta ladder** (prices and thresholds are a balance-pass starting point,
tuned against the invariants below — same status as gameplay/05's ship table):

| Class | Opens | Certified at | Credits | Qualification |
| --- | --- | --- | --- | --- |
| **E** | ERIDANI DRIFT | any SOL dock | 25,000 | Sol: 40,000cr of cargo sold · at least 1 run survived at CERES |
| **D** | KEPLER REACH | any ERIDANI dock | 120,000 | Eridani: 3 runs survived at SABLE HALO · 2 pirates destroyed |
| **C** | REDLINE EXPANSE | any KEPLER dock | 450,000 | Kepler: 150,000cr of cargo sold · 1 Legendary mined · 5 pirates destroyed |

Class E's price is deliberately the retired Jump Drive's exact price, so the
early-game credit curve is unchanged and the first wall lands where players
already expect it. Its CERES clause exists so a pilot cannot reach the drift
without having voluntarily flown Sol's high-pirate belt once.

### 3. The network

Four systems, each with a distinct **pressure**, not merely bigger numbers. A
system whose only difference is a higher multiplier is a reskin.

| System | Rating | Pressure — the verb the system teaches | Status |
| --- | --- | --- | --- |
| **SOL** | none | **Learn.** Safe, thin, always recoverable. The place a broke pilot can always climb out of. | shipped |
| **ERIDANI DRIFT** | E | **Fight.** Everything attacks; tribute is rarely offered. Weapons and the combat scope stop being optional. | shipped |
| **KEPLER REACH** | D | **Plan.** Enormous distances, few docks, punishing travel fuel, rich returns. The system where fuel capacity, mass and the round trip are the puzzle — and where a badly-planned departure strands you. | **new** |
| **REDLINE EXPANSE** | C | **Everything, faster.** Unstable belts that degrade while you drill, short pirate ETAs, a hard clock on every run. Extremely profitable, routinely lethal. | **new** |

Kepler is specified as a *logistics* system on purpose: Eridani already owns
combat pressure, and stacking a second combat system would make the map read
as one difficulty slider. Kepler's destinations should carry high
`travel_fuel`, high `required_fuel_capacity`, moderate `pirate_mul`, and rich
`rarity_bias` — a system that kills you by arithmetic rather than by gunfire.
Redline then combines both and adds the clock.

Links stay explicit and bidirectional in `worlds.toml`, and the existing
`systemsLinked` source-side check in `RouteLockReason` is kept: the chain is
`sol ↔ eridani ↔ kepler ↔ redline`. **A rating gates the gate, not the
system** — leaving Kepler back toward Eridani requires Class D exactly as
entering it did, so a rating can never be side-stepped by direction.

### 4. What a jump costs

A jump is resolved separately from ordinary destination travel and charges
**in addition to** the destination's `travel_fuel`.

**A jump is its own action, dock to dock.** Today `SystemID` is assigned when
the pilot arrives at a *belt* (`internal/sim/belt.go`), so changing system and
flying to a destination are one indivisible act. That is the deepest reason a
jump reads as a flight, and it must be split:

- `Jump(state, content, destSystemID)` — **dock → dock**, docked-only. Charges
  jump fuel, applies transit stress and the drift roll, and lands the pilot
  docked in the destination system where they can refuel and repair before
  committing to anything.
- `Depart` is unchanged and becomes **strictly intra-system**: it may only
  target a destination whose `system_id` equals `State.SystemID`. Selecting an
  out-of-system destination on the chart reports
  `NOT IN SYSTEM — JUMP TO KEPLER REACH FIRST`.

Splitting them is what makes arrival a moment rather than a loading step, and
it is also what makes the free-Skiff floor sufficient: a stranded pilot only
ever needs enough fuel to cross **one gate**, never a gate plus a destination.

**Fuel, scaled by mass:**

```
JumpFuel(ship, gate) = gate.jump_fuel_base * (totalMass / jump_reference_mass)^jump_mass_exponent
```

`totalMass` is gameplay/05's Base Mass + installed device mass — the *same*
number the Shipyard already displays, no parallel stat. With
`jump_reference_mass = 220` (the WARDEN's base mass, a mid-fleet anchor) and
`jump_mass_exponent = 0.8`, a lightly-fitted SKIFF pays ~0.62× the gate base
and a fully-loaded MULE ~1.74× — roughly a 2.8× spread, sublinear so the
freighter is expensive rather than impossible.

This is where the mass axis does its work. A MULE can be jumped, but the fuel
bill makes "which system does my freighter live in?" a standing decision, and
stripping heavy devices before a crossing becomes a real, sensible ritual the
Shipyard's existing mass readout already supports.

**Transit stress — deterministic, always:** every jump costs
`gate.hull_stress_pct` of max hull. Predictable, visible in the confirm, and
it means every crossing has a repair bill waiting on the far side (where
`port.drydock_surcharge_mul` already makes repairs pricier).

**The drift roll — probabilistic:**

```
driftChance = gate.drift_chance * (1 - jump.scanner_drift_reduction * scannerGrade)
```

Scanner grade reduces it. This gives the Scanner track a second job and makes
the **CICADA** — the survey specialist that can uniquely reach Scanner S —
the natural pathfinder for opening a new system, which is exactly the fantasy
its stat line promises and currently under-delivers on. On a failed roll, one
outcome from a weighted table:

| Outcome | Effect |
| --- | --- |
| **HARD TRANSLATION** | additional 10–20% max hull |
| **FUEL BLOOM** | lose 25–40% of fuel remaining after the jump |
| **HOT ARRIVAL** | the next run launched in this system starts with the pirate approach already underway at a reduced ETA |
| **MISALIGNMENT** | thrown back to the origin dock having paid the full fuel cost and stress (deepest gates only — see Open decisions) |

**Two hard safety rules.**

1. **A jump can never kill.** Transit stress and drift damage floor hull at 1.
   Death in this game is always the consequence of a decision the player made
   at the belt; an unavoidable death during a transition the player cannot
   influence would break that contract.
2. **The confirm must state the projected outcome.** The pre-jump overlay
   shows post-jump fuel and hull as numbers, and warns explicitly when the
   jump would leave the ship below `fleet.event_hull_gate_pct` (20%) — the
   threshold under which `tickEvents` starts rolling random events at all. A
   pilot must never discover after arriving that they crossed into the event
   zone.

### 5. The jump sequence (TUI)

A full-screen takeover that deliberately rhymes with `renderDeath` in blue.
The game's two biggest state changes both seize the screen; one is a failure,
one is an achievement.

- **Confirm overlay first** (reuse the `internal/tui/permit.go` pattern):
  gate name, rating check, jump fuel, destination travel fuel, transit stress,
  drift probability, projected hull/fuel, and any low-hull warning.
- **Countdown from 3**, one beat per second, each beat carrying a diegetic
  line rather than a bare numeral — the sequence should read as a procedure,
  not a wait:
  ```
  3   ALIGNMENT LOCKED — <GATE NAME>
  2   CHARGING ——————————— 68%
  1   THRESHOLD
      [ blue flash ]  ARRIVAL — KEPLER REACH
  ```
- **Skippable.** Any key skips to arrival, and `Settings.ReducedMotion` hard-
  skips the whole sequence. This is not optional polish: a three-second
  unskippable animation on a route flown forty times is the single most
  reliable way to turn a loved effect into a hated one.
- **Arrival differs from departure.** The arrival card prints the destination
  system's own signature line, and hostile systems arrive on a warning tone —
  first arrival in a system additionally prints a one-shot introduction naming
  its pressure.
- **The drift roll resolves in the cinematic.** On a bad translation the final
  beat renders red instead of blue and the arrival card names the failure
  (`HARD TRANSLATION — HULL 42%`). This is what makes the sequence
  load-bearing rather than decorative: it is the only place in the game where
  the countdown itself carries suspense.

### 6. Ships are property, and the hauler ferry

`ShipInstance` gains `SystemID string` — the dock the hull is parked at.
Invariant: `ActiveShipID` must always name a ship whose `SystemID` equals
`State.SystemID`. Jumping moves the active hull only; every other ship stays
where it was left.

- **Switching the active ship** is docked-only *and* location-scoped: the
  Shipyard's HANGAR panel lists local hulls as selectable and remote hulls
  greyed with `AT ERIDANI DRIFT`, never hidden — you must be able to see your
  fleet from anywhere.
- **Ship purchase is location-scoped.** `ShipModel` gains
  `sold_in = ["sol", ...]`. The starter four stay available in Sol and
  Eridani; frontier hulls (§ 7) sell only in their home system. A purchased
  hull materialises at the dock you bought it from.
- **Buyback** is offered at the system where the hull was destroyed, or at any
  dock selling that model.
- **The free-Skiff floor spawns locally.** A pilot with no hull at their
  current dock is always offered the Skiff at buyback price 0, at that dock.
  This is what makes located ships safe: stranding is impossible by
  construction, exactly as gameplay/05 intended, and no new safety net is
  needed.

**The hauler ferry** (`FerryShip(state, content, modelID, destSystemID)`)
relocates a non-active hull for credits, instantly, so the model never
degrades into repositioning chores:

```
FerryPrice = round(ferry_markup * port.refuel_per_point * sum(JumpFuel(ship, gate) for gate in path))
```

With `ferry_markup = 3.0`, flying a hull yourself is roughly three times
cheaper than shipping it — the ferry is a convenience you pay through the
nose for, never a trap and never the optimal play.

Three rules, all exploit-closing:

1. **The ferry requires the pilot's rating for every gate on the path.**
   Without this the ferry launders hulls past ratings the pilot has not
   earned.
2. **The destination must be a system the pilot has personally visited**
   (`Frontier[sys].Visited`). First arrival stays a feat that must be flown.
3. **Docked-only, and never the active ship.** Ferrying the hull you are
   standing on is rejected before any credits move.

### 7. Frontier hulls

Three new models. The design rule is **break a rule, don't raise a number** —
a frontier hull that is strictly better than the WARDEN makes the WARDEN dead
content, and gameplay/05's four-way tradeoff shape is load-bearing.

| Ship | Class | Home | Price | Slots (U/W/I) | Base Mass | The rule it breaks | What it gives up |
| --- | --- | --- | --- | --- | --- | --- | --- |
| **LANTERN** | Miner | Kepler | 140,000 | 3/0/**2** | 150 | **Two Internal slots** — the only hull in the game that can run two internal modules (Fuel Miner *and* Seismic Sensors, or Jammer *and* Heat Sink). Scanner caps at A. | No weapon slot at all, and a Hull cap of D — it solves Kepler's logistics puzzle and dies instantly to anything that catches it. |
| **HALBERD** | Fighter | Kepler | 210,000 | 2/2/1 | 300 | **Power Generator caps at S** — the only hull that can run an S Shield *and* a real weapon at once, which gameplay/05 explicitly denies the MULE. | Worst Fuel Efficiency cap in the game (E→D) and heavy — brutal to jump, brutal to run in the system it is sold in. |
| **VESPER** | Freighter | Redline | 600,000 | 4/2/1 | 340 | **Repairs itself between runs** (a fixed % of max hull on every docked arrival, no credits) and carries freighter cargo on a fighter's weapon count. | **No buyback, ever.** If a VESPER dies it is gone until you pay full price again. It cannot mount a Shield. |

VESPER's no-buyback rule is the point of the endgame hull: the game's whole
thesis is "ships are lives," and the capstone is the one ship where that
sentence has no discount attached. It should be genuinely frightening to fly,
and flying it anyway should be the flex.

Prices sit deliberately above the Class C certification (450,000) so that
reaching Redline and *equipping for* Redline are two separate mountains.

### 8. Lock-reason vocabulary

`RouteLockReason` is rewritten to speak in ratings. It must always name the
next concrete action, never merely state the failure:

```
LOCKED — NEED JUMP RATING CLASS D
LOCKED — CERTIFY CLASS D AT AN ERIDANI DOCK
LOCKED — JUMP FUEL 96 · TANK 74
LOCKED — NO CHARTED ROUTE BEYOND REDLINE EXPANSE
LOCKED — SHIP IS AT ERIDANI DRIFT
LOCKED — NEED FIGHTER-CLASS SHIP
LOCKED — BUY NAV PERMIT 7000 cr
```

### 9. The certification overlay

`[J]` on the star chart opens **CERTIFICATION**, which renders the next
class's requirements as a live checklist:

```
  ◇ JUMP RATING — CLASS D                        ERIDANI DRIFT → KEPLER REACH

    ✓  CREDITS                             120,000 / 120,000
    ✓  RUNS SURVIVED — SABLE HALO                    3 / 3
    ✗  PIRATES DESTROYED — ERIDANI DRIFT             1 / 2

       [ CERTIFY ]  — 1 REQUIREMENT OUTSTANDING
```

This is the highest-leverage screen in the feature. It converts the wall from
"you cannot go there" into "here is precisely what to go do," and it is
structurally a quest board — which is the natural seam into the storyline work
(beta item 2). Spec it so a later quest system can render into the same
component.

## Balance data

```toml
[jump]
reference_mass = 220.0
mass_exponent = 0.8
scanner_drift_reduction = 0.10   # per Scanner grade; S = -50% drift chance
hull_floor = 1                   # a jump can never kill
countdown_seconds = 3

[ferry]
markup = 3.0

[[gates]]                        # one per link, both directions
from = "sol"
to = "eridani"
required_rating = 0              # Class E
jump_fuel_base = 55
hull_stress_pct = 0.04
drift_chance = 0.10

[[ratings]]
class = 0                        # E
name = "CLASS E"
opens = "eridani"
certify_in = "sol"
price = 25000
req_cargo_sold_system = "sol"
req_cargo_sold_value = 40000
req_runs_destination = "ceres_claims"
req_runs_count = 1
```

Gates for `eridani→kepler` (`jump_fuel_base = 85`, stress 0.07, drift 0.18)
and `kepler→redline` (`jump_fuel_base = 130`, stress 0.11, drift 0.28) follow
the same shape, as do ratings D and C.

## Balance invariants (encode in tests/01)

1. **No gate can strand any pilot.** An unfitted free SKIFF (base mass 100) on
   a full base tank crosses every gate in the game in either direction. Every
   ship's base capacity is `pilot.start_fuel` = 100, so this reduces to a hard
   ceiling on content: `jump_fuel_base * (100/220)^0.8 < 100`, i.e. no gate's
   base may exceed ~188. At the shipped bases (55 / 85 / 130) a bare Skiff
   pays 29.3 / 45.2 / 69.2 — comfortable at every gate, tightest at Redline.
   **This is the invariant that lets located ships be safe**; assert it as a
   property test over all gates so no future gate can quietly break it.
2. **A loaded freighter must be deliberately outfitted to emigrate.** Fuel
   capacity is `100 + 10*(grade+1)` per Extra Fuel Tank utility device. A MULE
   at ~440 total mass pays 226 fuel at the Redline gate, which two S-grade
   tanks (220 capacity) cannot cover and three (280) can — so reaching the
   endgame system in a freighter costs three of its four utility slots. That
   cost is **designed, not accidental**: it is the "strip the ship down to
   jump it" ritual, and it is the whole reason the Shipyard's existing mass
   readout becomes a planning tool. Assert the 2-tank/3-tank boundary directly
   so a balance pass cannot erase the decision by accident.
3. Flying a hull across a gate is strictly cheaper than ferrying it, at every
   mass and every gate.
4. No sequence of purchases leaves a pilot unable to act: from any dock, with
   0 credits and no local hull, the free Skiff plus the existing salvage
   advance funds one local run.
5. Certification qualifications are unreachable without entering the system
   they are scored in — property test over all rating definitions.
6. A jump never reduces hull below 1, at any gate, with any drift outcome.
7. Frontier hull prices exceed the certification price of the system they are
   sold in.

## Acceptance criteria

- [ ] Jump Rating survives ship death, cannot be sold, stored, or moved; no
  code path writes it outside `CertifyRating`.
- [ ] v7→v8 migration grants Class E to every pilot holding a Jump Drive or an
  Eridani permit, refunds removed devices, and stamps every `ShipInstance`
  with a `SystemID` — round-tripped in a store test against real v7 blobs.
- [ ] `SlotJumpDrive`, `ItemJumpDrive` and `ErrRouteKeyRequired` are absent
  from the codebase; the Shipyard renders no JUMP DRIVE row.
- [ ] A gate refuses in both directions without the rating (table test over
  every gate × every class).
- [ ] Jump fuel is charged on top of destination travel fuel and scales with
  installed device mass — a table test with a light and a heavy loadout on the
  same hull.
- [ ] Transit stress is deterministic; the drift roll is seeded from the sim
  RNG and replays identically (determinism test, per project convention 3).
- [ ] Hull floors at 1 across an exhaustive drift-outcome table.
- [ ] The confirm overlay's projected hull/fuel match the post-jump state
  exactly, and the sub-20% warning fires exactly at
  `fleet.event_hull_gate_pct`.
- [ ] The jump sequence is skippable by any key and fully bypassed under
  `ReducedMotion`; golden renders for departure, arrival, and drift-failure
  frames at 80×24 and 144×48.
- [ ] Remote hulls render greyed with their location and cannot be activated;
  ferry rejects unrated paths, unvisited destinations, and the active ship
  before credits change.
- [ ] A pilot with no local hull is always offered the free Skiff at their
  current dock.
- [ ] Frontier hulls are purchasable only in their home system; VESPER has no
  buyback entry at any dock.
- [ ] Invariants 1–7 pass against the shipped numbers.

## Open decisions (need product sign-off before implementation)

1. **`State.Inventory` is account-wide and free to re-equip from any hull.**
   Under located ships that is a leak: a device stored at Sol can be equipped
   in Redline, so modules teleport while hulls do not. Options: (a) scope
   inventory per system — consistent, more friction, more UI; (b) keep it
   global and justify it in fiction as courier freight covered by dock fees.
   **Recommendation: (b) for beta.** The inconsistency is invisible in play,
   and (a) taxes the loadout experimentation gameplay/05 is built around.
2. **A fourth brand for frontier hulls.** gameplay/05 defines three
   (Federation, Alliance, Independent). Frontier yards reading as a distinct
   builder would strengthen "these ships are unique," at the cost of one more
   flavor axis. **Recommendation: add one** — it is a flavor field, and the
   frontier ships otherwise inherit brand identities written for the starter
   fleet.
3. **MISALIGNMENT** (thrown back to origin having paid full cost) is the
   harshest outcome in the table. It is memorable and it makes the countdown
   genuinely tense; it is also the single most likely thing to read as unfair.
   **Recommendation: deepest gate only, low weight, and never on a pilot's
   first crossing of that gate.**
4. **Should Class E remain purchasable in Sol only?** Certifying at the near
   side of the gate is the rule; Sol has multiple docks and no reason to
   restrict further. Confirmed as written unless dock-level identity later
   matters.

## Out of scope / handoffs

- **Quests and storyline** (beta item 2) — the certification checklist is
  deliberately shaped as a quest-board component, but quest state, givers, and
  narrative beats belong to that doc. Do not build a general quest system here.
- **Dedicated pirate hunting** (beta item 3) — this doc consumes
  `PiratesDestroyed` as a qualification input and adds a per-system counter;
  it does not change combat.
- **Engineered modifications and ship naming** (beta item 4) — the freed Jump
  Drive slot is deleted here, not repurposed. If engineering wants a
  per-hull slot, it should claim it deliberately rather than inherit a vacancy.
- **Stations and cosmetics** remain unbuilt (see `../README.md` § "Designed
  but not built"). Kepler's "late-game station value" promise in
  `../01-concept-and-story.md` is *not* cashed in by this doc.
- Star chart layout and region allocation → tui/06's frame contract; the jump
  sequence is a full-frame takeover and must honour it.
