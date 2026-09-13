# Tests 01 — Simulation Engine Test Suite

**Area:** Tests · **Phase:** 4 (start any time after gameplay/01 lands) ·
**Depends on:** gameplay/01–04 · **Parallel-safe with:** everything

## Workflow

Apply [the test-first workflow](README.md): audit affected tests, write a failing
behavioral test, implement, and verify. The coverage goals below supplement
feature tests; they do not defer testing until a later phase.

## Goal

Prove the engine is deterministic, revamp-faithful, and balance-sound. The
per-task acceptance tests already cover basics; this task builds the deeper
suite: determinism harness, statistical distribution checks, property/fuzz
tests, death/cargo/station invariants, and the balance invariants from
gameplay/03 — the suite a balance-tuning agent will rely on to not break the
game.

## References

| Source | What to take |
| --- | --- |
| `../ssh-idlefarmer/internal/sim/sim_test.go` | table-driven style, fixture content via `testdata/*.toml` |
| `../ssh-idlefarmer/internal/sim/testdata/` | pattern: tests load a pinned content fixture, not the live `data/` files |
| gameplay/01–04 specs | formulas and invariants to assert |

## Deliverables

- `internal/sim/` — `determinism_test.go`, `distribution_test.go`,
  `invariants_test.go`, `fuzz_test.go`, `testdata/worlds.toml`,
  `testdata/balance.toml` (pinned fixtures)

## Spec

### Fixtures

Target coverage: introduce `testdata/*.toml` (a snapshot of shipped numbers), so balance
tuning in `data/` fails invariant tests *deliberately* rather than silently
shifting every unit expectation. Invariant tests run against **both** the
fixture and the live `data/` files; unit-math tests run against the fixture
only.

### Determinism harness

- `Script` helper: `(seed, []Action)` → final state hash, where Action is a
  tagged union (Depart/Lock/Tick(dt)/BailOrDepart/AcceptTribute/RefuseTribute/
  SellCargo/Refuel/BuyShip/BuyUpgrade/BuyPermit/FundStation/ClaimStationIncome/
  BuyCosmetic/SetCosmetic…).
- Golden test: pinned scripts for a starter cargo sale, a manual depleted
  depart, a tribute-paid escape, an escaped-under-fire run, a ship-loss respawn,
  and a station-income claim with committed final-state JSON goldens. Any
  formula change shows up as a readable golden diff.
- Cross-check: same script executed twice in one process and via
  encode→decode at a persisted boundary produces identical hashes. Active
  runs and scans are intentionally omitted from saves; test their disconnect
  resolution separately rather than demanding a mid-run round trip.

### Distribution tests (belt generation)

10,000 belts per world with fixed seeds:

- Field ranges: every generated value within gameplay/01 bounds (units step,
  mine seconds, risk 8–96, dots 1–5, coords in range, and at least one
  contact within the active Scanner lock).
- Tier mix: observed tier frequencies within ±2% absolute of the analytic
  weights per world (e.g. Io legendary ≈ 0.08+0.05·0.4 = 10%; Titan ≈ 20%).
- Ordering: belts sorted by distance; IDs unique; names match
  `^[A-Z]{2}-\d{4}$`.
- Monotonicity spot checks: value strictly increases with tier at fixed units;
  mine time monotone in units.

### Property/fuzz tests

- `FuzzTickRun`: random (valid) states + random dt/action sequences — assert
  invariants: resource/cargo/radar gauges clamped, fuel/hull/cargo never
  negative, credits do not change during mining until explicit sale/station
  claim, cargo value ≤ asteroid value, exactly one outcome fires, death reset
  preserves credits/permits/stations/cosmetics, state stays encodable.
- `FuzzDecodeState`: garbage and truncated blobs never panic — error or
  valid state only.
- Action-sequence property: any legal action sequence keeps
  `Credits ≥ 0 && Hull ∈ [0,maxHull] && Fuel ∈ [0,tank] && Cargo.Used ∈ [0,capacity]`.

### Balance invariants (from gameplay/03, exact bounds here)

1. **Baseline starter loop**: median Vesta Local common rock, sold at public
   dock after successful escape, funds another run plus progress toward fuel
   tank I.
2. **Risk pays**: Monte-Carlo 5,000 scripted strategies shows locked Sol rare
   destinations beat Vesta Local expected sale value by ≥ 2x after accounting
   for tribute/attack/death losses.
3. **Death preservation**: scripted ship loss clears ship/upgrades/cargo and
   preserves credits, permits, cosmetics, stations.
4. **No unreachable starter belts**: 10k starter belts, assert at least one
   asteroid per belt is reachable by starter skiff reserve.
5. **Station locality**: station refuel/sale modifiers apply in owning system
   only.
6. **Upgrade horizon**: total cost to buy top ship, max one top ship, unlock
   every system, and complete a late-game station is within the documented
   endgame-horizon range.
7. **Low hull danger**: event/bad-event frequency at 25 hull is meaningfully
   higher than at 90 hull under the same seed batch.

Document each with a comment linking back to gameplay/03.

### Performance guardrail

`BenchmarkTickRun` and `BenchmarkGenerateBelt`; assert (in a test, loosely)
that a tick is < 50 µs on the CI box — 100 concurrent miners at 4 Hz must be
negligible load.

## Acceptance criteria

- [ ] `go test ./internal/sim/... -count=2` passes (determinism × repeat).
- [ ] `go test -race` clean; fuzz corpora committed with at least the seed
  corpus; `go test -fuzz=FuzzTickRun -fuzztime=30s` finds nothing.
- [ ] Goldens are human-readable JSON and regenerate via
  `go test -run TestGolden -update` flag pattern.
- [ ] Deliberately breaking a formula (e.g. drop a clamp) fails at least
  one named test — verify once, note in PR.
- [ ] Suite runs in < 60 s locally.

## Out of scope

- Server/store tests → tests/02. TUI → tests/03.
- CI wiring — post-MVP.
