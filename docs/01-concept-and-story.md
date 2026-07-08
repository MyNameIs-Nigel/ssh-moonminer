# Moon Miner — Concept & Story

This document is shared context for every task. It defines what the game is,
how it feels, and where the TUI version deliberately diverges from the HTML
prototype (`../moon-miner-materials/Web-Prototype/`). It is not itself a task.

## Story

It's the far edge of settled space. Corporate haulers own the safe lanes;
everyone else scrapes a living in locked-down systems where a route permit can
cost more than a ship. You are an independent rig pilot — one disposable ship,
one drill, one tank of fuel, and a cargo hold that turns every escape into a
liability. Ore pays. Rare ice pays better. But every minute your drill is spun
up, your reactor signature is a beacon, and belt pirates triangulate closer.

Ships are lives. When the hull breaches, the cockpit terminal dies, the ship is
gone, the cargo is gone, and the pilot wakes back at a dock in a starter salvage
skiff. Credits and hard-won infrastructure survive, but superior ships and
installed upgrades must be bought again.

The fiction is delivered through the interface itself: the player is looking
at their ship's cockpit terminal. Screens are "instruments" (star chart,
belt scanner, drill console, ship's log), system messages are terse and
diegetic ("CONTACT LOCKED", "TANKS DRY — EMERGENCY TOW DISPATCHED"), and the
whole thing renders like a phosphor CRT. There is no narrator and no cut
scenes; the story is the tension in the gauges. Death is delivered as a dark
red terminal failure: `CONNECTION LOST`.

## The core loop

```
┌────────────┐ choose route/ship ┌──────────────┐ lock target ┌──────────────┐
│ STAR CHART ├──────────────────►│ ASTEROID BELT├────────────►│ MINING SITE  │
│ dock/stn   │                   │ scan/select  │             │ drill + radar│
└─────▲──────┘                   └──────▲───────┘             └──────┬───────┘
      │ sell/repair/build               │ continue                    │ bail/depart
      │                          ┌──────┴───────┐             ┌───────▼──────┐
      └──────────────────────────┤ RUN SUMMARY  │◄────────────┤ ESCAPE/DEATH │
                                 └──────────────┘             └──────────────┘
```

1. **Star Chart** — choose a solar system and destination, inspect lock
   reasons, buy fuel/repairs/ships/upgrades/cosmetics, sell cargo, and fund
   local station construction. Most destinations are locked at first.
2. **Asteroid Belt** — a scanner shows procedurally generated contacts. Each
   rock trades off resource value, mining duration, cargo load, flight fuel,
   and pirate attention. Estimates are intentionally rough.
3. **Mining Site** — the long real-time screen. The asteroid is mined into the
   cargo hold while the pirate radar closes. The player must manually leave:
   flashing yellow **BAIL** while resources remain, green **DEPART** after the
   asteroid is depleted.
4. **Escape / Pirate Contact** — leaving takes time. A full hold makes escape
   slow. If pirates arrive they either demand tribute or attack, and under
   attack the hull drops every second until the ship escapes or dies.
5. **Run Summary** — a ship's-log entry: cargo recovered, cargo sold or held,
   hull/fuel damage, pirate event, death losses if any. Continue into the same
   belt, dock, or respawn after death.

Primary outcomes:

| Outcome | Trigger | Payout | Extra |
| --- | --- | --- | --- |
| **DEPARTED** | player leaves after depletion and escapes | mined cargo kept; sold later | safest profitable outcome |
| **BAILED** | player leaves before depletion and escapes | partial cargo kept; sold later | leaves resources behind |
| **TRIBUTE PAID** | pirates demand cargo and player accepts | remaining cargo kept after jettison | avoids direct hull damage |
| **UNDER FIRE** | pirates attack or tribute refused | cargo kept only if escape succeeds | hull falls during flee |
| **SHIP LOST** | hull reaches 0 | unsold cargo lost | active ship + installed upgrades destroyed |

The design's heart is the **manual extraction decision**: cargo value rises
while escape time gets worse, the pirate ETA is never exact, and a damaged hull
makes random bad events more likely. Bailing is not perfectly safe; it is only
safer than staying.

## Systems, worlds, and ships

The initial content set should be small enough to implement but structured for
expansion:

| System | Starting access | Design role |
| --- | --- | --- |
| **SOL** | unlocked | tutorial economy; several planets locked by fuel tank or permit |
| **ERIDANI DRIFT** | locked by transfer fee + ship class | harsher pirates, better ore |
| **KEPLER REACH** | locked by expensive transfer + higher jump rating | late-game station value |
| **REDLINE EXPANSE** | locked endgame route | extremely profitable, routinely lethal |

Starter Sol destinations should demonstrate the lock model: one accessible
low-yield belt, one nearby planet that needs a fuel tank upgrade, one lucrative
planet locked behind a permit, and one outer destination that needs both a
bigger tank and a better ship.

Ships gate range and risk capacity:

| Ship class | Role |
| --- | --- |
| **SALVAGE SKIFF** | free respawn ship; tiny hold, poor hull, Sol-local |
| **PROSPECTOR** | first purchased ship; reaches more Sol planets |
| **CUTTER** | first inter-system ship; larger hold makes escapes risky |
| **HAULER** | high cargo value and very slow flee time |
| **SURVEYOR FRIGATE** | expensive late-game platform for distant systems |

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
  bars and pirate radar distance ramp blue → amber → red as they worsen. Full
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
| Star Chart | ↑/↓ select route/destination · Enter depart · F refuel · H repair hull · S shipyard · C cosmetics · B build station · L log · T tweaks · Q quit |
| Belt | ↑/↓/←/→ cycle contacts · Enter/Space lock & fly · V cycle view · R rescan · Q dock |
| Mining | B/Esc bail while resources remain · Enter depart after depletion · D accept tribute/drop cargo · R refuse tribute |
| Escape | no menu actions; ship is fleeing under current conditions |
| Summary/Death recap | Enter continue · Q dock |
| Anywhere | ? help overlay · Ctrl+C disconnect |

## Deliberate divergences from the HTML prototype

Agents should follow **this list**, not the prototype, where they differ:

1. **The prototype is not balance-authoritative.** It remains useful for
   layout and palette references only. The TUI version uses longer mining
   durations, cargo holds, pirate choices, ship death, locked systems, and
   stations.
2. **Persistent pilots.** The prototype reset on refresh. We persist
   credits, permits, stations, cosmetics, settings, lifetime stats, current
   ship, cargo, and local belt state per SSH key (see framework docs).
   Disconnecting mid-operation auto-starts a bail/escape resolution.
3. **Hull means life.** At hull 0 the active ship is destroyed. The pilot
   respawns in a starter skiff; superior ships and installed upgrades must be
   repurchased. See gameplay/02 and gameplay/04.
4. **Manual mining and cargo.** Mining no longer resolves automatically at
   100%. The player must leave manually, cargo must be sold later, and cargo
   load directly slows escape.
5. **Locked systems and stations.** Planets, systems, ships, and station
   construction are the long-term economy. See
   [02-danger-economy-and-progression.md](02-danger-economy-and-progression.md).
6. **CRT glow → terminal equivalents.** CSS text-shadow doesn't exist in a
   terminal. The Tweaks panel instead offers reduced-motion, high-contrast,
   cosmetics, and ASCII-only fallback modes (see tui/04).
7. **New-pilot starting state**: starter skiff, 500 credits, a small fuel
   reserve, 100 hull, empty cargo, Sol access, and no station.

## What "done" looks like (MVP)

A stranger with any SSH client runs `ssh play.example.com`, gets a pilot tied
to their key, plays chart → belt → mine → flee → summary loops with keyboard or
mouse, dies often enough to understand ships are disposable, buys back into
better ships, unlocks at least one new destination, disconnects, comes back a
week later, and their credits, permits, stations, cosmetics, and ship's log are
still there. The server survives redeploys without losing a byte of anyone's
progress.
