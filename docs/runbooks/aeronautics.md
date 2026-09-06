# Aeronautics calculator checks

Local commands for the standalone calculator in `aeronautics/`. Root `make` and
`bazel test //...` deliberately skip this nested module, so these are the checks
that cover it, alongside its [CI workflow](../../.github/workflows/aeronautics.yml).

## Prerequisites

- Go 1.25.x, matching `aeronautics/go.mod`
- golangci-lint v2
- Node.js 22 and pnpm 10

Install frontend dependencies once:

```sh
cd aeronautics/frontend
pnpm install --frozen-lockfile
```

## Go module

From `aeronautics/`, with no root Make target involved:

```sh
go build ./...
go vet ./...
go test -race -timeout=2m ./...
golangci-lint run
```

`golangci-lint run` uses the module-local `.golangci.yml` because it is run from
this directory; the config is a copy, not an inheritance of the root file.

## Running it locally

Two processes, neither of which touches the GCS:

```sh
# from aeronautics/
go run ./cmd/aero serve            # /api/v1 on http://127.0.0.1:8081

# from aeronautics/frontend/, in another shell
pnpm dev                           # http://127.0.0.1:5173, proxying /api
```

`AERO_API` overrides the proxy target. A deployment needs the Go service as well
as the built assets: a static host serves the page but does not execute the
calculation core.

The frontend's contract types are generated from the Go types. After changing
anything in `api/`, regenerate them:

```sh
go run ./cmd/aero contract > frontend/src/api/contract.ts
```

`TestGeneratedTypeScriptMatchesTheCheckedInFile` fails if that is forgotten, so
the check is part of `go test ./...` rather than a separate step.

## Frontend

From `aeronautics/frontend/`:

```sh
pnpm typecheck
pnpm lint
pnpm test
pnpm build
```

## GCS gates are unaffected

The calculator must not change the existing gates. From the repository root:

```sh
make build test lint
bazel build //...
bazel test //...
bazel run //:gazelle && git diff --exit-code -- '*BUILD.bazel' 'MODULE.bazel'
```

`go list ./...` from the root resolves no `yalb.aero` package; the nested module
is invisible to root `go build`, `go vet` and `go test` because there is no
`go.work`.

## Verification record

Run on 2026-09-05 in this checkout. Go build, vet, race tests and lint pass;
frontend typecheck, lint, test and production build pass. Results and any
remaining gaps are recorded in the
[implementation log](../reference/aeronautics-implementation.md).

Version drift to watch: CI pins the golangci-lint action to v2.12.2 while this
checkout ran v2.2.2. Both accept the current sources, but a lint result that
differs between CI and a workstation should be checked against that gap first.
