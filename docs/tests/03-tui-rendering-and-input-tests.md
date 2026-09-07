# Tests 03 — TUI Rendering & Input Tests

**Area:** Tests · **Phase:** 4 (start after tui/01; extend per screen) ·
**Depends on:** tui/01–06 · **Parallel-safe with:** tests/01, tests/02

> **v1.7 test authority:** the dimensions, screen states, modes, allocator
> cases, clipping assertions, overlay constraints, and hitbox checks in
> [../tui/06-responsive-menu-overhaul.md](../tui/06-responsive-menu-overhaul.md)
> are mandatory. This PR intentionally introduces compilable failing
> specification tests and no production implementation; the follow-up layout
> PR makes them pass before refreshing goldens.

## Workflow

Apply [the test-first workflow](README.md): audit affected tests, write a failing
behavioral test, implement, and verify. The coverage goals below supplement
feature tests; they do not defer testing until a later phase.

## Goal

Test the TUI as a pure function: feed the model synthetic messages (keys,
mouse, ticks, snapshots, resizes), assert on the rendered string and the
emitted actions. No SSH, no real terminal — Bubble Tea models are plain
values, which is the whole reason the Elm architecture was chosen.

## References

| Source | What to take |
| --- | --- |
| `../ssh-idlefarmer/internal/tui/game_test.go`, `layout_test.go`, `overflow_test.go`, `format_test.go` | driving models with messages, layout/overflow assertions |
| tui/01's hitbox contract, tui/03's tribute/escape/event/death behavior | the trickiest units to pin down |

## Deliverables

- `internal/tui/` — focused `*_test.go` files beside the renderer or layout
  they exercise, plus `testdata/golden/*.txt`
- `internal/tui/hitbox/registry_test.go` — registry semantics
- `internal/tui/theme/` — golden tests live with tui/04; extend if gaps

## Spec

### Test doubles

- `fakeSession` implementing the `game.Session` interface surface the TUI
  uses: records actions (`Lock(3)`, `BailOrDepart()`, `AcceptTribute()`…),
  serves scripted snapshots. Lives in `internal/tui` as a `_test.go` helper.
- Snapshot fixtures: `snapDocked()`, `snapLockedSystems()`, `snapBelt()`,
  `snapMiningSite(resourceLeft, cargoLoad, pirateDistance)`,
  `snapTribute()`, `snapEscaping(underAttack)`, `snapDeath()`,
  `snapBroke()`, `snapStationOwner()`, `snapMaxed()` — builders with sensible
  defaults and override args.

### Update-loop tests (behavior)

Drive `Update` with message sequences; assert model state + recorded calls:

- Navigation: chart ↑/↓ wraps through systems/destinations; Enter on locked
  destination flashes the exact lock reason, Enter on affordable destination
  calls `Depart` and routes to belt.
- Belt: arrows cycle 7 rocks in every view mode; `V` cycles views and the
  selection index survives; Enter→`Lock`; `S`→`Scan` for the selected
  contact; `Q`→`Dock`.
- Mining Site: `B`, Esc, or Enter while resources remain calls
  `BailOrDepart` and routes to escape; Enter after depletion calls the same
  action with the green DEPART affordance.
- Tribute: `D` calls `AcceptTribute`; `R`/Esc calls `RefuseTribute`; synthetic
  timeout refuses automatically.
- Escape: normal mining/chart keys are ignored while escape is active; resolved
  survived outcome routes to summary; ship_lost routes to death screen.
- Death screen: first key after delay routes to death recap; no summary details
  render on the death screen itself.
- Overlays capture input: with help open, arrows scroll help and do NOT
  move world selection; Esc closes.
- Idle: ticks advancing past `idleSecs` without input produce `tea.Quit`;
  any key resets the timer.
- Resize below 80×24 → inert guard screen; back above → previous screen
  intact. Supported sizes grow the frame through 144×48, then center it
  independently on both axes without changing selection or scroll state.
- Kick message → kicked overlay → any key quits.

### Mouse tests

Synthetic mouse messages against a rendered-then-hit-tested model:

- Render at 80×24 and at horizontally/vertically centered capped frames, look
  up a destination row's frame-relative hitbox, translate to terminal
  coordinates, and click:
  selection moves. Click again: `Depart` or lock-reason flash. Same pattern for
  belt rocks in all three views, service and Shipyard buttons, BAIL/DEPART,
  tribute buttons, picker rows, and Tweaks values.
- Wheel over the world list moves selection; wheel over open help scrolls.
- Click on empty space or any horizontal/vertical frame gutter: no action, no
  panic.
- Click during min-size guard: swallowed.

### Golden render tests

`golden_test.go` renders known model states at fixed sizes and compares to
committed `.txt` files (with `-update` regeneration flag; strip/normalize
ANSI or store it — pick one and document; **prefer storing styled output**
so color regressions are caught, with a `stripansi` helper for readable
diffs):

- Chart docked (fresh pilot), chart locked-system detail, chart station owner,
  chart broke (dimmed buttons, salvage advance row), shipyard, cosmetics.
- Belt in tiles / orescan / radar with the same fixture belt, rock #3
  selected.
- Mining site at resource/cargo/pirate fixture, mining depleted with green
  DEPART, tribute modal, escaping under fire, each event HUD treatment, death
  screen.
- All revamped summary outcomes.
- Ship's log with 0 and 20 records.
- Sizes and variants follow tui/06's exhaustive matrix. Goldens cover at least
  80×24, an uncapped stretch size, 144×48, and a terminal larger than both
  caps (including blank gutters); focused/property tests cover every listed
  boundary. Include ASCII-safe, high-contrast, reduced-motion, and wide-glyph
  representatives.

### Layout/overflow property test

For every screen/state/mode in tui/06 × its width/height boundary matrix:
the frame has the exact capped dimensions and floor-centered origin, output
has height ≤ terminal height, every line's display width ≤ terminal width,
all nonblank content lies inside the frame, and chrome/body/keybar consume
exactly 3/`frameHeight-5`/2 rows. Use the same display-width library Lip Gloss
uses. Assert exact allocator results and selected-row visibility after
shrink/regrow, not only absence of overflow.

## Acceptance criteria

- [ ] `go test ./internal/tui/... -race` passes on Windows and Linux
  (goldens byte-identical cross-platform — beware `\r\n`: write goldens
  with LF and compare normalized).
- [ ] Every action reachable by keyboard is asserted reachable by mouse in
  at least one test (checklist comment in `mouse_test.go`).
- [ ] Goldens regenerate with a documented `-update` flag and the diff on a
  deliberate one-character style change is human-readable.
- [ ] No test depends on wall-clock timing (all ticks/timestamps injected).
- [ ] Suite < 30 s.
- [ ] The specification-test PR compiles and fails only on unimplemented v1.7
  expectations; production code and existing goldens are unchanged in that PR.

## Out of scope

- Real-terminal/real-SSH behavior → tests/02 smoke fragments + manual
  checklist in framework/04.
- teatest-style full-program harnesses — only if the model-message approach
  proves insufficient; do not add the dependency preemptively.
