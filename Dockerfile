# Stage 1: Build admin panel frontend (embedded in Go binary via go:embed)
FROM node:22-alpine AS adminbuilder

WORKDIR /build/web/admin
COPY web/admin/package.json web/admin/package-lock.json* ./
RUN npm ci
COPY web/admin/ .
# Vite outputs to ../../pkg/admin/dist relative to web/admin/ = /build/pkg/admin/dist
RUN npm run build

# Stage 2: Build Go binary (with embedded admin panel)
FROM golang:latest AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Copy built admin panel into the embed location
COPY --from=adminbuilder /build/pkg/admin/dist /src/pkg/admin/dist

ARG VERSION=0.3.0
RUN CGO_ENABLED=0 go build -ldflags "-X github.com/crystal-mush/gotinymush/pkg/server.Version=${VERSION}" -o /gotinymush ./cmd/server

# Stage 3: Final image
FROM alpine:latest

# su-exec for dropping privileges (lightweight gosu alternative)
RUN apk add --no-cache su-exec tzdata

# Create default mush user (UID/GID adjusted at runtime by entrypoint)
RUN addgroup -g 1000 mush && adduser -D -h /game -u 1000 -G mush mush

WORKDIR /game

COPY --from=builder /gotinymush /usr/local/bin/gotinymush
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Seed files: placed in /game/seed/ so they survive volume mounts on /game/data.
# The setup wizard copies these into the data directory on first boot.
COPY data/text /game/seed/text
COPY data/dict /game/seed/dict
COPY data/game.yaml /game/seed/game.yaml
COPY data/goTinyAlias.conf /game/seed/goTinyAlias.conf
COPY data/minimal.FLAT /game/seed/minimal.FLAT

RUN mkdir -p /game/data /game/certs && chown -R mush:mush /game

EXPOSE 6250 8443

# All paths configurable via environment variables.
# MUSH_BOLT and MUSH_DB are intentionally NOT set here:
# when omitted, the server starts in setup mode (admin panel only).
ENV MUSH_CONF=/game/data/game.yaml
ENV MUSH_TEXTDIR=/game/data/text
ENV MUSH_ALIASCONF=/game/data/goTinyAlias.conf
ENV MUSH_PORT=6250
ENV MUSH_DICTDIR=/game/data/dict
ENV MUSH_SEEDDIR=/game/seed

# Runtime user config (override in docker-compose)
ENV PUID=1000
ENV PGID=1000
ENV TZ=UTC

ENTRYPOINT ["/entrypoint.sh"]
