# syntax=docker/dockerfile:1

# ---- Build stage: compile a static, cgo-free binary -------------------------
# The pure-Go SQLite driver (modernc.org/sqlite) and embedded content files
# mean the result is a single self-contained executable.
FROM golang:1.26.4-alpine3.22 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
        -o /out/ssh-moonminer ./cmd/ssh-moonminer

# restore-check ships in the runtime image on purpose: the durability drills
# (scripts/restore-drill) verify a restored database from *inside* a container
# that has the volume mounted, and the quarterly drill runbook needs it on the
# live host without a Go toolchain there. It is read-only and never runs unless
# invoked explicitly.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
        -o /out/restore-check ./cmd/restore-check

# Pre-create the data directory with the runtime user's ownership so the
# named volume inherits writable permissions on first use.
RUN mkdir -p /out/data-dir && chown 65532:65532 /out/data-dir

# ---- Litestream: pinned, pulled as a static binary --------------------------
FROM litestream/litestream:0.5.12 AS litestream

# ---- Runtime stage --------------------------------------------------------
# alpine:3, not distroless: per the canonical fleet durability doc
# (../ssh-arcadelobby/docs/06-fleet-data-durability.md), the entrypoint needs
# a shell to seed the host key and hand off to litestream. The
# rest of the hardening (non-root, read-only rootfs, dropped capabilities)
# is unchanged and enforced in docker-compose.yml.
FROM alpine:3.22

RUN apk add --no-cache ca-certificates && \
    addgroup -g 65532 nonroot && \
    adduser -D -H -u 65532 -G nonroot nonroot

COPY --from=litestream /usr/local/bin/litestream /usr/local/bin/litestream
COPY --from=build /out/ssh-moonminer /app/ssh-moonminer
COPY --from=build /out/restore-check /app/restore-check
COPY --from=build --chown=65532:65532 /out/data-dir /var/lib/moonminer
COPY etc/litestream.yml /etc/litestream.yml
COPY entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh

# The app's own default port is 22; inside the container it listens on an
# unprivileged port instead and the operator maps host 22 to it, so the
# process never needs root or CAP_NET_BIND_SERVICE.
#
# HOME: the nonroot user has no /home/nonroot and cannot create one (/home is
# root-owned). Nothing requires it now that mc is gone, but a read-only rootfs
# with HOME unset is a footgun for any tool added later, so it stays pointed at
# the tmpfs.
ENV MOONMINER_LISTEN_PORT=2222 \
    MOONMINER_HOST_KEY_PATH=/var/lib/moonminer/ssh_host_key \
    MOONMINER_DB_PATH=/var/lib/moonminer/moonminer.db \
    HOME=/tmp

EXPOSE 2222

# Pilot saves and the SSH host key live here — always mount a volume, or
# both are lost (and clients see host-key warnings) on every redeploy. The
# volume is a cache: durability comes from litestream replicating it to S3
# (LITESTREAM_REPLICA_URL), not from the volume itself.
VOLUME ["/var/lib/moonminer"]

USER nonroot:nonroot

ENTRYPOINT ["/entrypoint.sh"]
