# TUI 05 — Standalone Shipyard Screen

**Area:** TUI · **Phase:** 5 · **Depends on:**
[../gameplay/05-fleet-ships-and-shipyard-economy.md](../gameplay/05-fleet-ships-and-shipyard-economy.md)
(owns every action this screen calls) · **Parallel-safe with:** nothing else
in phase 5 — this is the only TUI task this phase

> **v1.7 layout supersession:** [06-responsive-menu-overhaul.md](06-responsive-menu-overhaul.md)
> owns frame sizing and exact panel allocation: HANGAR/LOADOUT/STATUS minima
> 24/32/24 with 3/4/3 flex. STATUS is informational, not a focus pane.

## Goal

The shipped Shipyard is a standalone full screen: a destination the pilot
navigates *to* and *from*, styled as a cyber-futuristic dealership rather
than a price list. v1.7 preserves that behavior while replacing its fixed
20-column HANGAR and 22-column STATUS rails with the responsive allocations
below. Its hangar, per-ship track grades, four slot kinds, and power/mass
meters must remain usable from the 80×24 minimum through the 144×48 cap.

## References

| Source | What to take |
| --- | --- |
| [../gameplay/05-fleet-ships-and-shipyard-economy.md](../gameplay/05-fleet-ships-and-shipyard-economy.md) | every value/action rendered here: ships, grades, slots, power, mass, buyback |
| `internal/tui/chart.go` (current `scrShipyard` branch) | the track-grade circle rendering (`●`/`○`) and button styling to carry forward |
| `internal/tui/theme/theme.go` (`Panel`, `Button`, `Dots`, `Option`/`OptionHC`, `Hue*`) | reuse, don't reinvent — the "cyber-futuristic" feel comes from a new **layout and color accent**, not new primitives |
| `internal/tui/hitbox` | mouse hit-testing, same pattern as every other screen |
| [../01-concept-and-story.md](../01-concept-and-story.md) "Controls" table | this doc updates the Star Chart row (`S` opens Shipyard, replacing the in-place `U`) |

## Deliverables

- `internal/tui/shipyard.go` — new `scrShipyard` full-screen render + input,
  replacing the `renderChart()` branch it currently shares
- `internal/tui/game.go` — `scrShipyard` becomes its own case in
  `renderScreen()`'s switch (drop it from the `scrChart` case); dedicated
  shipyard navigation state (selected pane, selected ship, selected slot)
- `internal/tui/theme/theme.go` — a `HueViolet`-anchored "console" accent set
  if the existing hue doesn't give enough contrast for four independent
  meters (power, mass, credits, per-track price) on one screen — reuse
  existing hues first, only add what's missing
- Updates `internal/tui/chart.go`'s Port Services panel: drop the inline
  upgrade-track list, keep a single `[S] SHIPYARD` button that now navigates
  to the standalone screen instead of toggling `g.scr`'s render branch in
  place

## Spec

### Entry/exit

- From the Star Chart, `S` (mouse: click `[S] SHIPYARD`) navigates to
  `scrShipyard`. `Esc`/`Q` returns to the Star Chart. Docked-only, same as
  today — the button doesn't render while in a belt or mid-run (it can't be,
  since the chart itself isn't reachable then).
- Like every other screen, `?` opens help and mouse works everywhere the
  keyboard does (global contract, `01-concept-and-story.md`).

### Layout

Three panels: a left **HANGAR** rail (owned ships + acquisition), a center
**LOADOUT** panel (the selected ship's track grades and four slot groups), and
a right **STATUS** rail (power meter, mass meter, credits, and acquisition
feedback). Outer-width minima are 24/32/24 and all extra cells are allocated
with weights 3/4/3 by tui/06's shared deterministic allocator. STATUS is
informational; keyboard focus exists only in HANGAR and LOADOUT. This is a
genuinely different layout from the chart's two-column world-list/services
split — that's the point of giving it its own screen.

```
┌◇ HANGAR ─────────────┐┌◇ WARDEN — ALLIANCE · FIGHTER ───────────┐┌◇ STATUS ─┐
│ ▸ WARDEN      ACTIVE ││ THRUSTERS  ●●●○○ C→B      ◈ 2,145 [BUY]││ PWR 21/34│
│   SKIFF              ││ HULL       ●●●○○ C        MAXED         ││ ███████░░│
│   CICADA             ││ FUEL EFF   ●●○○○ D        ◈ 1,780 [BUY]││          │
│   [ MULE — LOCKED ]  ││ POWER GEN  ●●○○○ D        ◈ 3,020 [BUY]││ MASS 268 │
│   ── BUY NEW SHIP ── ││ SCANNER    ●○○○○ E        ◈ 1,610 [BUY]││ ×1.22 fx │
│                       ││                                          ││          │
│                       ││ UTILITY 1  [ SHIELD   ●●○○○ C ]  ⚡14   ││ ◈ 18,400 │
│                       ││ UTILITY 2  [ empty            ]         ││          │
│                       ││ WEAPON  1  [ TURRET    ●●●○○ B ]  ⚡13   ││          │
│                       ││ WEAPON  2  [ empty            ]         ││          │
│                       ││ INTERNAL   [ FUEL MINER ●●○○○ C ]  ⚡5   ││          │
│                       ││ JUMP DRIVE [ empty            ]         ││          │
└───────────────────────┘└──────────────────────────────────────────┘└──────────┘
 ↑/↓ SELECT · TAB HANGAR/LOADOUT · ENTER BUY/EQUIP · X REMOVE · Q CHART ─ █
```

- **HANGAR rail**: every ship model is listed, with the current active ship
  marked `ACTIVE` and not-yet-owned models showing their price. Enter/Space
  on the selected unowned model purchases it directly. A
  destroyed-but-buyback-eligible model renders as
  `[ MULE — LOST · BUYBACK ◈ 6,500 ]` in red/amber instead of the normal
  greyed "not owned" treatment, distinguishing "never bought" from "died,
  rebuyable at a discount."
- **LOADOUT panel** title bar shows the selected ship's name/brand/class.
  Five track rows (grade dots sized to each model-specific track cap, current
  letter, price or `CAPPED`) followed by the four slot groups.
  Utility/Weapon row count matches that ship's slot counts (table in
  gameplay/05); a ship with zero weapon slots (any Miner) omits the WEAPON
  rows entirely rather than showing them disabled — Miners simply don't have
  that row.
- Every ship also has a single **JUMP DRIVE** row after **INTERNAL**. Its
  picker contains only Jump Drives, making route progression visible without
  forcing a choice against sensors, fuel mining, jamming, or a heat sink.
- Each installed slot row shows item name, its own grade dots, and its power
  draw (`⚡14`) except Extra Cargo/Extra Fuel Tank, which show a capacity
  value instead of a power glyph (they cost none — make that visually obvious,
  not just a "0" that looks like a bug).
- **STATUS rail**: power bar (`used/capacity` + a block gauge, reusing
  `theme.RenderBar`) that goes amber near capacity and won't let the next
  equip through if it would overflow (see below); a mass readout with the
  derived fuel/escape multiplier from gameplay/05's mass formula so the
  tradeoff is visible *before* buying, not discovered mid-run; total credits;
  and, when the selected ship is a buyback-eligible destroyed model, a
  prominent noninteractive `BUYBACK QUOTE — ◈ 6,500` readout (activation
  remains Enter/Space on its HANGAR row; you can't view/edit a loadout that
  doesn't exist yet).

### Navigation

- `Tab`/`Shift+Tab` (or click) moves focus between HANGAR and LOADOUT.
  STATUS never receives selection, keyboard focus, or action hitboxes.
  Acquisition happens from the selected HANGAR row; STATUS may show its
  noninteractive quote and service consequences.
- In HANGAR: `↑/↓` moves the ship selection; `Enter` on an owned, non-active
  ship switches the active ship (docked-only, instant, no cost); `Enter` on
  an unowned model buys it when affordable; `Enter` on a buyback-eligible row
  buys it back at 25% price. Purchases start fully serviced but do not switch
  the active ship.
- In LOADOUT: `↑/↓` selects a track or slot row; `Enter`/`→` on a track row
  buys the next grade (present-but-disabled at cap, rendered `MAXED`);
  `Enter` on a slot row opens an item picker overlay scoped to that slot kind
  (Utility/Weapon/Internal/Jump Drive): every catalog item with an adjustable grade
  cursor (`←/→` changes grade, showing that grade's price/power live), plus
  a separate "in storage" section listing any device previously removed via
  `[X] REMOVE`'s **Store** choice (free to re-equip — already paid for).
  Unaffordable or power-exceeding combinations render visibly disabled
  rather than hidden, so the player learns the power ceiling by seeing it,
   not by guessing. Choosing a different item for an occupied slot opens the
   same **REMOVE MODULE** confirmation first, so the installed device is
   explicitly stored or sold before the selected replacement is equipped.
   Direct install calls reject occupied slots as an additional safeguard.
- `[X] REMOVE`/`Backspace` on an occupied slot opens a small confirm overlay
  with two choices: **Store** (moves the device into the pilot's
  account-wide inventory, free to re-equip on any owned ship later via the
  item picker above — no refund, no loss either) or **Sell** (refunds 95% of
  the device's current buy price in credits, gone for good). This replaced
  the originally-specced "no refund, TBD" default once playtesting the v1
  cycle-on-Enter behavior showed no-refund removal felt punitive for simple
  mind-changes; sim implementation is `sim.StoreSlotDevice`/
  `sim.SellSlotDevice`/`sim.SlotItemSellValue` and the inventory pool is
  `State.Inventory` (`internal/sim/slots.go`, `internal/sim/state.go`).
- Mouse: click any row to select it (same as list rows elsewhere), click a
  selected row or double-click to activate/buy, same convention as every
  other screen.

### Power/mass feedback

- The power bar must visibly distinguish three states: plenty of headroom
  (dim/cyan), tight (`<15%` headroom, amber), and full (red, further installs
  blocked). This is the screen's primary "cyber-futuristic" texture beat —
  lean on a block-gauge readout plus a small `⚡` glyph rather than prose
  warnings.
- Attempting to equip a device that would exceed capacity shows an inline
  rejection (`INSUFFICIENT POWER — 34/34 USED`) instead of silently failing;
  it must not deduct credits.

### Visual direction ("fancy cyber-futuristic store")

Within the existing phosphor-blue theme (no new palette system — see
`tui/04-theme-components-and-tweaks.md`), lean into the shipyard being the
one screen that feels like a showroom rather than an instrument panel:

- Violet (`HueViolet`, already reserved for the shipyard accent in
  `chartAccent()`) as the dominant accent across all three panels, not just
  the loadout list.
- Ship names rendered large relative to everything else on the screen (the
  panel title bar), reinforcing "you are choosing a vehicle," not "you are
  filling out a form."
- Grade dots keep the exact `●●●○○` glyph and coloring already shipped in
  `chart.go` — do not invent a new grade indicator.
- The power gauge is the one place a slightly more "technical readout" feel
  (monospaced numeric fraction + gauge, `⚡` glyphs on power-drawing rows) is
  warranted; keep it inside the existing box-drawing/ASCII-only fallback
  constraints (`Settings.ASCIISafe` must still substitute the `⚡`/`●`/`○`
  glyphs, same pattern as `theme.Glyph`).

### Fits within global constraints

- At 80×24 the three outer panel widths are exactly 24/32/24. They grow by
  weights 3/4/3 to exactly 43/58/43 at the 144-column frame cap. Extra height
  grows scroll viewports; LOADOUT and HANGAR keep their selected rows visible.
  Larger frames add breathing room and reflow, never information unavailable
  at 80×24.
- Reduced-motion/high-contrast/ASCII-safe tweaks (tui/04) apply exactly as on
  every other screen — no shipyard-specific opt-out.

## Acceptance criteria

- [ ] `scrShipyard` renders through its own function with its own layout —
  `renderScreen()`'s switch no longer groups it with `scrChart`.
- [ ] Hangar list correctly distinguishes active / owned-inactive /
  never-owned / destroyed-buyback-eligible for all 4 starter ships.
- [ ] Switching active ship, buying a new ship, buying back a destroyed ship,
  buying a track grade, and equipping/removing a slot device all round-trip
  through `game.Session` exactly like every other chart action (no direct
  state mutation from the TUI layer).
- [ ] A ship with zero weapon slots renders no WEAPON rows at all (verified
  for both Miners); a ship with one weapon slot renders exactly one.
- [ ] Power bar blocks an over-capacity equip attempt before any credit
  deduction, with a visible inline rejection message.
- [ ] Golden-render test at 80×24 and one larger size (mirrors
  `tests/03-tui-rendering-and-input-tests.md`'s existing pattern) for: empty
  hangar (just the Skiff), a fully-loaded ship, and the buyback prompt state.
- [ ] Mouse click/double-click parity with keyboard for every action listed
  under Navigation.
- [ ] Exact 24/32/24 minima and 3/4/3 weighted flex pass tui/06's boundary
  matrix; STATUS cannot become the active keyboard pane.
- [ ] `01-concept-and-story.md`'s Controls table and `chart.go`'s keybar hint
  both updated to `S` (not `U`) for Shipyard.

## Out of scope / handoffs

- All pricing, grade effects, power/mass formulas, and the hangar/buyback
  rules themselves → gameplay/05 (this doc only renders and invokes them).
- Cosmetics/station panels mentioned in the older tui/02 doc are unaffected
  and stay wherever tui/02 eventually places them; this doc only removes
  tui/02's in-place shipyard sub-view description (see README/tui-02 update).
