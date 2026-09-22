# Development tasks for featherctl.
BINARY  := bin/feather
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: help build install test test-short cover lint fmt vet tidy golden e2e snapshot

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

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
