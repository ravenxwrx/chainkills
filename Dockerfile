FROM golang:bookworm AS builder

WORKDIR /src
COPY . .
RUN go build -o dist/chainkills

FROM debian:bookworm

RUN apt-get update && apt-get install -y \
    ca-certificates

COPY --from=builder /src/dist/chainkills /usr/local/bin/chainkills
