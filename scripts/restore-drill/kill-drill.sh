#!/bin/sh
# The full durability kill drill, extending run.sh with the parts that make it
# a monitoring tool rather than a one-off test: a real scripted session (so
# there is genuine pilot data to lose), integrity_check + decode-every-save on
# the restored database (/app/restore-check, built into this same image), and a
# measured, enforced RPO. Exits non-zero on any integrity/decode failure or an
# RPO budget overrun, and prints the measured number either way.
#
# Usage: ./kill-drill.sh
# Requires Docker with compose v2 and an ssh client. Exits non-zero on any
# failure.
set -eu
cd "$(dirname "$0")"

# Autosave interval (30s default, see internal/config) plus a generous sync
# and drill-overhead buffer — see ../../../ssh-arcadelobby/docs/06-fleet-data-durability.md's
# RPO claim ("seconds" of litestream lag plus one autosave interval).
MAX_RPO_SECONDS="${MAX_RPO_SECONDS:-60}"

KEYDIR="$(mktemp -d)"
cleanup() {
	docker compose down -v --remove-orphans >/dev/null 2>&1 || true
	rm -rf "$KEYDIR"
}
trap cleanup EXIT

echo "== kill drill: fresh boot =="
docker compose up -d --build minio minio-setup
docker compose up -d moonminer

echo "== waiting for ssh-moonminer to start listening =="
# `docker compose exec` only proves the container's shell is execable — it goes
# green a second or so before the game starts listening (entrypoint.sh's mc
# host-key check and litestream restore run first), which lets the scripted
# connect below race the real listener and fail silently. Wait for the actual
# log line instead.
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
# The remote TUI never exits on its own (it's a full-screen bubbletea program,
# not a shell), so closing stdin doesn't end the session and this needs an
# outer timeout. The manager's Attach creates the save row on connect
# (LoadOrCreateSave, before any keystroke), so a bounded connect-and-drop is
# enough to produce genuine pilot data.
( sleep 2 ) | timeout 10 ssh -tt \
	-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 \
	-i "$KEYDIR/drill_key" -p 2224 drillpilot@127.0.0.1 >/dev/null 2>&1 || true

echo "== waiting for the session's save to land =="
sleep 2 # litestream's ~1s sync lag plus a buffer

echo "== killing the container without a graceful flush =="
kill_ts=$(date +%s)
docker compose kill moonminer

echo "== deleting the local volume (simulating instance/volume loss) =="
docker compose rm -f moonminer >/dev/null
docker volume rm restore-drill_moonminer-drill-data >/dev/null 2>&1 || true

echo "== fresh container, same (populated) bucket =="
docker compose up -d moonminer

i=0
until docker compose logs moonminer 2>&1 | grep -q "ssh server listening"; do
	i=$((i + 1))
	if [ "$i" -gt 30 ]; then
		echo "FAIL: server never reached listening state after restore"
		docker compose logs moonminer
		exit 1
	fi
	sleep 1
done

# Do NOT assert on the entrypoint's own "restoring ... if needed" line: it is
# echoed unconditionally, before litestream runs, so it is true even when the
# bucket is empty and nothing is restored. litestream's own "no matching
# backups found" is the honest signal.
echo "== confirming the restore actually used the replica =="
if docker compose logs moonminer 2>&1 | grep -q "no matching backups found"; then
	echo "FAIL: litestream found no backups — the bucket was empty, so nothing was restored"
	docker compose logs moonminer
	exit 1
fi

echo "== verifying the restored database: integrity_check + decode-every-save =="
check_out="$(docker compose exec -T moonminer sh -c '/app/restore-check -db "$MOONMINER_DB_PATH"')" || {
	echo "FAIL: restore-check reported a problem:"
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

latest_write="$(echo "$check_out" | sed -n 's/^latest_write: \([0-9]*\).*/\1/p')"
if [ -z "$latest_write" ] || [ "$latest_write" -le 0 ]; then
	echo "FAIL: restore-check reported no latest_write timestamp"
	exit 1
fi

rpo=$((kill_ts - latest_write))
echo "== measured RPO: ${rpo}s (kill_ts=${kill_ts}, latest_write=${latest_write}) =="
if [ "$rpo" -gt "$MAX_RPO_SECONDS" ]; then
	echo "FAIL: measured RPO ${rpo}s exceeds budget ${MAX_RPO_SECONDS}s"
	exit 1
fi
if [ "$rpo" -lt 0 ]; then
	echo "FAIL: measured RPO ${rpo}s is negative — clock skew between host and container?"
	exit 1
fi

echo "PASS: kill drill restored ${saves_count} save(s), integrity+decode clean, RPO ${rpo}s <= ${MAX_RPO_SECONDS}s budget"
