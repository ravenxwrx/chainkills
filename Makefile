.PHONY: build

VERSION = $(shell git describe --tags --abbrev=0)
HASH = $(shell git rev-parse --short HEAD)
DATE = $(shell date +%Y-%m-%dT%H:%M:%S%z)

build:
	go build -ldflags="-X 'git.sr.ht/~barveyhirdman/chainkills/version.tag=$(VERSION)' -X 'git.sr.ht/~barveyhirdman/chainkills/version.hash=$(HASH)' -X 'git.sr.ht/~barveyhirdman/chainkills/version.buildTime=$(DATE)'" -o dist/chainkills ./cmd/bot/...

run: build
	./dist/chainkills

test:
	go test -v -count=1 -cover ./...

clean:
	rm -rf dist
