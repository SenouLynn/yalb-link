# Task 01 — Establish the standalone project

**Outcome:** a separately runnable calculator project inside yalb-link with a Go
domain boundary and its own Vite/React frontend. No GCS frontend reuse or dependency.

Read [the handoff](README.md) first. Dependency: none.

## Work

- Follow the [handoff's CODE Lab foundation](README.md). The book's Python
  examples guide later Go implementations; no Python/notebook runtime is required.
  Reserve a documented place for source mappings and reference fixtures without
  implementing the later equation registry here.
- Choose a clear project directory, provisionally `aeronautics/`, holding a nested
  Go module and a separate frontend package. The directory name is a proposal; the
  exclusion entries below must match whatever name is chosen. The module path must
  differ from `yalb.gcs` (e.g. `yalb.aero`) — the root `# gazelle:prefix yalb.gcs`
  would otherwise claim the new packages. Add no `go.work`: root `go build ./...`,
  `go vet`, `go test` and `golangci-lint run` all skip nested modules, and that is
  what isolates the GCS gates. Pin Go 1.25.0 to match `go.mod` and `MODULE.bazel`.
- Establish a transport-free Go calculator package and treat it as a library: Task
  05 adds an HTTP server over it and Task 10 an MCP sidecar, so the layout must
  carry more than one binary. The only binary here prints its version and exits —
  no server, no endpoints, no flags.
- Keep the new tree out of the GCS build gates deliberately and visibly:
  `# gazelle:exclude <dir>` in the root `BUILD.bazel`, and the tree in
  `.bazelignore` and `.dockerignore`. These three shared files are the only GCS
  files this task may touch, one line of reason each. `bazel test //...` does not
  cover this module; record that in the module README with what covers it instead.
- Set strict checks with module-local configuration: a `.golangci.yml` seeded from
  the root one rather than inherited by directory lookup, so a later GCS lint
  change cannot silently move the calculator's gates. Relax `wrapcheck` or
  `funlen` only with a written reason. The frontend tsconfig is independent and
  does not `extends` across the boundary.
- Keep dependency lock files independent: a new `go.mod`/`go.sum` and a new
  `pnpm-lock.yaml`. Do not introduce a root pnpm workspace. Do not couple
  calculator startup to MAVLink, SITL, SQLite or GCS services.
- Add a CI job running the module's build, vet, test and lint, and the frontend
  typecheck, lint, test and build. Record the same commands in `docs/runbooks/`,
  having run them.
- Keep the frontend shell minimal: one route, static content, no fetch, no state
  library, no dev-server proxy, no mock data. Do not invent calculation endpoints
  or put placeholder physics in TypeScript before Tasks 02–05.

## Acceptance checks

- From inside the module directory, with no root Make target involved: Go build,
  vet, test and lint pass, and frontend typecheck, lint, test and build pass.
- A committed Go test asserts the core's transitive imports. `go list -deps` over
  the calculator package resolves to the module itself plus an allowlisted stdlib
  set (`math`, `fmt`, `errors`, `strconv`, `sort`, `testing`). `net/http`, `net`,
  `os`, `database/sql` and `time` fail it — Task 02 requires logging free of
  implicit I/O and timestamps, so the core must not reach the clock.
- The GCS gates still pass: `make build test lint`, `bazel build //...`,
  `bazel test //...`, and `bazel run //:gazelle` leaving
  `git diff --exit-code -- '*BUILD.bazel' 'MODULE.bazel'` clean.
- No diff in `go.mod`, `go.sum`, `internal/**`, `cmd/**`, `proto/**` or
  `frontend/**`. Diffs in `BUILD.bazel`, `.bazelignore` and `.dockerignore` are
  limited to the exclusions above.
- The module README distinguishes implemented shell behavior from future tasks and
  names the checks that cover this module. The root README gains one line.

**Stop after the boundary exists.** Equation models belong to Task 02; no generic
solver, worker pool, plugin system, or MCP server is required here.
