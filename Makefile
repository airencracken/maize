GO ?= go
BIN_DIR ?= bin
DOC_DIR ?= build/docs
GOCACHE ?= /tmp/maize-go-build
GOMODCACHE ?= /tmp/maize-go-mod
PREFIX ?= /usr/local
DESTDIR ?=
BASH_COMPLETION_DIR ?= $(PREFIX)/share/bash-completion/completions
MANDIR ?= $(PREFIX)/share/man
INFODIR ?= $(PREFIX)/share/info

export GOCACHE GOMODCACHE

.DEFAULT_GOAL := all

.PHONY: all build check completion-check docs docs-check fmt fmt-check help info install install-docs mod-check test test-race vet

all: check build

build:
	mkdir -p "$(BIN_DIR)"
	$(GO) build -buildvcs=false -o "$(BIN_DIR)/maize" ./cmd/maize

check: fmt-check mod-check completion-check docs-check vet test test-race

completion-check:
	bash -n completions/maize.bash

docs: info

docs-check:
	mandoc -T lint docs/maize.1
	makeinfo --no-split --output=/dev/null docs/maize.texi

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

install-docs: docs
	mkdir -p "$(DESTDIR)$(BASH_COMPLETION_DIR)" "$(DESTDIR)$(MANDIR)/man1" "$(DESTDIR)$(INFODIR)"
	install -m 0644 completions/maize.bash "$(DESTDIR)$(BASH_COMPLETION_DIR)/maize"
	install -m 0644 docs/maize.1 "$(DESTDIR)$(MANDIR)/man1/maize.1"
	install -m 0644 "$(DOC_DIR)/maize.info" "$(DESTDIR)$(INFODIR)/maize.info"

info:
	mkdir -p "$(DOC_DIR)"
	makeinfo --no-split --output="$(DOC_DIR)/maize.info" docs/maize.texi

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
	@echo "  docs        Build the Info manual"
	@echo "  docs-check  Validate the man and Texinfo sources"
	@echo "  fmt         Format Go sources"
	@echo "  fmt-check   Reject unformatted Go sources"
	@echo "  install     Install maize with go install"
	@echo "  install-docs Install completion, man, and Info documentation"
	@echo "  mod-check   Reject go.mod or go.sum changes from go mod tidy"
	@echo "  test        Run the test suite"
	@echo "  test-race   Run the test suite with the race detector"
	@echo "  vet         Run go vet"
