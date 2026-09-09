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

See [01-concept-and-story.md](01-concept-and-story.md) for the full concept
and [02-danger-economy-and-progression.md](02-danger-economy-and-progression.md)
for the expanded roguelite progression narrative. One paragraph: the player is
a fringe-system miner in a disposable ship, starting with only one safe planet
and a tiny cargo hold. They scan asteroid belts, commit to long manual mining
operations, watch a rough pirate ETA/radar close in, and must choose when to
leave. Leaving before depletion is a yellow **BAIL**; after depletion it becomes
a green **DEPART**. Pirates may demand tribute or attack outright; fleeing under
fire is slower with a full hold, and hull loss can cascade into power outages,
life-support failures, death, and ship loss. Credits buy fuel, repairs, ships,
fuel-tank/cargo upgrades, system-transfer permits, cosmetics, and eventually
space stations that generate local passive income.

The game was prototyped in HTML/React at
`C:\Users\user\Documents\Repositories\mynameis-nigel\moon-miner-materials\Web-Prototype\`
(`Moon Miner.dc.html` + `Moon Miner - Design Notes.md`). That prototype is now
**historical reference only** for terminal feel, palette, and some layout
ideas. The docs in this repository are authoritative for gameplay and balance;
where the prototype and these docs disagree, these docs win.

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

See [the beta progression audit](tests/04-beta-progression-audit.md) for the
2.0.0 test-first evidence, balance decisions, and migration behavior.

## Test-first implementation

Follow [tests/README.md](tests/README.md) for every feature and update. Start
with an affected-test audit and a failing acceptance or regression test; then
implement and refactor. Tests belong in the same change as the behavior they
protect. The historical phases below describe dependencies, not permission to
postpone testing until Phase 4.

## Build order and parallelism

```
Phase 0 (done):        repo + go.mod + this plan
Phase 1 (parallel):    framework/01  gameplay/01  tui/04(theme only)
Phase 2 (parallel):    framework/02  gameplay/02  tui/01
Phase 3 (parallel):    framework/03  gameplay/03  tui/02  tui/03
Phase 4 (parallel):    framework/04  gameplay/04  tests/01..03
Phase 5 (live, single-doc-pair): gameplay/05 + tui/05 (fleet ships, slots,
  shipyard economy — replaces the single-track upgrade system and the
  chart-overlay Shipyard; not parallel with anything else in this phase)

Post-Phase 5 (design review): gameplay/06 (full-loop gameplay edge-case audit
and resolution plan; sequence its implementation work after explicit product
decisions on route topology, recovery, and difficulty)

Phase 6 (live, single doc):   gameplay/07 (pirate combat, named roster,
  bounties — owns its own sim + TUI work)

Live fix (done, v1.6.2):
  framework/05 (reconnect and location restore — the shipped softlock)

Beta 2.0.0 (implemented): gameplay/08 (jump network, pilot
  ratings, frontier systems and hulls — supersedes the Jump Drive slot from
  gameplay/05 and the system transfer fee from gameplay/03; save schema v8)

v1.7 (implemented on integration branch):
  tui/06 (responsive 80×24–144×48 frame, deterministic region allocation,
  frame-relative hitboxes, constrained overlays, and responsive regression
  matrix)
```

Dependencies are listed per-task; the summary:

- `gameplay/01` (sim skeleton + belt gen) blocks `gameplay/02..04` and gives
  `tui/02,03` their data types.
- `framework/01` (SSH server) and `tui/01` (app shell) can be built with stub
  models/stores; they meet in Phase 3.
- Feature acceptance tests are written before implementation in every phase.
  `tests/*` adds deeper integration, replay, distribution, and fuzz coverage;
  it does not replace the tests required in each feature PR.

## Task-doc format

Every task file follows the same shape so agents can execute without extra
context:

- **Goal** — one paragraph of intent.
- **References** — exact files in `../ssh-idlefarmer` (or the prototype) to
  mirror for standalone game mechanics; `../ssh-arcadelobby` and `../ssh-farm`
  for anything touching the arcade fleet (identity, durability, deployment).
- **Test plan and affected-test audit** — observable expectations, boundary and
  failure cases, existing tests to review, and the focused command to run red
  before implementation.
- **Deliverables** — packages/files and tests to create or update.
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
   save before exit. An in-progress mining run resolves the same emergency
   bail/escape path used for disconnects, so cargo is kept only if the ship
   escapes and ship death remains possible.
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
   any task done; run `CGO_ENABLED=1 go test -race ./...` before a PR. Follow
   the [test-first workflow](tests/README.md), including an audit of existing
   tests affected by feature updates. Tests live alongside code as `*_test.go`.
10. **This game lives behind the arcade router.** Identity must resolve both
    direct connections (dev/local) and proxied connections from
    `ssh-arcadelobby` per its canonical protocol
    (`../ssh-arcadelobby/docs/02-bridge-and-identity-protocol.md`) — see
    framework/01. Persistence must follow the fleet's Litestream+S3 pattern
    (`../ssh-arcadelobby/docs/06-fleet-data-durability.md`) so an EC2 instance
    loss doesn't lose pilot saves — see framework/04. `../ssh-farm` is the
    worked example of both; copy its shape rather than re-deriving it.
11. **Durability is drilled, not assumed.** `scripts/restore-drill/` runs the
    real image against a local MinIO: kill without a graceful flush, delete
    the volume, prove a fresh container restores genuine pilot data, verified
    by `PRAGMA integrity_check` plus a decode of every save blob
    (`cmd/restore-check`). Run them after any change to `Dockerfile`,
    `entrypoint.sh`, `etc/litestream.yml`, or the store schema — those four
    are the parts no Go test covers. What they cannot cover is the EC2
    instance-role credential path, which only the live host exercises.

## Designed but not built (2026-08-15 audit)

Three subsystems are specified across these docs — some of them at length, and
some as acceptance criteria — that **do not exist in the code at all**. Nothing
in `internal/` or `data/` references them; they are not partial, they are
absent. Read the docs below with that in mind, and do not treat their
acceptance lists as regressions.

| Designed | Where it is specified | Reality on `main` (v1.6.1) |
| --- | --- | --- |
| **Space stations** — build cost, ownership, capped offline passive income, the late-game credit sink | `02-danger-economy-and-progression.md` (16 refs), `gameplay/03` (32 refs), `gameplay/01`, `gameplay/04`, `tui/02` (17 refs), `framework/03` ("the actor calls the gameplay/03 station-income helper on attach") | No `Station` identifier anywhere in the repo. The attach path has no income helper. `B build station` is not a key on any screen |
| **Cosmetics** — purchasable, persist through death, never change derived values | `gameplay/04` (18 refs), `tui/02` (12 refs), `tui/04`, `01-concept-and-story.md` | No `Cosmetic` identifier anywhere. `sim.Settings` carries only belt view, pirate aggression, high contrast, ASCII-safe, reduced motion, wrap, insurance-used. `C` on the star chart is **sell cargo**, not cosmetics |
| **KEPLER REACH / REDLINE EXPANSE systems** | `01-concept-and-story.md` § "Systems, worlds, and ships"; now fully specified in [gameplay/08-jump-network-and-frontier-progression.md](gameplay/08-jump-network-and-frontier-progression.md) | `data/worlds.toml` ships two systems (SOL, ERIDANI DRIFT) and eight destinations. The lock model those two were meant to demonstrate is instead carried by permits and required-item gates on the Sol/Eridani destinations. **Implemented on beta in 2.0.0** — gameplay/08 adds both systems, their distinct pressures, the rating ladder, and frontier hulls |

The economy is currently balanced without a station sink, so adding one is a
balance change, not a fill-in-the-blank. Decide whether stations and cosmetics
are still wanted before writing a task doc for either; if they are dropped,
strike them from the four docs above rather than leaving the acceptance lists
unachievable.

## Document map

| Doc | Task |
| --- | --- |
| [01-concept-and-story.md](01-concept-and-story.md) | Story, loop, screens, visual direction (read-only context) |
| [02-danger-economy-and-progression.md](02-danger-economy-and-progression.md) | Roguelite death/ship-loss, locked planets/systems, stations, events, open design questions (flee-only pirate behavior superseded by gameplay/07) |
| [framework/01-ssh-server-and-identity.md](framework/01-ssh-server-and-identity.md) | Wish SSH server, middleware chain, key identity |
| [framework/02-persistence-and-save-model.md](framework/02-persistence-and-save-model.md) | SQLite store, schema, save serialization |
| [framework/03-session-lifecycle-and-actors.md](framework/03-session-lifecycle-and-actors.md) | Save manager, actor goroutines, takeover policy, shutdown flush |
| [framework/04-config-content-and-deployment.md](framework/04-config-content-and-deployment.md) | `MOONMINER_*` config, TOML content loader, Docker deploy |
| [framework/05-reconnect-and-location-restore.md](framework/05-reconnect-and-location-restore.md) | **Fixed in v1.6.2.** Reconnecting pilots used to open on the star chart while still in a belt, where every docked-gated service refuses — a softlock, unrecoverable with a full hold. Sessions now open where the save says the pilot is, `Depart` is docked-gated, and a one-shot notice reports what the disconnect autopilot did |
| [gameplay/01-simulation-engine-and-belt-generation.md](gameplay/01-simulation-engine-and-belt-generation.md) | Sim state, RNG, world data, belt generation |
| [gameplay/02-mining-run-loop.md](gameplay/02-mining-run-loop.md) | Manual mining tick, pirate actions, escape, random events, ship death |
| [gameplay/03-economy-worlds-and-balance.md](gameplay/03-economy-worlds-and-balance.md) | Credits, port services, travel, balance TOML |
| [gameplay/04-progression-and-ship-log.md](gameplay/04-progression-and-ship-log.md) | Lifetime stats, ship upgrades (ships/upgrades sections superseded by gameplay/05), run history |
| [gameplay/05-fleet-ships-and-shipyard-economy.md](gameplay/05-fleet-ships-and-shipyard-economy.md) | 4-ship hangar, brands/classes, stat grades, Utility/Weapon/Internal/Jump Drive slots, power/mass, buyback, scanner range/speed, hull-gated events |
| [gameplay/06-gameplay-edge-case-audit.md](gameplay/06-gameplay-edge-case-audit.md) | Full-loop edge cases, gameplay-logic decisions, implementation order, and test plan |
| [gameplay/07-pirate-combat-and-bounties.md](gameplay/07-pirate-combat-and-bounties.md) | Named pirate roster, autocannon, missile, and pulse weapons, tactical-scope combat, bounty vouchers, COMBAT MODE interstitial (owns its sim + TUI work) |
| [gameplay/08-jump-network-and-frontier-progression.md](gameplay/08-jump-network-and-frontier-progression.md) | **Beta:** pilot Jump Ratings replace the Jump Drive module, four-system network, mass-scaled jump fuel + transit stress + drift roll, the jump cinematic, located hangar + hauler ferry, three frontier hulls (owns its sim + TUI work) |
| [tui/01-app-shell-input-and-mouse.md](tui/01-app-shell-input-and-mouse.md) | Root model, screen router, keyboard + mouse input, resize/idle |
| [tui/02-star-chart-and-run-summary.md](tui/02-star-chart-and-run-summary.md) | Star Chart screen, port services panel, Run Summary screen |
| [tui/03-belt-views-and-mining-screen.md](tui/03-belt-views-and-mining-screen.md) | Belt screen (3 view modes), target lock, live Mining screen |
| [tui/04-theme-components-and-tweaks.md](tui/04-theme-components-and-tweaks.md) | Phosphor-blue theme, panel/bar components, Tweaks overlay |
| [tui/05-shipyard-screen.md](tui/05-shipyard-screen.md) | Standalone cyber-futuristic Shipyard screen: hangar, loadout, power/mass meters |
| [tui/06-responsive-menu-overhaul.md](tui/06-responsive-menu-overhaul.md) | **Authoritative v1.7 layout contract:** centered capped frame, shared allocator, responsive screen regions, overlays, clipping, hitboxes, and exhaustive test matrix |
| [tests/01-sim-engine-tests.md](tests/01-sim-engine-tests.md) | Determinism, outcome tables, distribution + property tests |
| [tests/02-server-integration-tests.md](tests/02-server-integration-tests.md) | In-process SSH client tests: auth, PTY, policy, shutdown |
| [tests/03-tui-rendering-and-input-tests.md](tests/03-tui-rendering-and-input-tests.md) | Update-loop tests, golden renders, hitbox/mouse tests |
