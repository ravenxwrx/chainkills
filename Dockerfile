FROM golang:bookworm AS builder

WORKDIR /src
COPY . .
RUN go build -o dist/chainkills

FROM debian:bookworm
COPY --from=builder /src/dist/chainkills /usr/local/bin/chainkills
