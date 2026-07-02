# TUI 03 — Asteroid Belt Views & Live Mining Screen

**Area:** TUI · **Phase:** 3 · **Depends on:** tui/01, tui/04, gameplay/01,
gameplay/02 · **Parallel-safe with:** tui/02, framework/03

## Goal

The game's centerpiece: the **Asteroid Belt** screen with its three
switchable renderings (data tiles / ore-scan blobs / radar scope) and target
lock panel, and the real-time **Mining** screen with its three live gauges
and the overdrive/bail controls. If the game is remembered for one screen,
it's these.

## References

| Source | What to take |
| --- | --- |
| `../moon-miner-materials/Web-Prototype/Moon Miner.dc.html` (belt + mining markup, lines ~130–295; blob art in `genBelt`) | three view designs, blob glyphs, gauge layout |
| `../moon-miner-materials/Web-Prototype/Moon Miner - Design Notes.md` (Screens 2–3) | design intent per view |
| gameplay/01 `Asteroid` fields (incl. `X,Y`, `Size`) · gameplay/02 run state | everything rendered here |

## Deliverables

- `internal/tui/` — `belt.go`, `beltviews.go` (the three renderers),
  `mining.go` (+ tests)

## Spec

### Belt screen layout

- **Left column (~30 cols):** current world panel — ASCII art
  (sphere/ringed), name/sub, ring/pirate/rarity labels, contacts count.
  Below it, belt stats (rocks remaining, best value visible, fuel reserve).
- **Right column:** the contact field, one of **three views**, cycled with
  `V` (persisted default comes from Settings, tui/04):

  1. **DATA TILES** — scannable rows: `▸ KR-4711  ◆ UNCOMMON  VOL 2,340
     ⏱ 7.8s  ◈ 6,325  -12%  ●●●○○`. Columns: designation, tier glyph+label
     (tier-colored), volume, drill time, value, fuel cost (amber, negative),
     threat dots (blue→amber→red by count). Selected row highlighted.
  2. **ORE SCAN** — each rock drawn as its `Size` blob (sm/md/lg
     block-character shapes from the prototype: `▟▓▙ / ▜▓▛` etc.) placed at
     its `X,Y` percent coords scaled into the panel, tier-colored, selected
     blob framed by a marker + designation label. Overlapping placements:
     nudge to nearest free cell.
  3. **RADAR SCOPE** — a circle of `·` gridmarks with rocks as
     tier-colored `◉` blips at bearing derived from `X,Y`; selected blip
     gets a `┼` crosshair + callout line with designation. A sweep line
     rotating one step per UI tick (1 Hz is fine; it's ambience, not data).

  All three views select from the **same ordered list** (sorted by fuel
  cost, per gameplay/01): ↑/↓/←/→ cycle selection; click a row/blob/blip to
  select; click again or Enter/Space to lock. Wheel cycles selection in any
  view.
- **Target Lock panel (bottom strip, always visible):** full detail of the
  selected rock — designation, tier, volume, drill time, value, flight fuel
  cost vs current fuel (red if insufficient), risk with dots — plus
  `[ENTER] LOCK & FLY  [V] VIEW  [R] RESCAN  [Q] DOCK`.
- `R` rescan regenerates the belt (confirm nothing — it's free, per
  gameplay/01); empty belt (all rocks mined) auto-prompts rescan in the
  field area.
- Lock failure (fuel) → red flash, stay on belt.

### Mining screen (real time)

The actor pushes snapshots at 8 Hz during a run (gameplay/02); this screen
just renders the latest.

```
│ ◇ DRILLING — KR-4711 (◆ UNCOMMON)                        │
│                                                          │
│  ⛏ DRILL PROGRESS   ██████████████░░░░░░░░░░░  47%       │
│  ⛽ FUEL RESERVE     ████████████████░░░░░░░░  61% -1.7%/s│
│  ☠ PIRATE PROXIMITY ████████░░░░░░░░░░░░░░░░  33%        │
│                                                          │
│  CARGO VALUE ACCRUED   ◈ 2,975  of ◈ 6,325               │
│                                                          │
│  [SPACE] OVERDRIVE (hold)      [B] BAIL & BANK CARGO     │
│  Overdrive drills faster but burns fuel. Reach 100%      │
│  for full value — or bail before raiders arrive.         │
```

- Three gauges via tui/04 `Bar`, ramped: drill always bright blue; fuel
  bright→amber(<40)→red(<20); pirate blue→amber(≥50)→red(≥75). Show current
  drain/climb rates beside fuel and pirate bars (e.g. `-1.7%/s`), doubled
  display under overdrive.
- **Overdrive**: terminals don't send key-up, so "hold Space" is emulated —
  each Space press (auto-repeat included) sets overdrive on and arms a
  ~250 ms expiry; no repeat within the window ⇒ off. Send
  `SetOverdrive(true/false)` to the actor only on transitions. The **mouse
  button held on the OVERDRIVE control** is a true hold (press msg on,
  release msg off) — mention this in the help overlay as the pro move.
  While overdrive is on, the drill bar pulses (alternate bright/white on
  ticks) and an `▮▮ OVERDRIVE` badge shows in amber.
- `B` / `Esc` / click `[B] BAIL` → `Bail`; outcome switches to the Summary
  screen (tui/02). All four outcomes arrive the same way: the actor
  resolves, the screen transitions on the resolved-run message.
- No other input works during mining — it's the one screen where you can't
  leave except through an outcome (that's the point). `Ctrl+C` still
  disconnects (auto-bail protects the cargo, framework/03).

### Tension polish (cheap, worth it)

- Pirate bar ≥ 75%: prefix the panel title with `☠ CONTACT IMMINENT` in
  red, blink on alternate snapshots.
- Fuel < 15%: keybar hint swaps to `▼ FUEL CRITICAL — BAIL NOW?` in red.
- These are render-side only; no sim involvement.

## Acceptance criteria

- [ ] All three belt views render the same 7-rock belt inside their panel
  at 80×24 (goldens); selection stays consistent when cycling views.
- [ ] Ore-scan collision nudging never drops a rock or paints outside the
  panel (property test with adversarial X/Y).
- [ ] Radar bearing math maps X,Y percent coords inside the circle.
- [ ] Mining gauges match snapshot values (unit test with crafted
  snapshots at thresholds 0/19/39/49/74/100 checks color roles).
- [ ] Overdrive keyboard emulation: synthetic repeat sequence holds it on;
  gap turns it off; exactly 2 sim calls (on, off).
- [ ] Mouse: click-select vs click-activate on rocks, wheel cycling, hold-
  to-overdrive, bail click — hitbox tests all.
- [ ] Screen transitions: lock → mining, outcome → summary, dock → chart.

## Out of scope / handoffs

- Summary rendering → tui/02.
- Bar/panel primitives, colors → tui/04.
- Snapshot push plumbing → framework/03; tick math → gameplay/02.
