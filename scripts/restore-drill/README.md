# Restore drill (MinIO)

Boots ssh-moonminer against a local MinIO container using the exact
Dockerfile/entrypoint the fleet runs against real S3, kills the container
without a graceful flush, deletes its volume, and confirms a fresh container
restores from the bucket and starts serving.

```sh
./run.sh                   # framework/04's minimal check: empty volume + populated bucket restores and serves
./kill-drill.sh            # docker kill + integrity_check + decode-every-save + measured RPO
./restore-to-scratch.sh    # litestream restore straight from the S3 replica, no game container involved
./no-credentials-check.sh  # dev mode (no LITESTREAM_REPLICA_URL) never touches S3/MinIO at all
```

Requires Docker with Compose v2. `kill-drill.sh` and `restore-to-scratch.sh`
both play a real scripted SSH session to create genuine save data before
killing the container, so they also need `ssh`/`ssh-keygen` on the host.

The drill listens on **port 2224**, not farm's 2222, so both games' drills can
run side by side without colliding.

## What each script proves, and why there are four

They fail in different places on purpose:

- **`kill-drill.sh`** exercises the container's own restore-on-boot path
  (`entrypoint.sh` → `litestream restore` → the game starts serving) and
  measures the real RPO between the kill and the newest surviving write.
- **`restore-to-scratch.sh`** deliberately *bypasses* that path, restoring the
  replica into a scratch file with no game container involved. A bug in
  `entrypoint.sh`, wish, or host-key handling can then never mask — or be
  masked by — a real problem with the replica data itself.
- **`no-credentials-check.sh`** runs the image with `--network none` and no
  replica URL, proving dev and CI never need AWS credentials. The network
  isolation is the point: a stray S3 call would hang rather than quietly
  succeed against an ambient credential.
- **`run.sh`** is the fast smoke test of the same plumbing, for when you just
  want to know the restore works at all.

The verification step in the first two is `/app/restore-check`, built into the
runtime image from `cmd/restore-check`. It runs `PRAGMA integrity_check` and
then **decodes every save blob** through the same `sim.DecodeState` the game
boots with. Decoding is the part that matters: a torn restore can leave a file
that passes `integrity_check` while carrying a truncated JSON blob, and that
pilot's save is gone even though the file looks fine.

## The assertion trap these scripts deliberately avoid

`entrypoint.sh` echoes `entrypoint: restoring <db> from <url> if needed`
**unconditionally**, before litestream is invoked at all. Farm's scripts —
which these were ported from — wait for `grep -q "restoring"` as their proof
that a restore happened. That line is printed even when the bucket is empty
and litestream restores nothing, so the check passes vacuously; combined with a
`[ -s "$DB_PATH" ]` size test, which the game satisfies by creating its own
empty database on boot, `run.sh` could report PASS having restored nothing.

These scripts assert on litestream's own output instead: the **absence** of
`no matching backups found` on the restore boot, plus a full
`restore-check` (integrity + decode-every-save) on the result. Verified by
negative test — wiping the bucket between the kill and the restore boot makes
the check fire.

`ssh-farm/scripts/restore-drill/` still has the weaker assertion. Its
`kill-drill.sh` and `restore-to-scratch.sh` are backstopped by the
decode-every-save count, so only its `run.sh` is actually vacuous, but it is
worth fixing there too.

## What this cannot cover

Production leaves `LITESTREAM_ACCESS_KEY_ID`/`SECRET` **unset** so litestream
picks up the EC2 instance role; this stack sets them explicitly, because MinIO
has no instance-role concept. A broken IAM policy therefore passes every drill
here and still fails in production — which is exactly how the missing
`s3:GetBucketLocation` went unnoticed until a restore was attempted on the host.
The second blind spot, `MC_HOST_s3`, is gone: `mc` and its static key pair were
removed on 2026-09-08, so the instance role is now the only way anything in
production reaches S3.

These drills prove the *mechanism*. Only the live host proves the
*credentials*. See
[`../../../ssh-arcadelobby/docs/06-fleet-data-durability.md`](../../../ssh-arcadelobby/docs/06-fleet-data-durability.md).

## Why this lives here and not in ssh-farm

Doc 06 rule 3 says "the drill script lives in ssh-farm and is reused
fleet-wide", and `framework/04`'s acceptance rows say not to fork a new one.
That is the right instinct and the wrong location: **`ssh-farm` is a private
repo and `ssh-moonminer` is public**, so this repo cannot depend on scripts
that live there — not in CI, and not for anyone who clones only this game.

The scripts are otherwise a faithful port, differing only in service name,
port, env-var prefix, volume name and the restore-check invocation. If the
fleet ever consolidates them, the natural home is `ssh-arcadelobby`, which
already owns the canonical durability doc and is public — at which point this
directory becomes a thin caller rather than a copy.
