# BreachLine Build Targets
#
# Desktop (Wails) build - existing:
#   cd application && wails build -webview2 browser -tags webkit2_42
#
# CLI (TUI) build targets below.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
CLI_BIN := breachline-cli
CLI_PKG := ./cmd/breachline-cli/

.PHONY: build-cli build-cli-all clean test

# Build for the current platform
build-cli:
	go build $(LDFLAGS) -o $(CLI_BIN) $(CLI_PKG)

# Cross-compile for all release targets
build-cli-all: \
	dist/$(CLI_BIN)-linux-amd64 \
	dist/$(CLI_BIN)-linux-arm64 \
	dist/$(CLI_BIN)-darwin-amd64 \
	dist/$(CLI_BIN)-darwin-arm64 \
	dist/$(CLI_BIN)-windows-amd64.exe

dist/$(CLI_BIN)-linux-amd64:
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $@ $(CLI_PKG)

dist/$(CLI_BIN)-linux-arm64:
	@mkdir -p dist
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $@ $(CLI_PKG)

dist/$(CLI_BIN)-darwin-amd64:
	@mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $@ $(CLI_PKG)

dist/$(CLI_BIN)-darwin-arm64:
	@mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $@ $(CLI_PKG)

dist/$(CLI_BIN)-windows-amd64.exe:
	@mkdir -p dist
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $@ $(CLI_PKG)

# Run tests
test:
	go test ./tui/... ./app/... -v

# Clean build artefacts
clean:
	rm -f $(CLI_BIN)
	rm -rf dist/
