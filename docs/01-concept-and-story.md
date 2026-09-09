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
scenes; the story is the tension in the gauges. Death is delivered as the HUD
flickering out, then a full-screen dark red terminal failure: `CONNECTION
LOST`.

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

| System | Starting access | Design role | Built? |
| --- | --- | --- | --- |
| **SOL** | unlocked | tutorial economy; several planets locked by fuel tank or permit | shipped |
| **ERIDANI DRIFT** | Jump Rating Class E | harsher rings, distant pirates, lethal attacks | shipped |
| **KEPLER REACH** | Jump Rating Class D | **planning pressure** — vast distances, few docks, punishing fuel arithmetic | **shipped on beta 2.0.0** |
| **REDLINE EXPANSE** | Jump Rating Class C | **everything, faster** — unstable belts, short pirate ETAs, a hard clock | **shipped on beta 2.0.0** |

> **Updated by** [gameplay/08-jump-network-and-frontier-progression.md](gameplay/08-jump-network-and-frontier-progression.md):
> the "locked by an installed Jump Drive" gate on Eridani and the "expensive
> transfer + higher jump rating" gate on Kepler are both replaced by the pilot
> **Jump Rating** ladder (Class E → D → C). That doc also specifies the two
> frontier systems above, three frontier hulls, and the located-hangar model in
> which ships are parked at a specific dock rather than following the pilot.

`data/worlds.toml` on beta ships all four systems and eleven destinations.
Stations remain aspirational, and "late-game station value" is doubly
so — **stations and cosmetics were never implemented**; see `README.md`
§ "Designed but not built" before planning against either.

Starter Sol destinations should demonstrate the lock model: one accessible
low-yield belt, one nearby planet that needs a fuel tank upgrade, one lucrative
planet locked behind a permit, and one outer destination that needs both a
bigger tank and a better ship.

Ships gate range and risk capacity. **Superseded:** the five-ship list below
is historical — the authoritative beta fleet is now 7 ships across 4 brands
(Federation, Alliance, Independent, Frontier) and 3 classes (Miner, Fighter, Freighter),
each with persistent per-ship stat grades and swappable slot devices under a
per-ship power/mass budget, owned simultaneously in a hangar rather than
replaced one-for-one. See
[gameplay/05-fleet-ships-and-shipyard-economy.md](gameplay/05-fleet-ships-and-shipyard-economy.md)
for the full ship table and economy, and
[tui/05-shipyard-screen.md](tui/05-shipyard-screen.md) for the standalone
Shipyard screen this replaces the old chart-overlay Shipyard with.

| Ship class (historical) | Role |
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
- **Everything fits 80×24.** The v1.7 game frame grows independently to
  144×48, then centers horizontally and vertically inside larger terminals.
  Larger frames get more breathing room, reflow, and taller scroll viewports,
  never extra gameplay information. Chart is an equal 40/40 split; Shipyard
  uses 24/32/24 HANGAR/LOADOUT/STATUS minima with 3/4/3 flex; Mining/Combat
  uses 46/34 STATUS/TACTICAL minima with 3/2 flex. See
  [tui/06-responsive-menu-overhaul.md](tui/06-responsive-menu-overhaul.md).

## Controls (global contract)

Arrow keys move selection; **Enter/Space** confirms; the **mouse works
everywhere the keyboard does** — clicking a list row selects it, clicking a
selected row (or double-clicking) activates it, clicking `[F] REFUEL` style
buttons triggers them, and the scroll wheel moves selection in lists.

The table below is the **shipped** contract, reconciled against the code on
2026-08-15 (v1.6.1). It replaces an earlier table that listed keys for
unbuilt systems (`C cosmetics`, `B build station`) and had `H`/`R` on the wrong
actions:

| Screen | Keys |
| --- | --- |
| Star Chart | ↑/↓ select destination · Enter depart (opens the permit prompt if the route is buyable) · F refuel · R repair hull · C sell cargo + bounty vouchers · S shipyard · L log · J jump certification (gameplay/08) · I salvage advance (only while eligible) · T tweaks · Q quit |
| Belt | ↑/↓/←/→ cycle contacts · Enter/Space lock & fly · S scan selected contact · V cycle view mode · Q dock |
| Mining | B/Esc/Enter bail while ore remains, depart once depleted · Space/click fracture a lit pressure point |
| Tribute prompt | D accept (jettison and run) · R/Esc refuse (flee under fire) · F fight (armed ships only) |
| Combat | F pulse laser · G guided missile · B/Esc/Enter start the escape burn — see gameplay/07 |
| Escape | no menu actions; ship is fleeing under current conditions |
| Run Summary | Enter/Space continue into the same belt · Q dock |
| Ship-lost recap | Enter/Space/Q — all dock; there is no belt to return to |
| Anywhere | ? help overlay · Ctrl+C disconnect |

Note what the belt screen owns: `Q` is the **only** way to dock. The star chart
has no dock action, which is why a session that opened on the chart while
`WorldIdx >= 0` used to be a softlock. Since v1.6.2 a session opens where the
save says the pilot is, and a chart reached while `WorldIdx >= 0` shows its
port services disabled with `Enter` rebound to RETURN TO BELT — deliberately a
screen change only, never `sim.Dock`, since docking would hand out a free
jammer/EMP/missile rearm and full shield restore. See
[framework/05-reconnect-and-location-restore.md](framework/05-reconnect-and-location-restore.md).

## Deliberate divergences from the HTML prototype

Agents should follow **this list**, not the prototype, where they differ:

1. **The prototype is not balance-authoritative.** It remains useful for
   layout and palette references only. The TUI version uses longer mining
   durations, cargo holds, pirate choices, ship death, locked systems, and
   stations.
2. **Persistent pilots.** The prototype reset on refresh. We persist
   credits, permits, stations, cosmetics, settings, lifetime stats, current
   ship, cargo, and local belt state per SSH key (see framework docs).
   Disconnecting mid-operation auto-starts a bail/escape resolution — and the
   pilot comes back **at the belt they were working**, not at the dock, with a
   one-shot recap of what the autopilot did (framework/05, shipped v1.6.2;
   stations and cosmetics in this list were never built).
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
