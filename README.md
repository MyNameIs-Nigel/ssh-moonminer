# ssh-moonminer

**Moon Miner** is a hard roguelite asteroid mining game played entirely over SSH.
Your SSH public key is your account — no client install required.

```bash
ssh moonminer.example.com
```

## Status

**Playable prototype, live behind the router.** The documentation now targets a
revamped chart → belt → manual mining → escape/death → summary loop with
locked planets/systems, disposable ships, cargo holds, pirate demands/attacks,
random ship events, cosmetics, and late-game stations. The current deployed
prototype still has persistent pilots, arcade-router proxied identity
(ssh-farm pattern), and SQLite saves.
Beta branch: `v2.0.0 (beta)` adds pilot Jump Ratings, four systems, located fleets,
hauler ferries, and frontier hulls. See [the progression audit](docs/tests/04-beta-progression-audit.md). Docker image, Litestream/S3
durability, and CI/CD (`ci.yml`/`release.yml`, same shape as ssh-farm's) all deploy for
real to `play.ssharcade.dev` on every push to `main`.

## Stack

- Go 1.26+, pure Go (`CGO_ENABLED=0`)
- charm.land/wish/v2 + bubbletea/v2 + lipgloss/v2
- modernc.org/sqlite

## Run locally

```bash
MOONMINER_LISTEN_PORT=2222 go run ./cmd/ssh-moonminer
ssh -p 2222 -o StrictHostKeyChecking=accept-new localhost
```

Data persists under `var/moonminer.db` and `var/ssh_host_key`.

## Commands

```bash
go build -o bin/ssh-moonminer ./cmd/ssh-moonminer
go test ./...
go vet ./...
```

## Development workflow

Follow [the test-first workflow](docs/tests/README.md) for features and fixes:
write the acceptance/regression test, observe its failure, implement, then
refactor. Feature updates must audit affected existing tests for obsolete
expectations, invalid fixtures, and missing edge cases. Run build, vet, the full
test suite, and the race suite before opening a PR.

## Fleet integration

Behind `ssh-arcadelobby`, set `MOONMINER_PROXY_KEYS_PATH` to the router's bridge
public key file (same pattern as ssh-farm's `FARM_PROXY_KEYS_PATH`). The fleet
compose block in `ssh-arcadelobby/deploy/docker-compose.yml` is wired and live;
`games.toml` lists this game with `version = "1.6.3"` — a fallback only; the
router's prober reads the live version off the SSH banner, which comes from
`internal/version`.

## Deploy

Dockerfile + `entrypoint.sh` + `etc/litestream.yml` implement the fleet's canonical
Litestream/S3 durability pattern (`../ssh-arcadelobby/docs/06-fleet-data-durability.md`) —
no AWS credentials required in dev (`LITESTREAM_REPLICA_URL` unset skips replication).
`.github/workflows/ci.yml` runs `vet`/`build`/`test -race` on PRs and non-main pushes;
`release.yml` builds/publishes `ghcr.io/mynameis-nigel/ssh-moonminer` on merge to `main`
and stops there — nothing deploys automatically. Shipping that image to
`play.ssharcade.dev` is a manual `docker compose pull moonminer && up -d moonminer`
(`../ssh-arcadelobby/deploy/README.md` § "Deploying by hand"); the self-hosted runners the
fleet used to deploy with were removed on 2026-09-04 ahead of the repos going public.

**Durability drills** live in [`scripts/restore-drill/`](scripts/restore-drill/) and run
locally against a MinIO container using the exact image the fleet ships. They kill the
container without a graceful flush, delete its volume, and prove a fresh one restores
real pilot data from the bucket — verified with `PRAGMA integrity_check` plus a decode of
every save blob (`cmd/restore-check`, built into the runtime image), and a measured RPO:

```sh
cd scripts/restore-drill
./run.sh                   # empty volume + populated bucket restores and serves
./kill-drill.sh            # + real save data, integrity/decode, enforced RPO budget
./restore-to-scratch.sh    # restores the replica with no game container involved
./no-credentials-check.sh  # dev mode never touches S3, proven with --network none
```

They prove the *mechanism*. They cannot prove the *credentials*: production leaves the
litestream access-key vars unset so it picks up the EC2 instance role, while the drill
sets them explicitly. A broken IAM policy passes every drill here.

## Sibling references

- `../ssh-farm` — fleet SSH server, identity, store, actor model (primary reference)
- `../ssh-arcadelobby` — router bridge protocol and games registry
