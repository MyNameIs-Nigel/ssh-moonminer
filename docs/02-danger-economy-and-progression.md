# Moon Miner — Danger, Economy & Progression Narrative

This document expands the core fantasy into implementation-facing rules. It is
shared context for gameplay, TUI, framework, and test tasks.

## Design north star

Moon Miner should feel like mining in a failing cockpit at the edge of lawful
space. The player is not a superhero pilot. They are a contractor flying ships
they can barely afford, making greedy decisions in bad information while pirate
signals crawl across a radar scope.

The game should be hard to not die. Death is not a rare fail state reserved for
reckless players; it is a common pressure valve in the loop. The long-term arc
comes from learning when to bail, buying better ships, unlocking farther systems,
and eventually owning infrastructure that softens the cost of each loss.

## Progression shape

### Locked destinations

The chart is hierarchical:

1. **Solar systems** are the large travel regions.
2. **Planets / stations / belts** are destinations inside a system.

Most destinations start locked. A new pilot begins in the Sol system with only
one low-yield destination available. Nearby Sol destinations can be reached with
the starter ship only after fuel-tank upgrades; farther systems require both a
ship with a sufficient jump rating and a large credit transfer fee.

Locks should explain themselves in the UI:

- `LOCKED — NEED FUEL TANK II`
- `LOCKED — BUY SYSTEM TRANSFER: ◈ 75,000`
- `LOCKED — NEED CUTTER-CLASS SHIP`
- `LOCKED — STATION NAV BEACON REQUIRED`

Purchasing a system transfer is not a cheap fast-travel button. It is a major
decision with a route-dependent cost. The player should often decide to keep
working their current system because changing systems is expensive.

### Ships are lives

The active ship is a consumable run-defining asset:

- Hull at `0` means the ship is dead, not merely unspaceworthy.
- Death destroys the active ship, installed ship upgrades, unsold cargo, and any
  active asteroid claim.
- The pilot respawns at the Sol dock in a starter salvage skiff.
- Banked credits, cosmetic unlocks, system transfer permits, and owned space
  stations survive death.
- Superior ships must be repurchased after death. This is intentional: buying a
  better ship is powerful but never permanent safety.

This keeps death punishing without wiping account identity or late-game
infrastructure.

### Cargo is not money

Mining fills a cargo hold. Cargo becomes credits only when sold at a dock or at
an owned station. Unsold cargo is vulnerable:

- pirates can demand a large portion of the asteroid's current haul;
- attacks can destroy cargo during escape;
- death destroys the entire hold.

Station sales pay more than dock sales in the same system. This gives station
ownership a local, concrete advantage without turning stations into universal
buffs.

## Manual mining flow

Mining is no longer a fast automatic progress bar. The player must manually
leave every asteroid.

1. **Scan belt** — the belt screen lists contacts and gives imperfect estimates:
   value, mining duration, cargo demand, risk, and pirate arrival window.
2. **Select asteroid** — locking a target opens a dedicated mining-operation
   screen showing the asteroid being mined, the ship HUD, cargo load, rough
   pirate ETA, and a radar distance readout.
3. **Mine over time** — resources transfer into cargo slowly. The process should
   be long enough that the player watches the radar and second-guesses greed.
4. **Leave manually** — while resources remain, the action is flashing yellow
   `BAIL`. Once depleted, it becomes green `DEPART`.
5. **Escape sequence** — leaving starts a flee timer. A full ship takes much
   longer to flee. If pirates are attacking, hull drops every second until the
   ship escapes or dies.

The core choice is no longer "will I let the bar reach 100%?" It is "how much
cargo am I willing to drag through a slow escape while the radar closes?"

## Pirate behavior

Pirates are not a single automatic outcome. When they arrive, roll one action:

| Pirate action | Result |
| --- | --- |
| **Tribute demand** | Pirates demand a large portion of the value mined from the current asteroid. Accepting jettisons that cargo and starts escape. Refusing starts an attack. |
| **Immediate attack** | Pirates open fire immediately. The player must flee. |

The player cannot win a fight in the MVP. "Fighting" means surviving long enough
to flee while the hull falls. Future weapons can layer on top of this, but the
first implementation should make pirates terrifying and readable.

## Random events

Random events happen during mining and escape. The lower the hull percentage,
the higher the chance that the next event is bad.

Events are deterministic from the sim RNG and have TUI-visible effects:

| Event | Mechanical effect | HUD treatment |
| --- | --- | --- |
| **Power outage** | Mining pauses briefly; input still accepts `BAIL`/`DEPART`. | Most panels go black except a dim reboot timer. |
| **Radar blackout** | Pirate ETA/radar freezes and becomes less trustworthy for a short duration. | Radar panel fills with static/noise glyphs. |
| **Life-support failure** | Adds a short repair-or-flee countdown; if ignored, hull bleeds every tick. | Oxygen strip flashes red; screen edges pulse dark red. |
| **Cargo shift** | Escape time increases because the hold is unstable. | Cargo panel jitters between two alignments if motion is enabled. |
| **Reactor surge** | Fuel burn spikes; small chance to damage hull. | Fuel gauge blooms amber/white for one tick. |

At high hull, nuisance events should be more common than lethal ones. At low
hull, bad events should stack often enough that a damaged ship feels cursed.

## Death presentation

When the ship dies, the TUI cuts to a dark red full-screen failure state:

```text
CONNECTION LOST
```

No verbose recap appears on that screen. The next keypress returns the pilot to
the dock respawn summary, where the game explains what was lost: ship, installed
upgrades, and cargo. The stark death screen is part of the fiction: the cockpit
terminal went dead.

## Space stations

Stations are late-game, local infrastructure:

- A station is built in a specific solar system.
- Construction is staged and expensive; each stage can be funded separately.
- Completed stations generate passive income into a claimable local account.
- Stations reduce refuel costs only in their system.
- Selling cargo at your station pays a higher multiplier than selling at a
  public dock.
- Stations survive ship death.

Stations should not make early progression safe. They are the reward for
surviving the brutal early/mid game long enough to invest.

## Cosmetic configuration

Cosmetics are persistent account-level unlocks and should survive death:

- HUD palette/accent;
- panel border style;
- radar sweep style;
- ship paint/name/nose-art text;
- future ship-rendering parts once the renderer is known.

Cosmetics must not affect simulation math. They exist because pilots should
have identity even when ships are disposable.

## Clarifying questions to answer during implementation

These are not blockers for the documentation revamp, but they should be decided
before final balance/content authoring:

1. Should system-transfer fees be paid once as a permanent route permit, or on
   every transfer? This spec assumes a one-time permit plus smaller repeat fuel
   costs.
2. Should passive station income accrue while fully offline? This spec assumes
   capped offline accrual so stations feel passive without becoming infinite
   idle-game income.
3. Should the first implementation include player weapons, or keep pirates as
   flee-only pressure? This spec assumes flee-only for MVP.
4. Should death ever take a percentage of banked credits? This spec assumes no:
   ship/cargo/upgrade loss is already severe and clearer.
