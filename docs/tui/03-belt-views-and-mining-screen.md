# TUI 03 — Asteroid Belt Views & Live Mining Screen

**Area:** TUI · **Phase:** 3 · **Depends on:** tui/01, tui/04, gameplay/01,
gameplay/02 · **Parallel-safe with:** tui/02, framework/03

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

- `internal/tui/` — `belt.go`, `beltviews.go` (the three renderers),
  `mining.go` (+ tests)

## Spec

### Belt screen layout

- **Left column (~30 cols):** current world panel — ASCII art
  (sphere/ringed), name/sub, ring/pirate/rarity labels, contacts count.
  Below it, belt stats (rocks remaining, best value visible, fuel reserve).
- **Right column:** the contact field, one of **three views**, cycled with
  `V` (persisted default comes from Settings, tui/04):

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
  `[ENTER] LOCK & FLY  [V] VIEW  [R] RESCAN  [Q] DOCK`.
- `R` rescan regenerates the belt (confirm nothing — it's free, per
  gameplay/01); empty belt (all rocks mined) auto-prompts rescan in the
  field area.
- Lock failure (fuel) → red flash, stay on belt.

### Mining Site screen (real time)

The actor pushes snapshots at 4 Hz during a run (gameplay/02); this screen
just renders the latest.

```
◇ MINING SITE — KR-4711 (◆ UNCOMMON)
RESOURCE LEFT  ███████████░░░░░░░░░░░  58%
HULL           █████████████████░░░░░  82%
FUEL           ████████████░░░░░░░░░░  54%
CURRENT CUT VALUE  ◈ 2,975 of ◈ 5,100 (not sold)
PIRATE SIGNAL ETA ~34-58s  ·····●······
╭──────────────────────────────────────────────────────────╮
│                  .-~~~~~-.                               │
│               .-'   *     '-.       * lit / + hit / x miss│
│                '-._________.-'                           │
╰──────────────────────────────────────────────────────────╯
[B] BAIL — flashing yellow until asteroid depletion
```

- Gauges via tui/04 `Bar`, ramped:
  - resource left: blue until depleted;
  - hull: blue→amber(<55)→red(<30);
  - fuel: blue→amber(<40)→red(<20).
- The pirate-distance signal sits immediately below the mining-status block:
  a red `●` advances through a compact dotted signal beside a fuzzed
  `PIRATE ETA ~min-maxs` range — narrowed by high Scanner grades
  (gameplay/05, superseding the old Surveyor track),
  replaced with the exact `JAMMED <seconds>` equipment countdown while a
  Pirate Jammer is suppressing approach, then restored when suppression ends,
  replaced with `CONTACT LOST — NO ETA` while a `radar_blackout` event is
  active. The true distance/arrival time is never rendered as an exact
  number anywhere on this screen.
- A large, randomly selected ASCII asteroid sprite is centered inside four
  cyan cockpit corners. Each run has one to three pressure points on that
  surface. A lit point flashes gold/white with a `Space` prompt; a successful
  press or asteroid click turns it green and fractures a significant portion
  of remaining ore, while expiry turns it red. There is no narrow timing zone,
  no ship penalty for a miss, and no new point after depletion or a full hold.
- Cargo volume lives in the global top navigation bar at all times, including
  active-run ore. A Fuel Miner additionally reveals a clear `RICH VEIN` or
  `NO FUEL VEIN` readout; ships without the module see neither label.
- The main action button:
  - while asteroid ore remains: `[B] BAIL` in flashing yellow/amber;
  - when the cargo hold fills first: a steady `HOLD FULL — [B] BAIL WITH
    CURRENT LOAD`, while resource-left stays non-zero;
  - when the asteroid itself is depleted: `[ENTER] DEPART` in green;
  - if reduced motion is on, replace flashing with a steady amber `!`.
- `B` / `Esc` / click BAIL starts escape before depletion. `Enter` / click
  DEPART starts escape after depletion. Both call gameplay/02 `BailOrDepart`.
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

## Out of scope / handoffs

- Summary rendering → tui/02.
- Bar/panel primitives, colors → tui/04.
- Snapshot push plumbing → framework/03; tick math → gameplay/02.
