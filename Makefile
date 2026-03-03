.DEFAULT_GOAL := build

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

.PHONY: build install test clean

build:
	go build $(LDFLAGS) -o agent-perms ./cmd/agent-perms

install:
	go install $(LDFLAGS) ./cmd/agent-perms

test:
	go test ./...

clean:
	rm -f agent-perms
