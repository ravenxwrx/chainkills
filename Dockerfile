FROM golang:bookworm AS builder

WORKDIR /src
COPY . .
RUN go build -o dist/chainkills ./cmd/bot/...

FROM debian:bookworm

LABEL org.opencontainers.image.source=https://github.com/ravenxwrx/chainkills
LABEL org.opencontainers.image.description="Chainkills is a Discord bot that tracks EVE Online killmails in a Wanderer map."
LABEL org.opencontainers.image.licenses=MIT

RUN apt-get update && apt-get install -y \
    ca-certificates

COPY --from=builder /src/dist/chainkills /usr/local/bin/chainkills
