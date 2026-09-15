GO ?= go
ifneq ($(wildcard .tools/go/bin/go),)
GO := .tools/go/bin/go
export GOROOT := $(abspath .tools/go)
export GOTOOLCHAIN := local
export GOMODCACHE := $(abspath .tools/pkg/mod)
export GOCACHE := $(abspath .tools/gocache)
export GOPATH := $(abspath .tools/gopath)
endif
GOFMT ?= gofmt
BIN ?= easy
PKG ?= ./...
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%d)
LDFLAGS ?= -s -w \
	-X easy-cli/internal/version.Version=$(VERSION) \
	-X easy-cli/internal/version.Commit=$(COMMIT) \
	-X easy-cli/internal/version.Date=$(DATE)

.PHONY: build lint test test-int clean

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/easy

lint:
	@unformatted=$$($(GOFMT) -l cmd internal); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi
	$(GO) vet $(PKG)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed; skipped (gofmt + go vet ran)"; \
	fi

test:
	$(GO) test $(PKG)

test-int:
	$(GO) test -tags=integration -count=1 $(PKG)

clean:
	rm -f $(BIN) $(BIN).exe
	rm -rf dist
