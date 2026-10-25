# Kinlight in one image: the Go server serving the built web app beside its
# API, under Litestream, which restores the database from its backup when
# the volume is empty and then replicates every write. See docs/deploy.md.

# --- The web app ---------------------------------------------------------
FROM node:22-alpine AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# --- The server: one static binary, as `make build` makes ----------------
FROM golang:1.27.3-alpine AS server
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -ldflags="-s -w" -o /out/kinlight .
RUN CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

# --- Litestream, pinned and checked --------------------------------------
FROM alpine:3.22 AS litestream
ARG TARGETARCH
ARG LITESTREAM_VERSION=0.5.17
# From the release's checksums.txt, one per architecture.
ARG LITESTREAM_SHA256_amd64=cfb371176d164437ae869f8351cfde49bd1804ae71c61923f75c9cba9c9c006d
ARG LITESTREAM_SHA256_arm64=f8ca4a050095c1efbda2c4365172e61bf9d955ea0d9ac42f448b52e51819baa5
RUN set -eu; \
    case "$TARGETARCH" in \
      amd64) asset=x86_64; sum="$LITESTREAM_SHA256_amd64" ;; \
      arm64) asset=arm64; sum="$LITESTREAM_SHA256_arm64" ;; \
      *) echo "no Litestream checksum for $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    wget -qO /tmp/litestream.tar.gz "https://github.com/benbjohnson/litestream/releases/download/v${LITESTREAM_VERSION}/litestream-${LITESTREAM_VERSION}-linux-${asset}.tar.gz"; \
    echo "$sum  /tmp/litestream.tar.gz" | sha256sum -c -; \
    tar -xzf /tmp/litestream.tar.gz -C /usr/local/bin litestream

# --- What runs ------------------------------------------------------------
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 kinlight \
    && adduser -S -D -H -u 10001 -G kinlight kinlight \
    && mkdir -p /data /kinlight-demo && chown kinlight:kinlight /data /kinlight-demo
COPY --from=litestream /usr/local/bin/litestream /usr/local/bin/litestream
COPY --from=server /out/kinlight /app/kinlight
COPY --from=server /out/seed /app/seed
COPY --from=web /src/frontend/dist /app/web
COPY deploy/litestream.yml /etc/litestream.yml
COPY deploy/start.sh /app/start.sh
ENV DB_PATH=/data/kinlight.db \
    KINLIGHT_WEB_DIR=/app/web \
    KINLIGHT_HOST=0.0.0.0 \
    PORT=8080
USER kinlight
WORKDIR /app
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/api/health >/dev/null || exit 1
ENTRYPOINT ["/app/start.sh"]
