#!/bin/sh
# The lighter-weight companion to kill-drill.sh: proves the S3/litestream
# replica itself is restorable — via `litestream restore -o` straight into a
# scratch file — without ever booting the game container to do it.
#
# kill-drill.sh additionally proves the *container's own* restore-on-boot path
# (entrypoint.sh) works; this script deliberately bypasses that, so a bug in
# the game's boot sequence (entrypoint.sh, wish, host-key handling) can never
# mask — or be masked by — a real problem with the replica data itself.
#
# Usage: ./restore-to-scratch.sh
# Requires Docker with compose v2 and an ssh client. Exits non-zero on any
# failure.
set -eu
cd "$(dirname "$0")"

SCRATCH="$(mktemp -d)"
# mktemp -d defaults to 0700, but the container writes into this bind mount as
# its own nonroot uid (65532), which won't match the host user's uid —
# world-writable is fine, it's a throwaway dir removed on exit.
chmod 777 "$SCRATCH"
KEYDIR="$(mktemp -d)"
cleanup() {
	docker compose down -v --remove-orphans >/dev/null 2>&1 || true
	rm -rf "$SCRATCH" "$KEYDIR"
}
trap cleanup EXIT

echo "== restore-to-scratch: fresh boot =="
docker compose up -d --build minio minio-setup
docker compose up -d moonminer

echo "== waiting for ssh-moonminer to start listening =="
i=0
until docker compose logs moonminer 2>&1 | grep -q "ssh server listening"; do
	i=$((i + 1))
	if [ "$i" -gt 30 ]; then
		echo "FAIL: moonminer container never started listening"
		docker compose logs moonminer
		exit 1
	fi
	sleep 1
done

echo "== playing a real scripted session (creates a genuine save to lose) =="
ssh-keygen -t ed25519 -N "" -f "$KEYDIR/drill_key" -q -C "restore-drill"
( sleep 2 ) | timeout 10 ssh -tt \
	-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 \
	-i "$KEYDIR/drill_key" -p 2224 drillpilot@127.0.0.1 >/dev/null 2>&1 || true

echo "== waiting for the session's save to land =="
sleep 2 # litestream's ~1s sync lag plus a buffer

echo "== stopping moonminer (the S3 replica is now the only copy this script uses) =="
docker compose kill moonminer >/dev/null
docker compose rm -f moonminer >/dev/null

echo "== restoring straight from the S3 replica into a scratch file, then verifying =="
check_out="$(docker compose run --rm --entrypoint sh \
	-v "$SCRATCH:/scratch" \
	-e MOONMINER_DB_PATH=/var/lib/moonminer/moonminer.db \
	-e LITESTREAM_REPLICA_URL=s3://ssharcade-drill/moonminer/db \
	-e LITESTREAM_S3_REGION=us-east-1 \
	-e LITESTREAM_S3_ENDPOINT=http://minio:9000 \
	-e LITESTREAM_ACCESS_KEY_ID=drilluser \
	-e LITESTREAM_SECRET_ACCESS_KEY=drillpassword \
	moonminer -c 'litestream restore -o /scratch/moonminer.db -if-replica-exists "$MOONMINER_DB_PATH" && /app/restore-check -db /scratch/moonminer.db' 2>/dev/null)" || {
	echo "FAIL: scratch restore/verify reported a problem:"
	echo "$check_out"
	exit 1
}
echo "$check_out"

saves_line="$(echo "$check_out" | grep '^saves: ')"
saves_count="$(echo "$saves_line" | sed -n 's/^saves: \([0-9]*\).*/\1/p')"
if [ -z "$saves_count" ] || [ "$saves_count" -lt 1 ]; then
	echo "FAIL: expected at least 1 decoded save, restore-check reported: $saves_line"
	exit 1
fi

echo "PASS: scratch restore verified ${saves_count} save(s) from the S3 replica directly, integrity+decode clean"
