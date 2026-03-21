.DEFAULT_GOAL := help

# Colours for output
RED    := \033[0;31m
GREEN  := \033[0;32m
YELLOW := \033[0;33m
CYAN   := \033[0;36m
NC     := \033[0m

# Build variables
VERSION  ?= dev
BINARY   := gh-bench
LDFLAGS  := -s -w -X github.com/seanhalberthal/gh-bench/cmd.Version=$(VERSION)

# Test variables
FILTER   ?=
COVERAGE := coverage.out

# ═══════════════════════════════════════════════════════════════
# Help
# ═══════════════════════════════════════════════════════════════

.PHONY: help
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-18s$(NC) %s\n", $$1, $$2}'

# ═══════════════════════════════════════════════════════════════
# Build
# ═══════════════════════════════════════════════════════════════

.PHONY: build
build: ## Build the gh-bench binary
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

.PHONY: install
install: build ## Build and install as gh extension
	gh extension install .

.PHONY: uninstall
uninstall: ## Remove the gh extension
	gh extension remove bench

.PHONY: clean
clean: ## Remove build artefacts
	rm -f $(BINARY) $(COVERAGE)

# ═══════════════════════════════════════════════════════════════
# Quality
# ═══════════════════════════════════════════════════════════════

.PHONY: test
test: ## Run tests with race detection and coverage
	$(if $(FILTER),\
		go test -race -coverprofile=$(COVERAGE) -run "$(FILTER)" ./...,\
		go test -race -coverprofile=$(COVERAGE) ./...)

.PHONY: cover
cover: test ## Show coverage report in browser
	go tool cover -html=$(COVERAGE)

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Format all Go files
	gofmt -w .

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: check
check: vet lint test ## Run all checks (vet, lint, test)
