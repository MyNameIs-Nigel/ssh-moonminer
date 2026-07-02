# TUI 02 — Star Chart, Run Summary & Ship's Log Screens

**Area:** TUI · **Phase:** 3 · **Depends on:** tui/01, tui/04, gameplay/01
(types) · **Parallel-safe with:** tui/03, framework/03

## Goal

The bookend screens: the **Star Chart** (world selection + port services +
upgrade shop), the **Run Summary** (ship's-log entry for the run that just
ended), and the **Ship's Log** (lifetime stats + recent runs). These are the
screens where the player spends, plans, and reflects.

## References

| Source | What to take |
| --- | --- |
| `../moon-miner-materials/Web-Prototype/Moon Miner.dc.html` (chart + summary markup, lines ~60–130 & ~295–320) | layout intent, copy, service pricing display |
| [../01-concept-and-story.md](../01-concept-and-story.md) | screen list, controls, divergences (travel fuel, insurance, upgrades) |
| gameplay/01 `Depart`, gameplay/03 economy actions, gameplay/04 upgrades/log | the actions these screens invoke |

## Deliverables

- `internal/tui/` — `chart.go`, `summary.go`, `shiplog.go` (+ tests)

## Spec

### Star Chart (80×24 sketch)

```
┌ MOON MINER ─────────────────── PILOT: DEFAULT ── ◈ 4,820 ┐   ← shell HUD
│ ◇ STAR CHART                      │ ◇ PORT SERVICES       │
│                                   │ FUEL ████████████ 78% │
│ ▸ ◐ CERES   INNER BELT   ⛽ 8     │ [F] REFUEL TO 100%    │
│     RING DENSE · PIRATES HIGH     │     ◈ 198             │
│     RARITY RICH                   │ HULL ████████████ 100%│
│   ◓ VESTA   MID BELT     ⛽ 6     │ [H] REPAIR — FULL     │
│   ◍ TITAN   SATURN RINGS ⛽ 20    │ [U] SHIPYARD          │
│   ◑ IO      JOVIAN ORBIT ⛽ 14    │ [L] SHIP'S LOG        │
│                                   │ ◇ BRIEFING            │
│  "Crowded, lucrative, lawless.    │  Bigger rocks pay     │
│   Raiders everywhere."            │  more but drill slow… │
└ ↑↓ SELECT · ENTER DEPART · T TWEAKS · ? HELP · Q QUIT ── █ ┘
```

- Left panel: the four worlds as selectable rows — icon, name, sub, travel
  fuel (amber `⛽ n`, red if unaffordable), ring/pirate/rarity labels
  (rarity label colored: RICH `#7fa6c4`-ish per world `col`; keep simple —
  use theme roles). Selected row: `▸` marker + bright text + world's ASCII
  art (sphere/ringed from gameplay/01 content) rendered under or beside
  the list if height allows; the flavor `desc` always shows for the
  selection.
- Right panel: port services — fuel and hull bars (tui/04 `Bar`), refuel and
  repair buttons showing live prices (`◈ n` or `FULL`/dimmed when
  unaffordable), **INSURANCE ADVANCE** row only when eligible
  (gameplay/03), `[U] SHIPYARD` and `[L] SHIP'S LOG` navigation, and a
  static briefing blurb.
- **Shipyard** (sub-view or overlay of the chart, your call): five upgrade
  tracks with current level pips (`●●○○○`), effect description, next-level
  price; buy with Enter/click; maxed tracks dimmed with `MAX`.
- Actions map to sim calls: `Depart` (switches to belt screen on success),
  `Refuel`, `Repair`, `BuyUpgrade`, insurance claim. Typed errors → flash
  message in the keybar area (red/amber, ~1.8s expiry — mirror the
  prototype's `flash`).
- Full input: ↑/↓ world selection, Enter depart, F/H/U/L/T shortcuts; every
  row and button click/wheel-able per the tui/01 contract.

### Run Summary

Rendered when the actor reports a resolved run (gameplay/02's `RunRecord`):

- A single centered panel styled as a ship's-log entry:
  `◇ SHIP'S LOG — ENTRY 0047`, outcome label big and colored by outcome
  (gold/cyan/red/amber from the theme), the outcome description line, then
  a manifest table: ORE (asteroid name + tier glyph), CARGO BANKED
  (`+◈ n`), DRILL PROGRESS, FUEL REMAINING, HULL, NEW BALANCE.
- Raided runs show `HULL −25` in red; stranded shows `TOW FEE −40%` in
  amber.
- `[ENTER] RETURN TO BELT · [Q] DOCK AT PORT` — belt returns to tui/03's
  screen (rock already removed by the sim); if the belt is now empty, the
  belt screen offers rescan.

### Ship's Log screen

- Two stacked panels: **SERVICE RECORD** (lifetime stats grid from
  gameplay/04: runs by outcome with a mini bar chart made of block chars,
  credits earned/spent, legendaries, fuel burned, insurance claims, pilot
  since) and **RECENT RUNS** (the RunLog, newest first: when-ago, world,
  rock, tier glyph, outcome-colored label, banked). Wheel/arrows scroll if
  > panel height. `Esc`/`Q` back to chart.

### Onboarding tie-in

For `Created` pilots (fresh save), the shell shows the onboarding overlay
(tui/01) over the chart. This task provides its 3 pages of copy: (1) the
fiction + goal, (2) the loop diagram in words, (3) the control cheatsheet.

## Acceptance criteria

- [ ] All three screens render inside 80×24 without overflow at every
  reachable state (empty log, maxed upgrades, 6-digit credits) — golden
  tests via tests/03 patterns.
- [ ] Unaffordable states render dimmed and reject activation with a flash
  (unit test with a broke-pilot snapshot).
- [ ] Keyboard and mouse reach every action (hitbox tests: each button/row
  registers exactly one box).
- [ ] Insurance row appears/disappears per eligibility snapshot.
- [ ] Summary renders all four outcome variants correctly (table-driven).

## Out of scope / handoffs

- Belt + mining screens → tui/03.
- Bar/panel/flash primitives → tui/04.
- The sim actions themselves → gameplay (call through the session handle).
