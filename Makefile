# golangci-lint must be installed and in PATH (CI installs it via
# golangci/golangci-lint-action). Override with: make lint GOLANGCI_LINT=/path/to/bin
GOLANGCI_LINT ?= golangci-lint
# The Tailwind CSS v4 standalone CLI is only needed to rebuild or check
# static/app.css. CI pins its version; use the same one locally, since
# css-check compares the output byte for byte.
# Override with: make css TAILWIND=/path/to/tailwindcss
TAILWIND ?= tailwindcss

APP     := fromscope
CMD     := .

.DEFAULT_GOAL := help

.PHONY: help require-golangci-lint fmt lint test cover vuln tidy-check verify check build require-tailwind css css-check clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "} {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

require-golangci-lint:
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || { \
		echo "error: '$(GOLANGCI_LINT)' not found in PATH." >&2; \
		echo "Install it: https://golangci-lint.run/docs/welcome/install/" >&2; \
		exit 1; \
	}

require-tailwind:
	@command -v $(TAILWIND) >/dev/null 2>&1 || { \
		echo "error: '$(TAILWIND)' not found in PATH." >&2; \
		echo "Install it: https://tailwindcss.com/docs/installation/tailwind-cli" >&2; \
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

css: require-tailwind ## Build static/app.css from input.css and the templates
	$(TAILWIND) -i input.css -o static/app.css --minify

css-check: require-tailwind ## Fail if static/app.css is out of date with the templates
	@tmp=$$(mktemp) || exit 1; trap 'rm -f "$$tmp"' EXIT; \
	$(TAILWIND) -i input.css -o "$$tmp" --minify || exit 1; \
	cmp -s "$$tmp" static/app.css || { \
		echo "error: static/app.css is out of date, run: make css" >&2; \
		exit 1; \
	}

clean: ## Remove build artifacts
	rm -f $(APP) cover.out
