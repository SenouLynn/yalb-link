# Tier 3 — Codegen Infrastructure

## Overview

Wire protobuf codegen so that generated Go stubs and TypeScript clients are available for all subsequent service and frontend tiers. The generated files are committed to source control — they are not regenerated at build time. `buf generate` is a developer command run deliberately, not a build step.

## Dependencies

- Tier 0 complete (`buf lint` passing, `force` field removed)
- buf CLI installed and on PATH
- `buf.build/connectrpc/go` and `buf.build/bufbuild/es` plugins accessible (via BSR remote plugins or local install)

**BSR internet dependency — addressed upfront:** The `buf.gen.yaml` below uses remote BSR plugins. `buf generate` fetches and executes them via the BSR API. This requires internet access and BSR availability at the moment of regeneration only — not at build time (generated files are committed). For air-gapped CI or field machines, install plugins locally and use the `local:` variant:

```yaml
# Local plugin alternative (air-gapped / offline)
plugins:
  - local: protoc-gen-go
    out: ../internal/gen
    opt: paths=source_relative
  - local: protoc-gen-connect-go
    out: ../internal/gen
    opt: paths=source_relative
  - local: protoc-gen-es
    out: ../frontend/src/gen
  - local: protoc-gen-connect-es
    out: ../frontend/src/gen
```

Document local plugin install commands in `docs/dev-setup.md`. The CI freshness check (Chapter 5) uses BSR variant; provide an escape hatch (`PROTO_GEN_LOCAL=1 make proto-gen`) that selects the local variant.

## Chapters

---

### Chapter 1: buf.gen.yaml

**Goal:** Configure codegen output paths for both Go and TypeScript plugins.

**File:** `proto/buf.gen.yaml`

**Content:**
```yaml
version: v2
plugins:
  - plugin: buf.build/protocolbuffers/go
    out: ../internal/gen
    opt: paths=source_relative
  - plugin: buf.build/connectrpc/go
    out: ../internal/gen
    opt: paths=source_relative
  - plugin: buf.build/bufbuild/es
    out: ../frontend/src/gen
  - plugin: buf.build/connectrpc/es
    out: ../frontend/src/gen
```

**Notes:**
- `paths=source_relative` keeps Go package paths clean (no deep nesting)
- Run from `proto/` directory: `cd proto && buf generate`
- Output directories must exist before running; create with `mkdir -p internal/gen frontend/src/gen` in Makefile

---

### Chapter 2: Generated Go Stubs

**Goal:** Confirm generated Go files exist and import correctly.

**Expected output at `internal/gen/gcs/v1/`:**
```
telemetry.pb.go
telemetry_grpc.pb.go        (or telemetry.connect.go for connectrpc)
commands.pb.go
commands.connect.go
services.pb.go
services.connect.go
...
```

**Validation:**
- `go build ./internal/gen/...` must succeed
- No hand-edited files in `internal/gen/` — treat entire directory as owned by buf
- Do NOT add `//go:generate` directives in `internal/gen/` — `go generate` runs from the package directory, making relative paths to `../../proto` fragile and surprising. Use `make proto-gen` exclusively.

**`.gitattributes` — required, not optional:** Add the following to `.gitattributes` before the first generated file commit. Generated file noise in PRs kills review quality — this is not a cosmetic concern.

```
internal/gen/**          linguist-generated=true
frontend/src/gen/**      linguist-generated=true
```

---

### Chapter 3: Generated TypeScript Client

**Goal:** Confirm generated TS files exist and the import path is clean.

**Expected output at `frontend/src/gen/gcs/v1/`:**
```
telemetry_pb.ts
telemetry_connect.ts      (service client — verify naming against actual connectrpc/es plugin version)
commands_pb.ts
commands_connect.ts
...
```

**File naming caveat:** `buf.build/connectrpc/es` plugin output naming (`*_connect.ts` vs `*_connectweb.ts`) depends on the plugin version. Confirm the actual filenames by running `buf generate` once and checking what lands in `frontend/src/gen/` — then update this document. Do not assume filenames match this doc until verified.

**Validation:**
- `cd frontend && pnpm tsc --noEmit` must pass after codegen
- Import path example: `import { TelemetryEvent } from '@/gen/gcs/v1/telemetry_pb'`
- Configure `@/` alias in `vite.config.ts` to resolve to `frontend/src/`
- The `sampleFromEvent` shim in Tier 1 must switch its import from the hand-written stub (if used during parallel Tier 1 work) to the generated type at this point. Verify the generated field names match what the shim expected — any mismatch is a compile error, which is the desired behavior.

---

### Chapter 4: Makefile Targets

**Goal:** Developer ergonomics — one command to regenerate, one to validate.

**File:** `Makefile` (repo root)

```makefile
.PHONY: proto-gen proto-lint proto-breaking

proto-gen:
	mkdir -p internal/gen frontend/src/gen
	cd proto && buf generate

proto-lint:
	cd proto && buf lint

proto-breaking:
	cd proto && buf breaking --against '.git#branch=main'

proto: proto-lint proto-gen
```

**Constraint:** `make proto-gen` is for developer use. CI does NOT run codegen — it validates that committed generated files are up-to-date. See Chapter 5.

---

### Chapter 5: CI Gates

**Goal:** Two CI checks: lint always, breaking change on PRs to main.

**buf lint job** (runs on every push):
```yaml
- name: buf lint
  run: cd proto && buf lint
```

**buf breaking job** (runs on PRs to main):
```yaml
- name: buf breaking
  run: cd proto && buf breaking --against '.git#branch=main'
```

**Generated file freshness check (recommended with caveats):**
```yaml
- name: check proto gen is up-to-date
  run: |
    make proto-gen
    git diff --exit-code internal/gen/ frontend/src/gen/
```

This fails if a developer edits `.proto` files without regenerating. Good discipline. **Caveat:** This step calls `make proto-gen`, which calls `buf generate`, which hits BSR. The CI job needs internet access and BSR must be reachable. If BSR is unavailable, this step fails and blocks the whole PR pipeline — unacceptable for a production gate. Mitigate: run the freshness check as a non-blocking advisory job, or use the local plugin variant in CI (see Dependencies section).

**`buf breaking` CI — shallow clone gotcha:** `buf breaking --against '.git#branch=main'` requires the full git history. Shallow clones (GitHub Actions default `fetch-depth: 1`) will fail with a "branch not found" or empty diff error. The CI checkout step must set `fetch-depth: 0`:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0
```

Without this, the breaking-change gate silently passes on every PR (empty diff = no breaking changes detected), defeating its purpose entirely.

---

## Tier Exit Gate

- `buf generate` produces Go stubs in `internal/gen/gcs/v1/`
- `buf generate` produces TS client in `frontend/src/gen/gcs/v1/`
- `go build ./internal/gen/...` passes
- `cd frontend && pnpm tsc --noEmit` passes
- `buf lint` passes
- `buf breaking` job wired in CI config
- Generated files committed to source control
