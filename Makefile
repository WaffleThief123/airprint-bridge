# AirPrint Bridge Makefile

BINARY_NAME := airprint-bridge
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go build flags
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)"

# Installation paths
PREFIX := /usr
BINDIR := $(PREFIX)/bin
CONFDIR := /etc/airprint-bridge
SERVICEDIR := /etc/init.d

.PHONY: all build clean install uninstall test lint fmt dist dist-all

all: build

build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/airprint-bridge

build-static:
	@echo "Building static $(BINARY_NAME) $(VERSION)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/airprint-bridge

test:
	go test -v ./...

lint:
	go vet ./...
	@which golangci-lint > /dev/null && golangci-lint run || echo "golangci-lint not installed, skipping"

fmt:
	go fmt ./...

clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/
	go clean

# Create distribution packages using GoReleaser
dist:
	@echo "Building snapshot release..."
	goreleaser build --snapshot --clean

dist-all:
	@echo "Building full snapshot release with archives..."
	goreleaser release --snapshot --clean

# Check GoReleaser config
check-release:
	goreleaser check

deps:
	go mod download
	go mod tidy

install: build
	@echo "Installing $(BINARY_NAME)..."
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 $(BINARY_NAME) $(DESTDIR)$(BINDIR)/$(BINARY_NAME)
	install -d $(DESTDIR)$(CONFDIR)
	install -m 644 configs/airprint-bridge.yaml $(DESTDIR)$(CONFDIR)/airprint-bridge.yaml
	install -d $(DESTDIR)$(SERVICEDIR)
	install -m 755 configs/airprint-bridge.openrc $(DESTDIR)$(SERVICEDIR)/airprint-bridge
	@echo ""
	@echo "Installation complete!"
	@echo "  Binary:  $(DESTDIR)$(BINDIR)/$(BINARY_NAME)"
	@echo "  Config:  $(DESTDIR)$(CONFDIR)/airprint-bridge.yaml"
	@echo "  Service: $(DESTDIR)$(SERVICEDIR)/airprint-bridge"
	@echo ""
	@echo "To enable at boot: rc-update add airprint-bridge default"
	@echo "To start now:      rc-service airprint-bridge start"

uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	rc-service airprint-bridge stop 2>/dev/null || true
	rc-update del airprint-bridge default 2>/dev/null || true
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY_NAME)
	rm -f $(DESTDIR)$(SERVICEDIR)/airprint-bridge
	rm -f /etc/avahi/services/airprint-*.service
	@echo "Note: Config directory $(CONFDIR) was not removed"

help:
	@echo "Available targets:"
	@echo "  build         - Build the binary"
	@echo "  build-static  - Build a static binary for Linux amd64"
	@echo "  test          - Run tests"
	@echo "  lint          - Run linters"
	@echo "  fmt           - Format code"
	@echo "  clean         - Remove build artifacts"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  install       - Install binary, config, and service (Alpine)"
	@echo "  uninstall     - Remove installed files"
	@echo "  dist          - Build binaries for all platforms (goreleaser)"
	@echo "  dist-all      - Build full release with archives (goreleaser)"
	@echo "  check-release - Validate goreleaser config"
	@echo "  help          - Show this help"
