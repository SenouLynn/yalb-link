# Dev Setup

Toolchain needed to run the tier gates. Everything installs into `~/go/bin`, which
Go puts on your PATH — no system package manager, no sudo.

## Required

```sh
# Go 1.25+ — check first; everything else assumes it
go version

# buf — proto lint, breaking-change detection, codegen
go install github.com/bufbuild/buf/cmd/buf@latest

# bazelisk — reads .bazelversion and fetches the matching bazel
go install github.com/bazelbuild/bazelisk@latest
ln -sf ~/go/bin/bazelisk ~/go/bin/bazel

# golangci-lint v2 — the config uses the v2 schema and v1 will reject it
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# pnpm — pinned in package.json via packageManager
corepack enable
```

Verify:

```sh
buf --version              # 1.72+
bazel version              # 7.4.1, fetched by bazelisk
golangci-lint --version    # 2.x
```

## Running the gates

```sh
make gate-tier-0    # buf lint + buf breaking + contract checks
make gate-tier-3    # generated stubs compile (Go, TS, and under Bazel)
make gate-tier-1    # codec + resolver tests          (from Tier 1 onward)
make gate-tier-2    # capability matrix               (from Tier 2 onward)
```

Two build paths, deliberately:

```sh
make test           # fast local loop — go test + vitest via their own runners
make bazel-test     # checkpoint signal — same tests, pinned toolchains, both
                    # languages, nothing depending on what you have installed
```

They should never disagree. If they do, trust Bazel.

`make help` lists everything.

## Regenerating protos

```sh
make proto-gen      # buf generate; output is committed
make bazel-tidy     # go mod tidy -> gazelle -> bazel mod tidy
```

Run `make bazel-tidy` after adding a Go dependency or a new package. The order is
not arbitrary: `go.mod` is upstream of everything, gazelle writes BUILD files from
the source tree, and `bazel mod tidy` syncs `MODULE.bazel`'s `use_repo` list to
whatever those BUILD files now reference. Running them out of order produces a
`use_repo` list that does not match the BUILD files, and the error message points
at the lockfile rather than at the cause.

## Air-gapped / offline codegen

`buf generate` fetches remote plugins from the BSR. Build does not — generated files
are committed. For an air-gapped machine, install the plugins locally and switch
`remote:` to `local:` in `proto/buf.gen.yaml`:

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
npm install -g @bufbuild/protoc-gen-es
```

## Gotchas

**`buf breaking` must run from the repo root.** The `.git` input resolves relative to
the working directory, so `cd proto && buf breaking --against '.git#...'` looks for
`proto/.git` and fails. And without `subdir=proto` the imports do not resolve in the
compared ref. `make proto-breaking` has the correct invocation; use it rather than
retyping.

**`bazel build //frontend:typecheck` does not typecheck anything.** `ts_project` runs
tsc in a separate action whose outputs live in the `typecheck` output group; building
the default outputs succeeds with type errors in the tree. The real target is
`//frontend:typecheck_typecheck_test`, which `bazel test //...` picks up.

**Toolchain versions are coupled — bump them as a batch.** rules_go ↔ Go SDK ↔
gazelle, and aspect_rules_js ↔ Bazel version. rules_go 0.52.0 passes
`GOEXPERIMENT=coverageredesign`, removed in Go 1.25, so the stdlib build fails with an
error that names neither rules_go nor the SDK. aspect_rules_js 3.x declares
`bazel_compatibility = [">=7.6.0"]`. Read ADR-0006 before attempting an upgrade, and
check a ruleset's `bazel_compatibility` on the BCR before adding it.

**Do not put build outputs under `node_modules/`.** Bazel materialises that tree from
`pnpm-lock.yaml` and cannot also accept outputs into it — this is why
`tsBuildInfoFile` lives at `frontend/tsconfig.app.tsbuildinfo`.
