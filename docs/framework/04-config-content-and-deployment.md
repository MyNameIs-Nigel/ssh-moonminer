# Framework 04 — Config, Content Loading & Deployment

**Area:** Framework · **Phase:** 4 · **Depends on:** framework/01–03 existing
(consolidates their settings) · **Parallel-safe with:** gameplay/04, tests/*

## Goal

Every runtime setting comes from a `MOONMINER_*` environment variable with a
sane default; all game data (worlds, tiers, balance constants) loads from
TOML that is embedded in the binary but overridable on disk; and the whole
server ships as a hardened distroless Docker image with one named volume.

## References (mirror these)

| File | What to take |
| --- | --- |
| `../ssh-idlefarmer/internal/config/config.go` | env helpers, validation style, `ListenAddr()` |
| `../ssh-idlefarmer/internal/content/content.go` | TOML load + validate + embedded-vs-dir override |
| `../ssh-idlefarmer/data/embed.go` | `//go:embed` pattern |
| `../ssh-idlefarmer/docker-compose.yml` | volume, ports, `stop_grace_period`, restart policy |
| `../ssh-idlefarmer/CLAUDE.md` (Docker notes) | distroless, non-root uid 65532, read-only rootfs |

## Deliverables

- `internal/config/` — `config.go` + tests
- `internal/content/` — `content.go` + tests (loads what gameplay/03 defines)
- `data/embed.go` — embeds `data/*.toml`
- `Dockerfile`, `docker-compose.yml`, `.dockerignore`

## Spec

### Config (env vars)

| Variable | Default | Meaning |
| --- | --- | --- |
| `MOONMINER_LISTEN_HOST` | `0.0.0.0` | bind address |
| `MOONMINER_LISTEN_PORT` | `22` | bind port (use `2222` for rootless dev) |
| `MOONMINER_HOST_KEY_PATH` | `var/ssh_host_key` | SSH host key |
| `MOONMINER_DB_PATH` | `var/moonminer.db` | SQLite file |
| `MOONMINER_DATA_DIR` | *(empty = embedded)* | TOML content override dir |
| `MOONMINER_IDLE_TIMEOUT` | `30m` | UI-enforced idle disconnect |
| `MOONMINER_AUTOSAVE_INTERVAL` | `30s` | actor autosave cadence |
| `MOONMINER_SESSION_POLICY` | `takeover` | `takeover` or `refuse` |
| `MOONMINER_DEFAULT_SLOT` | `default` | slot when username sanitizes empty |
| `MOONMINER_MAX_CONNECTIONS` | `100` | global cap |
| `MOONMINER_MAX_SESSIONS_PER_KEY` | `2` | per-fingerprint cap |
| `MOONMINER_RATE_LIMIT_PER_SECOND` | `2` | new-connection rate |
| `MOONMINER_RATE_LIMIT_BURST` | `5` | burst |
| `MOONMINER_RATE_LIMIT_MAX_IPS` | `1000` | tracked IPs |
| `MOONMINER_LOG_LEVEL` | `info` | slog level |
| `MOONMINER_LOG_FORMAT` | `text` | `text` or `json` |

Validation mirrors idlefarmer: port range, positive rates, sanitizable
default slot, autosave ≥ 1s, policy enum. `Load()` returns
`(Config, error)`; `main.go` exits non-zero on error.

### Content loading

- `content.Load(dataDir)` reads `worlds.toml` and `balance.toml` — from
  `dataDir` if set, else from the embedded FS. **Validate hard at boot**:
  exactly the fields gameplay/03 specifies, 4+ worlds, 4 rarity tiers with
  ascending multipliers, all multipliers/costs positive, glyphs non-empty.
  A server that boots with bad balance data is worse than one that refuses
  to start.
- `content.Content` is immutable after load; passed by pointer to sim, TUI,
  and manager.

### Docker

- **Build stage**: `golang:1.26` (match toolchain), `CGO_ENABLED=0`,
  `go build -trimpath -ldflags="-s -w"`.
- **Run stage**: `gcr.io/distroless/static-debian12:nonroot`, uid 65532,
  `read_only: true` rootfs, `tmpfs: /tmp`.
- **Volume**: `moonminer-data` mounted at `/var/lib/moonminer`, holding both
  the DB and the host key (`MOONMINER_DB_PATH=/var/lib/moonminer/moonminer.db`,
  `MOONMINER_HOST_KEY_PATH=/var/lib/moonminer/ssh_host_key`) so redeploys
  keep both data *and* server identity — clients must not see host-key
  warnings after an upgrade.
- **Compose**: host port 22 → container port (configurable),
  `stop_grace_period: 45s` (must exceed the 30s shutdown-hook context so the
  save flush always wins), `restart: unless-stopped`.
- The binary must write **only** under the volume and `/tmp` — audit config
  defaults in the container to guarantee this.

### Developer experience

- `README.md` gains a "Run locally" section:
  `MOONMINER_LISTEN_PORT=2222 go run ./cmd/ssh-moonminer` then
  `ssh -p 2222 -o StrictHostKeyChecking=accept-new localhost`.
- Optional but appreciated: a `Makefile` or `justfile` with `build`, `test`,
  `run`, `deploy` targets.

## Acceptance criteria

- [ ] Unset env → defaults load and validate; each invalid value produces a
  named, actionable error (table-driven tests).
- [ ] Embedded content loads with `MOONMINER_DATA_DIR` unset; a dir override
  with modified numbers is picked up; a dir with an invalid file fails boot
  with a clear message.
- [ ] `docker compose up -d --build` serves SSH; `docker compose down` +
  `up` again preserves pilots **and** host key.
- [ ] `docker compose stop` (SIGTERM) with an active session flushes the
  save (verify via reconnect).
- [ ] Container runs non-root with read-only rootfs (inspect + smoke test).

## Out of scope / handoffs

- The TOML schemas' game meaning and numbers → gameplay/03 owns
  `data/worlds.toml` / `data/balance.toml` contents; this task owns the
  loader and validation plumbing.
- CI pipelines — out of scope for MVP.
