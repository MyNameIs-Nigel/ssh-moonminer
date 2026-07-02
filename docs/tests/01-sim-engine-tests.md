# Tests 01 — Simulation Engine Test Suite

**Area:** Tests · **Phase:** 4 (start any time after gameplay/01 lands) ·
**Depends on:** gameplay/01–04 · **Parallel-safe with:** everything

## Goal

Prove the engine is deterministic, prototype-faithful, and balance-sound.
The per-task acceptance tests already cover basics; this task builds the
deeper suite: determinism harness, statistical distribution checks,
property/fuzz tests, and the balance invariants from gameplay/03 — the
suite a balance-tuning agent will rely on to not break the game.

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

Tests load `testdata/*.toml` (a snapshot of the shipped numbers), so balance
tuning in `data/` fails invariant tests *deliberately* rather than silently
shifting every unit expectation. Invariant tests run against **both** the
fixture and the live `data/` files; unit-math tests run against the fixture
only.

### Determinism harness

- `Script` helper: `(seed, []Action)` → final state hash, where Action is a
  tagged union (Depart/Lock/Tick(dt)/Overdrive/Bail/Refuel/Buy…).
- Golden test: three pinned scripts (a clean-run session, a raided session,
  a broke-pilot-insurance session) with committed final-state JSON goldens.
  Any formula change shows up as a readable golden diff.
- Cross-check: same script executed twice in one process and via
  encode→decode mid-script produces identical hashes (persistence can't
  fork reality).

### Distribution tests (belt generation)

10,000 belts per world with fixed seeds:

- Field ranges: every generated value within gameplay/01 bounds (vol
  240–4600 step 20, drill 2.2–16, fuel 5–36, risk 8–96, dots 1–5, coords in
  range).
- Tier mix: observed tier frequencies within ±2% absolute of the analytic
  weights per world (e.g. Io legendary ≈ 0.08+0.05·0.4 = 10%; Titan ≈ 20%).
- Ordering: belts sorted by fuel cost; IDs unique; names match
  `^[A-Z]{2}-\d{4}$`.
- Monotonicity spot checks: value strictly increases with tier at fixed
  volume; drill time monotone in volume.

### Property/fuzz tests

- `FuzzTickRun`: random (valid) states + random dt sequences — assert
  invariants: gauges clamped to [0,100], fuel never negative, credits never
  decrease during a run, yield ≤ value, exactly one outcome fires, state
  stays encodable.
- `FuzzDecodeState`: garbage and truncated blobs never panic — error or
  valid state only.
- Action-sequence property: any legal action sequence keeps
  `Credits ≥ 0 && Hull ∈ [0,100] && Fuel ∈ [0, tank]`.

### Balance invariants (from gameplay/03, exact bounds here)

1. **Baseline profit**: median Vesta common rock (analytic: vol 2420, value
   ≈ round(2420·1.15/25)·25) nets ≥ 3× (travelFuel+flightFuel)·9 credits.
2. **Risk pays**: Monte-Carlo 5,000 runs, naive strategy (no overdrive,
   never bail): mean banked credits/run on Ceres > Io by ≥ 25%.
3. **Tank recovery**: mean of 2 median Vesta clean runs ≥ 900 cr.
4. **No stranded belts**: 10k belts, assert min fuel cost per belt ≤ 36 <
   base tank (trivially true — keep as a regression tripwire for balance
   edits).
5. **Upgrade horizon**: total max-out cost within 2.8M–3.6M credits.

Document each with a comment linking back to gameplay/03.

### Performance guardrail

`BenchmarkTickRun` and `BenchmarkGenerateBelt`; assert (in a test, loosely)
that a tick is < 50 µs on the CI box — 100 concurrent miners at 8 Hz must
be negligible load.

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
