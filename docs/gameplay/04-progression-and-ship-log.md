# Gameplay 04 — Progression, Upgrades & Ship's Log

**Area:** Gameplay · **Phase:** 4 · **Depends on:** gameplay/01–03 ·
**Parallel-safe with:** framework/04, tests/*, tui tasks

## Goal

Give the credits a long-term purpose and the pilot a history. Three systems,
all extensions beyond the HTML prototype (sanctioned by the concept doc):
**ship upgrades** (permanent credit sinks that bend the risk curves),
**lifetime stats**, and the **ship's log** (recent run history). Together
they turn a 5-minute toy into something worth SSHing back into.

## References

| Source | What to take |
| --- | --- |
| [../01-concept-and-story.md](../01-concept-and-story.md) | divergence list item 4 |
| `../ssh-idlefarmer/internal/sim/state.go` | how idlefarmer models upgrade levels + stats in state |
| gameplay/02's `RunRecord` return | the log entry source |

## Deliverables

- `internal/sim/` — `upgrades.go`, `stats.go`
- `[upgrades]` section added to `data/balance.toml`
- Extends `sim.State` (`Upgrades`, `Stats`, `RunLog` — stubs left by
  gameplay/01)

## Spec

### Upgrades

Five tracks, each 0–4 levels, purchasable only while docked at the chart.
Prices rise steeply; effects are multiplicative on the gameplay/02 formulas:

| Track | Flavor | Effect per level (cumulative) |
| --- | --- | --- |
| **DRILL HEAD** | tungsten → plasma | drill rate ×1.15 |
| **FUEL TANK** | drop tanks | tank size +25 (base 100) |
| **HULL PLATING** | whipple shielding | raid hull damage −4 (floor 5) |
| **SIG DAMPER** | reactor baffles | pirate rate ×0.88 |
| **SURVEYOR** | deep scanner | belt size +1 asteroid |

Price curve: `base · 3^level` with bases (drill 600, tank 500, plating 550,
damper 700, surveyor 800) — all in `balance.toml [upgrades]`, alongside the
per-level effect coefficients. Level 0 = stock ship, no effect.

- `BuyUpgrade(state, content, track) error` — requires docked, affordable,
  below max level; typed errors otherwise.
- Effects apply inside existing sim functions (gameplay/01's belt size and
  tank derivations, gameplay/02's rates) via small
  `effectiveX(state, content)` helpers in `upgrades.go` so the touch points
  stay greppable.

### Lifetime stats

```go
type Stats struct {
    RunsTotal, RunsClean, RunsBailed, RunsRaided, RunsStranded int
    CreditsEarned, CreditsSpent int
    LegendariesMined int
    FuelBurned float64
    InsuranceClaims int
    FirstSeen, LastSeen int64 // unix seconds (set by actor attach)
}
```

Updated inside outcome resolution / economy actions (single choke points —
resist scattering `stats.X++` around the codebase).

### Ship's log

`RunLog []RunRecord`, capped at the **20** most recent (drop oldest).
`RunRecord` (finalized here; gameplay/02 introduced it):

```go
type RunRecord struct {
    When      int64
    World     string // "TITAN"
    Asteroid  string // "KR-4711"
    Tier      int
    Outcome   string // clean|bail|raided|stranded
    Banked    int
    DrillPct  int    // progress at resolution
    Overdrove bool   // was overdrive used at all
}
```

### Consumers (informative)

tui/02 renders stats + log on a Ship's Log screen reachable from the chart
(`L` key); the Run Summary screen shows the newest record. No TUI work in
this task — but keep the structs render-friendly (exported, plain types).

## Acceptance criteria

- [ ] Each upgrade track measurably changes its target formula in a
  table-driven test (e.g. damper level 2 ⇒ pirate rate ×0.88² within
  epsilon; surveyor level 3 ⇒ 10-rock belts).
- [ ] Price curve and max-level enforcement tested; buying while in a belt
  fails.
- [ ] Stats counters advance correctly across a scripted session touching
  all four outcomes + a refuel + an upgrade purchase.
- [ ] RunLog caps at 20 with FIFO eviction; survives encode/decode.
- [ ] Balance sanity: total cost to max everything (≈ 3.2M credits) is
  documented in `balance.toml` comments as the "endgame horizon."

## Out of scope / handoffs

- Upgrade shop UI + Ship's Log screen → tui/02 (extend, or a follow-up TUI
  task if it outgrows that doc).
- Global leaderboards / cross-pilot anything — deliberately post-MVP (would
  need store queries across fingerprints; note it in code TODOs only).
