# golangci-lint must be installed and in PATH (CI installs it via
# golangci/golangci-lint-action). Override with: make lint GOLANGCI_LINT=/path/to/bin
GOLANGCI_LINT ?= golangci-lint

APP     := fromscope
CMD     := .

.DEFAULT_GOAL := help

.PHONY: help require-golangci-lint fmt lint test cover vuln tidy-check verify check build clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "} {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

require-golangci-lint:
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || { \
		echo "error: '$(GOLANGCI_LINT)' not found in PATH." >&2; \
		echo "Install it: https://golangci-lint.run/docs/welcome/install/" >&2; \
		exit 1; \
	}

fmt: require-golangci-lint ## Format code (gofumpt + goimports)
	$(GOLANGCI_LINT) fmt

lint: require-golangci-lint ## Run linters (also fails on unformatted code)
	$(GOLANGCI_LINT) run

test: ## Run tests with race detector and coverage
	go test -race -cover ./...

cover: ## Generate coverage report (cover.out) and show per-function summary
	go test -race -coverprofile=cover.out ./...
	go tool cover -func=cover.out

vuln: ## Check dependencies for known vulnerabilities
	go tool govulncheck ./...

tidy-check: ## Fail if go.mod/go.sum are not tidy
	go mod tidy -diff

verify: ## Verify downloaded modules have not been modified
	go mod verify

check: tidy-check verify lint test vuln ## Run every quality check (used by CI)

build: ## Build the binary
	go build -trimpath -o $(APP) $(CMD)

clean: ## Remove build artifacts
	rm -f $(APP) cover.out
