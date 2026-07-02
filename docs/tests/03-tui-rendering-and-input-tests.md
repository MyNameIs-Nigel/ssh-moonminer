# Tests 03 — TUI Rendering & Input Tests

**Area:** Tests · **Phase:** 4 (start after tui/01; extend per screen) ·
**Depends on:** tui/01–04 · **Parallel-safe with:** tests/01, tests/02

## Goal

Test the TUI as a pure function: feed the model synthetic messages (keys,
mouse, ticks, snapshots, resizes), assert on the rendered string and the
emitted actions. No SSH, no real terminal — Bubble Tea models are plain
values, which is the whole reason the Elm architecture was chosen.

## References

| Source | What to take |
| --- | --- |
| `../ssh-idlefarmer/internal/tui/game_test.go`, `layout_test.go`, `overflow_test.go`, `format_test.go` | driving models with messages, layout/overflow assertions |
| tui/01's hitbox contract, tui/03's overdrive emulation | the trickiest units to pin down |

## Deliverables

- `internal/tui/` — `game_test.go`, `screens_test.go`, `mouse_test.go`,
  `golden_test.go`, `testdata/golden/*.txt`
- `internal/tui/theme/` — golden tests live with tui/04; extend if gaps

## Spec

### Test doubles

- `fakeSession` implementing the `game.Session` interface surface the TUI
  uses: records actions (`Lock(3)`, `SetOverdrive(true)`…), serves scripted
  snapshots. Lives in `internal/tui` as a `_test.go` helper.
- Snapshot fixtures: `snapDocked()`, `snapBelt7()`, `snapMining(drill,
  fuel, pirate)`, `snapBroke()`, `snapMaxed()` — builders with sensible
  defaults and override args.

### Update-loop tests (behavior)

Drive `Update` with message sequences; assert model state + recorded calls:

- Navigation: chart ↑/↓ wraps; Enter on unaffordable world flashes, on
  affordable world calls `Depart` and routes to belt.
- Belt: arrows cycle 7 rocks in every view mode; `V` cycles views and the
  selection index survives; Enter→`Lock`; `R`→`Rescan`; `Q`→`Dock`.
- Mining: Space-repeat sequence (press, press@+50ms, press@+100ms,
  silence) yields `SetOverdrive(true)` then `SetOverdrive(false)` after
  the expiry tick — exactly two calls. `B` → `Bail`.
- Overlays capture input: with help open, arrows scroll help and do NOT
  move world selection; Esc closes.
- Idle: ticks advancing past `idleSecs` without input produce `tea.Quit`;
  any key resets the timer.
- Resize below 80×24 → guard screen; back above → previous screen intact.
- Kick message → kicked overlay → any key quits.

### Mouse tests

Synthetic mouse messages against a rendered-then-hit-tested model:

- Render at 80×24, look up a world row's hitbox, click its coords: selection
  moves. Click again: `Depart`. Same pattern for belt rocks in all three
  views, service buttons, bail button, tweaks values.
- Wheel over the world list moves selection; wheel over open help scrolls.
- Hold-press on OVERDRIVE (press msg without release) keeps overdrive on
  across ticks; release msg turns it off.
- Click on empty space: no action, no panic.
- Click during min-size guard: swallowed.

### Golden render tests

`golden_test.go` renders known model states at fixed sizes and compares to
committed `.txt` files (with `-update` regeneration flag; strip/normalize
ANSI or store it — pick one and document; **prefer storing styled output**
so color regressions are caught, with a `stripansi` helper for readable
diffs):

- Chart docked (fresh pilot), chart broke (dimmed buttons, insurance row).
- Belt in tiles / orescan / radar with the same fixture belt, rock #3
  selected.
- Mining at 47/61/33 (the doc sketch), mining with overdrive + pirate ≥ 75
  (badge + danger panel).
- All four summary outcomes.
- Ship's log with 0 and 20 records.
- Sizes: 80×24 (canonical) and 120×40 (stretch) for each. ASCII-safe-mode
  variant for one screen of each family.

### Layout/overflow property test

For every screen × sizes from 80×24 up to 200×60 (sampled): rendered output
has height ≤ h and every line's display width ≤ w (use the same width
library lipgloss uses). Mirrors idlefarmer's `overflow_test.go`.

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

## Out of scope

- Real-terminal/real-SSH behavior → tests/02 smoke fragments + manual
  checklist in framework/04.
- teatest-style full-program harnesses — only if the model-message approach
  proves insufficient; do not add the dependency preemptively.
