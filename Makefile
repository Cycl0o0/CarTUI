# SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
# SPDX-License-Identifier: AGPL-3.0-or-later

BINARY    := cartui
PKG       := github.com/cycl0o0/cartui
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE      := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -s -w \
             -X $(PKG)/internal/version.Version=$(VERSION) \
             -X $(PKG)/internal/version.Commit=$(COMMIT) \
             -X $(PKG)/internal/version.Date=$(DATE)

GO        ?= go
GOFLAGS   ?=

.PHONY: all
all: build

.PHONY: build
build:
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/cartui

.PHONY: install
install:
	$(GO) install $(GOFLAGS) -ldflags '$(LDFLAGS)' ./cmd/cartui

.PHONY: run
run: build
	./$(BINARY)

.PHONY: test
test:
	$(GO) test -race -count=1 -cover ./...

.PHONY: cover
cover:
	$(GO) test -race -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: lint
lint:
	@if command -v golangci-lint >/dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed; running go vet" && $(GO) vet ./...; \
	fi

.PHONY: fmt
fmt:
	gofmt -s -w .
	@if command -v goimports >/dev/null; then goimports -w .; fi

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: clean
clean:
	rm -f $(BINARY) $(BINARY)-* coverage.out coverage.html
	rm -rf dist build

.PHONY: release
release:
	@if command -v goreleaser >/dev/null; then \
		goreleaser release --clean; \
	else \
		echo "goreleaser not installed; install from https://goreleaser.com"; \
		exit 1; \
	fi

.PHONY: snapshot
snapshot:
	@if command -v goreleaser >/dev/null; then \
		goreleaser release --snapshot --clean; \
	else \
		echo "goreleaser not installed"; \
		exit 1; \
	fi

.PHONY: demo
demo: build
	./scripts/demo.sh

.PHONY: help
help:
	@echo "Targets:"
	@echo "  build     compile the cartui binary (default)"
	@echo "  install   go install into GOBIN"
	@echo "  run       build then run"
	@echo "  test      go test -race -cover"
	@echo "  cover     coverage HTML report"
	@echo "  lint      golangci-lint (or go vet fallback)"
	@echo "  fmt       gofmt + goimports"
	@echo "  tidy      go mod tidy"
	@echo "  clean     remove built artefacts"
	@echo "  release   goreleaser release"
	@echo "  snapshot  goreleaser snapshot"
	@echo "  demo      run scripts/demo.sh"
