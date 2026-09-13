# Test-first development workflow

Use this workflow for new features, bug fixes, balance changes, and updates to
existing features. The numbered test documents describe coverage goals; they
are not a later phase that permits implementation without tests.

## Before implementation

1. Read the authoritative feature spec and identify the observable behavior
   being added or changed. Separate implemented behavior from future designs.
2. Find the affected actions, derived values, persistence paths, session
   messages, and TUI consumers. Search their existing tests before changing code.
3. Write a short test plan in the task or PR: normal behavior, boundary values,
   rejected actions, and relevant disconnect/reconnect or migration behavior.
4. Add or update the smallest regression or acceptance test first. Run it and
   observe the expected failure. A compile failure for a new API is an initial
   step; obtain a behavioral failure once the API exists. Record the failing
   assertion, not merely a red suite caused by setup or environmental errors.
5. Implement the smallest change that satisfies the contract. Re-run the
   focused tests, then refactor while they remain green.

For documentation-only changes, explain why executable tests are unaffected.
For performance changes, capture a representative benchmark before editing and
repeat the same benchmark afterward. Verify behavior and ownership, not just
speed. Avoid timing thresholds in unit tests.

## Audit affected tests when behavior changes

Do not simply update expected output until the suite passes. For every affected
suite, check the following:

- Does the assertion still describe the intended contract? For example, a full
  cargo hold pauses mining but does not mean the asteroid is depleted.
- Does setup actually reach the tested branch? Check action errors immediately;
  do not discard them or let later nil dereferences hide a failed precondition.
- Is the expected result independent of the implementation? Prefer explicit
  boundary outcomes, resource accounting, or a separate replay path over copying
  production formulas into the test.
- Are rejected actions checked for unintended changes to credits, inventory,
  fuel, hull, location, and run state where relevant?
- Could randomness, a changed balance value, or a conditional skip make the test
  pass without exercising its purpose? Pin seeds and the relevant fixture
  values. A broken fixture should fail with an explanation, not skip.
- Are asynchronous completion and cleanup verified? Use channels, bounded waits,
  and actor-owned access. Keep real timing only for scheduler/integration tests;
  do not read live actor state from another goroutine.
- Does a copied snapshot isolate every map, slice, pointer, and nested record?
  Extend clone-isolation tests whenever reference-bearing state fields change.
- Are migration and serialization assertions non-vacuous? Populate transient
  fields before proving they are omitted, and check persistent values after a
  real encode/decode or database round trip.
- For rendering changes, check semantic text, display-cell dimensions, selected
  item visibility, and mouse coordinates. A non-empty string alone proves little.

Review existing tests that fail as well as those that still pass. A passing test
may assert an obsolete contract or never reach the changed code. Remove a test
only when its behavior is deliberately retired, explaining its replacement or
why it no longer applies.

## Fixtures and test layers

Most current tests load embedded content. This catches compatibility with the
shipped game but couples fixtures to balance edits. For focused behavior tests,
load a fresh content instance and explicitly set the small number of values the
scenario depends on. Keep separate tests against untouched embedded content.
The pinned TOML fixture suite in tests/01 is a future coverage target, not a
claim that those fixture files already exist.

- `internal/sim`: supplied timestamps, fixed RNG seeds, deterministic action
  sequences, resource conservation, rejection boundaries, and save migration.
  No filesystem, network, logging, or wall-clock calls in production sim code.
- `internal/content` and `internal/config`: valid defaults, malformed input,
  unknown TOML keys, non-finite values, and invalid operational limits. Tests
  using `t.Setenv` must not run in parallel.
- `internal/game` and `internal/store`: temporary SQLite, session ownership,
  takeover/refusal, autosave, failed writes, disconnect resolution, and shutdown.
- `internal/server` and `internal/identity`: real SSH on local ephemeral ports,
  auth/PTY refusal, direct/proxied identity, and session limits.
- `internal/tui`: input transitions, ordered snapshots, responsive layouts,
  overlays, and hitboxes. Keep integration tests for session-driven outcomes.

## Completion checks

Run these from the repository root:

```sh
go test ./internal/sim -run 'TestAffectedBehavior' -count=1  # substitute the target package/test
go build ./...
go vet ./...
go test ./...
CGO_ENABLED=1 go test -race ./...
```

The race suite is required before a PR. It needs a working C toolchain; ordinary
production builds remain pure Go. SSH tests need permission to bind loopback
ports. If the environment restricts the normal Go cache, use a writable
`GOCACHE`; report environmental blockers rather than skipping checks silently.

Use targeted repeat or fuzz runs when they address a specific concern:

```sh
go test ./internal/game -run 'TestTakenOver|TestIdleAttach' -count=20
go test ./internal/sim -run '^$' -fuzz '^FuzzDecodeState$' -fuzztime=30s
go test ./internal/sim -run '^$' -bench BenchmarkStateClone -benchmem -count=3
```

Changes to Docker, the durability entrypoint, Litestream configuration, or the
store schema also require the restore drills described in the root README.
Coverage percentages are a guide to untested paths, not a correctness target.

The PR should state the contract, the regression test observed failing before
implementation, affected tests audited, checks run, benchmark evidence when
applicable, and any remaining limits. Never push directly to `main`: it deploys.
