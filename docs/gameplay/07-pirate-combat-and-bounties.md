# Gameplay 07 — Pirate Combat, Named Pirates & Bounties

**Area:** Gameplay + TUI · **Phase:** 7 (post-audit, single-doc feature) ·
**Depends on:** gameplay/02 (run loop), gameplay/05 (slots/weapon slots),
tui/03 (mining screen) · **Supersedes:** the "pirate behavior" table and the
"the player cannot win a fight in the MVP" rule in
[../02-danger-economy-and-progression.md](../02-danger-economy-and-progression.md)
— this doc is the "future weapons layer" that spec explicitly reserved.

Unlike gameplay/05 (which paired with tui/05), this doc owns both the sim and
the TUI deliverables — the combat screen and the sim model are too coupled to
spec separately.

## Goal

### v1.6.1 equipment balance update

This update supersedes the Mass Driver portions of this document and the
single-manual-fire-key wording below:

- Legacy `mass_driver` devices migrate at the same grade to
  `missile_launcher`; installed and stored launchers begin with a full
  magazine. The save schema is version 7.
- **G** fires one guided Missile Launcher shot: it is a 100% hit, deals
  `60 + 20*grade` damage, spends one missile, and begins a 2-second
  launcher-only cooldown. It never uses the shared laser heat capacitor.
  Each launcher carries E..S magazines of **3, 4, 6, 7, 9, 10** and fitted
  launchers are reloaded at dock. With several fitted launchers, the
  highest-grade loaded launcher fires first (fitted-slot order breaks ties).
- **F** now fires only the heat- and solution-limited Pulse Laser volley, so a
  ship can fit and independently use both weapon types. Autocannons remain
  continuous and their higher 2.0 power coefficient is their balancing cost.
- An installed Jump Drive migrates from Internal to a dedicated one-per-ship
  Jump Drive slot. **(Superseded by [08-jump-network-and-frontier-progression.md](08-jump-network-and-frontier-progression.md) —
  the slot and the device are both deleted; route access is the pilot's Jump
  Rating.)** It still draws no power and still gates Eridani Drift, but
  it no longer consumes the Internal slot.
- Pirate Jammer suppression lasts E..S **3, 5, 8, 12, 15, 25 seconds**.

The combat HUD renders missile ammunition, its cooldown, and the separate
F/G controls; empty magazines explicitly direct the pilot to dock and reload.

Turn pirates from anonymous flee-only pressure into **named ships with a
price on their hull**. Every encounter rolls a specific pirate from a TOML
roster — with hull, damage, maneuvering profile and a **bounty**. A ship with
at least one weapon installed gains **[F] FIGHT** at the tribute prompt (next
to Drop and Refuse/Run); pirates that attack outright skip the prompt and drop
the player straight into combat. Combat plays out on a **tactical scope**: the
pirate blip maneuvers across a radar box. The **Autocannon Turret** tracks and
damages it continuously; the player presses **F** for other weapons when the
blip crosses their firing arc, with a shared **HEAT** capacitor punishing
manual-fire spam. Destroying the pirate seals the run and awards a **bounty
voucher**, redeemed with the cargo sale (**C**) at dock. An **EST. ODDS %**
readout on the prompt and combat header tells the player what they're signing
up for.

Combat is an interesting dynamic, **not the main attraction** — a fight
should resolve in roughly 10–25 seconds, and running remains a first-class
answer. The mining loop, its timers, and the "cargo is kept only if the ship
escapes" rule are unchanged.

## References

| Source | What to take |
| --- | --- |
| [02-mining-run-loop.md](02-mining-run-loop.md) | the phase machine, tribute flow, escape formula, `resolveRun` outcomes this doc extends |
| [05-fleet-ships-and-shipyard-economy.md](05-fleet-ships-and-shipyard-economy.md) | weapon slots, device grades E..S, power/mass budgets, `[slots]` price curve |
| [../02-danger-economy-and-progression.md](../02-danger-economy-and-progression.md) | pirate fiction + "cargo is not money" doctrine (bounty vouchers deliberately are money — see Spec) |
| `internal/sim/run.go` (`rollPirateAction`, `startEscape`, `tickEscape`, `applyAttackDamageRate`, `EmergencyResolve`) | the exact functions this doc modifies |
| `internal/sim/slots.go` (`weaponItems`, `AttackDamageMul`) | the passive-weapon catalog this doc retires |
| `internal/sim/rng.go` (`runRNG`) | the only legal RNG source; salts below |
| `internal/tui/death.go` (`renderDeathFinal`) | the full-screen flood-card pattern the COMBAT MODE interstitial copies (amber, no flicker) |
| `internal/tui/mining.go` (`renderTribute`, `renderPirateRadar`, `keyMining`) | prompt + radar the combat screen grows out of |

## Migration: what's retired

| Old behavior | Disposition |
| --- | --- |
| **Defense Turret passive mitigation** (`AttackDamageMul`, `turret_pct_per_grade`) | Deleted. The turret keeps its item ID (`turret`), grade, price curve, power and mass, and becomes an **always-firing weapon** ("AUTOCANNON TURRET"). Live saves need no rewrite — installed turrets simply change behavior. The mitigation loss is offset by the shield buff below; **release-note it**. |
| `shield_hp_per_grade = 20` | Buffed to **30** — the shield is now the only damage-mitigation layer, absorbing attack damage before hull exactly as today. |
| Immediate attack / refused tribute → auto-started escape burn (`startEscape(underAttack=true)`) | Replaced by **PhaseCombat** (below). The burn auto-starts only in the cases in the burn matrix; PhaseEscaping with `UnderAttack=true` no longer occurs naturally. `attackHullDamagePerSecond` stays only as a defensive fallback for `Run.Combat == nil` (legacy tests set that shape directly). |
| "The player cannot win a fight" (02-danger doc) | Superseded by this doc. |

EMP Launcher delays the pirate's action roll on arrival. Pirate Jammer now
suppresses approach for a timed 10-second window rather than granting
full-run immunity; both gate when combat starts, not how it plays.

## Deliverables

- `internal/sim/combat.go` — roster pick, combat state + tick, fire/fight/
  escape actions, odds estimate, win/lose resolution
- `internal/sim/run.go`, `state.go`, `slots.go`, `economy.go` — edits per Spec
- `data/pirates.toml` (new), `data/balance.toml` `[combat]` + weapon entries
- `internal/content` — `Pirate`/`CombatConfig` structs, loading, validation
- `internal/game/session.go` — `FightPirates`, `FireWeapons`, `CombatEscape`
- `internal/tui/combat.go` (new: interstitial + tactical scope),
  `game.go`, `mining.go`, `chart.go`, `help.go`, `summary.go` edits
- `internal/sim/combat_test.go` + updates to existing sim/tui tests
- `internal/version` → 1.5.0

## Spec

### Pirate roster — `data/pirates.toml`

```toml
[[pirates]]
id = "jackal"
name = "JACKAL"                # rendered uppercase everywhere
hull = 55.0
damage_per_second = 5.0        # multiplied by world pirate_attack_mul (default 1.0)
maneuver = 0.8                 # blip speed multiplier; higher = harder firing solution
bounty = 400                   # credits, paid as a voucher on kill
min_threat = 0.0               # spawn band, inclusive
max_threat = 0.7
weight = 3.0                   # weighted pick among eligible entries
```

Starter roster (defaults; a balance pass during implementation is expected):

| id | name | hull | dps | maneuver | bounty | threat band | weight |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `jackal` | JACKAL | 55 | 5.0 | 0.8 | ◈ 400 | 0.0 – 0.7 | 3.0 |
| `marauder` | MARAUDER | 90 | 7.5 | 1.15 | ◈ 900 | 0.4 – 1.1 | 2.5 |
| `corsair` | CORSAIR | 140 | 10.5 | 1.5 | ◈ 1,900 | 0.8 – 1.6 | 2.0 |
| `dreadwing` | DREADWING | 210 | 14.0 | 1.9 | ◈ 3,800 | 1.3 – 3.0 | 1.0 |

**Spawn rule.** At `Lock` (immediately after `PirateBearing` is rolled),
compute `threat = float64(ast.Risk)/100 * world.PirateMul` (risk spans 8..96;
`pirate_mul` spans 0.70..2.10, so threat spans ≈ 0.06..2.0). Collect roster
entries whose `[min_threat, max_threat]` contains it; if none match, take the
single entry with the nearest band edge. Weighted pick with
`runRNG(s, run, 7100)`; store the result as `run.PirateID`. The pirate is
rolled at Lock — before the player has seen anything — so the same seed and
belt always produce the same hunter, and the radar's approaching blip *is*
that pirate. Higher difficulty ⇒ higher bounty by construction: threat bands
put tougher entries on riskier rocks.

### Encounter state machine

One new phase, `PhaseCombat`, appended after `PhaseEscaping`:

```
PhaseMining ──(distance 0, EMP resolved)──> rollPirateAction
    ├─ tribute roll (0.55) ──> PhaseTribute
    │      ├─ [D] AcceptTribute ──> PhaseEscaping (no attack)        (unchanged)
    │      ├─ [R] RefuseTribute ──> PhaseCombat, burn RUNNING
    │      ├─ [F] FightPirates  ──> PhaseCombat, burn NOT started    (armed only)
    │      └─ 12 s timeout ──> RefuseTribute                          (unchanged)
    └─ attack ──> PhaseCombat directly
           ├─ armed:    burn NOT started
           └─ unarmed:  burn RUNNING (preserves today's auto-escape)

PhaseCombat
    ├─ autocannon damage applies continuously (when installed)
    ├─ [F] FireWeapons (manual weapon, not overheated)
    ├─ [B]/[Enter] CombatEscape — starts the burn if not started
    │      (BAIL/DEPART intent by RunDepleted, exactly like BailOrDepart)
    ├─ burn completes ──> resolveRun(OutcomeEscapedUnderFire)
    ├─ pirate hull ≤ 0 ──> resolveRun(OutcomePirateDestroyed)
    │      └─ cargo sealed; run summary shown immediately
    └─ own hull ≤ 0 ──> resolveRun(OutcomeShipLost)                   (unchanged)
```

**Burn matrix** (the one Fight-vs-Run difference, per design): the escape
burn is pre-started on entry to combat for **RefuseTribute** (any loadout)
and for **immediate attack against an unarmed ship** (so weaponless players
are never worse off than today's auto-escape — they see the combat screen,
but their ship is already running). It is *not* started for **Fight** or for
**immediate attack against an armed ship**; those players are standing their
ground until they press B/Enter, exactly like choosing when to leave a rock
while mining. `configureEscape` is `startEscape`'s duration math extracted so
it can run without switching phase (escape seconds are computed at burn
start, from cargo load at that moment, same formula).

While in PhaseCombat: mining is paused (like tribute), the active skill check
is cleared and no new skill checks roll, and **no new random events roll**
(active events keep ticking down; their fuel/escape effects still apply).
Combat has enough going on.

### Combat state (transient — `ActiveRun` is never persisted)

```go
// state.go
PirateID string       `json:"pirate_id,omitempty"` // rolled at Lock
Combat   *CombatState `json:"combat,omitempty"`    // non-nil during/after combat start

type CombatState struct {
    PirateName     string   // denormalized for rendering
    PirateHull     float64
    PirateMaxHull  float64
    PirateDPS      float64  // roster dps * world.PirateAttackMul (default 1.0)
    Bounty         int
    OddsPct        int      // EstimateOdds at combat start, for the header
    Bearing        float64  // 0..1 blip angle, sim-computed each tick
    Range          float64  // 0..1 normalized range, sim-computed each tick
    Solution       float64  // 0..1 firing solution, sim-computed each tick
    Heat           float64  // 0..HeatCapacity(ship), shared ship capacitor
    LockRemaining  float64  // > 0 ⇒ weapons locked (overheat)
    Phase1, Phase2 float64  // maneuver phase offsets, rolled once at combat start
    EscapeStarted  bool     // burn matrix above
    Log            []string // last ~6 engagement lines, newest first
    ShotsFired     int
    ShotsHit       int
}
```

On `State`: `BountyVouchers int` (unredeemed credits). On `Stats`:
`PiratesDestroyed int`, `BountyCreditsEarned int`. On `RunRecord`:
`PirateDestroyed string`, `BountyEarned int` (both omitempty). `StateVersion`
stays 6 — every new field is additive with correct zero values.

### Combat model (all in `internal/sim/combat.go`)

**Maneuver — deterministic, no per-tick RNG.** With `t = float64(run.TickCount)
/ TickHz`, `m = pirate.maneuver`, and phases rolled once at combat start
(`runRNG(s, run, 9100)`, uniform `[0, 2π)`):

```
Bearing = wrap01(arc_center + amp1·sin(2π·w1·m·t + Phase1)
                            + amp2·sin(2π·w2·m·t + Phase2))
Range   = range_min + (1 − range_min)·|sin(2π·w3·m·t + Phase1)|
```

A pure function of `TickCount` ⇒ identical replays, and the 4 Hz snapshot
stream carries authoritative positions.

**Firing solution**, recomputed every tick and shown live:

```
arcFactor   = clamp(1 − |angDist01(Bearing, arc_center)| / arc_half_width, 0, 1)
rangeFactor = solution_range_floor + (1 − solution_range_floor)·(1 − Range)
Solution    = clamp(arcFactor·rangeFactor + Σ installed weapon solution_bonus, 0, 1)
```

**Fire — `FireWeapons(s, c, now)`.** Valid only in PhaseCombat with a manual
weapon (Mass Driver or Pulse Laser) and `LockRemaining ≤ 0`; otherwise a typed
error (TUI flashes it). One press fires all installed **manual weapons** as one volley:
`damage = Σ WeaponDamagePerShot(item, grade)`, `heatCost = Σ
WeaponHeatPerShot(item, grade)`. Single hit roll `runRNG(s, run,
10000+run.TickCount).Float64() < Solution` — full damage on hit, nothing on
miss; heat is spent either way. `Heat += heatCost`; if `Heat ≥
HeatCapacity(ship)` (base 100 plus any internal Heat Sink)
→ `LockRemaining = overheat_lock_seconds` and a "CAPACITORS OVERHEATED" log
line. Latency note: the roll uses the solution at the tick the action is
*processed*, not when the key went down — arcs are deliberately generous
(SSH round-trips are part of the terrain; same reasoning as the skill-check
design in gameplay/02).

**Per tick — `tickCombat(s, c, run, dt, now)`:** recompute Bearing/Range/Solution;
`Heat = max(0, Heat − heat_decay_per_second·dt)`; `LockRemaining =
max(0, LockRemaining − dt)`; installed autocannons deal their deterministic
`damage_per_shot × shots_per_second × dt` directly to pirate hull; incoming
fire `applyAttackDamageRate(s, c, run, Combat.PirateDPS, dt)` (shield absorbs
first, remainder to hull); if `EscapeStarted`, run the escape
bookkeeping shared with `tickEscape` (elapsed, escape fuel drain, fuel-out
penalty — factored into a helper so the damage line isn't double-applied).
All math must clamp for huge `dt` (EmergencyResolve fast-forwards in one
giant tick) — no NaN, no negative pools.

**Winning — `winCombat`:** `s.BountyVouchers += Bounty`; bump stats; event
log `pirate_destroyed`; stash name/bounty for the `RunRecord`; then resolve
the run as `OutcomePirateDestroyed`. Cargo is sealed, the asteroid remnant is
updated by the ordinary run-resolution path, and the TUI goes directly to the
run summary. This prevents a finished fight from returning to mining with a
zero-distance pirate contact.

**Bounty vouchers are not cargo.** They are registered kill confirmations:
they live on `State`, **survive ship loss**, and convert to credits only at
dock — `SellCargo` (the C key) becomes "sell cargo **and redeem bounties**":
gate `s.CargoValue > 0 || s.BountyVouchers > 0`, add vouchers to the credit
payout and the DOCK SALE run-log record, then zero them. Deliberate carve-out
from the "cargo is not money" doctrine: the fight risked the ship *now*; the
reward should not also evaporate with it. (Docking is still required, so a
kill never pays out mid-run.)

### Weapon catalog rework (`slots.go` + `[slots]`)

`weaponItems = [turret, mass_driver, pulse_laser]`. `AttackDamageMul` is
deleted; new helpers `WeaponDamagePerShot`, `WeaponHeatPerShot`,
`WeaponSolutionBonus`, `HasWeapon(s) bool`, and
`AutocannonDamagePerSecond(s, c)`. Grades work exactly like every other device
(price = base · `grade_price_curve`^grade; power = `power_k`; mass per
grade). Damage/heat per grade g (0..5 = E..S):

| Item | Name | dmg/shot | heat/shot | sol. bonus | base ◈ | power_k | mass/grade | Profile |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `turret` | AUTOCANNON TURRET | 8 + 3g | — | — | 700 (existing) | 1.6 (existing) | 7 (existing) | fires continuously at 0.82 shots/s |
| `mass_driver` | MASS DRIVER | 16 + 6g | 45 | 0 | 1100 | 2.2 | 10 | alpha strike, heat-hungry |
| `pulse_laser` | PULSE LASER | 5 + 2g | 12 | +0.10 | 900 | 1.8 | 5 | sustained, accurate |

Only the Warden (2 weapon slots) and Mule (1) can fight at all — the class
identity gameplay/05 promised the "fighter" finally lands. A single shared
heat capacitor across both Warden slots is deliberate v1 simplicity;
per-weapon heat is a future tune, not this task.

The internal **Heat Sink** raises that shared capacity by `25*(g+1)` at grade
`g` (E..S), from 125 at E through 250 at S. It does not increase decay, so it
extends burst firing without changing sustained-fire recovery.

### EST. ODDS — `EstimateOdds(s, c, pirate) float64`

Deterministic, content-only (no RNG), exported for the tribute prompt and
combat header. It is an honest *estimate*, not a promise:

```
avgSol     = clamp(odds_avg_solution_base / pirate.maneuver, 0.15, 0.90)
manualDPS  = Σ dmg/shot · (heat_decay_per_second / Σ heat/shot) · avgSol
autoDPS    = Σ turret dmg/shot · turret_shots_per_second
pDPS       = manualDPS + autoDPS
ttkPirate  = pirate.hull / max(pDPS, ε)
ttkSelf    = (Hull + ShieldHP) / (pirate.dps · world.PirateAttackMul)
odds       = clamp(ttkSelf / (ttkSelf + ttkPirate), odds_floor, odds_ceiling)
```

Unarmed ⇒ 0 (and no FIGHT button). Worked example: stock Skiff can't fight;
a Warden with a C autocannon (14 dmg, 0.82 shots/s) deals 11.5 pDPS
continuously ⇒ ttkPirate ≈ 8 s; 140 hull + 90 shield vs 7.5 dps ⇒
ttkSelf ≈ 31 s ⇒ **EST. ODDS 66%**. On the always-attack Eridani worlds
`pirate_attack_mul` (3.0–3.5) multiplies pirate DPS, so odds there collapse
toward the floor — fighting a Dreadwing over Sable is correctly advertised
as suicide.

### Determinism & EmergencyResolve

New salts (registry: 4000+tick events, 5000+tick surge, 6500+tick skill
checks, 7000, 9001 pirate action): **7100** roster pick (at Lock), **9100**
maneuver phases (combat start), **10000+TickCount** fire hit roll. Pirate
damage is a constant rate — no roll. Replay contract: same seed + same
action-at-tick script ⇒ identical combat transcript, outcome, and voucher
total (hard test requirement).

`EmergencyResolve` gains `case PhaseCombat:` — if `!Combat.EscapeStarted`,
call `CombatEscape` first; then the existing fast-forward
(`TickRun(remaining+1)`) resolves it. Combat damage keeps applying during
the fast-forward until the first terminal event, so **disconnecting mid-fight
can still kill the ship**. A victory may resolve `OutcomePirateDestroyed`; an
escape resolves `OutcomeEscapedUnderFire`. The existing anti-infinite-loop
clamp covers the phase once the completion check accepts combat-with-burn.

### Save impact

None structural. State is a JSON blob; all new fields are additive
`omitempty` with correct zero-value semantics, so `StateVersion` stays 6 and
old saves load unchanged. Live saves with installed turrets change behavior
(passive → active) — release-note, no data migration.

### TUI spec

**COMBAT MODE interstitial.** Every entry into PhaseCombat (Fight, Refuse,
immediate attack) first shows a full-screen **amber flood card** — the
death screen's `renderDeathFinal` pattern (`lipgloss.Place` +
`WithWhitespaceStyle`) with `hueColors(HueAmber)` (fg `#ffae3f`, bg
`#3a2208`): big **"COMBAT MODE"**, subtitle `☠ MARAUDER — BOUNTY ◈ 900`,
dim hint line. **Solid pop, no flicker** — nothing like the death strobe.
Hold 2 s via a one-shot `tea.Tick(2*time.Second)`; any key skips; under
`Settings.ReducedMotion` the hold is skipped entirely. Detect the phase
transition in both the `snapMsg` handler and `refreshSnap` (so the player's
own F/R keypress triggers it synchronously). While the card is up, the sim
keeps ticking — the 2 s is presentation, not protection; with the burn
matrix above, nobody is eating damage they wouldn't have eaten anyway.

**Tactical scope screen** (replaces `renderEscape` whenever
`Phase == PhaseCombat`; rendered from `renderMining`'s dispatch):

```
─ COMBAT ─ K-TYPE MARAUDER ─ BOUNTY ◈ 900 ─ EST. ODDS 66% ─────

  ┌ TACTICAL SCOPE ──────────────┐   YOU
  │        ·      ●              │   HULL   [█████████░░░]  71%
  │   ·        ╱                 │   SHIELD [███████░░░░░]  55%
  │        ╱  firing arc         │   HEAT   [███░░░░░░░░░]  LOW
  │     ▲━━━━━━━━━╲              │
  │        ╲       ·             │   MARAUDER
  │   ·        ·                 │   HULL   [████░░░░░░░░]  34%
  └──────────────────────────────┘   RANGE  410m  CLOSING

  [F] FIRE when the blip crosses your arc — solution 78%
  Overheating locks weapons for 4s.

  > Direct hit! Marauder hull -16
  > Marauder returns fire — shield absorbs 9

  [B] BAIL  ·  [ENTER] DEPART (begin escape burn)
```

- Scope ≈ 34×10 (a bigger sibling of the 16×6 `renderPirateRadar`): player
  `▲` anchor, firing-arc rays brightening with `Solution`, pirate blip `●`
  plotted from sim `Bearing`/`Range`, dim `·` starfield.
- Right column: player HULL/SHIELD (reuse the mining screen's bar helpers),
  an AUTOCANNON DPS readout when fitted, and a HEAT bar only when a manual
  weapon is fitted; pirate name + hull bar; `RANGE ~410m` cosmetic meters
  (`120 + Range·680`).
- `SOLUTION nn%` line tracks `Combat.Solution`; last 3 `Combat.Log` lines.
- When `EscapeStarted`: the escape-burn progress bar (reuse `renderEscape`'s
  bar block) appears below the log, and the keybar drops B/Enter for
  `ESCAPE BURN nn%`.
- Autocannon-only: `AUTOCANNON ONLINE — CONSTANT DAMAGE n.n DPS`, without an
  F key. Unarmed: no FIRE line/HEAT bar; instead a flashing
  `NO WEAPONS — ESCAPE BURN RUNNING` banner (their burn auto-started).
- Keys (`keyMining`, PhaseCombat case): `f` → `sess.FireWeapons`,
  `b`/`esc`/`enter` → `sess.CombatEscape`. Tribute phase adds `f` →
  `sess.FightPirates` when `sim.HasWeapon`. Hitboxes: `btn:combat:fire`,
  `btn:combat:escape`, `btn:tribute:fight` wired in `updateClick` like the
  existing tribute buttons.
- **Animation contract:** a 100 ms `tea.Tick` chained only while in
  PhaseCombat interpolates the blip *cosmetically* between the last two 4 Hz
  snapshots (must tolerate dropped snapshots; skipped under ReducedMotion).
  Solution %, heat, hulls, and every number the player acts on render from
  the latest snapshot only — the TUI never invents gameplay state.

**Tribute prompt** (`renderTribute`): add the pirate's name to the
transmission copy, plus a bounty/odds line and — when armed — a third
button: `[F] FIGHT — EST. ODDS 66%` (`theme.Button(..., theme.HueRed)`).

**Elsewhere:** chart's PORT SERVICES sell button becomes
`[C] SELL CARGO + BOUNTY ◈ <cargo+vouchers>` (enabled when either > 0) with
a `BOUNTY ◈ n` line; run summary shows
`PIRATE DESTROYED — MARAUDER, BOUNTY ◈ 900 (voucher held)`; help overlay and
onboarding copy gain the combat keys.

## Balance defaults — `data/balance.toml`

```toml
[combat]
heat_capacity = 100.0
heat_decay_per_second = 18.0
overheat_lock_seconds = 4.0
arc_center = 0.5            # bearing 0..1; the arc points "up" from the anchor
arc_half_width = 0.18       # generous on purpose — SSH latency is part of the game
solution_range_floor = 0.35
range_min = 0.35
maneuver_w1 = 0.11          # Hz, sum-of-sines blip path (scaled by pirate maneuver)
maneuver_w2 = 0.043
maneuver_w3 = 0.07
maneuver_amp1 = 0.30
maneuver_amp2 = 0.18
odds_avg_solution_base = 0.55
odds_floor = 0.02
odds_ceiling = 0.98

[slots]  # deltas only
# removed: turret_pct_per_grade
shield_hp_per_grade = 30            # was 20 — sole mitigation layer now
turret_damage_per_shot_base = 8.0
turret_damage_per_shot_per_grade = 3.0
turret_shots_per_second = 0.82
mass_driver_base_price = 1100
mass_driver_power_k = 2.2
mass_driver_mass_per_grade = 10
mass_driver_damage_per_shot_base = 16.0
mass_driver_damage_per_shot_per_grade = 6.0
mass_driver_heat_per_shot = 45.0
mass_driver_solution_bonus = 0.0
pulse_laser_base_price = 900
pulse_laser_power_k = 1.8
pulse_laser_mass_per_grade = 5
pulse_laser_damage_per_shot_base = 5.0
pulse_laser_damage_per_shot_per_grade = 2.0
pulse_laser_heat_per_shot = 12.0
pulse_laser_solution_bonus = 0.10
heat_sink_base_price = 1000
heat_sink_capacity_per_grade = 25.0
```

Content validation (`internal/content`): roster non-empty; unique ids;
`hull/damage_per_second/maneuver/weight > 0`; `bounty ≥ 0`;
`0 ≤ min_threat ≤ max_threat`; the union of bands must cover the achievable
threat range for shipped worlds (warn-level acceptable for gaps at the
extremes as long as nearest-band fallback exists). Combat: capacities,
decay, lock positive; `arc_half_width ∈ (0, 0.5]`; odds clamps ordered.

## Acceptance criteria

- [ ] Deterministic replay: same seed + same action-at-tick script ⇒
  identical combat transcript (positions, hit rolls, heat), outcome, and
  voucher totals.
- [ ] Roster pick at Lock is threat-band-correct and deterministic; the same
  pirate appears on the prompt, interstitial, combat header, and run record.
- [ ] `pirates_always_attack` worlds enter PhaseCombat with no prompt; burn
  pre-started iff unarmed (burn matrix holds in all five entry paths).
- [ ] FIGHT is offered only when a weapon is installed; `FightPirates`
  without one is a typed error; tribute timeout still auto-refuses into
  combat-with-burn.
- [ ] Autocannon: pirate hull falls continuously at the deterministic fitted
  turret DPS; it consumes no heat and needs no F key.
- [ ] Manual fire: heat accrues on hit and miss; `Heat ≥ capacity` locks for
  `overheat_lock_seconds` and decays back; firing while locked is an error,
  not a crash.
- [ ] Kill: voucher credited, cargo sealed, and `OutcomePirateDestroyed`
  opens the run summary immediately; no further pirate action can roll.
- [ ] Hull 0 in combat ⇒ `OutcomeShipLost` exactly as today (cargo zeroed,
  ship destroyed, respawn) — but `BountyVouchers` survive.
- [ ] `SellCargo` pays cargo + vouchers, zeroes both, logs one DOCK SALE
  record including `BountyEarned`; works with vouchers and empty hold.
- [ ] `EmergencyResolve` from PhaseCombat (both burn states) always
  terminates: pirate-destroyed, escaped-under-fire, or ship-lost — never a
  hang (fuel-out clamp covered by test).
- [ ] `EstimateOdds`: 0 unarmed; monotone up in weapon grade/shield/hull;
  monotone down in pirate hull/dps/maneuver; clamped to [floor, ceiling].
- [ ] Interstitial shows on every combat entry, holds ~2 s, any-key skip,
  no hold under ReducedMotion; combat screen numbers come from snapshots
  only (interpolation is cosmetic and drop-tolerant).
- [ ] Existing suites updated: no references to `AttackDamageMul` /
  `turret_pct_per_grade`; refuse-tribute tests assert PhaseCombat.
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` clean; live drive
  over real ssh (tmux) covering fight-win, fight-die, run-under-fire,
  unarmed-always-attack, disconnect mid-combat.

## Out of scope / handoffs

- Per-weapon heat pools, weapon-specific fire keys, pirate shields/EMP use,
  multi-pirate encounters — future tuning docs.
- Bounty boards, wanted levels, faction reputation — future design.
- Rebalancing tribute chance/demand or world pirate multipliers beyond the
  shield buff — flag findings during the implementation balance pass instead
  of folding them in silently.
