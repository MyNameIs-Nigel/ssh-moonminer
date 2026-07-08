# TUI 04 — Theme, Components & Tweaks

**Area:** TUI · **Phase:** 1 (theme+components first — everything imports
them) · **Blocks:** tui/01–03 · **Parallel-safe with:** framework/01,
gameplay/01

## Goal

One place for how Moon Miner looks: the phosphor-blue palette as lipgloss
styles with **semantic roles**, cosmetic HUD variants, reusable render
primitives (notched panel, block bar, threat dots, radar/static/event widgets,
buttons, flash messages), and the **Tweaks** overlay where players adjust
accessibility/difficulty. No screen may hardcode a hex value or draw its own
border after this lands.

## References

| Source | What to take |
| --- | --- |
| `../moon-miner-materials/Web-Prototype/Moon Miner.dc.html` line ~326 (`this.C = {...}`) | exact palette (transcribed below) |
| `Moon Miner - Design Notes.md` (Visual direction, Tweakable) | design rationale |
| `../ssh-idlefarmer/internal/tui/format.go` + `layout.go` | formatting/layout helper conventions |
| `charm.land/lipgloss/v2` | styling API |

## Deliverables

- `internal/tui/theme/` — `theme.go` (palette + roles + cosmetic variants),
  `panel.go`, `bar.go`, `widgets.go` (dots, buttons, badges, flash,
  radar/static/event widgets), tests
- `internal/tui/` — `tweaks.go` (overlay content; hosted by tui/01's overlay
  system)
- `Settings` struct fields agreed with gameplay/01 (persisted in the save)

## Spec

### Palette (from the prototype, authoritative)

| Role | Hex | Use |
| --- | --- | --- |
| `bg` | `#04101f` | background (near-black navy) |
| `line` | `#16456e` | panel borders, dividers |
| `txt` | `#bfe2ff` | primary text |
| `dim` | `#5e87aa` | labels, secondary text |
| `bright` | `#3fb6ff` | selection, active elements, healthy gauges |
| `white` | `#dce9f5` | COMMON tier |
| `cyan` | `#3fe0ff` | UNCOMMON tier, bail outcome |
| `violet` | `#b48fff` | RARE tier |
| `gold` | `#ffcf5e` | LEGENDARY tier, high-value cargo |
| `red` | `#ff5a6a` | danger, under-fire outcome |
| `amber` | `#ffae3f` | warning, BAIL/tribute outcomes, costs |
| `green` | `#5fe0a0` | confirmations, DEPART outcome/control |
| `darkred` | `#3a0508` | death screen wash / connection lost |

Expose **roles, not colors**: `theme.TierStyle(i)`, `theme.OutcomeStyle(kind)`,
`theme.GaugeStyle(pct, invert bool)` (the blue→amber→red ramp; `invert` for
gauges where high is bad), `theme.Selected`, `theme.Dim`, `theme.Button`,
`theme.ButtonDisabled`, `theme.DeathScreen`, `theme.CosmeticVariant(id)`.
Screens never touch hexes.

Cosmetic HUD themes are small role remaps, not new hardcoded palettes. Default
themes for MVP:

| Theme | Flavor | Notes |
| --- | --- | --- |
| `phosphor_blue` | stock cockpit | current palette |
| `amber_terminal` | old industrial station | bright role shifts amber; danger remains red |
| `green_crt` | salvage surplus | bright role shifts green; tier colors remain distinguishable |
| `highline_white` | accessibility unlock | brighter text/lines; pairs with high contrast |

Terminals without truecolor: rely on the color-profile downgrading that
lipgloss/colorprofile already does (framework/01 forces a profile on
Windows-host sessions); verify the palette degrades legibly to 256-color
(spot-check the ramp stays distinguishable).

### Panel primitive

```
╭◇ TARGET LOCK ─────────────────╮      ┌◇ TARGET LOCK ──────────────┐
```

`panel.Render(title string, w, h int, body string) string` — single-line
border in `line`, floating `◇ TITLE` notch in `dim` with the diamond in
`bright`, body clipped/padded to interior. Use `┌─┐│└┘` box-drawing (the
prototype's square corners, not rounded). Variants: `panel.Danger` (red
border — pirate-imminent), title-right slot for small status text.

### Bar primitive

`bar.Render(pct float64, width int, style lipgloss.Style) string` — filled
`█` and unfilled `█` in a dim/dark style (prototype uses same glyph, darker
color `#0d3252` for empty — do that; it reads as a recessed channel), no
brackets. `bar.Ramp(pct, width, invert)` applies the standard color ramp.
Also `bar.Mini(n, of, width)` for stat-grid bars in the ship's log.

### Widgets

- **Threat dots**: `dots.Render(n, of int)` → `●●●○○` colored by n (1–2
  blue, 3 amber, 4–5 red).
- **Button**: `[F] REFUEL TO 100%  ◈ 198` — bracketed key in `bright`,
  label in `txt`, price in `amber`; disabled = all `dim` + border dimmed.
  Renders + registers its own hitbox via a passed registry (keeps hitboxes
  in sync with pixels — the tui/01 contract).
- **Badge**: `☠ CONTACT IMMINENT`, `LIFE SUPPORT`, `POWER REBOOT` — small
  colored tags.
- **Flash**: one-line transient message, kind→color, ~1.8 s expiry via the
  shell's tick (state lives in the root model; this provides render).
- **Blinking cursor**: `cursor.Render(tick int)` — `█` on even ticks,
  space on odd; used in keybars.
- **Radar distance**: compact ring/line widget for pirate distance and ETA
  confidence. Must support a static/noise state for radar blackout.
- **Event treatment helpers**: renderable overlays/badges for
  power_outage/radar_blackout/life_support_failure/cargo_shift/reactor_surge
  so tui/03 does not hand-roll each effect.
- **Death screen**: `death.Render(w,h, tick, reducedMotion)` returns a brief
  monochrome/inverted flicker of the last HUD frame (skipped under reduced
  motion), settling on a full-screen dark red `CONNECTION LOST` frame with a
  blinking end-of-line cursor and intentionally no summary content.

### Tweaks overlay (player settings, persisted in the save)

| Setting | Values | Default | Effect |
| --- | --- | --- | --- |
| BELT VIEW | tiles / orescan / radar | tiles | default belt rendering (tui/03) |
| PIRATE AGGRESSION | 0.75 / 1.0 / 1.25 / 1.5 | 1.0 | multiplier into pirate arrival/event pressure (gameplay/02) — self-serve difficulty, still dangerous |
| HIGH CONTRAST | on / off | off | swaps `dim` for `txt`, thickens selection markers |
| ASCII SAFE MODE | on / off | off | replaces ◇◆✦★⛽☠◈● with ASCII (`* + # @ F ! $ o`) for fonts without the glyphs |
| REDUCED MOTION | on / off | off | disables blink, radar sweep, event jitter/pulse |
| EVENT FLASH INTENSITY | normal / subdued | normal | lowers power/life-support/cargo-shift flashing without changing mechanics |

Arrow keys/click to change values; changes apply immediately and persist
via a `sim` settings action (coordinate the `Settings` struct with
gameplay/01). Note the divergence doc: CRT glow from the prototype has no
terminal equivalent; these accessibility toggles replace it.

Cosmetic configuration (HUD theme, border style, radar sweep, ship paint/name)
lives in the chart Cosmetics screen from tui/02 because it has unlock costs and
account progression. Tweaks are immediate accessibility/difficulty settings.

ASCII SAFE MODE implementation: all special glyphs route through
`theme.Glyph(name)` which consults the setting — screens use the indirection
from day one so the toggle is one map swap.

## Acceptance criteria

- [ ] `go test ./internal/tui/theme` — goldens for panel (several widths,
  long titles truncate with `…`), bars at 0/33/100%, ramp color roles at
  thresholds, dots, buttons enabled/disabled, radar/static widgets, event
  overlays, death screen, cosmetic variants, ASCII-safe variants.
- [ ] No other package contains a hex color literal (grep-able check noted
  in the test file).
- [ ] Panel + bar handle degenerate sizes (w<4, h<3) without panicking —
  return clipped output.
- [ ] Tweaks overlay renders, changes a live value, and the value survives
  an encode/decode of `sim.State` (integration-ish test with the sim
  package).
- [ ] Cosmetic HUD theme selection changes theme roles without changing any sim
  derived values (fixture comparison).
- [ ] Every widget that is interactive registers hitboxes through the
  passed registry (unit test).

## Out of scope / handoffs

- Overlay hosting/input capture → tui/01.
- Where settings are stored → gameplay/01's `Settings` (agree on fields
  early; both tasks are Phase 1 — this doc's table is the field list).
- Screen layouts → tui/02, tui/03.
