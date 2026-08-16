# Tier 3 — Codegen Infrastructure

## Overview

Generate Go stubs and TypeScript types from the protos. Generated files are
committed — `buf generate` is a developer command, never a build step.

**This runs second, immediately after Tier 0**, not after Tiers 1 and 2. Nothing
here depends on them, and Tier 1's TS shim needs the generated `TelemetryEvent`
types. Running codegen late means hand-writing a proto type stub and keeping it in
sync by hand across two tiers before deleting it — pure drift risk for no gain.

Every command and filename below was verified by running it against these protos.

## Dependencies

- Tier 0 complete (`make gate-tier-0` green)
- buf CLI on PATH

**BSR dependency.** The plugins below are remote: `buf generate` fetches and
executes them via the BSR, so regeneration needs internet access. Build does not —
generated files are committed. For air-gapped work, install the plugins locally and
swap `remote:` for `local:`:

```yaml
plugins:
  - local: protoc-gen-go
    out: internal/gen
    opt: paths=source_relative
  - local: protoc-gen-connect-go
    out: internal/gen
    opt: paths=source_relative
  - local: protoc-gen-es
    out: frontend/src/gen
    opt: target=ts
```

---

## Chapters

### Chapter 1: `proto/buf.gen.yaml`

```yaml
version: v2

plugins:
  - remote: buf.build/protocolbuffers/go
    out: internal/gen
    opt: paths=source_relative

  - remote: buf.build/connectrpc/go
    out: internal/gen
    opt: paths=source_relative

  - remote: buf.build/bufbuild/es
    out: frontend/src/gen
    opt: target=ts
```

Three things here differ from a v1-era config and each one is a failure if carried
over:

- **`remote:`, not `plugin:`.** `plugin:` is the v1 key.
- **No `buf.build/connectrpc/es`.** protobuf-es v2 emits message types *and* service
  descriptors in one file, and `@connectrpc/connect` v2 consumes those descriptors
  directly. The separate connect-es plugin belongs to the v1 line. `package.json`
  pins `@bufbuild/protobuf` 2.x and `@connectrpc/connect` 2.x, so this is the
  matching generator.
- **`out:` paths are relative to the working directory**, which is the repo root —
  not `../internal/gen` relative to `proto/`.

**Managed mode is off deliberately.** `go_package` is declared in every `.proto`, so
the import path is visible where the contract lives rather than inferred from
generator config.

---

### Chapter 2: Running it

```
make proto-gen
```

which is:

```
buf generate proto --template proto/buf.gen.yaml
```

**`--template` is required.** buf looks for `buf.gen.yaml` in the working directory,
not in the input directory, so `buf generate proto` alone fails with
`read buf.gen.yaml: file does not exist` while returning output that looks like a
missing-config problem rather than a path problem.

---

### Chapter 3: Generated Go

**Actual output** (verified — 18 files):

```
internal/gen/gcs/v1/*.pb.go                          one per .proto
internal/gen/gcs/v1/gcsv1connect/services.connect.go  service stubs
```

Connect stubs land in a `gcsv1connect/` subpackage, and there is exactly one because
only `services.proto` declares services. Do not expect `telemetry.connect.go` /
`commands.connect.go` — that layout assumes services spread across files.

**Validation:** `go build ./internal/gen/...` passes. Verified that the generated
Connect stub imports `yalb.gcs/internal/gen/gcs/v1` and compiles — which is the
check that `go_package` matches the module path.

**Rules:**
- No hand edits in `internal/gen/`. buf owns the directory; `.golangci.yml` excludes
  it from linting.
- No `//go:generate` directives there — `go generate` runs from the package
  directory, making relative paths to `../../proto` fragile. `make proto-gen` only.
- `.gitattributes` marks `internal/gen/**` and `frontend/src/gen/**` as
  `linguist-generated=true` so regeneration noise does not drown out reviewable
  changes. Add it before the first generated commit.

---

### Chapter 4: Generated TypeScript

**Actual output** (verified — 17 files):

```
frontend/src/gen/gcs/v1/telemetry_pb.ts
frontend/src/gen/gcs/v1/services_pb.ts
frontend/src/gen/gcs/v1/commands_pb.ts
...
```

`*_pb.ts` only — no `*_connect.ts`, for the reason in Chapter 1.

**Validation:** `pnpm typecheck` passes after codegen.

Import path: `import { TelemetryEvent } from '@/gen/gcs/v1/telemetry_pb'`. The `@/`
alias is configured in both `vite.config.ts` (for vite and vitest) and
`tsconfig.app.json` (for tsc) — it must be in both or tests and typecheck disagree.

---

### Chapter 5: Makefile targets

`make proto-lint`, `make proto-breaking`, `make proto-gen`, `make proto`.

CI does **not** run codegen; it validates that committed generated files are current
(Chapter 6).

---

### Chapter 6: CI gates

Wired in `.github/workflows/ci.yml`.

**buf lint** — every push.

**buf breaking** — pull requests to main:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0
- run: buf breaking proto --against '.git#branch=origin/main,subdir=proto'
```

All three details matter and each produces a silently-passing gate when wrong. See
Tier 0 Chapter 10 — the command is stated once there and encoded once in
`make proto-breaking`.

**Generated-file freshness** — advisory, `continue-on-error: true`:

```yaml
- run: |
    make proto-gen
    git diff --exit-code internal/gen/ frontend/src/gen/
```

This catches a `.proto` edit that was not regenerated. It is deliberately
non-blocking because it calls `buf generate`, which fetches remote plugins from the
BSR: a BSR outage would otherwise block every PR, and an external service has no
business gating the pipeline. If the annotation is red, run `make proto-gen` and
commit.

---

## Tier Exit Gate

```
make gate-tier-3
```

- `internal/gen/gcs/v1/` and `frontend/src/gen/gcs/v1/` exist and are committed
- `go build ./internal/gen/...` passes
- `pnpm typecheck` passes
- `buf lint` passes
- `buf breaking` wired in CI with `fetch-depth: 0`
