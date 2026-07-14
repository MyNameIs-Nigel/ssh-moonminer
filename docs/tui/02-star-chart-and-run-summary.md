# TUI 02 — Star Chart, Run Summary & Ship's Log Screens

**Area:** TUI · **Phase:** 3 · **Depends on:** tui/01, tui/04, gameplay/01
(types) · **Parallel-safe with:** tui/03, framework/03

> **Superseded in part:** the Shipyard is no longer a Star Chart sub-view/
> overlay. It is now its own screen — see
> [05-shipyard-screen.md](05-shipyard-screen.md) and
> [../gameplay/05-fleet-ships-and-shipyard-economy.md](../gameplay/05-fleet-ships-and-shipyard-economy.md).
> Wherever this doc below describes the Shipyard as a sub-view/overlay
> (the `[S] SHIPYARD` sketch row and the "Shipyard (sub-view or overlay)"
> paragraph), tui/05 wins. The Star Chart still owns the `[S] SHIPYARD`
> button that *navigates* there, plus everything else on this page
> (port services, cosmetics, stations, run summary, death, ship's log).

## Goal

The bookend screens: the **Star Chart** (system/destination selection + port
services + shipyard + cargo/station actions), the **Run Summary** (ship's-log
entry for the run that just ended), the **Death Recap** after the dark red
connection-loss screen, and the **Ship's Log** (lifetime stats + recent runs).
These are the screens where the player spends, plans, sells cargo, rebuilds
after death, and reflects.

## References

| Source | What to take |
| --- | --- |
| `../moon-miner-materials/Web-Prototype/Moon Miner.dc.html` (chart + summary markup, lines ~60–130 & ~295–320) | layout intent and service-pricing display only |
| [../01-concept-and-story.md](../01-concept-and-story.md) | screen list, controls, locked systems, death |
| [../02-danger-economy-and-progression.md](../02-danger-economy-and-progression.md) | station/cosmetic/death narrative |
| gameplay/01 `Depart`, gameplay/03 economy actions, gameplay/04 ships/upgrades/cosmetics/log | the actions these screens invoke |

## Deliverables

- `internal/tui/` — `chart.go`, `summary.go`, `death.go`, `shiplog.go`,
  `cosmetics.go`, `station.go` (+ tests). `shipyard.go` moved to
  [05-shipyard-screen.md](05-shipyard-screen.md).

## Spec

### Star Chart (80×24 sketch)

```
┌ MOON MINER ─────────────────── PILOT: DEFAULT ── ◈ 4,820 ┐   ← shell HUD
│ ◇ STAR CHART                      │ ◇ PORT / STATION      │
│ ◆ SOL  permit owned               │ SKIFF HULL █████ 100% │
│ ▸ VESTA LOCAL       ⛽ 6           │ FUEL  ███████░ 45/60  │
│   CERES CLAIMS      LOCK TANK I   │ CARGO ██░░░░░ 12/40   │
│   IO SHADOW         LOCK ◈25,000  │ [F] REFUEL ◈135       │
│ ◇ ERIDANI DRIFT     LOCK CUTTER   │ [H] REPAIR FULL       │
│ KEPLER REACH        LOCK HAULER   │ [S] SHIPYARD          │
│                                   │ [C] COSMETICS         │
│  "Thin, legal, and picked over.   │ [B] BUILD STATION     │
│   Good enough to buy a tank."     │ [L] SHIP'S LOG        │
└ ↑↓ SELECT · ENTER DEPART · S SHIPYARD · C COSMETICS · Q ─ █ ┘
```

- Left panel: grouped system/destination rows. Systems can be selected to show
  permit/transfer details; destinations can be selected to depart.
  - The ship's current system uses a solid diamond (`◆`) and brighter,
    higher-contrast styling for its header and every destination beneath it.
    Other systems retain the hollow diamond (`◇`) and normal dim styling.
  - Unlocked rows show travel fuel, risk dots, rarity label, and fuel
    affordability color.
  - Locked rows are dimmed but selectable; the detail panel must show the exact
    lock reason from gameplay/01 (`NEED FUEL TANK II`, `BUY TRANSFER ◈75,000`,
    `NEED CUTTER`, etc.).
  - Selected row: `▸` marker + bright text + destination/system ASCII art if
    height allows; flavor `desc` always shows.
- Right panel: port/station services — current ship, hull/fuel/cargo bars,
  refuel and repair prices, sell cargo button(s), salvage advance row only when
  eligible, shipyard/cosmetics/station/log navigation.
  - If the current system has a completed owned station, label the panel
    `◇ YOUR STATION` and show refuel discount + sale bonus.
  - If a station is building, show stage progress and `[B] FUND STATION`.
  - If no station exists and the system is eligible, show `[B] BEGIN STATION`
    dimmed until affordable.
- **Shipyard** (sub-view or overlay): ship classes, active ship marker, price,
  jump tier, tank, hold, hull, and flee profile. Also installed upgrade tracks
  for the active ship with level pips (`●●○○○`), effect description,
  next-level price; maxed tracks dimmed with `MAX`. Copy must make clear that
  ships and installed upgrades are lost on death.
- **Cosmetics** (sub-view or overlay): HUD theme/accent, border style, radar
  sweep, ship paint, ship name/nose art. Locked cosmetic rows show price;
  unlocked rows can be equipped. No stat changes displayed.
- Actions map to sim calls: `Depart` (switches to belt screen on success),
  `Refuel`, `Repair`, `SellCargo`, `BuySystemPermit`, `BuyShip`,
  `BuyUpgrade`, `BuyCosmetic`, `SetCosmetic`, `StartOrFundStation`,
  `ClaimStationIncome`, salvage advance. Typed errors → flash message in the
  keybar area (red/amber, ~1.8s expiry).
- Full input: ↑/↓ route selection, Enter depart/buy selected route action,
  F/H/S/C/B/L/T shortcuts; every row and button click/wheel-able per the
  tui/01 contract.

### Run Summary

Rendered when the actor reports a resolved run (gameplay/02's `RunRecord`):

- A single centered panel styled as a ship's-log entry:
  `◇ SHIP'S LOG — ENTRY 0047`, outcome label big and colored by outcome
  (green/amber/red/dark-red from the theme), the outcome description line, then
  a manifest table: ORE (asteroid name + tier glyph), CARGO RECOVERED, CARGO
  LOST, HULL DELTA, FUEL DELTA, EVENTS, CURRENT HOLD, and SELL VALUE ESTIMATE.
- `DEPARTED` and `BAILED` emphasize that cargo is **not sold yet**.
- `TRIBUTE PAID` shows jettisoned cargo in amber.
- `ESCAPED UNDER FIRE` shows hull loss in red.
- `SHIP LOST` should usually be preceded by the Death Screen below; the recap
  explains active ship, upgrades, and cargo lost.
- `[ENTER] RETURN TO BELT · [Q] DOCK AT PORT` — if the ship survived. If death
  occurred, `[ENTER] RESPAWN AT SOL DOCK`.

### Death screen

The immediate death presentation is intentionally stark and separate from the
recap:

```
┌────────────────────────────────────────────────────────────┐
│                                                            │
│                                                            │
│                 CONNECTION LOST █                         │
│                                                            │
│                                                            │
│                     press any key                         │
└────────────────────────────────────────────────────────────┘
```

- On the tick hull reaches 0, the last mining/escape HUD frame flickers for a
  handful of fast ticks — cycling monochrome and inverted renders of that
  frame to read as a dying CRT — before settling on the card above.
- The settled card fills the *entire* screen (not just a centered box) in
  deep red on near-black, `CONNECTION LOST` with a cursor block that blinks
  at the end of the line, and a dim `press any key` beneath it. No stats, no
  tips, no other buttons.
- Reduced motion skips the flicker entirely (cuts straight to the settled
  card) and holds the cursor solid instead of blinking.
- The next key transitions to the Death Recap / Run Summary.
- Tests should assert that no cargo/ship-loss details leak onto this screen.

### Ship's Log screen

- Panels for **SERVICE RECORD** (runs by outcome, ships lost, credits
  earned/spent, cargo sold/lost, systems unlocked, stations completed,
  cosmetics purchased, fuel burned, salvage advances, pilot since),
  **CURRENT ASSETS** (active ship, permits, stations, cosmetic theme), and
  **RECENT RUNS** (newest first: when-ago, system/destination, rock, tier
  glyph, outcome-colored label, cargo recovered/lost). Wheel/arrows scroll if
  > panel height. `Esc`/`Q` back to chart.

### Onboarding tie-in

For `Created` pilots (fresh save), the shell shows the onboarding overlay
(tui/01) over the chart. This task provides its 3 pages of copy: (1) the
fiction + goal, (2) the loop diagram in words, (3) the control cheatsheet.

## Acceptance criteria

- [ ] All chart/bookend screens render inside 80×24 without overflow at every
  reachable state (empty log, locked systems, active station, maxed upgrades,
  death recap, 7-digit credits) — golden tests via tests/03 patterns.
- [ ] Unaffordable states render dimmed and reject activation with a flash
  (unit test with a broke-pilot snapshot).
- [ ] Keyboard and mouse reach every action (hitbox tests: each button/row
  registers exactly one box).
- [ ] Salvage advance row appears/disappears per eligibility snapshot.
- [ ] Locked destination rows show exact lock reasons.
- [ ] Summary renders departed, bailed, tribute paid, escaped under fire, and
  ship lost variants correctly (table-driven).
- [ ] Death screen renders only `CONNECTION LOST` plus minimal continue affordance.

## Out of scope / handoffs

- Belt + mining screens → tui/03.
- Bar/panel/flash primitives → tui/04.
- The sim actions themselves → gameplay (call through the session handle).
