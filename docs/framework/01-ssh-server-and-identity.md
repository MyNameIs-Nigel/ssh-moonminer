# Framework 01 — SSH Server & Identity

**Area:** Framework · **Phase:** 1 · **Blocks:** framework/03, tests/02 ·
**Parallel-safe with:** gameplay/01, tui/01, tui/04

## Goal

Stand up the Wish v2 SSH server that is the game's only front door. Any SSH
client with any public key gets in (trust-on-first-use); the key's
fingerprint is the account; the SSH username picks a save slot under that
key; and the only thing a session can ever reach is the Bubble Tea game — no
shell, no exec, no forwarding.

```bash
ssh play.example.com            # default save slot
ssh scout@play.example.com      # second pilot under the same key
```

## References (mirror these)

| File | What to take |
| --- | --- |
| `../ssh-idlefarmer/internal/server/server.go` | server construction, middleware order, `attachSave` shape, `teaHandler` |
| `../ssh-idlefarmer/internal/server/pty.go` | `RequirePTY()` middleware + `\r\n` message convention |
| `../ssh-idlefarmer/internal/server/limits.go` | per-key session caps + global connection cap — **but key on the resolved identity, not the wire key** (see below) |
| `../ssh-idlefarmer/internal/server/teaprogram.go` | **copy nearly verbatim** — Windows-host PTY fixes (color profile + `cursorDownWriter`) |
| `../ssh-idlefarmer/internal/server/shutdown.go` | shutdown-hook registry |
| `../ssh-idlefarmer/internal/identity/identity.go` | fingerprint + slot sanitization (`Fingerprint`, `SanitizeSlot`, `ResolveSlot`) |
| `../ssh-arcadelobby/docs/02-bridge-and-identity-protocol.md` | **canonical** — the trusted-proxy protocol this game must implement to sit behind the arcade router |
| `../ssh-farm/internal/identity/resolver.go` + `proxy.go` | **copy this shape wholesale**, renaming `Farm`→`Moonminer` where it appears — `Resolver`/`NewResolver`/`Resolve`, `isProxyKey`/`resolveDirect`/`resolveProxied`, `LoadProxyKeys`, `ParseProxiedUsername`/`EncodeProxiedUsername`, `ErrProxiedIdentity`. This is doc 02 already implemented and running in production; do not re-derive it from the doc alone. |
| `../ssh-farm/internal/server/server.go` (`NewSessionLimits(cfg.MaxConnections, cfg.MaxSessionsPerKey, resolver)`) | how session caps key on the resolver, not the raw wire key |
| `../ssh-farm/cmd/ssh-farm/serve.go` | wiring order: config → logger → content → store → **load proxy keys → build resolver** → manager → server → shutdown (mirror this over idlefarmer's simpler `main.go`) |

## Deliverables

- `internal/server/` — `server.go`, `pty.go`, `limits.go`, `teaprogram.go`,
  `shutdown.go`
- `internal/identity/` — `identity.go` (fingerprint/slot), `proxy.go`
  (proxied-username encode/parse), `resolver.go` (direct vs. proxied
  resolution) — mirror `../ssh-farm/internal/identity/` file-for-file
- `cmd/ssh-moonminer/main.go` — replace the stub with real wiring
- `internal/log/` — slog constructor (copy `../ssh-idlefarmer/internal/log/log.go`)

## Spec

### Authentication & identity

- `wish.WithPublicKeyAuth` accepts any non-nil key. Password/keyboard-
  interactive auth is never offered; keyless clients fail at the protocol
  layer.
- `identity.Fingerprint(key)` → `"SHA256:" + base64(rawstd, sha256(key.Marshal()))`.
- `identity.ResolveSlot(username, defaultSlot)` sanitizes the SSH username to
  `[a-z0-9_-]{1,32}` (lowercase, drop everything else); empty result falls
  back to the configured default slot. Identical to idlefarmer's
  `SanitizeSlot`.

### Arcade proxied identity

This game sits behind the arcade router — idlefarmer, as built, did not.
Unlike a standalone idlefarmer deployment, moonminer's only public entry
point in production is `ssh-arcadelobby`'s router, dialing in over the
private Docker network with the router's own key. Raw `identity.Fingerprint`
on the wire key alone would resolve **every player to the router's one
key** — this must never happen. Implement `identity.Resolver` exactly like
`../ssh-farm/internal/identity/resolver.go`:

- `NewResolver(defaultSlot string, proxyKeys []ssh.PublicKey) *Resolver`.
- `Resolve(s SessionSource) (ResolveResult, error)`: if the session's wire
  key exactly matches one of `proxyKeys` (loaded from
  `cfg.ProxyKeysPath`/`MOONMINER_PROXY_KEYS_PATH` via
  `identity.LoadProxyKeys`, an authorized_keys-format file — empty path is
  valid and means "direct-only dev, no trusted proxies"), parse the
  router-encoded username (`identity.ParseProxiedUsername`, protocol v1 per
  `../ssh-arcadelobby/docs/02-bridge-and-identity-protocol.md`: 64 lowercase
  hex chars, `.`, sanitized slot) and resolve the *player's* fingerprint from
  that, never the router's key. Otherwise resolve directly from the wire key
  (unchanged legacy/dev behavior).
- A malformed username on a proxied connection is a protocol error
  (`ErrProxiedIdentity`) — refuse the session with a `\r\n` message
  (`identity.ProxiedIdentityMessage()`); **never** fall back to treating the
  router's key as a player account.
- `ResolveResult.Proxied` distinguishes the two paths for logging; both paths
  produce the same `SessionIdentity{Fingerprint, Slot}` shape everything
  downstream consumes.
- **Session caps (below) and `game.Manager.Attach` must key on the *resolved*
  fingerprint, not the wire key** — otherwise every arcade player collapses
  onto one cap bucket / one save (the router's), exactly the bug fixed for
  real in ssh-farm's deploy config on 2026-07-06
  (`FARM_PROXY_KEYS_PATH` wiring).

### Middleware chain

Composed exactly like idlefarmer (Wish runs the list bottom-up; effective
inbound order is logging → rate limit → session caps → PTY requirement →
attach save → Bubble Tea handler):

```go
wish.WithMiddleware(
    bubbletea.MiddlewareWithProgramHandler(srv.newTeaProgram),
    srv.attachSave(),
    RequirePTY(),
    limits.Middleware(),
    ratelimiter.Middleware(rl),
    logging.Middleware(),
)
```

- **`RequirePTY()`** rejects no-PTY sessions with a `\r\n`-terminated hint
  (`ssh -t user@host`) and exits 0.
- **Session limits**: global max connections and max sessions per key (keyed
  on `identity.Resolver.Resolve`'s fingerprint, not the wire key — see
  above), both from config, both rejecting politely over-limit.
- **Rate limiter**: `wish/v2/ratelimiter` with per-second rate, burst, and
  max-tracked-IPs from config. The router is the public-facing enforcement
  point for real player IPs (doc 02); this game's own limiter still runs
  (direct/dev use), just tuned generously in the fleet compose.
- **`attachSave`** calls `srv.identity.Resolve(s)` (direct or proxied), then
  `game.Manager.Attach` (framework/03; stub it with an interface until that
  task lands) with the resolved identity, writes a friendly `\r\n` error and
  exits 1 on failure (distinguish "save busy" from generic failure and from a
  proxied-identity protocol error), defers detach, stores session state on
  the `ssh.Context`, logs fingerprint/slot/remote-addr/proxied.

### Bubble Tea program construction

- `newTeaProgram` builds the per-session program:
  `opts := append(bubbletea.MakeOptions(s), windowsPtyOptions(s)...)` then
  **append the mouse option** — Moon Miner requires mouse support
  (`tea.WithMouseCellMotion()`; verify the exact option name against the
  vendored bubbletea v2 — see tui/01 for the input contract).
- Copy `windowsPtyOptions` + `cursorDownWriter` from idlefarmer's
  `teaprogram.go` including its long explanatory comment. This is the fix for
  emulated SSH PTYs on Windows hosts (stripped colors, left-smearing
  incremental updates). Do not "simplify" it.
- The initial window size comes from `s.Pty()` (default 80×24 if absent);
  pass it to the TUI constructor along with the session handle, content, and
  the idle-timeout seconds (idle disconnect is enforced *inside* the UI,
  because the once-a-second render keeps the transport busy — mirror
  idlefarmer's comment in `teaHandler`).

### Host key & lifecycle

- Host key at `cfg.HostKeyPath` (default `var/ssh_host_key`), directory
  created `0o700`, auto-generated by Wish on first boot. Never committed. In
  the fleet deploy this path lives on the durability volume and is
  backed up/restored by `entrypoint.sh` (framework/04) — no code-level
  change here, just don't assume the file is always freshly generated.
- `main.go`/`serve()`: config → logger → content → store → **load proxy keys
  → build `identity.Resolver`** → manager → server; register the manager's
  `Shutdown` as a shutdown hook; on SIGINT/SIGTERM run hooks with a 30s
  context, then `srv.Shutdown`. Mirror `../ssh-farm/cmd/ssh-farm/serve.go`'s
  structure (idlefarmer's plain `main.go` is the base shape, but it has no
  proxy-key step to insert).

## Acceptance criteria

- [ ] `ssh -p 2222 -o StrictHostKeyChecking=accept-new localhost` (with
  `MOONMINER_LISTEN_PORT=2222`) reaches the TUI (or a placeholder model if
  tui/01 hasn't landed) with keyboard and mouse events flowing.
- [ ] `ssh -T` (no PTY) prints the hint and exits cleanly.
- [ ] Password auth is impossible; two different keys get two different
  fingerprints; `ssh scout@host` and `ssh host` resolve different slots.
- [ ] A session authenticating with a key listed in `MOONMINER_PROXY_KEYS_PATH`
  and a router-encoded username (`<64-hex-fp>.<slot>`) resolves to *that
  fingerprint's* account, not the proxy key's; a malformed proxied username
  is refused with the protocol-error message and never silently resolves to
  the proxy key's own account. A non-trusted key with the same
  `fp.slot`-shaped username is treated as an ordinary direct connection
  (harmless slot name under the attacker's own key) — confirms the fleet
  security-notes' "no forgery without the private proxy key" property.
- [ ] Exceeding session caps or rate limits rejects without crashing; caps
  are enforced per resolved fingerprint (two proxied sessions for two
  different players don't share one cap bucket).
- [ ] SIGTERM runs shutdown hooks before the listener closes; second Ctrl+C
  not required.
- [ ] Session renders correctly when the **server host is Windows** (colors
  present, no left-smear on partial redraw).
- [ ] `go vet ./...` and `go test ./...` pass; `identity` has table-driven
  tests for fingerprinting, slot sanitization, and proxied-username
  encode/parse/reject (mirror `../ssh-farm/internal/identity`'s test
  coverage).

## Out of scope / handoffs

- The real `game.Manager` (framework/03) — build against a minimal interface
  and a fake.
- The real TUI model (tui/01) — a "hello, pilot" placeholder model is fine.
- Config struct fields you need (framework/04) — add them to
  `internal/config` if it doesn't exist yet; framework/04 will consolidate.
