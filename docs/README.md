# Moon Miner — Build Plan Index

This folder is the complete specification for building **ssh-moonminer**, a
terminal (TUI) space-mining game that anyone can play by SSHing into a server.
Each document under `framework/`, `gameplay/`, `tui/`, and `tests/` is a
**self-contained task** written so an agent can pick it up and work on it
independently of the others. This README is the contract that keeps parallel
work compatible: read it first, then your task file, then the referenced
source in the sibling reference project.

## The reference projects

Three sibling repos in this same fleet (all at `../<repo>` relative to this
one) are reference implementations. Each solves a different layer:

| Repo | What to mirror |
| --- | --- |
| `../ssh-idlefarmer` | The original **standalone** game on the exact same stack: SSH/session/persistence layer, Windows-host PTY workarounds. Still the right reference for anything that doesn't touch the arcade fleet (store internals, actor model, TUI mechanics). |
| `../ssh-arcadelobby` | **Canonical fleet contracts**, cited by path from task docs below: `docs/02-bridge-and-identity-protocol.md` (trusted-proxy identity — moonminer sits behind this router, unlike idlefarmer when it was built), `docs/03-games-registry-and-health.md` (the `games.toml` schema this game registers under), `docs/06-fleet-data-durability.md` (canonical Litestream+S3 pattern — **supersedes idlefarmer's plain-SQLite-volume story and this doc's own older "distroless" convention below**, see framework/04). |
| `../ssh-farm` | **The proven second fleet member** — a second game that already did the "port a standalone SSH game onto the fleet" work moonminer is about to do. Its `docs/framework/01-03` + actual code (`internal/identity/`, `Dockerfile`, `entrypoint.sh`, `.github/workflows/`) are worked examples of applying arcadelobby's contracts for real, including bugs found and fixed along the way. Prefer farm's code over idlefarmer's wherever the two diverge on fleet-specific concerns. |

**Rule: when a task doc cites a reference file, mirror its approach.** Do
not invent a new pattern a reference repo already solved. Rename
farm-specific things (`FARM_*` → `MOONMINER_*`, farm copy → ship copy),
keep the structure. Where idlefarmer and farm/arcadelobby disagree (identity,
Docker base image, deployment), **farm/arcadelobby win** — they reflect
decisions made after idlefarmer shipped standalone.

## What the game is

See [01-concept-and-story.md](01-concept-and-story.md) for the full concept.
One paragraph: the player pilots a mining ship, picks a world from a star
chart, drops into its asteroid belt, locks onto a rock, and drills it in real
time while fuel drains and pirate proximity climbs. They can hold overdrive
(faster drill, faster burn) or bail early to bank partial cargo. Four
outcomes per run — clean, bailed, raided, stranded — then back to the belt.
Credits buy fuel, hull repair, and ship upgrades.

The game was prototyped in HTML/React at
`C:\Users\user\Documents\Repositories\mynameis-nigel\moon-miner-materials\Web-Prototype\`
(`Moon Miner.dc.html` + `Moon Miner - Design Notes.md`). The prototype's
formulas and balance numbers are transcribed into the gameplay docs — you
should not need to read the prototype, but it is the tie-breaker if a spec
here is ambiguous.

## Stack (locked)

| Dependency | Purpose |
| --- | --- |
| Go 1.26+, `CGO_ENABLED=0` | language / pure-Go builds |
| `charm.land/wish/v2` | SSH server framework (wraps `github.com/charmbracelet/ssh`) |
| `charm.land/bubbletea/v2` | TUI framework (Elm-style model/update/view) |
| `charm.land/lipgloss/v2` | styling/layout |
| `modernc.org/sqlite` | cgo-free SQLite driver |
| `github.com/BurntSushi/toml` | game content/balance files |
| `golang.org/x/time` | connection rate limiting |

Match the versions in `../ssh-idlefarmer/go.mod` unless newer patch releases
exist.

## Package layout (the contract)

Agents working in parallel must respect these boundaries. A task doc owns the
packages it lists under "Deliverables"; touch other packages only in the ways
its "Handoffs" section allows.

| Path | Owner area | Purpose |
| --- | --- | --- |
| `cmd/ssh-moonminer/` | Framework | Entry point: config → store → game manager → SSH server → graceful shutdown |
| `internal/config/` | Framework | All settings from `MOONMINER_*` env vars |
| `internal/server/` | Framework | Wish server, middleware chain, PTY requirement, limits, shutdown hooks, Windows PTY fixes |
| `internal/identity/` | Framework | Public-key fingerprint + SSH username → save-slot, **plus** resolving the arcade router's proxied identity per `../ssh-arcadelobby/docs/02-bridge-and-identity-protocol.md` (mirror `../ssh-farm/internal/identity/`) |
| `internal/game/` | Framework | Save lifecycle: `Manager` (attach/detach), per-save actor goroutines, autosave |
| `internal/store/` | Framework | SQLite persistence + append-only migrations |
| `internal/content/` | Framework | Loads/validates worlds + balance from TOML |
| `internal/sim/` | Gameplay | Headless deterministic engine: state, belt generation, mining tick, outcomes, economy |
| `internal/tui/` | TUI | Bubble Tea root model, screens, components, theme, input (keys + mouse) |
| `data/` | Gameplay | `worlds.toml` + `balance.toml`, embedded via `data/embed.go` |
| `docs/` | — | This plan |

**Key inter-area interfaces** (define these first, everything else builds
against them):

1. **`sim.State` + `sim.Tick`/action functions** — the Gameplay area owns the
   engine; the TUI renders `sim` state and calls `sim` actions, never
   mutating state directly. `internal/sim` must stay pure: no I/O, no
   logging, no wall-clock reads — time comes in as arguments (mirror
   `../ssh-idlefarmer/internal/sim/`).
2. **`game.Session`** — Framework owns it; the TUI talks to the save through
   a session handle exactly like `../ssh-idlefarmer/internal/game/session.go`.
3. **`content.Content`** — immutable game data loaded once at boot, passed to
   both sim and TUI.
4. **`identity.Resolver`** — Framework owns it; resolves both a direct SSH
   connection (own key = own account, dev/local use) and a connection
   proxied through the arcade router (trusted proxy key + router-encoded
   username = the *player's* account, not the router's). Everything
   downstream (session caps, store rows, logging) keys on the *resolved*
   fingerprint, never the wire key. See framework/01.

## Build order and parallelism

```
Phase 0 (done):        repo + go.mod + this plan
Phase 1 (parallel):    framework/01  gameplay/01  tui/04(theme only)
Phase 2 (parallel):    framework/02  gameplay/02  tui/01
Phase 3 (parallel):    framework/03  gameplay/03  tui/02  tui/03
Phase 4 (parallel):    framework/04  gameplay/04  tests/01..03
```

Dependencies are listed per-task; the summary:

- `gameplay/01` (sim skeleton + belt gen) blocks `gameplay/02..04` and gives
  `tui/02,03` their data types.
- `framework/01` (SSH server) and `tui/01` (app shell) can be built with stub
  models/stores; they meet in Phase 3.
- `tests/*` tasks can start as soon as their target area has landed; writing
  them in the same PR as the feature is also fine.

## Task-doc format

Every task file follows the same shape so agents can execute without extra
context:

- **Goal** — one paragraph of intent.
- **References** — exact files in `../ssh-idlefarmer` (or the prototype) to
  mirror for standalone game mechanics; `../ssh-arcadelobby` and `../ssh-farm`
  for anything touching the arcade fleet (identity, durability, deployment).
- **Deliverables** — packages/files to create.
- **Spec** — requirements, including exact formulas/values where they exist.
- **Acceptance criteria** — checklist the work must pass.
- **Out of scope / handoffs** — what belongs to a different task.

## Project-wide conventions (apply to every task)

Most of these are inherited from idlefarmer's hard-won lessons
(`../ssh-idlefarmer/CLAUDE.md`); items 8–10 are fleet-wide conventions that
postdate idlefarmer and come from `../ssh-arcadelobby`/`../ssh-farm` instead:

1. **Never commit runtime data.** `var/`, `*.db`, and host-key files are
   gitignored and must stay out of git.
2. **Middleware order matters** in `server.New()` — Wish runs middleware
   bottom-up. Keep logging/rate-limit outermost and the save-attach
   middleware adjacent to the Bubble Tea handler.
3. **Sim purity.** `internal/sim` takes timestamps and RNG state as
   arguments. Deterministic replay is a test requirement, not a nice-to-have.
4. **Raw-session messages use `\r\n`** line endings (no tty cooking before
   the TUI starts) — see `../ssh-idlefarmer/internal/server/pty.go`.
5. **Graceful shutdown is load-bearing.** SIGTERM must flush every active
   save before exit. An in-progress mining run auto-bails (banks accrued
   yield) rather than being lost.
6. **Game balance lives in TOML, not code.** Formulas in Go, tunable
   constants in `data/*.toml`, embedded at build time, overridable at runtime
   via `MOONMINER_DATA_DIR`.
7. **Windows host support.** The dev machine runs Windows. Mirror
   `../ssh-idlefarmer/internal/server/teaprogram.go` (color-profile forcing +
   `cursorDownWriter` newline rewriting) verbatim — sessions render garbage
   on Windows hosts without it.
8. **Docker runtime is hardened, but not distroless.** `alpine:3`, non-root
   uid 65532, read-only root FS, only the data volume and `/tmp` writable —
   the image needs a real shell and `litestream`/`mc` binaries for the
   durability entrypoint (item 10), which distroless can't run. This
   supersedes any older "distroless" guidance; see framework/04.
9. Run `go build ./...`, `go vet ./...`, and `go test ./...` before declaring
   any task done. Table-driven tests live alongside code as `*_test.go`.
10. **This game lives behind the arcade router.** Identity must resolve both
    direct connections (dev/local) and proxied connections from
    `ssh-arcadelobby` per its canonical protocol
    (`../ssh-arcadelobby/docs/02-bridge-and-identity-protocol.md`) — see
    framework/01. Persistence must follow the fleet's Litestream+S3 pattern
    (`../ssh-arcadelobby/docs/06-fleet-data-durability.md`) so an EC2 instance
    loss doesn't lose pilot saves — see framework/04. `../ssh-farm` is the
    worked example of both; copy its shape rather than re-deriving it.

## Document map

| Doc | Task |
| --- | --- |
| [01-concept-and-story.md](01-concept-and-story.md) | Story, world, loop, screens, visual direction (read-only context) |
| [framework/01-ssh-server-and-identity.md](framework/01-ssh-server-and-identity.md) | Wish SSH server, middleware chain, key identity |
| [framework/02-persistence-and-save-model.md](framework/02-persistence-and-save-model.md) | SQLite store, schema, save serialization |
| [framework/03-session-lifecycle-and-actors.md](framework/03-session-lifecycle-and-actors.md) | Save manager, actor goroutines, takeover policy, shutdown flush |
| [framework/04-config-content-and-deployment.md](framework/04-config-content-and-deployment.md) | `MOONMINER_*` config, TOML content loader, Docker deploy |
| [gameplay/01-simulation-engine-and-belt-generation.md](gameplay/01-simulation-engine-and-belt-generation.md) | Sim state, RNG, world data, belt generation |
| [gameplay/02-mining-run-loop.md](gameplay/02-mining-run-loop.md) | Real-time drill tick, overdrive, four outcomes |
| [gameplay/03-economy-worlds-and-balance.md](gameplay/03-economy-worlds-and-balance.md) | Credits, port services, travel, balance TOML |
| [gameplay/04-progression-and-ship-log.md](gameplay/04-progression-and-ship-log.md) | Lifetime stats, ship upgrades, run history |
| [tui/01-app-shell-input-and-mouse.md](tui/01-app-shell-input-and-mouse.md) | Root model, screen router, keyboard + mouse input, resize/idle |
| [tui/02-star-chart-and-run-summary.md](tui/02-star-chart-and-run-summary.md) | Star Chart screen, port services panel, Run Summary screen |
| [tui/03-belt-views-and-mining-screen.md](tui/03-belt-views-and-mining-screen.md) | Belt screen (3 view modes), target lock, live Mining screen |
| [tui/04-theme-components-and-tweaks.md](tui/04-theme-components-and-tweaks.md) | Phosphor-blue theme, panel/bar components, Tweaks overlay |
| [tests/01-sim-engine-tests.md](tests/01-sim-engine-tests.md) | Determinism, outcome tables, distribution + property tests |
| [tests/02-server-integration-tests.md](tests/02-server-integration-tests.md) | In-process SSH client tests: auth, PTY, policy, shutdown |
| [tests/03-tui-rendering-and-input-tests.md](tests/03-tui-rendering-and-input-tests.md) | Update-loop tests, golden renders, hitbox/mouse tests |
