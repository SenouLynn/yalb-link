# Every tier exit gate is a target here. A gate is a command that exits non-zero,
# not a paragraph of prose — that is the whole point. If you cannot express a
# gate as a target, the gate is not yet real.
#
# Tool overrides: make BUF=/path/to/buf proto-lint

BUF   ?= buf
GO    ?= go
PNPM  ?= pnpm

.DEFAULT_GOAL := help

.PHONY: help
help: ## List targets
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  %-22s %s\n", $$1, $$2}'

# --- proto ------------------------------------------------------------------

.PHONY: proto-lint proto-breaking proto-gen proto
proto-lint: ## buf lint
	$(BUF) lint proto

proto-breaking: ## buf breaking against origin/main
	# Run from the repo root with subdir=proto. Both parts matter: the .git input
	# is resolved relative to the working directory (running this from proto/
	# looks for proto/.git and fails), and without subdir= buf reads the module
	# from the ref's root, where the imports do not resolve.
	$(BUF) breaking proto --against '.git#branch=origin/main,subdir=proto'

proto-gen: ## Regenerate Go + TS stubs (developer command; CI does not run this)
	# --template is required: buf looks for buf.gen.yaml in the working directory,
	# not in the input directory, so `buf generate proto` alone fails from the root.
	# The `out:` paths in buf.gen.yaml are relative to this working directory.
	mkdir -p internal/gen frontend/src/gen
	$(BUF) generate proto --template proto/buf.gen.yaml

proto: proto-lint proto-gen ## Lint then regenerate

# --- build and test ---------------------------------------------------------

.PHONY: build test test-go test-ts lint-go
build: ## Build the Go backend
	$(GO) build ./...

test-go: ## Go tests with the race detector
	$(GO) test -race ./...

test-ts: ## Frontend logic tests
	cd frontend && $(PNPM) vitest run

test: test-go test-ts ## Both suites

lint-go: ## golangci-lint
	golangci-lint run

# --- tier exit gates --------------------------------------------------------
# Ordered as built: 0 -> 3 -> 1 and 2 in parallel.

.PHONY: gate-tier-0 gate-tier-3 gate-tier-1 gate-tier-2
gate-tier-0: ## Tier 0: contracts lint clean and non-breaking
	$(MAKE) proto-lint
	$(MAKE) proto-breaking
	./scripts/check-tier-0.sh

gate-tier-3: ## Tier 3: generated stubs exist, compile, and are current
	test -d internal/gen/gcs/v1 || { echo "FAIL: internal/gen/gcs/v1 missing — run make proto-gen"; exit 1; }
	test -d frontend/src/gen/gcs/v1 || { echo "FAIL: frontend/src/gen/gcs/v1 missing — run make proto-gen"; exit 1; }
	$(GO) build ./internal/gen/...
	cd frontend && $(PNPM) tsc --noEmit

gate-tier-1: ## Tier 1: codec and resolvers pass known-answer tests
	$(GO) test -race ./internal/codec/...
	cd frontend && $(PNPM) vitest run src/logic

gate-tier-2: ## Tier 2: capability matrix complete and fixtures present
	./scripts/check-matrix.sh
	$(GO) test ./internal/codec/... -run TestMatrixCoverage

# --- bazel ------------------------------------------------------------------
# ADR-0001 keeps bazel as the hermetic checkpoint signal. go.mod stays the source
# of truth for versions; gazelle generates BUILD files from it.

.PHONY: bazel-tidy bazel-test
bazel-tidy: ## Regenerate BUILD files and MODULE.bazel use_repo entries
	bazel mod tidy
	bazel run //:gazelle

bazel-test: ## Hermetic test signal
	bazel test //...
