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

# Create distribution package
DIST_DIR := dist
DIST_NAME := $(BINARY_NAME)-$(VERSION)

dist: build
	@echo "Creating distribution package..."
	mkdir -p $(DIST_DIR)/$(DIST_NAME)
	cp $(BINARY_NAME) $(DIST_DIR)/$(DIST_NAME)/
	cp scripts/install.sh $(DIST_DIR)/$(DIST_NAME)/
	cp README.md $(DIST_DIR)/$(DIST_NAME)/
	cp LICENSE $(DIST_DIR)/$(DIST_NAME)/ 2>/dev/null || echo "MIT" > $(DIST_DIR)/$(DIST_NAME)/LICENSE
	cd $(DIST_DIR) && tar -czvf $(DIST_NAME)-linux-amd64.tar.gz $(DIST_NAME)
	@echo "Created $(DIST_DIR)/$(DIST_NAME)-linux-amd64.tar.gz"

# Build for multiple architectures
dist-all:
	@echo "Building for multiple architectures..."
	mkdir -p $(DIST_DIR)
	# Linux amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/airprint-bridge
	# Linux arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/airprint-bridge
	# Linux armv7
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-armv7 ./cmd/airprint-bridge
	# macOS arm64 (Apple Silicon)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/airprint-bridge
	# macOS amd64 (Intel)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/airprint-bridge
	# Create Linux packages
	@for arch in amd64 arm64 armv7; do \
		mkdir -p $(DIST_DIR)/$(DIST_NAME)-linux-$$arch; \
		cp $(DIST_DIR)/$(BINARY_NAME)-linux-$$arch $(DIST_DIR)/$(DIST_NAME)-linux-$$arch/$(BINARY_NAME); \
		cp scripts/install.sh $(DIST_DIR)/$(DIST_NAME)-linux-$$arch/; \
		cp README.md $(DIST_DIR)/$(DIST_NAME)-linux-$$arch/; \
		cp LICENSE $(DIST_DIR)/$(DIST_NAME)-linux-$$arch/ 2>/dev/null || echo "MIT" > $(DIST_DIR)/$(DIST_NAME)-linux-$$arch/LICENSE; \
		cd $(DIST_DIR) && tar -czvf $(DIST_NAME)-linux-$$arch.tar.gz $(DIST_NAME)-linux-$$arch && cd ..; \
	done
	# Create macOS packages
	@for arch in arm64 amd64; do \
		mkdir -p $(DIST_DIR)/$(DIST_NAME)-darwin-$$arch; \
		cp $(DIST_DIR)/$(BINARY_NAME)-darwin-$$arch $(DIST_DIR)/$(DIST_NAME)-darwin-$$arch/$(BINARY_NAME); \
		cp scripts/install.sh $(DIST_DIR)/$(DIST_NAME)-darwin-$$arch/; \
		cp README.md $(DIST_DIR)/$(DIST_NAME)-darwin-$$arch/; \
		cp LICENSE $(DIST_DIR)/$(DIST_NAME)-darwin-$$arch/ 2>/dev/null || echo "MIT" > $(DIST_DIR)/$(DIST_NAME)-darwin-$$arch/LICENSE; \
		cd $(DIST_DIR) && tar -czvf $(DIST_NAME)-darwin-$$arch.tar.gz $(DIST_NAME)-darwin-$$arch && cd ..; \
	done
	@echo "Created distribution packages in $(DIST_DIR)/"

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
	@echo "  build        - Build the binary"
	@echo "  build-static - Build a static binary for Linux amd64"
	@echo "  test         - Run tests"
	@echo "  lint         - Run linters"
	@echo "  fmt          - Format code"
	@echo "  clean        - Remove build artifacts"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  install      - Install binary, config, and service (Alpine)"
	@echo "  uninstall    - Remove installed files"
	@echo "  dist         - Create distribution package (amd64)"
	@echo "  dist-all     - Create packages for all architectures"
	@echo "  help         - Show this help"
