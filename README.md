# ssh-moonminer

**Moon Miner** is a space-mining extraction game played entirely over SSH.
No client to install, no account to create — your SSH public key *is* your
account:

```bash
ssh moonminer.example.com
```

Pilot a mining ship between worlds, drop into an asteroid belt, and gamble on
which rocks to drill. Bigger asteroids pay more but take longer and burn more
fuel; rarer ones pay much more but attract pirates. Every run is
push-your-luck: drill to 100% for full value, or bail early and bank what you
have.

## Status

**Planning.** The game is not implemented yet. `docs/` contains the complete
build plan: [docs/README.md](docs/README.md) is the index and architecture
contract, and each file under `docs/framework/`, `docs/gameplay/`,
`docs/tui/`, and `docs/tests/` is a self-contained task an agent can pick up
independently.

## Stack (planned)

- **Go 1.26+**, pure Go (`CGO_ENABLED=0`)
- **charm.land/wish/v2** — SSH server framework
- **charm.land/bubbletea/v2** + **lipgloss/v2** — terminal UI (keyboard + mouse)
- **modernc.org/sqlite** — cgo-free persistence
- **Docker** — distroless static image

## Commands

```bash
go build -o bin/ssh-moonminer ./cmd/ssh-moonminer   # build
go test ./...                                        # test
go vet ./...                                         # vet
```

## Sibling project

`../ssh-idlefarmer` is a finished game on the same stack. Its SSH server,
identity, persistence, and session-management code is the reference
implementation this project mirrors — the docs cite specific files from it.
