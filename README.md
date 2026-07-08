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
Ships as `v1.0.0 (alpha)` — hardly polished, but stable. Docker image, Litestream/S3
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

## Fleet integration

Behind `ssh-arcadelobby`, set `MOONMINER_PROXY_KEYS_PATH` to the router's bridge
public key file (same pattern as ssh-farm's `FARM_PROXY_KEYS_PATH`). The fleet
compose block in `ssh-arcadelobby/deploy/docker-compose.yml` is wired and live;
`games.toml` lists this game with `version = "1.0.0"`.

## Deploy

Dockerfile + `entrypoint.sh` + `etc/litestream.yml` implement the fleet's canonical
Litestream/S3 durability pattern (`../ssh-arcadelobby/docs/06-fleet-data-durability.md`) —
no AWS credentials required in dev (`LITESTREAM_REPLICA_URL` unset skips replication).
`.github/workflows/ci.yml` runs `vet`/`build`/`test -race` on PRs and non-main pushes;
`release.yml` builds/publishes `ghcr.io/mynameis-nigel/ssh-moonminer` on merge to `main`
and redeploys on the same self-hosted `play.ssharcade.dev` runner ssh-arcadelobby and
ssh-farm use.

## Sibling references

- `../ssh-farm` — fleet SSH server, identity, store, actor model (primary reference)
- `../ssh-arcadelobby` — router bridge protocol and games registry
