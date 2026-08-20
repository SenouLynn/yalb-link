# Tool overrides: make BUF=/path/to/buf proto-lint

BUF     ?= buf
GO      ?= go
PNPM    ?= pnpm
COMPOSE ?= docker compose

.DEFAULT_GOAL := help

.PHONY: help
help: ## List targets
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  %-22s %s\n", $$1, $$2}'

# --- proto ------------------------------------------------------------------

.PHONY: proto-lint proto-gen proto
proto-lint: ## buf lint
	$(BUF) lint proto

proto-gen: ## Regenerate committed Go and TypeScript stubs
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

test-ts: ## Frontend logic, stream, and component tests
	cd frontend && $(PNPM) vitest run

test: test-go test-ts ## Both suites via their native runners

lint-go: ## golangci-lint
	golangci-lint run

.PHONY: lint-ts lint
lint-ts: ## eslint the frontend
	cd frontend && $(PNPM) lint

lint: lint-go lint-ts ## Both linters

.PHONY: check-contracts check-codegen check-codec check-matrix check-integration build-sitl
check-contracts: ## Protobuf contract lint and repository checks
	$(MAKE) proto-lint
	./scripts/check-contracts.sh

check-codegen: ## Generated stubs exist and compile
	test -d internal/gen/gcs/v1 || { echo "FAIL: internal/gen/gcs/v1 missing — run make proto-gen"; exit 1; }
	test -d frontend/src/gen/gcs/v1 || { echo "FAIL: frontend/src/gen/gcs/v1 missing — run make proto-gen"; exit 1; }
	$(GO) build ./internal/gen/...
	# `pnpm tsc --noEmit` here would be a no-op: the root tsconfig.json is
	# solution-style and lists only references, so tsc has no files to check.
	# The typecheck script uses `tsc -b`, which builds the referenced projects.
	cd frontend && $(PNPM) typecheck
	bazel build //internal/gen/...
	bazel test //frontend:vitest_test //frontend:typecheck_typecheck_test

check-codec: ## Codec and resolvers pass known-answer tests
	$(GO) test -race ./internal/codec/...
	cd frontend && $(PNPM) vitest run src/logic

check-matrix: ## Capability matrix matches the codec and fixtures
	./scripts/check-matrix.sh
	$(GO) test ./internal/codec/... -run TestMatrixCoverage

check-integration: ## Vehicle, routing, streaming, and container checks
	$(GO) test -race ./internal/vehicle/... ./internal/routes/... \
		./internal/bridge/... ./internal/stream/...
	./scripts/check-containers.sh
	$(COMPOSE) config >/dev/null
	$(COMPOSE) build gcs-backend
ifndef SKIP_SITL_BUILD
	$(MAKE) build-sitl
else
	@echo "SKIPPED: SITL image build (SKIP_SITL_BUILD set) — CI still runs it"
endif

build-sitl: ## Build the Copter SITL image
	$(COMPOSE) build ardupilot-sitl-copter-1

# --- bazel ------------------------------------------------------------------
# Bazel is the hermetic checkpoint signal. go.mod stays the source
# of truth for Go dependency versions; gazelle generates BUILD files from it.
#
# After adding a Go dependency or a new package, run `make bazel-tidy`. Order
# matters: go mod tidy first (go.mod is upstream of everything), then gazelle to
# write BUILD files, then bazel mod tidy to sync MODULE.bazel's use_repo list to
# whatever those BUILD files now reference.

.PHONY: bazel-tidy bazel-build bazel-test
bazel-tidy: ## Sync go.mod -> BUILD files -> MODULE.bazel use_repo
	$(GO) mod tidy
	bazel run //:gazelle
	bazel mod tidy

bazel-build: ## Hermetic build signal
	bazel build //...

bazel-test: ## Hermetic test signal — Go and frontend, one command
	bazel test //...
