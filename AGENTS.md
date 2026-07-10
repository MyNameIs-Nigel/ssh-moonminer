# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## What this is

**Moon Miner**: a hard roguelite asteroid-mining game played entirely over SSH — the player's SSH
public key is their account, no client install required. It runs live behind the `ssh-arcadelobby`
router as part of the ssharcade fleet (see `/home/nigel/repos/ssh-games/AGENTS.md` for fleet-wide
context — sibling repos `../ssh-farm` and `../ssh-arcadelobby` are reference implementations for
fleet-specific patterns like identity and durability).

**`main` deploys for real** on every push (`release.yml` builds/publishes to `ghcr.io` and
redeploys `play.ssharcade.dev` via a self-hosted runner) — never commit directly to `main`.

## Commands

```bash
go build ./...                              # build
go vet ./...                                # vet
go test ./...                               # all tests
go test ./internal/sim -run 'TestName'      # single test
MOONMINER_LISTEN_PORT=2222 go run ./cmd/ssh-moonminer
ssh -p 2222 -o StrictHostKeyChecking=accept-new localhost
```

Run `go build ./...`, `go vet ./...`, and `go test ./...` before declaring any task done.
`go test -race` doesn't work in this dev sandbox (no CGO) — CI (`.github/workflows/ci.yml`) runs
it on PRs and non-main pushes; race conditions have historically surfaced only there.

Local data lands in gitignored `var/` (`var/moonminer.db`, `var/ssh_host_key`) — never commit it.

## Architecture

One process, Elm-style TUI over SSH. The player's SSH connection is either direct (dev/local) or
proxied through the arcade router's trusted-proxy protocol; either way identity resolves to a
save-slot fingerprint that everything downstream keys on.

Boot sequence in `cmd/ssh-moonminer`: config → store → content → game manager → SSH server →
graceful shutdown (SIGTERM must flush every active save — see `internal/game/manager.go`).

Package layout (boundaries matter — see `docs/README.md` § "Package layout" for the full
task-ownership contract this was built under):

| Path | Purpose |
| --- | --- |
| `internal/config/` | `MOONMINER_*` env vars → `Config` |
| `internal/server/` | Wish SSH server, middleware chain, PTY handling, rate limits, shutdown hooks |
| `internal/identity/` | Public-key fingerprint → save-slot; resolves both direct and router-proxied connections per `../ssh-arcadelobby/docs/02-bridge-and-identity-protocol.md` |
| `internal/game/` | Save lifecycle: `Manager` (attach/detach), per-save actor goroutines (`actor.go`), `Session` handle, autosave |
| `internal/store/` | SQLite persistence + append-only migrations |
| `internal/content/` | Loads/validates `data/*.toml` (worlds + balance) |
| `internal/sim/` | Headless, pure, deterministic simulation engine — state, belt generation, mining-run tick (`run.go`), skill checks, escape/death outcomes, economy |
| `internal/tui/` | Bubble Tea root model (`game.go`) + screens: chart, belt, mining, death, summary, ship log, help, tweaks overlay |
| `internal/tui/theme/` | Phosphor-blue color theme |
| `internal/tui/hitbox` | Mouse hit-testing for TUI components |
| `data/` | `worlds.toml` + `balance.toml`, embedded via `data/embed.go`, overridable at runtime via `MOONMINER_DATA_DIR` |

**The one rule that matters most: `internal/sim` stays pure.** No I/O, no logging, no wall-clock
reads — timestamps and RNG state always come in as arguments, because deterministic replay is a
hard test requirement (see `internal/sim/run_test.go` for the pattern). The TUI renders `sim`
state and calls `sim` action functions; it never mutates state directly.

Other conventions worth knowing before touching code:

- **Wish middleware runs bottom-up** in `server.New()` — logging/rate-limit outermost, the
  save-attach middleware adjacent to the Bubble Tea handler.
- **Raw-session messages use `\r\n`** (no tty cooking before the TUI starts).
- **Windows-host PTY fixes** (color-profile forcing + `cursorDownWriter` newline rewriting) in
  `internal/server` must be preserved — sessions render garbage on Windows hosts without them. The
  dev machine is Windows.
- **Balance lives in TOML, not code** — tunable constants go in `data/*.toml`, formulas in Go.
- A mining run's emergency bail/escape path (`internal/sim/run.go`) is also what graceful shutdown
  and disconnects resolve through — cargo is kept only if the ship escapes, and ship death remains
  possible even on shutdown.

## Docs as the build plan

`docs/README.md` is the index into `docs/framework/`, `docs/gameplay/`, `docs/tui/`,
`docs/tests/` — each numbered doc is a self-contained task (goal, references, deliverables, spec,
acceptance criteria) written against this package layout. `docs/01-concept-and-story.md` and
`docs/02-danger-economy-and-progression.md` are the authoritative gameplay/story spec (an earlier
HTML/React prototype is historical reference only, superseded where the two disagree). Consult
these docs for *why* a system is shaped the way it is before changing sim/economy behavior.

## Deploy

`Dockerfile` + `entrypoint.sh` + `etc/litestream.yml` implement the fleet's Litestream/S3
durability pattern — no AWS credentials needed in dev (`LITESTREAM_REPLICA_URL` unset skips
replication). Image is hardened alpine (not distroless): non-root uid 65532, read-only root FS —
needed because the entrypoint requires a shell plus the `litestream`/`mc` binaries.
