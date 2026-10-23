FROM golang:1.27-bookworm AS builder
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN go generate

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /mongodb-backup-bot .

FROM ubuntu:24.04 AS runtime

ARG TARGETARCH
ARG MONGODB_TOOLS_VERSION=100.19.1

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       ca-certificates curl libssl3t64 zlib1g libstdc++6 libgssapi-krb5-2 \
    && rm -rf /var/lib/apt/lists/*

RUN set -eu; \
    arch="${TARGETARCH:-$(dpkg --print-architecture)}"; \
    case "$arch" in \
      amd64) mongo_arch=x86_64 ;; \
      arm64) mongo_arch=aarch64 ;; \
      *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSL --retry 3 "https://fastdl.mongodb.org/tools/db/mongodb-database-tools-ubuntu2404-${mongo_arch}-${MONGODB_TOOLS_VERSION}.tgz" -o /tmp/mongodb-tools.tgz; \
    mkdir /tmp/mongodb-tools; \
    tar -xzf /tmp/mongodb-tools.tgz -C /tmp/mongodb-tools --strip-components=1; \
    cp /tmp/mongodb-tools/bin/mongodump /tmp/mongodb-tools/bin/mongorestore /usr/local/bin/; \
    mongodump --version; \
    mongorestore --version; \
    rm -rf /tmp/mongodb-tools.tgz /tmp/mongodb-tools; \
    apt-get purge -y --auto-remove curl; \
    mongodump --version; \
    mongorestore --version

RUN groupadd --gid 10001 bot \
    && useradd --uid 10001 --gid bot --create-home bot \
    && mkdir -p /data/database /data/backups \
    && chown -R bot:bot /data

COPY --from=builder /mongodb-backup-bot /usr/local/bin/mongodb-backup-bot
COPY --from=builder /build/libtdjson.so* /usr/local/lib/
ENV TDLIB_LIBRARY=/usr/local/lib/libtdjson.so.1.8.69

WORKDIR /data
USER bot
STOPSIGNAL SIGTERM

CMD ["mongodb-backup-bot"]
