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
    CGO_ENABLED=1 GOARCH=amd64 go build -ldflags="-s -w" -o /ev-charge-api-server . && \
    CGO_ENABLED=1 GOARCH=amd64 go build -ldflags="-s -w" -o /seed ./cmd/seed

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

FROM debian:bookworm-slim AS run

ARG UID=1000
ARG GID=1000

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates wget gosu && \
    rm -rf /var/lib/apt/lists/* && \
    if [ "${UID}" = "0" ]; then \
      APP_USER=root; \
    else \
      groupadd -g "${GID}" appuser && useradd -u "${UID}" -g "${GID}" -m appuser && \
      APP_USER=appuser; \
    fi && \
    echo "${APP_USER}" > /tmp/appuser

COPY --from=build /ev-charge-api-server /ev-charge-api-server
COPY --from=build /seed /seed

RUN APP_USER=$(cat /tmp/appuser) && \
    chown "${APP_USER}:${APP_USER}" /ev-charge-api-server /seed

COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["/ev-charge-api-server"]
