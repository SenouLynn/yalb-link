# Changelog

All notable changes follow [Keep a Changelog](https://keepachangelog.com) conventions.
Format: `## [version] - YYYY-MM-DD`. Unreleased changes accumulate at the top.

---

## [Unreleased]

### Added
- Go module (`ligma.gcs`) with `cmd/gcs` entrypoint — minimal HTTP server on `:8080` with `/healthz`
- React frontend scaffold — Vite 6, React 19, TypeScript 5.7, TanStack Query 5 wired at root
- `pnpm` as package manager (pinned `10.30.1`); all frontend deps pinned to exact versions with 1-week stability buffer policy
- TypeScript strict compiler suite: `verbatimModuleSyntax`, `noImplicitReturns`, `noImplicitOverride`, `noUncheckedIndexedAccess`, `noPropertyAccessFromIndexSignature`, `exactOptionalPropertyTypes`, `allowUnreachableCode: false`
- `typescript-eslint` `strictTypeChecked` + `stylisticTypeChecked` — type-aware lint via Node 25 native TS execution
- `@types/node` scoped to tooling tsconfig only; browser bundle has no Node globals
- Go strict lint via `golangci-lint v2` — `errorlint`, `wrapcheck`, `exhaustive`, `govet --enable-all`, `gocritic`, `cyclop ≤15`, `funlen ≤80`
- Bazel `MODULE.bazel` wired with `rules_go 0.52.0`, `gazelle 0.40.0`, `go_sdk.host()`
- Protobuf schema (`proto/gcs/v1`) — fleet, vehicle, telemetry, commands, missions, parameters, track, mesh, video, chat, auth, security, calibration
- ADRs 0001–0005: Bazel build system, Go/React/Connect/Docker stack, frontend adapter pattern, multi-protocol track layer, security and auth model
- QGC/Mission Planner research doc

---

## What's next

Infrastructure still needed before feature work starts:

1. **Docker compose** — `ardupilot-sitl`, `redis`, `gcs-backend`, `gcs-frontend` services; hermetic SITL from day one per ADR-0002
2. **Connect (Buf) codegen** — `rules_buf` in Bazel, `buf.gen.yaml`, Go server stubs and TypeScript client from the proto schema
3. **Go hexagonal skeleton** — typed channel boundaries per the pipeline (`Transport → Frame codec → Message router → Service protocols → Vehicle model → Outbound adapters`); no implementation, just the interfaces and empty adapters
4. **WebSocket SITL adapter** — backend `:8081` endpoint for the test harness (ADR-0003)
5. **Redis adapter stub** — pub/sub and last-known-state scaffolding
6. **Frontend adapter context** — `ConnectAdapter` / `WebSocketAdapter` / `MockAdapter` provider shell; no component logic yet (ADR-0003)
7. **CI pipeline** — `bazel test //...` as the canonical health signal; lint + build gates
