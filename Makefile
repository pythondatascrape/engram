.PHONY: all build start stop restart test test-all install dev-install uninstall reinstall clean lint

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"
CONFIG  ?= engram.yaml
LINK_DIR ?= $(HOME)/.local/bin

all: build start

build:
	go build $(LDFLAGS) -o bin/engram ./cmd/engram
	cp bin/engram engram

start: build
	@pkill -f "engram serve" 2>/dev/null || true
	@./bin/engram serve --config $(CONFIG)

stop:
	@pkill -f "engram serve" 2>/dev/null && echo "engram daemon stopped" || echo "engram daemon was not running"

restart: stop start

test:
	go test ./...

test-all: test test-python test-node test-go-sdk

test-python:
	cd sdk/python && uv run --extra dev pytest -v

test-node:
	cd sdk/node && node --test src/*.test.js

test-go-sdk:
	cd sdk/go && go test ./...

install: build
	@./bin/engram serve --install-daemon --config $(abspath $(CONFIG))

# Install from this checkout for Claude Code: the daemon runs bin/engram and
# `engram` on PATH is a symlink to it, so a rebuild is picked up on reinstall.
dev-install: build
	@mkdir -p $(LINK_DIR)
	ln -sf $(CURDIR)/bin/engram $(LINK_DIR)/engram
	./bin/engram install --claude-code --source $(CURDIR)

uninstall:
	@if [ -x ./bin/engram ]; then ./bin/engram uninstall; else engram uninstall; fi
	@if [ -L $(LINK_DIR)/engram ]; then rm $(LINK_DIR)/engram && echo "Removed $(LINK_DIR)/engram"; fi

reinstall: uninstall dev-install

clean:
	rm -rf bin/ dist/

lint:
	go vet ./...
