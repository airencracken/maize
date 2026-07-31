GO ?= go
BIN_DIR ?= bin
GOCACHE ?= /tmp/maize-go-build
GOMODCACHE ?= /tmp/maize-go-mod

export GOCACHE GOMODCACHE

.DEFAULT_GOAL := all

.PHONY: all build check fmt fmt-check help install mod-check test test-race vet

all: check build

build:
	mkdir -p "$(BIN_DIR)"
	$(GO) build -buildvcs=false -o "$(BIN_DIR)/maize" ./cmd/maize

check: fmt-check mod-check vet test test-race

fmt:
	$(GO) fmt ./...

fmt-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		echo "Go files require formatting:" >&2; \
		echo "$$files" >&2; \
		exit 1; \
	fi

install:
	$(GO) install ./cmd/maize

mod-check:
	$(GO) mod tidy -diff

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

help:
	@echo "Maize development targets:"
	@echo "  all         Run every check and build bin/maize (default)"
	@echo "  build       Build bin/maize"
	@echo "  check       Run formatting, module, vet, test, and race checks"
	@echo "  fmt         Format Go sources"
	@echo "  fmt-check   Reject unformatted Go sources"
	@echo "  install     Install maize with go install"
	@echo "  mod-check   Reject go.mod or go.sum changes from go mod tidy"
	@echo "  test        Run the test suite"
	@echo "  test-race   Run the test suite with the race detector"
	@echo "  vet         Run go vet"
