FROM node:22-alpine AS webbuilder

WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

FROM golang:latest AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /gotinymush ./cmd/server

FROM alpine:latest
RUN adduser -D -h /game mush
WORKDIR /game

COPY --from=builder /gotinymush /usr/local/bin/gotinymush
COPY --from=webbuilder /web/dist /game/web/dist
# Copy text files (connect.txt, motd.txt, etc.)
COPY data/text /game/data/text
# Copy game config (YAML)
COPY data/game.yaml /game/data/game.yaml
# Copy alias config
COPY data/goTinyAlias.conf /game/data/goTinyAlias.conf
# Copy seed database for first-time setup
COPY data/minimal.FLAT /game/data/minimal.FLAT

RUN chown -R mush:mush /game
USER mush

EXPOSE 6250 8443

# All paths configurable via environment variables
ENV MUSH_CONF=/game/data/game.yaml
ENV MUSH_BOLT=/game/data/game.bolt
ENV MUSH_DB=/game/data/minimal.FLAT
ENV MUSH_TEXTDIR=/game/data/text
ENV MUSH_ALIASCONF=/game/data/goTinyAlias.conf
ENV MUSH_PORT=6250
ENV MUSH_DICTDIR=/game/data/dict

ENTRYPOINT ["gotinymush"]
