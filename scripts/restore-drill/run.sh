#!/bin/sh
# Minimal restore drill for framework/04's acceptance criterion: a fresh
# container with an empty volume and a populated bucket restores and serves.
# This proves the plumbing this repo ships (Dockerfile, entrypoint.sh,
# etc/litestream.yml) actually restores from an S3-compatible bucket.
#
# kill-drill.sh and restore-to-scratch.sh in this directory own the full
# suite (real save data, integrity_check, decode-every-save, measured RPO).
#
# Usage: ./run.sh
# Requires Docker with compose v2. Exits non-zero on any failure.
set -eu
cd "$(dirname "$0")"

cleanup() {
	docker compose down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "== restore drill: fresh boot, empty volume, empty bucket =="
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
sleep 2 # let litestream take its first snapshot

echo "== killing the container without a graceful flush =="
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
# backups found" is the honest signal — its absence means the replica existed
# and was used. (`docker compose rm -f` above removed the previous container,
# so these logs belong only to the fresh one.)
echo "== confirming the restore actually used the replica =="
if docker compose logs moonminer 2>&1 | grep -q "no matching backups found"; then
	echo "FAIL: litestream found no backups — the bucket was empty, so nothing was restored"
	docker compose logs moonminer
	exit 1
fi

echo "== verifying the restored database with integrity_check + decode-every-save =="
docker compose exec -T moonminer sh -c '/app/restore-check -db "$MOONMINER_DB_PATH"' || {
	echo "FAIL: restored database failed integrity/decode verification"
	exit 1
}

echo "PASS: fresh container restored from the bucket and is serving"
echo "NOTE: this is framework/04's minimal integration check. The full drill"
echo "(real save data + integrity_check + decode-every-blob + measured RPO)"
echo "is kill-drill.sh and restore-to-scratch.sh."
