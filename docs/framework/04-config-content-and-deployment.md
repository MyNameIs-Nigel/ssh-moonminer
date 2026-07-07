# Framework 04 — Config, Content Loading & Deployment

**Area:** Framework · **Phase:** 4 · **Depends on:** framework/01–03 existing
(consolidates their settings) · **Parallel-safe with:** gameplay/04, tests/*

## Goal

Every runtime setting comes from a `MOONMINER_*` environment variable with a
sane default; all game data (worlds, tiers, balance constants) loads from
TOML that is embedded in the binary but overridable on disk; and the whole
server ships as a hardened Docker image with one named volume, implementing
the fleet's canonical durability pattern (`../ssh-arcadelobby/docs/06-fleet-data-durability.md`)
so an EC2 instance/volume loss doesn't lose pilot saves. **Not distroless** —
see Docker section below; that was this doc's own earlier guidance and it's
now superseded fleet-wide.

## References (mirror these)

| File | What to take |
| --- | --- |
| `../ssh-idlefarmer/internal/config/config.go` | env helpers, validation style, `ListenAddr()` |
| `../ssh-idlefarmer/internal/content/content.go` | TOML load + validate + embedded-vs-dir override |
| `../ssh-idlefarmer/data/embed.go` | `//go:embed` pattern |
| `../ssh-arcadelobby/docs/06-fleet-data-durability.md` | **canonical** — the Litestream+S3 pattern every fleet game implements; bucket layout, IAM policy shape, container pattern, drill requirements |
| `../ssh-farm/internal/config/config.go` | the concrete env-var/validation shape to mirror, including `ProxyKeysPath`/`FARM_PROXY_KEYS_PATH` (rename to `MOONMINER_*`) |
| `../ssh-farm/Dockerfile` | **the reference implementation of doc 06** for a fleet game: `alpine:3.22` runtime (not distroless — the entrypoint needs a shell), litestream + `mc` copied in as static binaries, non-root uid 65532, `HOME=/tmp` (mc needs a writable config dir the nonroot user otherwise lacks) |
| `../ssh-farm/entrypoint.sh` | **copy this shape nearly verbatim**, renaming `FARM_*`→`MOONMINER_*` — host-key restore/backup via `mc`, `litestream restore`/`replicate`, the dev-mode skip when `LITESTREAM_REPLICA_URL` is unset. Includes two real bugs already found and fixed here: a bare `>` redirect creates a 0-byte file before `mc` runs (fooling the "key already exists" check on a genuinely empty bucket — write to a `.tmp` path and only promote it if `mc` produced non-empty content), and `mc` silently no-ops every call without a writable `$HOME`. |
| `../ssh-farm/docker-compose.yml` | the game's own **standalone** dev compose (ports, volume-as-cache framing, hardening block) — separate from the fleet compose (below) |

## Deliverables

- `internal/config/` — `config.go` + tests
- `internal/content/` — `content.go` + tests (loads what gameplay/03 defines)
- `data/embed.go` — embeds `data/*.toml`
- `Dockerfile`, `entrypoint.sh`, `docker-compose.yml` (standalone dev compose),
  `.dockerignore`, `etc/litestream.yml`

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
| `MOONMINER_PROXY_KEYS_PATH` | *(empty)* | authorized_keys-format file of trusted arcade proxy public keys (framework/01); empty = direct-only dev, no arcade router trusted |

Plus the durability env vars `entrypoint.sh` reads directly (not through
`internal/config` — they're shell/litestream/mc concerns, not app config; see
Docker section): `LITESTREAM_REPLICA_URL`, `LITESTREAM_S3_REGION`,
`MOONMINER_HOST_KEY_MC_PATH`, `MC_HOST_s3`. All optional and unset in dev.

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

- **Build stage**: `golang:1.26.4-alpine3.22` (match toolchain), `CGO_ENABLED=0`,
  `go build -trimpath -ldflags="-s -w"`. Mirror `../ssh-farm/Dockerfile`'s
  build stage, including its `restore-check`-style pattern if a
  durability-drill CLI is added later (tests/02, post-MVP for this game).
- **Litestream + mc stages**: pull `litestream/litestream:0.5.12` and
  `minio/mc:RELEASE.2025-08-13T08-35-41Z` (pin exact versions, same as farm)
  as separate `FROM ... AS` stages purely to copy their static binaries into
  the runtime stage — no compilation of either.
- **Run stage**: `alpine:3.22`, **not distroless** — per the canonical fleet
  doc, the entrypoint needs a real shell (`sh`) to run `mc`/`litestream`
  before handing off to the game binary. `apk add ca-certificates`, create a
  non-root `nonroot:nonroot` (uid/gid 65532), copy in `litestream`, `mc`, the
  game binary, and `entrypoint.sh` (`chmod 755`). Keep the rest of the
  hardening: `read_only: true` rootfs (enforced in compose, not the
  Dockerfile), `tmpfs: /tmp`, `cap_drop: [ALL]`, `no-new-privileges`.
  `ENV HOME=/tmp` — `mc` writes its config there even with every alias coming
  from an `MC_HOST_*` env var, and the nonroot user has no writable
  `/home/nonroot`.
- **`entrypoint.sh`** (mirror `../ssh-farm/entrypoint.sh` renaming
  `FARM_*`→`MOONMINER_*`):
  1. Dev-mode escape hatch: if `LITESTREAM_REPLICA_URL` is unset, log it
     loudly and `exec /app/ssh-moonminer` directly — local dev and CI must
     never require AWS/MinIO credentials.
  2. If `MOONMINER_HOST_KEY_MC_PATH` is set and no local host key exists yet,
     restore it from the bucket via `mc cat ... >"$PATH.tmp"`, promoting the
     temp file only if it's non-empty (the empty-bucket-first-boot 0-byte-file
     gotcha farm hit for real), then `chmod 600`.
  3. `litestream restore -if-db-not-exists -if-replica-exists "$DB_PATH"`
     (config from `/etc/litestream.yml`, templated from `../ssh-farm/etc/litestream.yml`
     with `FARM_*`→`MOONMINER_*` var names — see `dbs[0].path`/`replicas[0].url`
     expansion).
  4. Background-upload the host key once Wish generates it on a genuinely
     first-ever boot (poll up to 30s), so this instance seeds the bucket for
     the next one.
  5. `exec litestream replicate -exec "/app/ssh-moonminer"`.
- **Volume**: `moonminer-data` mounted at `/var/lib/moonminer`, holding both
  the DB and the host key (`MOONMINER_DB_PATH=/var/lib/moonminer/moonminer.db`,
  `MOONMINER_HOST_KEY_PATH=/var/lib/moonminer/ssh_host_key`). This volume is a
  **fast local cache, not the durability story** — with `LITESTREAM_REPLICA_URL`
  set, every write also streams to S3, so losing the volume loses at most one
  autosave interval; without it (dev mode), the volume is the only copy.
- **Standalone compose** (this repo's own `docker-compose.yml`, for local
  dev/testing — mirror `../ssh-farm/docker-compose.yml`): host port 22 →
  container port 2222 (`MOONMINER_LISTEN_PORT=2222` in the image, unprivileged),
  `stop_grace_period: 45s` (must exceed the 30s shutdown-hook context so the
  save flush always wins), `restart: unless-stopped`, durability env vars
  commented out by default, plus the hardening block above.
- **Fleet compose**: this repo does *not* own its production compose entry —
  `../ssh-arcadelobby/deploy/docker-compose.yml` already has a `moonminer`
  service block (currently disconnected, pending this repo shipping a real
  image) and `../ssh-arcadelobby/deploy/games.toml` already has a `moonminer`
  registry entry (`addr = "moonminer:2222"`, per
  `../ssh-arcadelobby/docs/03-games-registry-and-health.md`'s schema).
  **No published ports** on the game service — the router is the only public
  entry point (doc 02's requirement); this repo's job is just to publish a
  working `ghcr.io/mynameis-nigel/ssh-moonminer` image the existing block can
  pull. CI/CD to build and publish that image (and, once ready, deploy it on
  the same self-hosted `play.ssharcade.dev` runner arcadelobby/farm already
  use) is a small, mechanical fast-follow — copy `../ssh-farm/.github/workflows/{ci,release}.yml`
  nearly verbatim; not blocking for this task.
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
- [ ] With `LITESTREAM_REPLICA_URL` unset, the container boots and serves
  with no AWS/MinIO credentials present (dev-mode path).
- [ ] Against a local MinIO (reuse `../ssh-farm/scripts/restore-drill/`'s
  scripts pointed at this game's compose service — doc 06 rule 3 says the
  drill script is fleet-wide, don't fork a new one): `kill-drill.sh` and
  `restore-to-scratch.sh` pass, and `no-credentials-check.sh` confirms no
  AWS keys anywhere in the image/compose.

## Out of scope / handoffs

- The TOML schemas' game meaning and numbers → gameplay/03 owns
  `data/worlds.toml` / `data/balance.toml` contents; this task owns the
  loader and validation plumbing.
- The fleet compose entry / registry cutover in `games.toml` → owned by
  `ssh-arcadelobby`'s `deploy/`; this task only needs to ship a pullable
  image with the right env-var contract.
- CI/CD workflow files — small effort once this task lands (copy farm's
  verbatim), but not part of this task's acceptance criteria.
