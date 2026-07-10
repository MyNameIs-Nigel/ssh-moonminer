# TUI 05 — Standalone Shipyard Screen

**Area:** TUI · **Phase:** 5 · **Depends on:**
[../gameplay/05-fleet-ships-and-shipyard-economy.md](../gameplay/05-fleet-ships-and-shipyard-economy.md)
(owns every action this screen calls) · **Parallel-safe with:** nothing else
in phase 5 — this is the only TUI task this phase

## Goal

Today the Shipyard is a second column bolted onto the Star Chart panel
(`scrShipyard` renders through `renderChart()`, `internal/tui/chart.go:78,159`)
— five upgrade-track rows appended below the port-services buttons. Replace it
with its own full screen: a proper destination the pilot navigates *to* and
*from*, styled as a fancy cyber-futuristic dealership rather than a price
list. It has to show a lot more than five rows now — a hangar of owned ships,
per-ship track grades, three kinds of slots with power/mass meters, and a
buy-new-ship catalog — so it needs the full 80×24 (or larger) canvas the chart
screen currently keeps to itself.

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

Three panels: a left **HANGAR** rail (owned ships + a switch/buy toggle), a
center **LOADOUT** panel (the selected ship's track grades and three slot
groups), and a right **STATUS** rail (power meter, mass meter, credits,
buyback prompt if the selected ship is currently destroyed-and-buyable). This
is a genuinely different layout from the chart's two-column world-list/
services split — that's the point of giving it its own screen.

```
┌◇ HANGAR ─────────────┐┌◇ WARDEN — ALLIANCE · FIGHTER ───────────┐┌◇ STATUS ─┐
│ ▸ WARDEN      ACTIVE ││ THRUSTERS  ●●●○○ C→B      2,145 cr [BUY]││ PWR 21/34│
│   SKIFF              ││ HULL       ●●●○○ C        MAXED         ││ ███████░░│
│   CICADA             ││ FUEL EFF   ●●○○○ D        1,780 cr [BUY]││          │
│   [ MULE — LOCKED ]  ││ POWER GEN  ●●○○○ D        3,020 cr [BUY]││ MASS 268 │
│   ── BUY NEW SHIP ── ││ SCANNER    ●○○○○ E        1,610 cr [BUY]││ ×1.22 fx │
│                       ││                                          ││          │
│                       ││ UTILITY 1  [ SHIELD   ●●○○○ C ]  ⚡14   ││ 18,400 cr│
│                       ││ UTILITY 2  [ empty            ]         ││          │
│                       ││ WEAPON  1  [ TURRET    ●●●○○ B ]  ⚡13   ││          │
│                       ││ WEAPON  2  [ empty            ]         ││          │
│                       ││ INTERNAL   [ FUEL MINER ●●○○○ C ]  ⚡5   ││          │
└───────────────────────┘└──────────────────────────────────────────┘└──────────┘
 ↑/↓ SHIP · TAB PANE · ←/→ SELECT · ENTER BUY/EQUIP · B BUY SHIP · Q CHART ─ █
```

- **HANGAR rail**: owned ships listed first (current active ship marked
  `ACTIVE`), then not-yet-owned models greyed with their price, then a
  `BUY NEW SHIP` row that opens the purchase flow for a highlighted unowned
  model. A destroyed-but-buyback-eligible model renders as
  `[ MULE — LOST · BUYBACK 6,500 cr ]` in red/amber instead of the normal
  greyed "not owned" treatment, distinguishing "never bought" from "died,
  rebuyable at a discount."
- **LOADOUT panel** title bar shows the selected ship's name/brand/class.
  Five track rows (grade dots at fixed width 5, current letter, price or
  `MAXED` at cap) followed by the three slot groups. Utility/Weapon row count
  matches that ship's slot counts (table in gameplay/05); a ship with zero
  weapon slots (any Miner) omits the WEAPON rows entirely rather than showing
  them disabled — Miners simply don't have that row.
- Each installed slot row shows item name, its own grade dots, and its power
  draw (`⚡14`) except Extra Cargo/Extra Fuel Tank, which show a capacity
  value instead of a power glyph (they cost none — make that visually obvious,
  not just a "0" that looks like a bug).
- **STATUS rail**: power bar (`used/capacity` + a block gauge, reusing
  `theme.RenderBar`) that goes amber near capacity and won't let the next
  equip through if it would overflow (see below); a mass readout with the
  derived fuel/escape multiplier from gameplay/05's mass formula so the
  tradeoff is visible *before* buying, not discovered mid-run; total credits;
  and, when the selected ship is a buyback-eligible destroyed model, a single
  prominent `[B] BUY BACK — 6,500 cr` button in place of the normal track
  list (you can't view/edit a loadout that doesn't exist yet).

### Navigation

- `Tab`/`Shift+Tab` (or click) moves focus between the three panes.
- In HANGAR: `↑/↓` moves the ship selection; `Enter` on an owned, non-active
  ship switches the active ship (docked-only, instant, no cost); `Enter` on
  an unowned model or the `BUY NEW SHIP` row opens a confirm-purchase prompt
  showing price and a one-line stat-cap summary; `Enter` on a buyback-eligible
  row buys it back at 25% price and makes it active.
- In LOADOUT: `↑/↓` selects a track or slot row; `Enter`/`→` on a track row
  buys the next grade (present-but-disabled at cap, rendered `MAXED`);
  `Enter` on a slot row opens an item picker scoped to that slot kind
  (Utility/Weapon/Internal) showing every item at every grade the player can
  afford *and* power-fit, with unaffordable/power-exceeding combinations
  visibly disabled rather than hidden (so the player learns the power ceiling
  by seeing it, not by guessing). Internal is a single-select swap: choosing
  a new module uninstalls the current one (no refund — matches "installed
  upgrades are lost, not banked" tone) unless the current one is unequipped
  first via an explicit `[X] REMOVE` action, which does refund a fraction TBD
  by gameplay/05 balance passes (default: no refund, matching ship-track
  purchases being one-way; revisit only if playtesting shows this feels
  punitive for simple mind-changes).
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

- Must degrade to 80×24 without losing information — three panels at that
  width means each is ~26 columns; the mockup above already assumes that
  width. Larger terminals get more breathing room in the LOADOUT panel
  (e.g. showing both track price *and* effect-at-next-grade), never new
  information unavailable at 80×24.
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
- [ ] `01-concept-and-story.md`'s Controls table and `chart.go`'s keybar hint
  both updated to `S` (not `U`) for Shipyard.

## Out of scope / handoffs

- All pricing, grade effects, power/mass formulas, and the hangar/buyback
  rules themselves → gameplay/05 (this doc only renders and invokes them).
- Cosmetics/station panels mentioned in the older tui/02 doc are unaffected
  and stay wherever tui/02 eventually places them; this doc only removes
  tui/02's in-place shipyard sub-view description (see README/tui-02 update).
