# ssh-moonminer

**Moon Miner** is a push-your-luck asteroid mining game played entirely over SSH.
Your SSH public key is your account — no client install required.

```bash
ssh moonminer.example.com
```

## Status

**Playable prototype.** Full chart → belt → mine → summary loop with persistent pilots,
arcade-router proxied identity (ssh-farm pattern), and SQLite saves.

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
compose block in `ssh-arcadelobby/deploy/docker-compose.yml` is ready once a
container image is published.

## Sibling references

- `../ssh-farm` — fleet SSH server, identity, store, actor model (primary reference)
- `../ssh-arcadelobby` — router bridge protocol and games registry
