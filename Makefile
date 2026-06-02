SHELL := /bin/bash

.DEFAULT_GOAL := build

.PHONY: build frontcli front help fmt fmt-check lint test test-scripts ci tools release release-upload release-verify-public

BIN_DIR := $(CURDIR)/bin
BIN := $(BIN_DIR)/frontcli
CMD := ./cmd/frontcli
RELEASE_DIR ?= $(BIN_DIR)/release
RELEASE_PREFIX ?= frontcli

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo "")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/dedene/frontapp-cli/internal/cmd.version=$(VERSION) -X github.com/dedene/frontapp-cli/internal/cmd.commit=$(COMMIT) -X github.com/dedene/frontapp-cli/internal/cmd.date=$(DATE)

TOOLS_DIR := $(CURDIR)/.tools
GOFUMPT := $(TOOLS_DIR)/gofumpt
GOIMPORTS := $(TOOLS_DIR)/goimports
GOLANGCI_LINT := $(TOOLS_DIR)/golangci-lint

# Allow passing CLI args as extra "targets":
#   make frontcli -- --help
#   make frontcli -- conv list
ifneq ($(filter frontcli front,$(MAKECMDGOALS)),)
RUN_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
$(eval $(RUN_ARGS):;@:)
endif

build:
	@mkdir -p $(BIN_DIR)
	@go build -ldflags "$(LDFLAGS)" -o $(BIN) $(CMD)

frontcli: build
	@if [ -n "$(RUN_ARGS)" ]; then \
		$(BIN) $(RUN_ARGS); \
	elif [ -z "$(ARGS)" ]; then \
		$(BIN) --help; \
	else \
		$(BIN) $(ARGS); \
	fi

front: frontcli

help: build
	@$(BIN) --help

tools:
	@mkdir -p $(TOOLS_DIR)
	@GOBIN=$(TOOLS_DIR) go install mvdan.cc/gofumpt@v0.9.2
	@GOBIN=$(TOOLS_DIR) go install golang.org/x/tools/cmd/goimports@v0.41.0
	@GOBIN=$(TOOLS_DIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.8.0

release:
	@RELEASE_DIR="$(RELEASE_DIR)" RELEASE_PREFIX="$(RELEASE_PREFIX)" bash scripts/release.sh build

release-upload:
	@RELEASE_DIR="$(RELEASE_DIR)" RELEASE_PREFIX="$(RELEASE_PREFIX)" bash scripts/release.sh upload

release-verify-public:
	@RELEASE_DIR="$(RELEASE_DIR)" RELEASE_PREFIX="$(RELEASE_PREFIX)" bash scripts/release.sh verify-public

fmt: tools
	@$(GOIMPORTS) -local github.com/dedene/frontapp-cli -w .
	@$(GOFUMPT) -w .

fmt-check: tools
	@$(GOIMPORTS) -local github.com/dedene/frontapp-cli -w .
	@$(GOFUMPT) -w .
	@git diff --exit-code -- '*.go' go.mod go.sum

lint: tools
	@$(GOLANGCI_LINT) run

test:
	@go test ./...
	@$(MAKE) test-scripts

test-scripts:
	@bash scripts/release_test.sh

ci: fmt-check lint test
