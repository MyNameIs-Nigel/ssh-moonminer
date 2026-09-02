# TUI 01 — App Shell, Input & Mouse

**Area:** TUI · **Phase:** 2 · **Depends on:** tui/04 (theme; stub-able) ·
**Blocks:** tui/02, tui/03 · **Parallel-safe with:** framework/*, gameplay/*

> **v1.7 layout supersession:** [06-responsive-menu-overhaul.md](06-responsive-menu-overhaul.md)
> is authoritative for frame geometry, vertical chrome/body/keybar structure,
> overlays, clipping, and hitbox coordinates. This document remains
> authoritative for routing and input semantics.

## Goal

Build the Bubble Tea root model that every screen plugs into: screen
routing, the global input contract (**arrow keys + Enter/Space, full mouse
support**), window-size handling, the idle-disconnect timer, and the shared
overlay system (help, tweaks, kicked-notice). After this task, screens are
plug-in modules that receive sized regions and emit actions.

## References

| Source | What to take |
| --- | --- |
| `../ssh-idlefarmer/internal/tui/game.go` | root model shape: screen enum, overlay enum, tick msg, kick msg, idle enforcement |
| `../ssh-idlefarmer/internal/tui/layout.go` + `views.go` | layout composition patterns |
| `../ssh-idlefarmer/internal/server/server.go` (`teaHandler`) | constructor signature the server calls |
| bubbletea v2 docs/source (vendored) | mouse message types + enable option — **verify exact v2 API names before coding** |

## Deliverables

- `internal/tui/` — root model/routing/input in `game.go`, shared geometry in
  `layout.go`, overlays in `overlay.go`, help in `help.go`, and `errscreen.go`
- `internal/tui/hitbox/registry.go` — per-frame hitbox registry

## Spec

### Root model

```go
type Game struct {
    sess    *game.Session      // framework/03 handle (interface until it lands)
    content *content.Content
    id      identity.SessionIdentity

    snap    sim.Snapshot       // latest state snapshot (value copy)
    width, height int
    scr     screen             // scrChart | scrBelt | scrMining | scrSummary | scrLog
    overlay overlay            // ovNone | ovHelp | ovTweaks | ovKicked | ovOnboard
    hits    *hitbox.Registry   // rebuilt every View()

    lastInput int64            // unix seconds, for idle disconnect
    idleSecs  int64
}
```

- `Init` starts a 1 Hz UI tick (`tickMsg`) — drives clock displays, blinking
  cursor, idle checks. During an active mining run the actor pushes
  snapshots faster (see gameplay/02); both arrive as messages.
- Constructor `tui.New(id, sess, content, width, height, now, idleSecs)` —
  keep the signature the server task expects. Also export
  `tui.NewErrScreen()` (mirror idlefarmer's `errscreen`).
- `View()` returns `tea.View` with `AltScreen = true` and a window title
  (`MOON MINER — <pilot slot>`).

### Screen routing

`screen` enum + a small interface each screen file implements:

```go
type screenModel interface {
    Update(g *Game, msg tea.Msg) tea.Cmd   // screen-local input
    View(g *Game, w, h int) string          // renders into the content region
    Hitboxes(g *Game, reg *hitbox.Registry) // registered during View
}
```

(Exact shape may flex; the requirement is that tui/02 and tui/03 can add
screens without touching the router beyond one registration line.)

Global chrome owned by the shell: three top rows (rule, HUD strip, rule), a
flexible body, and a two-row bottom keybar (rule plus context-sensitive hints
and ship marker). Exact row allocation is owned by tui/06.

### Keyboard contract

Handled in the shell **before** screen dispatch:

| Key | Action |
| --- | --- |
| `?` | toggle help overlay |
| `T` | toggle tweaks overlay (chart screen only — while docked) |
| `Ctrl+C` / `q` at chart | quit (confirm overlay if a run is active — which can't happen at chart; plain quit) |
| `Esc` | close overlay if one is open, else screen-specific |

Everything else goes to the active screen (arrow keys, Enter, Space, letter
commands per the concept doc's control table). **Any** key press updates
`lastInput`. When `now − lastInput > idleSecs`, quit with a farewell notice
(idle enforcement lives here, not in the transport — see the comment in
idlefarmer's `teaHandler`).

### Mouse contract (the headline feature)

- The server enables mouse reporting via a program option
  (`tea.WithMouseCellMotion()` — cell-motion mode: clicks, wheel, and
  drag-motion; all-motion is unnecessary and noisy over SSH). Coordinate
  with framework/01's `newTeaProgram`; **verify the exact bubbletea v2
  option/message names against the vendored source** (v2 renamed several
  v1 APIs; expect `tea.MouseClickMsg`, `tea.MouseReleaseMsg`,
  `tea.MouseWheelMsg` or the v2 equivalents).
- **Hitbox registry** (`internal/tui/hitbox`): during every `View()` pass,
  each interactive element registers a rect + action ID:

  ```go
  type Box struct {
      X, Y, W, H int
      ID   string      // "world:2", "svc:refuel", "belt:rock:5", "btn:bail"
      Data any
  }
  reg.Add(box)
  hit, ok := reg.At(x, y)   // topmost-last-registered wins
  ```

  Screens compute rects from the same layout math they render with — a
  helper that wraps "render a row, register its box" keeps the two in sync.
  Boxes are frame-relative; the root translates terminal mouse coordinates
  exactly once using the centered frame origin. The registry is rebuilt each
  frame, so stale boxes are impossible. See tui/06 for gutter behavior,
  overlay coordinates, and vertical centering.
- **Semantics** (uniform across screens):
  - Click on a selectable row/tile → select it.
  - Click on an already-selected row, or double-click (two clicks on one ID
    within 400 ms) → activate (same as Enter).
  - Click on a button-styled element (`[F] REFUEL…`) → trigger immediately.
  - Wheel up/down over a list → move selection; over scrollable text
    (help) → scroll.
  - Mining Site screen: clicking `[B] BAIL` or green `[ENTER] DEPART` activates
    the same escape action as the keyboard; tribute buttons and station/port
    buttons are ordinary button-styled elements.
- Mouse must never be the *only* path: every action keeps a key binding
  (accessibility + terminals without mouse reporting).

### Window size & minimums

- `tea.WindowSizeMsg` re-layouts everything; store w/h in the model.
- Below **80×24**, render a centered "RESIZE TERMINAL — need 80×24, have
  W×H" screen and suspend hitboxes (mirror idlefarmer's guard if present;
  else build it).
- At and above the minimum, use tui/06's centered frame: it grows independently
  in width and height to **144×48**, then leaves blank terminal gutters.
  Larger frames add breathing room, never gameplay information.

### Overlays

Shared overlay system rendering a bordered box over a dimmed screen: help
(static keymap text per screen), kicked-notice ("boarded from another
terminal…" — shown on the actor's kick message, any key exits), onboarding
(3 short pages for `Created` pilots), tweaks (owned by tui/04, hosted
here). Overlays capture all input while open. Their intrinsic minima,
frame-relative centering, clamp/reflow/scroll behavior, and hitbox rules are
defined by tui/06.

## Acceptance criteria

- [ ] Model compiles against stub session/content fakes; `go test` unit
  tests for: routing, idle-quit firing, min-size guard, overlay capture.
- [ ] Hitbox registry: table-driven `At()` tests incl. overlap (topmost
  wins) and rebuild-clears-old-boxes.
- [ ] Double-click detection tested with synthetic timestamps.
- [ ] With framework/01 landed: a real SSH session shows the shell +
  placeholder screens; clicking and arrow keys both move a demo selection;
  wheel scrolls help text.
- [ ] Idle for `idleSecs` disconnects with the farewell message.

## Out of scope / handoffs

- Actual screens → tui/02 (chart, summary, log), tui/03 (belt, mining).
- Theme constants, panel/bar renderers → tui/04 (import; stub if racing).
- Program options wiring on the server side → framework/01 (one line —
  coordinate).
