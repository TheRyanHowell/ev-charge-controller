# syntax=docker/dockerfile:1

FROM golang:1.26-bookworm AS base

WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY . .

FROM base AS build

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOARCH=amd64 go build -ldflags="-s -w" -o /mock-tasmota-server .

FROM base AS test

FROM base AS lint

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

FROM base AS dev

ARG UID=1000
ARG GID=1000

# Create user or use root when UID=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    apt-get update && apt-get install -y --no-install-recommends gosu && rm -rf /var/lib/apt/lists/* && \
    if [ "${UID}" = "0" ]; then \
      APP_USER=root; \
    else \
      groupadd -g "${GID}" appuser && useradd -u "${UID}" -g "${GID}" -m appuser && \
      APP_USER=appuser; \
    fi && \
    echo "${APP_USER}" > /tmp/appuser && \
    go install github.com/air-verse/air@v1.65.3 && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

# Download modules to image layer (not cache mount) so air can access them at runtime
RUN --mount=type=cache,target=/root/.cache/go-build \
    go mod download && \
    APP_USER=$(cat /tmp/appuser) && \
    chown -R "${APP_USER}:${APP_USER}" /go/pkg/mod /go/bin

COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

ENTRYPOINT ["/docker-entrypoint.sh"]

FROM scratch AS run

COPY --from=build /mock-tasmota-server /mock-tasmota-server
COPY --from=build /etc/ssl/certs/ca-certificates.crt /ca-certificates.crt

EXPOSE 8081

ENTRYPOINT ["/mock-tasmota-server"]

FROM debian:bookworm-slim AS ci

RUN apt-get update && \
    apt-get install -y --no-install-recommends wget && \
    rm -rf /var/lib/apt/lists/*

COPY --from=build /mock-tasmota-server /mock-tasmota-server

EXPOSE 8081

ENTRYPOINT ["/mock-tasmota-server"]
