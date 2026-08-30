# TUI 06 — Responsive Menu Overhaul

**Area:** TUI · **Release:** v1.7 · **Depends on:** tui/01–05 and the shipped
screen set · **Implementation status:** implemented on the v1.7 integration branch

> **Authoritative layout contract:** this document supersedes fixed-size and
> one-axis-only layout guidance in tui/01–05 and tests/03. Gameplay, controls,
> visual semantics, and screen ownership remain with those documents unless
> this document explicitly changes them.

## Goal

Make every Moon Miner screen use one predictable responsive frame instead of
independent fixed-width calculations. The frame must remain fully usable at
80×24, grow deliberately until 144×48, and sit centered in larger terminals.
Every panel, overlay, clipped line, scroll viewport, and mouse hitbox must be
derived from the same frame-relative layout result.

This is a layout overhaul, not a gameplay redesign. It preserves the shipped
actions and information hierarchy while giving additional terminal space to
the parts of each screen that benefit from it.

## Scope and deliverables

The implementation PR following this specification owns:

- `internal/tui/layout.go` — frame geometry, deterministic flex allocation,
  clipping/padding helpers, and frame-relative hitbox translation.
- `internal/tui/game.go` — one frame placement path for normal screens,
  interstitials, death, and overlays.
- Existing renderers in `internal/tui/` — replace screen-local terminal sizing
  with allocated frame/body regions. Do not create parallel “wide” screens.
- `internal/tui/hitbox/registry.go` and callers — boxes are registered in
  frame coordinates and translated exactly once for mouse lookup/rendering.
- Responsive specification and regression tests described below.

The initial documentation/specification PR intentionally added compilable
failing tests before production changes. The v1.7 implementation now satisfies
that executable contract; future changes must not weaken it to reintroduce
fixed geometry or silent clipping.

## Terms and coordinate systems

- **Terminal:** the full PTY cell grid reported by `tea.WindowSizeMsg`.
- **Frame:** the bounded game surface placed inside the terminal.
- **Chrome:** the shared three-row HUD at the top of the frame.
- **Body:** the flexible screen-specific region between chrome and keybar.
- **Keybar:** the shared two-row rule-and-command area at frame bottom.
- **Region:** a panel or viewport allocated within the body.
- **Frame coordinates:** `(0,0)` is the frame's upper-left cell.
- **Terminal coordinates:** `(0,0)` is the PTY's upper-left cell.

Renderers and hitbox producers work only in frame coordinates. Terminal
offsets belong to the final placement/input boundary.

## Frame contract

The minimum supported terminal and frame are **80×24**. Below either minimum,
render only the centered resize guard:

`RESIZE TERMINAL — need 80×24, have W×H`

The guard has no active hitboxes.

For supported terminals:

```text
frameWidth  = clamp(terminalWidth,  80, 144)
frameHeight = clamp(terminalHeight, 24, 48)
frameX      = floor((terminalWidth  - frameWidth)  / 2)
frameY      = floor((terminalHeight - frameHeight) / 2)
bodyHeight  = frameHeight - 3 - 2
```

An odd spare column or row remains on the right or bottom. This floor rule is
part of the contract so rendering and pointer translation cannot disagree.

Width and height grow independently. A 200×30 terminal therefore renders a
144×30 frame centered horizontally only; an 80×60 terminal renders an 80×48
frame centered vertically only. No normal screen may bypass this frame,
including `CONNECTION LOST`, combat interstitials, or the dimmed base beneath
an overlay.

### Shared vertical structure

Every screen is exactly:

1. three rows of chrome;
2. `frameHeight - 5` rows of flexible body;
3. two rows of keybar.

The keybar is always attached to the frame bottom, not the terminal bottom.
Short screen content pads inside the body. Tall content clips or scrolls
inside its assigned body region and may never push the keybar down.
Presentation-only states such as settled `CONNECTION LOST` keep these row
allocations but paint the chrome/keybar rows without normal HUD data or hints;
the death contract still forbids leaking pilot or loss details.

## Deterministic flex allocator

All multi-region screens use one integer allocator. Each region declares a
minimum extent and positive flex weight. The allocator:

1. verifies that the available extent is at least the sum of minima;
2. reserves every minimum;
3. computes each region's exact share of the remaining cells as
   `remaining * weight / totalWeight`;
4. assigns the floor of each exact share;
5. distributes leftover cells by descending fractional remainder, with ties
   resolved by declaration order.

This largest-remainder rule is deterministic. Region order, minima, and
weights are therefore API-level layout data and tests must assert exact values
at boundary and odd-remainder widths. Separators or borders are included in
the declared outer region width; renderers must not add unbudgeted cells after
allocation.

The same helper may allocate vertical subregions. Zero or negative weights,
negative minima, insufficient available space, or integer overflow are
programmer errors covered by unit tests rather than silently repaired.

## Screen allocations

### Star Chart

The route list/details region and port-services region have equal priority:

| Region | Minimum width | Weight |
| --- | ---: | ---: |
| CHART | 40 | 1 |
| PORT / STATUS | 40 | 1 |

At 80 columns the result is 40/40; at 81 it is 41/40 because declaration
order breaks the equal remainder; at 144 it is 72/72. Existing route and
service actions remain unchanged. Additional height expands descriptions and
list breathing room; it does not reveal actions unavailable at 80×24.

### Shipyard

| Region | Minimum width | Weight |
| --- | ---: | ---: |
| HANGAR | 24 | 3 |
| LOADOUT | 32 | 4 |
| STATUS | 24 | 3 |

The minimum is 24/32/24. Extra width follows 3/4/3; at 144 columns the result
is 43/58/43. `STATUS` is informational: it displays power, mass, credits,
service state, and acquisition feedback, but is not a keyboard-focus pane.
`Tab`/`Shift+Tab` switch between interactive HANGAR and LOADOUT only. STATUS
registers no action hitboxes; button-like acquisition copy moves to HANGAR or
is restyled as a noninteractive quote/readout.

Vertical overflow belongs primarily to LOADOUT. Track and slot rows scroll as
one viewport while the selected row remains visible. HANGAR scrolls only when
its model list exceeds its region. STATUS reflows labels before clipping and
never steals body rows from the other regions.

### Mining and combat

| Region | Minimum width | Weight |
| --- | ---: | ---: |
| STATUS / ACTIONS | 46 | 3 |
| TACTICAL VIEWPORT | 34 | 2 |

The minimum is 46/34. Extra width follows 3/2; at 144 columns the result is
84/60. Drilling, tribute, escape, combat, and event treatments use this same
split so controls do not jump horizontally when run state changes. The left
region owns gauges, warnings, prompts, and BAIL/DEPART/combat actions. The
right region owns pirate scanner, asteroid art, pressure-point targets, and
combat scope. If content must reflow, action labels and dangerous state take
precedence over decorative art.

### Single-region screens

Asteroid Belt, Run Summary, Death Recap, Ship's Log, and the settled
`CONNECTION LOST` presentation each receive the full body as one region.
Internal lists/tables may use responsive columns, but they do not participate
in a top-level horizontal split. At larger heights, belt fields and log/summary
viewports grow. Death remains sparse by design, centers its minimal message
within the frame body, and leaves its reserved chrome/keybar rows free of
normal HUD data and controls.

## Overlays

Overlays are centered within the **frame**, never the terminal. Each overlay
declares intrinsic minimum width and height based on its border, title,
mandatory controls, and shortest valid copy layout.

Allocation order:

1. use the intrinsic size when it fits;
2. grow only dimensions the overlay marks flexible, up to the frame body;
3. clamp to the frame/body bounds;
4. reflow prose and labels to the clamped inner width;
5. if mandatory rows still exceed the clamped inner height, expose a
   deterministic scroll viewport and keep the selected/active row visible.

Help, Tweaks, permit purchase, slot picker, module removal, tribute, reconnect,
and development overlays follow this contract. No overlay may be truncated by
the final frame clip as its normal overflow strategy. Overlay hitboxes use the
same clamped rectangle and scroll offset as rendering. Input remains captured
by the topmost overlay.

## Clipping, reflow, and scroll policy

Clipping is the last line of defense, not a layout mechanism:

- Every produced frame line is clipped by display-cell width (ANSI-aware) to
  `frameWidth`; every frame is clipped to `frameHeight`.
- Clip before adding terminal gutters. Never clip terminal-positioned output
  back to frame width, because that would count the left gutter twice.
- ANSI reset sequences must remain balanced after clipping.
- User-visible prose wraps at word boundaries. Identifiers, prices, gauges,
  key labels, and outcome/state words stay atomic where possible.
- Dynamic values clip before their labels. Safety-critical labels and actions
  (`HULL`, `FUEL`, `BAIL`, `DEPART`, `UNDER FIRE`, countdowns) outrank flavor
  copy and decorative art.
- Lists and tables scroll vertically when their complete reachable state does
  not fit. Selection is always visible after keyboard, wheel, resize, or
  snapshot changes.
- Keybar hints are priority-ordered and may truncate with an ellipsis, but the
  final ship marker and at least the primary action must remain visible.
- Decorative blank space may absorb growth; larger frames never introduce
  gameplay information or controls absent at 80×24.

Tests measure display width, not byte count, in styled, high-contrast, and
ASCII-safe modes.

## Hitbox and input contract

Every hitbox is frame-relative. A renderer registers exactly the rectangle it
painted using frame coordinates. The root input boundary converts a mouse
position once:

```text
frameMouseX = terminalMouseX - frameX
frameMouseY = terminalMouseY - frameY
```

Positions outside `[0, frameWidth) × [0, frameHeight)` are ignored. No screen,
component, or overlay may add `frameX`/`frameY` itself. The registry is rebuilt
on every view, and resize invalidates all old geometry before the next input.
This preserves topmost-last-registered overlap semantics while preventing
double offsets when the frame is centered in either axis.

## Phased implementation plan

### Phase 0 — Executable specification (this PR)

- Land this authoritative document and reconcile older docs.
- Add compilable failing tests for frame geometry, flex allocation, centering,
  clipping, overlay constraints, and hitbox translation.
- Keep failures precise: each should name the missing v1.7 contract.
- Do not modify production rendering or “temporarily” update goldens.

### Phase 1 — Geometry foundation

- Introduce immutable frame/rect data and the shared allocator in `layout.go`.
- Route supported-size views through one width-and-height capped frame.
- Move terminal offset handling to final placement and mouse translation.
- Preserve the resize guard and prove gutters have no hitboxes.

### Phase 2 — Shared shell and single-region screens

- Make chrome/body/keybar consume exact frame rows.
- Migrate Belt, Summary, Death, Ship's Log, interstitials, and the settled
  death presentation to the full single body region.
- Add frame-bound clipping and scrolling without changing controls.

### Phase 3 — Multi-region screens

- Migrate Chart to 40/40, Shipyard to 24/32/24 with 3/4/3 flex, and all
  Mining/Combat states to 46/34 with 3/2 flex.
- Remove independent hard-coded top-level widths.
- Keep selection visible through resize and preserve state across allocations.

### Phase 4 — Overlay migration

- Give each overlay intrinsic constraints and flexible dimensions.
- Reflow and scroll constrained contents inside the frame body.
- Verify mouse and keyboard parity after every overlay resize.

### Phase 5 — Golden refresh and cleanup

- Keep the direct geometry, semantic-anchor, accessibility-mode, overflow,
  selection-visibility, and deterministic-render matrix green. The repository
  had no pre-v1.7 golden harness to regenerate; snapshots may be added later
  as a complementary style-regression layer, not as a substitute for these
  structural assertions.
- Remove superseded sizing helpers and terminal-coordinate hitbox arithmetic.
- Run build, vet, unit tests, race tests, and a real 80×24/large-PTY smoke test.

## Exhaustive test matrix

The specification suite must exercise the Cartesian product of the following
frame boundaries where practical, with pairwise coverage only for expensive
goldens:

| Axis | Values | Purpose |
| --- | --- | --- |
| terminal width | 79, 80, 81, 82, 119, 143, 144, 145, 200 | guard, minimum, allocator remainders, cap, horizontal gutters |
| terminal height | 23, 24, 25, 30, 47, 48, 49, 60 | guard, minimum, flex body, cap, vertical gutters |

Required screen/state rows:

- Chart: fresh, locked route, unaffordable services, in-belt recovery,
  long names, seven-digit credits.
- Shipyard: one ship, all ships, unowned, buyback, capped tracks, maximum slot
  rows, picker and remove overlays; HANGAR/LOADOUT focus only.
- Belt: data tiles, ore scan, radar, empty belt, long contact labels.
- Mining: drilling, hold full, depleted, skill check, every event treatment,
  tribute, escape, under fire, combat intro, and combat.
- Bookends: every summary outcome, ship-lost recap, empty and long Ship's Log,
  death flicker frame, reduced-motion `CONNECTION LOST`.
- Overlays: Help, Tweaks, permit, picker, remove, reconnect, and dev overlay at
  intrinsic, clamped, reflowed, and scrolling states.
- Modes: styled Unicode default, high contrast, ASCII-safe, reduced motion,
  and styled content containing wide glyphs.

For every supported case assert:

1. exact frame width, height, and floor-centered offsets;
2. exactly three chrome rows, `frameHeight - 5` body rows, and two keybar rows;
3. no rendered display line exceeds the frame or terminal;
4. no content appears outside the frame except blank gutters;
5. exact allocator outputs, including ties and odd remainders;
6. selected rows remain visible after grow, shrink, and regrow;
7. each keyboard action has a mouse-equivalent box at its painted cells;
8. clicks in all four gutters and the resize guard do nothing;
9. overlay boxes and hitboxes remain inside the frame;
10. repeated renders of identical state and dimensions are byte-identical.

Allocator unit tests also cover empty input, one region, zero remaining cells,
large remaining extents, invalid minima/weights, insufficient space, and sum
preservation.

## Acceptance criteria

- [x] Terminals below 80×24 show only the inert resize guard.
- [x] The frame grows from 80×24 through 144×48 and is centered on both axes
  independently above either cap.
- [x] All screens use the three-row chrome, flex body, and two-row keybar.
- [x] Chart, Shipyard, and Mining/Combat produce the exact region allocations
  specified above at every tested width.
- [x] STATUS is informational and Shipyard navigation exposes only HANGAR and
  LOADOUT focus.
- [x] Belt, Summary, Death, Log, and `CONNECTION LOST` use one top-level body
  region.
- [x] Every overlay uses intrinsic minima plus clamp/reflow/scroll behavior.
- [x] Clipping is ANSI/display-width safe and never substitutes for required
  scrolling or reflow.
- [x] Every hitbox is frame-relative and remains aligned with horizontal and
  vertical terminal gutters.
- [x] The boundary and representative screen-state matrix passes in default,
  high-contrast, ASCII-safe, and
  reduced-motion modes.
- [x] No sim, economy, persistence, or control semantics change.

## Out of scope

- New gameplay actions, information, ships, modules, events, or balance.
- A layout preference or user-selectable frame cap.
- Support below 80×24.
- Responsive server/raw-session messages before Bubble Tea starts.
- Changes to the phosphor-blue palette or accessibility mode semantics.
