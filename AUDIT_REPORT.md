# Moon Miner beta — Go code and test audit

Audit date: 2026-09-06 (America/Boise). Branch: `beta`.

The audit fixed confirmed defects in session ownership, disconnect resolution,
save handling, configuration validation, snapshot delivery, and UI input. It
also replaced JSON-based snapshot copying with explicit deep copies and
established a test-first development workflow.

This report is intentionally untracked. Changes remain in the worktree; no
commit, push, merge, or deployment was performed.

## Scope and method

Reviewed the runtime boundaries from configuration/content loading through SSH
identity and limits, save-manager/actor/session ownership, SQLite persistence,
simulation actions and derived values, and TUI input/rendering. Particular
attention went to run phases, escape/death, ship condition, inventory changes,
reconnect behavior, and the tests protecting those contracts.

Started from a clean worktree and ran the existing suite with coverage. Added
focused regression tests and observed their failures before implementing the
fixes. Reviewed affected existing tests rather than weakening assertions to fit
the new behavior. Finished with build, vet, full tests, race detection, a bounded
save-decoder fuzz run, and before/after snapshot benchmarks.

This is a code and automated-test audit, not proof that every possible defect
has been eliminated. Remaining coverage and operational limits are listed below.

## Bugs solved

| Finding | Consequence before the fix | Resolution |
| --- | --- | --- |
| Replaced sessions retained access to the actor | After takeover, the old terminal could still read the save or submit actions | Both action and snapshot requests verify ownership on the actor goroutine and return `ErrSessionClosed` for stale sessions |
| Snapshot listeners never closed | A detached/replaced terminal could leave a listener waiting indefinitely | Closing a session also closes its snapshot stream |
| LastSeen changes were not marked dirty | An idle reconnect/disconnect lost its new timestamp | Attach marks the updated state dirty; a SQLite round-trip test verifies persistence |
| Failed disconnect writes discarded the actor | The only current copy of unsaved progress could be removed from memory | Failed saves retain their actor for autosave/reconnect retry; unowned actors stop simulation ticks |
| Shutdown swallowed final-flush errors | Callers received success despite a failed database write | Actors retain final-flush errors and the manager returns their aggregate |
| Emergency escape used a giant tick and a broken forced-finish fallback | Fuel-out penalties could leave the run unresolved; coarse stepping also differed from live combat/event behavior | Emergency resolution uses the normal tick interval and honors growing escape requirements |
| Missing-asteroid emergency resolution relabeled old history | An unrelated prior run became a disconnected run and generated a misleading notice | Notices and historical flags require an actual new outcome |
| Null parked ships crashed save decoding | A JSON hangar entry such as `"parked": null` caused a nil-pointer panic | Invalid parked entries are removed without replacing a valid active ship |
| `Lock` omitted scanner range validation | A scanned but unreachable asteroid could bypass scanner progression | `Lock` applies the same range predicate as scanning, before charging fuel |
| Render-facing run predicates mutated accounting | Asking whether a run was depleted/full modified the snapshot and could panic on a nil run | Predicates use read-only accounting helpers and handle absent runs |
| Tick snapshots could arrive after newer action results | A delayed mining snapshot could overwrite a docked state and reopen mining | Actor snapshots carry increasing revisions; the TUI rejects older snapshots |
| Outcome consumption could precede the completed snapshot | The summary could be entered while the displayed snapshot still represented an active run | Consumption waits for a snapshot with no active run |
| Ctrl+C was blocked by minimum-size input gating | A player could not quit normally from the resize-required screen | Quit is handled before overlay and viewport gating |
| `DEV_MODE=false` enabled dev tools | Any non-empty value activated the development mode | Parse a boolean; false/0 disable it and malformed values reject startup |
| IPv6 listen addresses were malformed | A host such as `::1` became `::1:2222` | Use `net.JoinHostPort`, producing `[::1]:2222` |

Primary implementation files: `internal/game/{manager,actor,session}.go`,
`internal/sim/{state,run,belt}.go`, `internal/tui/game.go`, and
`internal/config/config.go`.

## Edge cases solved

- Scanner range is inclusive at the exact boundary, including seismic
  pre-scans. An out-of-range lock returns the expected error without changing
  fuel, run state, or other save values.
- Encoding invalid persistent numeric state returns an error instead of
  dereferencing a failed JSON-based clone. Encoding also strips populated
  transient run/scan/dev state without mutating its source.
- Configuration rejects non-finite connection rates, non-positive burst/IP
  cache sizes, and non-positive idle timeouts.
- Content loading rejects unknown TOML keys, catching misspelled tuning fields
  instead of silently substituting zero values.
- All current floating-point content fields, including nested slices, are
  checked for NaN/infinity once at startup. Ordinary range comparisons alone
  did not reject NaN.
- Content validation rejects empty asteroid-name pools, invalid volume bounds,
  zero/negative volume/value steps, invalid drill divisors/ranges, negative scan
  cost, invalid starting resources, and tick rates beyond timer resolution.
- Save-decoder fuzz seeds cover valid saves, null parked ships, future versions,
  and malformed JSON. Successful decodes must yield an active ship and remain
  encodable.

## Performance improvements

`State.Clone` previously marshaled the entire state to JSON and unmarshaled it
for every snapshot. It now copies value fields directly and explicitly clones
all nested maps, slices, pointers, devices, records, scan state, and combat/run
state. `Encode` now uses a shallow envelope copy and one marshal; its edits are
limited to top-level mirrors and transient fields. Tick code also avoids
building snapshots without a receiving session.

A representative fixture includes a generated belt, 20 history entries,
installed/stored devices, and nested active-run data. Measurements used
`BenchmarkStateClone`, three runs per implementation, Go 1.27.1 on Apple M4 Pro
(darwin/arm64):

| Measurement | Before | After |
| --- | ---: | ---: |
| Median time per clone | 45,574 ns | 1,270 ns |
| Allocated bytes per clone | approximately 26,473 | 8,560 |
| Allocations per clone | 95 | 50 |

That is approximately **36× faster**, **68% fewer allocated bytes**, and **47%
fewer allocations** for this fixture. These are microbenchmark results, not a
claim of equivalent improvement in total server throughput. No timing threshold
was added to unit tests. Clone tests verify isolation across all current
reference-bearing fields.

## Inconsistencies corrected

- `RunDepleted` documentation now distinguishes exhausted rock from a full hold.
- The bail API comment now describes rejection outside the mining phase.
- The session acquisition comment correctly says buying a ship does not activate
  it; switching is a separate action.
- Seismic pre-scans and normal scans share the inclusive range boundary.
- Session lifecycle docs distinguish takeover, which transfers a running actor,
  from detach/shutdown, which resolves an abandoned run.
- Test-plan documentation no longer implies tests may wait until Phase 4.
- Test documentation identifies pinned TOML fixtures as a future coverage target,
  uses distance as belt ordering, and places replay persistence checkpoints at
  persistable boundaries rather than demanding mid-run serialization.
- SSH test docs accurately describe local loopback networking rather than
  claiming no network access is needed.

## Tests added and improved

Added **25 top-level tests**, **one fuzz target**, and **one benchmark**, with
additional table-driven cases:

| Package | Added coverage |
| --- | --- |
| `config` | Defaults, invalid limits, explicit dev booleans, IPv6 address formatting |
| `content` | Embedded content acceptance, unsafe numeric/generation inputs, unknown TOML fields |
| `game` | Stale-session rejection and stream closure, LastSeen persistence, refuse policy, failed shutdown flush, increasing snapshot revisions, retained failed disconnect saves |
| `server` | Real public-key SSH authentication, password rejection, exact no-PTY response including CRLF, per-key/global limit accounting and release |
| `sim` | Deep-copy equality/isolation, non-vacuous transient encoding checks, encoding errors, null parked saves, scanner boundaries, read-only predicates, fuel-out emergency escape, accurate disconnect history, full-state emergency/live escape parity across all four phases |
| `tui` | Delayed snapshot rejection and quitting below minimum viewport size |

Existing tests were also audited and tightened:

- Checked previously discarded departure errors throughout `run_test.go`.
- Made the low-hull event and prior-cargo tribute fixtures explicitly reachable;
  their purpose is independent of scanner progression.
- Pinned tribute probability and isolated depletion from pirate arrival; replaced
  conditional skips with failures when required scenarios are not reached.
- Fixed new session seeds in the game test helper and registered manager cleanup
  before closing its database.
- Strengthened the ticker-starvation test to require the target to become scanned,
  rather than accepting disappearance of the scan alone.
- Replaced a shipyard cap-dependent skip with an explicit fixture failure.

Selected package coverage, measured with `go test ./... -coverprofile=...`:

| Package | Baseline | Final |
| --- | ---: | ---: |
| config | 0.0% | 80.8% |
| content | 0.0% | 59.9% |
| game | 65.2% | 70.5% |
| sim | 74.7% | 76.3% |
| tui | 72.9% | 73.2% |
| server | Baseline listener blocked by sandbox | 36.2% |

Coverage describes executed statements, not test quality or correctness. For
example, integration coverage remains materially lower than simulation coverage.

## Documentation workflow delivered

Added [`docs/tests/README.md`](docs/tests/README.md), linked from the root README,
the build-plan index, and all three numbered testing documents.

The workflow requires:

1. Define observable behavior and audit affected existing tests.
2. Write a focused acceptance/regression test and observe its meaningful failure.
3. Implement, re-run the focused tests, and refactor while green.
4. Review fixtures, independent expectations, rejection atomicity, asynchronous
   cleanup, copy ownership, migrations, and TUI semantics.
5. Run build, vet, full tests, and race detection; use benchmarks/fuzzing where
   relevant and document evidence in the PR.

Framework docs also record the new configuration checks and session persistence
semantics. Repository guidance against pushing directly to `main` still applies.

## Validation completed

- `go build ./...` — passed.
- `go vet ./...` — passed.
- `go test ./... -count=1 -coverprofile=/tmp/moonminer-audit-final.cover` — passed.
- `CGO_ENABLED=1 go test -race ./... -count=1` — passed.
- `go test ./internal/sim -run '^$' -fuzz '^FuzzDecodeState$' -fuzztime=10s -parallel=2`
  — passed, 359,691 executions in approximately 11 seconds.
- `go test ./internal/sim -run '^$' -bench BenchmarkStateClone -benchmem -count=3`
  — passed before and after the performance change.
- `git diff --check` — passed.

Commands used `GOCACHE=/tmp/moonminer-go-cache` because the sandbox restricts the
normal build cache. SSH tests ran with permission to bind local ephemeral ports.
Build succeeded while emitting a non-fatal module metadata-cache write warning.
An intermediate unused test import was corrected before the final passing vet,
full-suite, and race runs.

## Remaining limits and follow-up work

- No Docker/Litestream restore drill or live SSH visual playthrough was performed.
  Store schema, durability scripts, deployment configuration, and Windows PTY
  rewriting were not changed. The race run covers this Mac host, not a Windows
  runtime or the production Linux host.
- The broader documented proxy/PTY/session integration matrix is not complete.
  Identity unit tests pass, but real routed-session identity, shutdown under
  transport failure, and complete terminal interaction warrant further tests.
  App boot/signal handling and much of the theme package remain lightly tested.
- Failed-write actors remain cached until reconnect/shutdown, even if autosave
  later succeeds. This preserves retryable state but can retain memory after
  a large database outage. A future eviction design should preserve retry and
  concurrent attach guarantees.
- The manager still holds a global lock across portions of attach/detach I/O;
  slow persistence can delay unrelated pilots. Measure multi-pilot contention
  before redesigning its lifecycle locking.
- Optional kick callbacks still execute synchronously during actor operations.
  Production SSH attaches currently supply nil; any future callback must avoid
  blocking or re-entering manager/session methods without a dispatch redesign.
- Emergency resolution has a 10,000-step CPU safety budget. Pathological states
  that exhaust it remain unresolved and are refused by detached persistence;
  shutdown reports failure rather than manufacturing a successful escape.
- Strict TOML validation is intentionally more restrictive: external overrides
  containing misspelled or obsolete keys now fail startup. Audit those files
  before deploying. Numeric range validation is strengthened, not exhaustive
  validation of every possible extreme balance configuration.
- No new gameplay systems, balance redesign, dependency upgrades, or broad
  architectural rewrites were included. The report does not claim comprehensive
  security certification or production load-test results.
