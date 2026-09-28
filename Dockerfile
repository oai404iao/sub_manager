# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e

FROM --platform=$BUILDPLATFORM node:24-alpine@sha256:d32cdf619f63fe0471182d08996dd516c6275bb5fd31ae06e55a570bd9e1ad43 AS web-builder

WORKDIR /src
RUN npm install --global pnpm@12.4.1
COPY web/package.json web/pnpm-lock.yaml ./web/
RUN --mount=type=cache,target=/root/.local/share/pnpm/store pnpm -C web install --frozen-lockfile
COPY web ./web
RUN pnpm -C web run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS go-builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web-builder /src/internal/webassets/dist ./internal/webassets/dist
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
      -ldflags="-s -w \
        -X github.com/oai404iao/sub_manager/internal/version.Version=$VERSION \
        -X github.com/oai404iao/sub_manager/internal/version.Commit=$COMMIT \
        -X github.com/oai404iao/sub_manager/internal/version.BuildDate=$BUILD_DATE" \
      -o /out/sub-manager ./cmd/server

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="Sub Manager" \
      org.opencontainers.image.description="Xray and Mihomo subscription and node manager" \
      org.opencontainers.image.source="https://github.com/oai404iao/sub_manager" \
      org.opencontainers.image.version=$VERSION \
      org.opencontainers.image.revision=$COMMIT \
      org.opencontainers.image.created=$BUILD_DATE

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 submanager \
    && adduser -S -D -H -u 10001 -G submanager submanager \
    && mkdir -p /data \
    && chown submanager:submanager /data

COPY --from=go-builder /out/sub-manager /usr/local/bin/sub-manager

USER submanager
WORKDIR /data

ENV SUBMAN_ADDR=0.0.0.0:8080 \
    SUBMAN_DB=/data/sub-manager.db

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/sub-manager"]
