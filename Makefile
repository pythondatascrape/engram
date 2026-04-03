.PHONY: build test test-all install clean lint

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/engram ./cmd/engram

test:
	go test ./...

test-all: test test-python test-node test-go-sdk

test-python:
	cd sdk/python && uv run --extra dev pytest -v

test-node:
	cd sdk/node && node --test src/*.test.js

test-go-sdk:
	cd sdk/go && go test ./...

install:
	go install $(LDFLAGS) ./cmd/engram

clean:
	rm -rf bin/ dist/

lint:
	go vet ./...
