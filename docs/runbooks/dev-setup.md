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

**`bazel test //...` exits 4 until Tier 1.** "No test targets were found" is a real
error code, not a pass. CI runs `bazel build //...` until the first test lands.

**rules_go must keep pace with the Go SDK.** rules_go 0.52.0 passes
`GOEXPERIMENT=coverageredesign`, which Go 1.25 removed, so the stdlib build fails with
`unknown GOEXPERIMENT coverageredesign`. If you bump the Go version in `go.mod` and
`MODULE.bazel`, bump `rules_go` with it.
