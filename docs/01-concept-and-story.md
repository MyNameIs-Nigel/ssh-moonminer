# Moon Miner — Concept & Story

This document is shared context for every task. It defines what the game is,
how it feels, and where the TUI version deliberately diverges from the HTML
prototype (`../moon-miner-materials/Web-Prototype/`). It is not itself a task.

## Story

It's the far edge of the settled system. Corporate haulers own the safe
lanes; everyone else scrapes a living in the belts. You are an independent
rig pilot — one ship, one drill, one tank of fuel — working the asteroid
fields between Ceres and the Jovian moons. Ore pays. Rare ice pays better.
But every minute your drill is spun up, your reactor signature is a beacon,
and the belt pirates triangulate a little closer.

The fiction is delivered through the interface itself: the player is looking
at their ship's cockpit terminal. Screens are "instruments" (star chart,
belt scanner, drill console, ship's log), system messages are terse and
diegetic ("CONTACT LOCKED", "TANKS DRY — EMERGENCY TOW DISPATCHED"), and the
whole thing renders like a phosphor CRT. There is no narrator and no cut
scenes; the story is the tension in the gauges.

## The core loop

```
┌────────────┐    pick world     ┌──────────────┐    lock target    ┌──────────┐
│ STAR CHART ├──────────────────►│ ASTEROID BELT├──────────────────►│  MINING  │
│ (port svc) │                   │ (scan/select)│                   │ (live!)  │
└─────▲──────┘                   └──────▲───────┘                   └────┬─────┘
      │ dock                            │ continue                       │ any outcome
      │                          ┌──────┴───────┐                        │
      └──────────────────────────┤ RUN SUMMARY  │◄───────────────────────┘
                                 └──────────────┘
```

1. **Star Chart** — choose one of four worlds, each with a distinct
   risk/reward profile, and buy port services (refuel, hull repair).
2. **Asteroid Belt** — a scanner shows ~7 procedurally generated contacts.
   Each rock trades off volume (value, drill time), rarity (value multiplier,
   pirate attention), fuel cost to reach it, and threat level. Pick one.
3. **Mining** — the one real-time screen. Three gauges move every tick:
   drill progress up, fuel down, pirate proximity up. Hold overdrive to
   drill ~2× faster while burning ~2× fuel. Bail at any moment to keep the
   cargo value accrued so far.
4. **Run Summary** — a ship's-log entry: what you banked, what it cost,
   colored by outcome. Continue into the same belt (rock removed) or dock.

Four outcomes, one per run:

| Outcome | Trigger | Payout | Extra |
| --- | --- | --- | --- |
| **CLEAN EXTRACTION** | drill reaches 100% | full asteroid value | — |
| **CARGO SECURED** (bail) | player bails | accrued yield | always safe — the coward's profit |
| **RAIDED** | pirate proximity hits 100% | 55% of accrued yield | −25 hull |
| **STRANDED** | fuel hits 0 | 60% of accrued yield | emergency tow (the fee is the lost 40%) |

The design's heart is the **push-your-luck bail decision**: cargo value
scales linearly with drill progress, so bailing is always safe but always
leaves money on the table.

## The worlds

Four destinations reweight the risk/reward curve (not just reskins):

| World | Locale | Belt | Pirates | Rarity bias | Travel cost | Flavor |
| --- | --- | --- | --- | --- | --- | --- |
| **CERES** | Inner belt | Dense | HIGH (×1.55) | +0.18 | 8 fuel | "Crowded, lucrative, lawless. Raiders everywhere." |
| **VESTA** | Mid belt | Moderate | MED (×1.05) | +0.10 | 6 fuel | "Close to port. Steady, honest pickings." |
| **TITAN** | Saturn rings | Ringed | MED (×0.95) | +0.30 | 20 fuel | "Rare ices in the rings. A long, cold haul." |
| **IO** | Jovian orbit | Sparse | LOW (×0.70) | +0.05 | 14 fuel | "Quiet and safe. Slim takings for the patient." |

Asteroid rarity tiers (the one place color breaks from blue monochrome):

| Tier | Glyph | Color | Value multiplier |
| --- | --- | --- | --- |
| COMMON | ◇ | white `#dce9f5` | ×1.0 |
| UNCOMMON | ◆ | cyan `#3fe0ff` | ×2.3 |
| RARE | ✦ | violet `#b48fff` | ×4.6 |
| LEGENDARY | ★ | gold `#ffcf5e` | ×9.5 |

## Visual direction

- **Pure terminal aesthetic** — the prototype faked a TUI in HTML; we are
  building the real thing. Box-drawing panels with a floating label notch
  (`◇ PANEL NAME`) in the top border, block-character gauges
  (`████████░░░░`), ASCII planet art, a blinking `█` cursor in footers.
- **Phosphor-blue palette** on near-black navy. Danger reads warm: fuel/hull
  bars and pirate proximity ramp blue → amber → red as they worsen. Full
  palette and semantic roles live in
  [tui/04-theme-components-and-tweaks.md](tui/04-theme-components-and-tweaks.md).
- **Everything fits 80×24.** Larger terminals get more breathing room, never
  extra information.

## Controls (global contract)

Arrow keys move selection; **Enter/Space** confirms; the **mouse works
everywhere the keyboard does** — clicking a list row selects it, clicking a
selected row (or double-clicking) activates it, clicking `[F] REFUEL` style
buttons triggers them, and the scroll wheel moves selection in lists.

| Screen | Keys |
| --- | --- |
| Star Chart | ↑/↓ select world · Enter depart · F refuel · H repair hull · T tweaks · Q quit |
| Belt | ↑/↓/←/→ cycle contacts · Enter/Space lock & fly · V cycle view · R rescan · Q dock |
| Mining | hold Space overdrive · B or Esc bail |
| Summary | Enter continue · Q dock |
| Anywhere | ? help overlay · Ctrl+C disconnect |

## Deliberate divergences from the HTML prototype

Agents should follow **this list**, not the prototype, where they differ:

1. **World travel costs fuel.** The prototype displayed a `travel` cost but
   never charged it; we charge it on departure from the chart. This makes
   Titan/Io's better belts a real investment and gives the refuel button a
   job.
2. **Persistent pilots.** The prototype reset on refresh. We persist
   credits, fuel, hull, upgrades, settings, and lifetime stats per SSH key
   (see framework docs). Disconnecting mid-drill auto-bails.
3. **Hull matters.** In the prototype hull only fell (raids) and was
   repaired. We add: at hull 0 the ship is **unspaceworthy** — belts are
   locked until repaired, and repairs at 0 hull cost extra ("dry-dock
   surcharge"). Death spiral protection: a pilot who is broke, dry, and
   broken gets a one-time insurance bailout (see gameplay/03).
4. **Ship upgrades** (drill head, fuel tank, hull plating, signature damper,
   surveyor) are a new credit sink giving long-term goals — the prototype had
   nothing to spend on but fuel. See gameplay/04.
5. **CRT glow → terminal equivalents.** CSS text-shadow doesn't exist in a
   terminal. The Tweaks panel instead offers reduced-motion, high-contrast,
   and ASCII-only fallback modes (see tui/04).
6. **New-pilot starting state**: 1,000 credits, 100 fuel, 100 hull (the
   prototype's 4,820/78/100 was a mid-game snapshot for screenshot purposes;
   a fresh start should need a few runs before the first big refuel).

## What "done" looks like (MVP)

A stranger with any SSH client runs `ssh play.example.com`, gets a pilot tied
to their key, plays chart → belt → mine → summary loops with keyboard or
mouse, disconnects, comes back a week later, and their credits, upgrades, and
ship's log are still there. The server survives redeploys without losing a
byte of anyone's progress.
