# Development tasks for featherctl.
BINARY  := bin/feather
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

# RELEASE_VERSION is the version `make bump` releases, for example 0.2.0.
RELEASE_VERSION ?=
BUMP_FLAGS ?=

.PHONY: help build install test test-short cover lint fmt vet tidy golden e2e snapshot bump release-notes changelog-check

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## Build the CLI into bin/feather
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/feather

install: ## Install the CLI into GOBIN
	go install -ldflags "$(LDFLAGS)" ./cmd/feather

test: ## Run every test, including the end-to-end suite
	go test ./...

test-short: ## Run the fast tests only
	go test -short ./...

e2e: ## Run only the end-to-end suite
	go test -count=1 ./e2e/...

cover: ## Report coverage for the internal packages
	go test -short -coverprofile=coverage.out ./internal/...
	go tool cover -func=coverage.out | tail -1

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format the source tree
	gofmt -l -w $$(go list -f '{{.Dir}}' ./...)

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy go.mod without pinning a toolchain
	go mod tidy
	go mod edit -toolchain=none

golden: ## Regenerate the golden fixtures after an intentional template change
	go test ./internal/generator -update

snapshot: ## Build release binaries locally without publishing
	goreleaser release --snapshot --clean

bump: ## Release a version: promote [Unreleased], commit and tag (RELEASE_VERSION=0.2.0)
	@test -n "$(RELEASE_VERSION)" || { echo 'usage: make bump RELEASE_VERSION=0.2.0 [BUMP_FLAGS="--dry-run --push"]'; exit 2; }
	go run ./cmd/feather-release bump $(RELEASE_VERSION) $(BUMP_FLAGS)

release-notes: ## Print the CHANGELOG.md notes for a version (RELEASE_VERSION=0.2.0)
	@test -n "$(RELEASE_VERSION)" || { echo 'usage: make release-notes RELEASE_VERSION=0.2.0'; exit 2; }
	@go run ./cmd/feather-release notes $(RELEASE_VERSION)

changelog-check: ## Fail if a tag has no CHANGELOG.md section
	go run ./cmd/feather-release check
