# ===========================================
#  Yao Registry
#
#  Multi-stage build: compile from source then copy into minimal alpine image.
#  Produces a single static binary with zero runtime dependencies.
#
#  Build (single platform):
#    docker build --build-arg VERSION=1.0.0 -t yaoapp/registry:1.0.0 -f docker/Dockerfile .
#
#  Build (multi-platform):
#    docker buildx build --platform linux/amd64,linux/arm64 \
#      --build-arg VERSION=1.0.0 \
#      -t yaoapp/registry:1.0.0 -t yaoapp/registry:latest \
#      -f docker/Dockerfile --push .
#
#  Run:
#    docker run -d -p 8080:8080 -v registry-data:/data yaoapp/registry:1.0.0
#
# ===========================================

# --- Stage 1: Build ---
FROM golang:1.25-alpine AS builder
ARG VERSION=1.0.0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X github.com/yaoapp/registry/handlers.Version=${VERSION}" \
    -o /out/registry .

# --- Stage 2: Runtime ---
FROM alpine:latest
ARG VERSION=1.0.0
LABEL org.opencontainers.image.title="Yao Registry" \
      org.opencontainers.image.description="Lightweight package registry for Yao assets" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/YaoApp/registry"

RUN addgroup -S registry && adduser -S -G registry registry && \
    mkdir -p /data && chown -R registry:registry /data

COPY --from=builder /out/registry /usr/local/bin/registry

USER registry
VOLUME /data
WORKDIR /data
EXPOSE 8080

CMD ["registry", "start", \
     "--host", "0.0.0.0", "--port", "8080", \
     "--db-path", "/data/registry.db", \
     "--data-path", "/data/storage", \
     "--auth-file", "/data/.auth"]
