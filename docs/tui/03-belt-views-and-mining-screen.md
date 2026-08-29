# TUI 03 — Asteroid Belt Views & Live Mining Screen

**Area:** TUI · **Phase:** 3 · **Depends on:** tui/01, tui/04, gameplay/01,
gameplay/02 · **Parallel-safe with:** tui/02, framework/03

> **v1.7 layout supersession:** [06-responsive-menu-overhaul.md](06-responsive-menu-overhaul.md)
> owns frame sizing and responsive allocation. Belt is a full-body
> single-region screen. Every drilling, tribute, escape, and combat state uses
> the same 46/34 minimum STATUS/TACTICAL split with 3/2 flex.

## Goal

The game's centerpiece: the **Asteroid Belt** screen with its three switchable
renderings (data tiles / ore-scan blobs / radar scope) and target lock panel,
and the real-time **Mining Site** screen with the asteroid being mined, rough
pirate ETA, radar distance, cargo load, event-distorted HUD, yellow BAIL /
green DEPART control, tribute prompts, and escape-under-fire state. If the game
is remembered for one screen, it's this failing cockpit.

## References

| Source | What to take |
| --- | --- |
| `../moon-miner-materials/Web-Prototype/Moon Miner.dc.html` (belt + mining markup, lines ~130–295; blob art in `genBelt`) | three view designs, blob glyphs, gauge layout only |
| `../moon-miner-materials/Web-Prototype/Moon Miner - Design Notes.md` (Screens 2–3) | design intent per view |
| gameplay/01 `Asteroid` fields (incl. `X,Y`, `Size`) · gameplay/02 run state | everything rendered here |
| [../02-danger-economy-and-progression.md](../02-danger-economy-and-progression.md) | event HUD treatments and death tone |

## Deliverables

- `internal/tui/` — `belt.go` (including the three view renderers),
  `mining.go`, and `combat.go` (+ tests)

## Spec

### Belt screen layout

The belt owns the full body as a single top-level region. Its internal world
details, contact view, and target-lock information may reflow with available
space, but do not use the Chart or Mining top-level split.

- World context: ASCII art (sphere/ringed), name/sub,
  ring/pirate/rarity labels, contacts count, rocks remaining, best visible
  value, and fuel reserve.
- Contact field: one of **three views**, cycled with `V` (persisted default
  comes from Settings, tui/04):

  1. **DATA TILES** — scannable rows: `▸ KR-4711  ◆ UNCOMMON  80u
     ≈74s  ◈ 6,325  ⛽12  ETA ?  ●●●○○`. Columns: designation, tier
     glyph+label (tier-colored), cargo units, rough mine time, dock value
     estimate, approach fuel, rough pirate ETA/risk confidence, threat dots
     (blue→amber→red by count). Selected row highlighted.
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
  selected rock — designation, tier, units, estimated mine time, public dock
  value, cargo hold impact, flight fuel cost vs current fuel (red if
  insufficient), rough pirate ETA/risk with dots — plus
  `[ENTER] LOCK & FLY  [S] SCAN  [V] VIEW  [Q] DOCK`.
- `S` scans the selected contact. Empty-belt behavior remains owned by the
  shipped belt/sim flow; this screen does not expose a free `R` rescan action.
- Lock failure (fuel) → red flash, stay on belt.

### Mining Site screen (real time)

The actor pushes snapshots at 4 Hz during a run (gameplay/02); this screen
just renders the latest.

The body allocator gives **STATUS / ACTIONS** at least 46 columns (weight 3)
and the **TACTICAL VIEWPORT** at least 34 columns (weight 2). The split is
stable across drilling, tribute, escape, under-fire, and combat transitions.

```
◇ MINING SITE — KR-4711 (◆ UNCOMMON)
RESOURCE LEFT  ███████████░░░░░░░░░░░  58%
HULL           █████████████████░░░░░  82%
FUEL           ████████████░░░░░░░░░░  54%
CURRENT CUT VALUE  ◈ 2,975 of ◈ 5,100 (not sold)
⚙ PRESSURE POINT ACTIVE
  ███████████░░░░░░  2.1s  [SPACE] FRACTURE
[B] BAIL
                                      ╭──                    ──╮
                                      │ PIRATE ETA ~34-58s       │
                                      │ · · · ● · · · ·          │
                                      │       ▲                  │
                                      │       .-~~~~~-.          │
                                      │    .-'   *     '-.       │
                                      │     '-._________.-'      │
                                      ╰──                    ──╯
```

- Gauges via tui/04 `Bar`, ramped:
  - resource left: blue until depleted;
  - hull: blue→amber(<55)→red(<30);
  - fuel: blue→amber(<40)→red(<20).
- The pirate scanner stays at the upper-left of the large **right-hand**
  cockpit viewport, above the asteroid, with a fixed player anchor (`▲`) and
  a red pirate blip (`●`) at a per-run cosmetic bearing. It shows a fuzzed
  `PIRATE ETA ~min-maxs` range — narrowed by high Scanner grades
  (gameplay/05, superseding the old Surveyor track),
  replaced with the exact `JAMMED <seconds>` equipment countdown while a
  Pirate Jammer is suppressing approach, then restored when suppression ends,
  replaced with `CONTACT LOST — NO ETA` while a `radar_blackout` event is
  active. The true distance/arrival time is never rendered as an exact
  number anywhere on this screen.
- A large, randomly selected ASCII asteroid sprite is centered inside four
  cyan cockpit corners in that right-hand viewport. Each run has one to three
  pressure points on that surface. A lit point flashes gold/white; its
  countdown and `Space` prompt stay in the **left** status column. A
  successful keypress or asteroid click turns the point green and fractures a
  significant portion of remaining ore, while expiry turns it red. There is no
  narrow timing zone, no ship penalty for a miss, and no new point after
  depletion or a full hold.
- Cargo volume lives in the global top navigation bar at all times, including
  active-run ore. A Fuel Miner additionally reveals a clear `RICH VEIN` or
  `NO FUEL VEIN` readout; ships without the module see neither label.
- The main action button:
  - while asteroid ore remains: `[B] BAIL` in flashing yellow/amber;
  - when the cargo hold fills first: a steady `HOLD FULL — [B] BAIL WITH
    CURRENT LOAD`, while resource-left stays non-zero;
  - when the asteroid itself is depleted: `[ENTER] DEPART` in green;
  - if reduced motion is on, replace flashing with a steady amber `!`.
- `B` / `Esc` / `Enter` / click BAIL starts escape before depletion.
  `Enter` / click DEPART starts escape after depletion. Both call gameplay/02
  `BailOrDepart`.
- No overdrive in the revamped loop. The pressure comes from time, cargo load,
  bad information, events, and escape risk.
- `Ctrl+C` still disconnects; framework/03 resolves this as an emergency
  bail/escape attempt, not a free pause.

### Tribute prompt

If pirates arrive and roll a tribute demand, the mining screen is interrupted by
a modal panel:

```
◇ PIRATE TRANSMISSION
"Drop ◈ 4,250 from this rock or we open your hull."

[D] DROP CARGO      [R] REFUSE / RUN
decision timeout: 12s
```

- `D`/click DROP calls `AcceptTribute`.
- `R`/Esc/click REFUSE calls `RefuseTribute`.
- Timeout refuses automatically.
- Background HUD remains visible but dimmed; radar is red at contact.

### Escape state

After BAIL/DEPART/tribute/refusal, the view changes in-place:

- Title becomes `◇ ESCAPE BURN` or danger variant `☠ UNDER FIRE`.
- Main bar is `ESCAPE VECTOR` counting up to required seconds.
- Cargo hold remains visible because cargo load explains the slow flee.
- If under attack, hull ticks down every snapshot and the panel border flashes
  red unless reduced motion is on.
- No menu actions are accepted; the ship either escapes or dies.

### Event HUD effects

Every random event from gameplay/02 must have a unique HUD treatment:

| Event | Rendering requirement |
| --- | --- |
| `power_outage` | Most panels blank to near-black; only a dim reboot countdown and BAIL/DEPART affordance remain. |
| `radar_blackout` | Radar panel fills with static glyphs; ETA shows `SIGNAL LOST`. |
| `life_support_failure` | Oxygen strip appears at top; dark red edge pulse; keybar says `LIFE SUPPORT FAILURE — LEAVE NOW`. |
| `cargo_shift` | Cargo panel jitters between two offsets unless reduced motion; steady amber warning otherwise. |
| `reactor_surge` | Fuel gauge flashes amber/white for one tick; event badge persists until effect ends. |

These are render-side treatments keyed by the event enum; mechanics stay in sim.

### Death handoff

When gameplay/02 resolves `ship_lost`, tui/03 must immediately route to
tui/02's Death Screen. Do not render the summary first. Do not show a normal
flash message. The cockpit is gone.

## Acceptance criteria

- [ ] All three belt views render the same fixture belt inside their panel
  at 80×24 (goldens); selection stays consistent when cycling views.
- [ ] Ore-scan collision nudging never drops a rock or paints outside the
  panel (property test with adversarial X/Y).
- [ ] Radar bearing math maps X,Y percent coords inside the circle.
- [ ] Mining Site gauges match snapshot values (unit test with crafted
  snapshots at thresholds 0/19/29/39/49/74/100 checks color roles).
- [ ] BAIL renders flashing yellow/amber while resources remain; DEPART renders
  green only after depletion.
- [ ] Tribute modal accepts/refuses/times out and emits the correct sim action.
- [ ] Escape state rejects normal mining inputs and updates hull/escape bars.
- [ ] Every random event enum has a golden or focused render test proving its
  unique HUD treatment.
- [ ] Mouse: click-select vs click-activate on rocks, wheel cycling, BAIL,
  DEPART, tribute buttons — hitbox tests all.
- [ ] Screen transitions: lock → mining site, bail/depart → escape, survived
  outcome → summary, ship_lost → death screen, dock → chart.
- [ ] Every run state uses tui/06's exact 46/34 minimum, 3/2 weighted split;
  state transitions do not move the region boundary.

## Out of scope / handoffs

- Summary rendering → tui/02.
- Bar/panel primitives, colors → tui/04.
- Snapshot push plumbing → framework/03; tick math → gameplay/02.
