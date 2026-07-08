# Framework 02 — Persistence & Save Model

**Area:** Framework · **Phase:** 2 · **Depends on:** nothing (pure storage) ·
**Blocks:** framework/03 · **Parallel-safe with:** everything else

## Goal

One SQLite file holds every pilot. Accounts are keyed by SSH-key fingerprint;
saves are keyed by `(fingerprint, slot)` and store the pilot's entire game
state as a versioned JSON blob. Writes happen on autosave, on disconnect, and
on shutdown flush — a redeploy must never lose progress.

## References (mirror these)

| File | What to take |
| --- | --- |
| `../ssh-idlefarmer/internal/store/store.go` | store type, WAL single-connection setup, `TouchAccount`, `LoadOrCreateSave`, `SaveState` |
| `../ssh-idlefarmer/internal/store/migrations.go` | append-only migration runner + schema shape |
| `../ssh-idlefarmer/internal/store/store_test.go` | test patterns (temp-file DB, create/load/update round-trips) |
| `../ssh-farm/internal/store/` | this schema/mechanism proven a second time on the fleet stack (WAL + single-connection, append-only migrations); its extra tables (leaderboard/moderation columns) don't apply here, but its migration numbering discipline does |

## Deliverables

- `internal/store/` — `store.go`, `migrations.go`, `store_test.go`

## Spec

### Engine & connection discipline

- `modernc.org/sqlite` (cgo-free). WAL mode, busy timeout, and a **single
  connection** (`db.SetMaxOpenConns(1)`) — the actor model (framework/03)
  already serializes writers per save; one connection sidesteps SQLite write
  contention entirely. Mirror idlefarmer's pragmas.
- `store.Open(ctx, path)` creates parent directories, opens, migrates.
  Default path `var/moonminer.db`; in Docker
  `/var/lib/moonminer/moonminer.db` on a named volume.

### Schema (migration 001)

```sql
CREATE TABLE accounts (
    fingerprint TEXT PRIMARY KEY,          -- "SHA256:..."
    public_key  TEXT NOT NULL,             -- authorized_keys format, for auditing
    created_at  INTEGER NOT NULL,          -- unix seconds
    last_seen   INTEGER NOT NULL
);

CREATE TABLE saves (
    fingerprint TEXT NOT NULL REFERENCES accounts(fingerprint),
    slot        TEXT NOT NULL,             -- sanitized username, e.g. "default"
    state       BLOB NOT NULL,             -- JSON-encoded sim.State
    version     INTEGER NOT NULL,          -- sim state schema version
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    PRIMARY KEY (fingerprint, slot)
);
```

Future schema changes are **append-only migrations** in `migrations.go` (a
numbered list run in order inside a transaction, tracked in a
`schema_migrations` table or `user_version` pragma — copy idlefarmer's
mechanism exactly).

### API surface (what framework/03 consumes)

```go
Open(ctx, path) (*Store, error)
(*Store) Close() error
(*Store) TouchAccount(ctx, fingerprint, publicKey string, now int64) error
(*Store) LoadOrCreateSave(ctx, fingerprint, slot string, now int64,
         create func() (state []byte, version int, err error)) (SaveRow, bool, error)
(*Store) SaveState(ctx, fingerprint, slot string, state []byte, version int, now int64) error
```

`LoadOrCreateSave` returns `(row, created, err)` — `created=true` drives the
new-pilot onboarding in the TUI.

### The save blob (contract with gameplay/01)

The blob is `sim.State` encoded as JSON with a `Version int` field. The sim
area owns the struct; the store treats it as opaque bytes. Decode errors on
load must surface as errors (never silently reset a pilot). Versioned decode
lives in `internal/sim` (`sim.DecodeState` handles old versions
forward-compatibly), mirroring idlefarmer.

What the blob will contain (informative, owned by gameplay/01): credits, fuel,
hull, active ship class, ship-installed upgrade levels, cargo manifest, system
and destination permits, station build/ownership/passive-income state,
cosmetic unlocks/selections, settings/tweaks, lifetime stats, recent run log,
RNG seed/state, and the current belt (system/destination + remaining asteroids)
so a reconnecting pilot finds the belt they left. **No active-run state is
persisted** — disconnecting mid-mining/escape resolves an emergency bail/escape
first (framework/03).

### Write points

1. **Autosave** — every `MOONMINER_AUTOSAVE_INTERVAL` (default 30s) while a
   save is attached and dirty.
2. **Disconnect** — on session detach.
3. **Shutdown** — SIGTERM flush of every active save.

All three go through the actor (framework/03); the store just exposes
`SaveState`.

## Acceptance criteria

- [ ] Fresh DB file is created with schema on first open; second open
  migrates nothing and works.
- [ ] `LoadOrCreateSave` create-path invokes the callback once and reports
  `created=true`; load-path returns stored bytes unmodified.
- [ ] Round-trip test: create → save new state → load returns the new state,
  `updated_at` advanced.
- [ ] Two slots under one fingerprint are independent rows.
- [ ] Corrupt blob in the DB → load returns an error (test with garbage
  bytes).
- [ ] All tests use temp-dir DB files; `go test ./internal/store` passes on
  Windows and Linux.

## Out of scope / handoffs

- Actor scheduling, autosave timers, dirty tracking → framework/03.
- The contents/encoding of `sim.State` → gameplay/01 (store sees `[]byte`).
- Config plumbing for `MOONMINER_DB_PATH` → framework/04.
