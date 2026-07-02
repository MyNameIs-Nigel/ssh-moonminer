# Tests 02 — SSH Server Integration Tests

**Area:** Tests · **Phase:** 4 (start after framework/01; extend after
framework/03) · **Depends on:** framework/01–03 · **Parallel-safe with:**
tests/01, tests/03, gameplay/*

## Goal

Exercise the real SSH stack in-process: a `golang.org/x/crypto/ssh` client
dialing an ephemeral server instance — no Docker, no network, no mocks of
Wish. These tests are what let an agent refactor middleware or the actor
model without fear.

## References

| Source | What to take |
| --- | --- |
| `../ssh-idlefarmer/internal/server/server_test.go` | the whole harness: ephemeral port, test client keys, PTY request, session assertions |
| `../ssh-idlefarmer/internal/server/hostkey_test.go` | host-key persistence checks |
| `../ssh-idlefarmer/internal/game/game_test.go` | manager-level attach/detach patterns |

## Deliverables

- `internal/server/` — `server_test.go`, `harness_test.go` (shared helpers)
- `internal/game/` — lifecycle tests beyond framework/03's unit set (cross-
  layer: SSH in, actor out)

## Spec

### Harness

`startTestServer(t, cfgMutators...)` helper:

- Temp dir for host key + SQLite; port `127.0.0.1:0` (read the bound port
  back); real content (embedded); real store, manager, server; `t.Cleanup`
  runs full shutdown and asserts no goroutine leaks (compare
  `runtime.NumGoroutine` with settle-retry, or use `goleak` if adding a
  test-only dep is acceptable — prefer no new deps; idlefarmer manages
  without).
- `dial(t, key, username, requestPTY bool)` returns a connected
  `*ssh.Session` with an 80×24 xterm PTY when requested, plus stdout capture.
- Generated ed25519 test keys, cached per test run.

### Test matrix

**Auth & identity**
- Any valid public key connects; the same key reconnecting maps to the same
  pilot (assert via store row, not UI scraping).
- Password auth: client configured with password-only fails before session.
- Usernames `Scout`, `sc out!!`, `""` resolve slots `scout`, `scout`,
  `default` (store rows prove it).

**Transport policies**
- No-PTY session receives the hint text and a clean exit.
- Session cap per key: N+1th concurrent session politely refused.
- Global cap: same shape.
- Rate limit: burst+1 rapid dials — later ones rejected; server stays
  healthy for a subsequent slow dial.

**Session lifecycle (needs framework/03)**
- Connect, wait for first autosave-or-detach write, disconnect: store has
  the pilot with starting state.
- Takeover: session A connected, session B same key+slot connects → A's
  stream ends (with kick text), B lives, no race (`-race`).
- Refuse policy (config mutator): B is refused, A lives.
- **Mid-run disconnect auto-bail**: drive session A into a mining run
  (either by scripted key writes to the PTY, or — more robustly — by
  reaching into the manager to start a run on A's save), hard-close the TCP
  connection, then reload the save: run is nil, credits include the bailed
  yield.

**Shutdown**
- With 3 live sessions (one mid-run), call the shutdown hooks with a 30s
  ctx: all sessions receive the maintenance kick, all saves flushed
  (mid-run one bailed), `Shutdown` returns before deadline, subsequent
  dial refused.

**Host key stability**
- Boot server, capture host key; stop; boot again on same dir: identical
  key (no MITM warnings for players across restarts).

### Conventions

- Every test tolerates Windows (paths, no unix-socket assumptions) — the
  dev box is Windows; CI may be Linux.
- Timeouts generous (5s+) with polling helpers, never bare `time.Sleep`
  as the assertion.
- PTY output assertions match substrings (`"MOON MINER"`, kick text), never
  full-screen goldens — that's tests/03's job at the model layer.

## Acceptance criteria

- [ ] `go test ./internal/server ./internal/game -race -count=2` passes on
  Windows and Linux.
- [ ] Suite needs no root, no free well-known ports, no external processes.
- [ ] Killing the test binary mid-suite leaves nothing listening (all
  servers on ephemeral ports with cleanup).
- [ ] Total runtime < 90 s.
- [ ] Each matrix row above exists as a named test an agent can run alone.

## Out of scope

- TUI rendering correctness → tests/03 (here we only assert reachable text
  fragments).
- Docker/compose smoke tests → framework/04's acceptance list (manual).
- Load/soak testing — post-MVP.
