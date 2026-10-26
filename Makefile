# TokenFactory — Makefile
#
# Common entry points. Every target is safe to run repeatedly and has no
# side effects outside this repository.

SHELL := /bin/bash
.DEFAULT_GOAL := help

GO ?= go
GOFLAGS ?=
GOPROXY ?= https://goproxy.cn,direct

# Directories
LABS_DIR := labs
THIRD_PARTY_DIR := third_party

# Third-party sources imported via git subtree.
# Format: <dir-name> <upstream-url> <ref>
VENDOR_PROJECTS := \
	golang-jwt-jwt	https://github.com/golang-jwt/jwt.git	v5

export GOPROXY

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ---------------------------------------------------------------------------
# Build & test
# ---------------------------------------------------------------------------

.PHONY: build
build: ## Compile all packages
	$(GO) build ./...

.PHONY: test
test: ## Run all tests
	$(GO) test ./...

.PHONY: test-race
test-race: ## Run all tests with the race detector
	$(GO) test -race ./...

.PHONY: test-verbose
test-verbose: ## Run all tests verbosely
	$(GO) test -v ./...

.PHONY: cover
cover: ## Run tests and report coverage
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -20
	@echo "HTML report: go tool cover -html=coverage.out"

.PHONY: bench
bench: ## Run all benchmarks
	$(GO) test -bench=. -benchmem -run='^$$' ./$(LABS_DIR)/...

.PHONY: bench-save
bench-save: ## Run benchmarks and save results for comparison
	@mkdir -p bench-results
	$(GO) test -bench=. -benchmem -run='^$$' ./$(LABS_DIR)/... | tee bench-results/$$(date +%Y%m%d-%H%M%S).txt
	@echo "Saved to bench-results/"

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: fmt
fmt: ## Format all Go code
	$(GO) fmt ./...

.PHONY: fmt-check
fmt-check: ## Verify formatting without modifying files
	@out=$$($(GO) fmt ./...); \
	if [ -n "$$out" ]; then \
		echo "The following files need formatting:"; echo "$$out"; exit 1; \
	fi

.PHONY: tidy
tidy: ## Tidy go.mod and refresh the vendor directory
	$(GO) mod tidy
	$(GO) mod vendor

# ---------------------------------------------------------------------------
# Third-party (git subtree) management
# ---------------------------------------------------------------------------

.PHONY: vendor-list
vendor-list: ## List subtree-imported third-party projects
	@printf "%-24s %-45s %-8s %s\n" "DIRECTORY" "UPSTREAM" "REF" "INTRODUCED"
	@for entry in $(VENDOR_PROJECTS); do \
		set -- $$entry; \
		dir="$(THIRD_PARTY_DIR)/$$1"; \
		if [ -d "$$dir" ]; then \
			commit=$$(git log --format='%h %ad' --date=short -1 -- "$$dir" 2>/dev/null | head -1); \
			printf "%-24s %-45s %-8s %s\n" "$$1" "$$2" "$$3" "$$commit"; \
		else \
			printf "%-24s %-45s %-8s %s\n" "$$1" "$$2" "$$3" "(not yet imported)"; \
		fi; \
	done

.PHONY: vendor-sync
vendor-sync: ## Sync one subtree project: make vendor-sync PROJECT=<dir> [REF=<ref>]
	@test -n "$(PROJECT)" || { echo "usage: make vendor-sync PROJECT=<dir> [REF=<ref>]"; exit 1; }
	@bash tools/sync-vendor.sh "$(PROJECT)" "$(REF)"

.PHONY: vendor-sync-all
vendor-sync-all: ## Sync every subtree project listed in VENDOR_PROJECTS
	@bash tools/sync-vendor.sh --all

.PHONY: vendor-verify
vendor-verify: ## Verify every subtree project matches its recorded upstream ref
	@bash tools/sync-vendor.sh --verify

# ---------------------------------------------------------------------------
# Docs
# ---------------------------------------------------------------------------

.PHONY: docs-index
docs-index: ## Regenerate the notes index
	@bash tools/gen-notes-index.sh

.PHONY: docs-check
docs-check: ## Check that every note has the required front-matter
	@bash tools/check-notes.sh

# ---------------------------------------------------------------------------
# Housekeeping
# ---------------------------------------------------------------------------

.PHONY: clean
clean: ## Remove build artifacts
	$(GO) clean -cache -testcache
	rm -f coverage.out
	rm -rf bench-results/

.PHONY: tree
tree: ## Show the repository structure (excluding third-party sources)
	@find . -type d \( -name .git -o -name vendor -o -name node_modules \) -prune -o -type f -print \
		| grep -v '^\./\.' | sort | sed 's|^\./||'
