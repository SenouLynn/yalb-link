# Changelog

All notable changes follow [Keep a Changelog](https://keepachangelog.com) conventions.
Format: `## [version] - YYYY-MM-DD`. Unreleased changes accumulate at the top.

---

## [Unreleased]

### Changed — adversarial review of Tiers 0–3 (2026-08-16)

Findings and full reasoning: `docs/roadmap/tier-0-3-adversarial-review.md`.

**Contracts (Tier 0 decisions taken before codegen, where they are free):**
- `ProtocolEvent` added (`protocol.proto`) — PARAM_VALUE, MISSION_COUNT, MISSION_ITEM_INT,
  MISSION_ACK and COMMAND_ACK are transaction responses and no longer forced into
  `TelemetryEvent`. `MissionCount` message added; it did not exist
- Vehicle identity moved to the envelope only — `vehicle_id` removed from all 15
  telemetry payload messages (fields renumbered from 1; safe pre-codegen)
- `MissionService.UploadMission` converted from client-streaming to unary over
  `UploadMissionRequest` — connect-web cannot call client-streaming RPCs from a browser
- `StreamTelemetryRequest.payload_types` typed as `TelemetryPayloadType` (was
  `repeated string`); `OperatorContext.roles` typed as `OperatorRole`
- `options.proto` adds `required_role` method option; all 37 RPCs annotated. Absent
  option = deny, so a new unannotated RPC is unreachable rather than public
- `go_package` corrected to `yalb.gcs/internal/gen/gcs/v1;gcsv1`; project name settled on
  `yalb-gcs` across go.mod, MODULE.bazel, buf module and BUILD files
- `MavType` replaced with the complete current MAV_TYPE list (0–49). The roadmap's
  hand-copied fixed-wing table was pre-2019 and mapped ROCKET(9) and GROUND_ROVER(10)
  onto KITE and FLAPPING_WING
- `TrackAffiliation` gains an explicit `_UNSPECIFIED = 0`; "unknown affiliation" is a
  classification and must stay distinguishable from an unpopulated field
- Force-arm control stated on `CommandLong`: removing `SetArmedRequest.force` does not
  close force-arm, since `SendCommand` accepts cmd 400 with param2=21196 and a
  `raw_command` passthrough. Enforcement is the Tier 8 allowlist

**Gates that could not fail, now do:**
- `Makefile` with `gate-tier-0` … `gate-tier-3`; every exit gate is a command with an
  exit code rather than prose
- `.github/workflows/ci.yml` — first CI in the repo. `fetch-depth: 0`, and
  `buf breaking proto --against '.git#branch=origin/main,subdir=proto'` run from the
  repo root. Both parts verified: without `subdir=` imports do not resolve, and run
  from `proto/` buf looks for `proto/.git` and fails
- `scripts/check-matrix.sh` replaces the Tier 2 one-liner, whose trailing `|| true`
  made it print ERROR and exit 0
- `scripts/check-tier-0.sh` — five contract checks buf lint cannot express
- `.golangci.yml` migrated to the v2 schema; the previous file used the v1
  `linters-settings` key and was rejected outright, so none of its settings applied

**Bazel (ADR-0001's hermetic signal, now actually running):**
- `bazel build //...` green over 5 targets, including the generated proto library and
  Connect stubs; wired into CI with a BUILD-files-are-current check
- `rules_go` 0.52.0 → **0.62.0**, `gazelle` 0.40.0 → **0.52.2**. The pinned versions
  predated the host toolchain: rules_go 0.52.0 passes `GOEXPERIMENT=coverageredesign`,
  which Go 1.25 removed, so the stdlib build failed outright
- `go_sdk.host()` → `go_sdk.download(version = "1.25.0")`. A host SDK makes the build
  depend on ambient environment state, which is the thing ADR-0001 exists to avoid
- `# gazelle:proto disable_global` — gazelle was emitting `proto_library` +
  `go_proto_library` claiming the same importpath as the committed buf output (two
  generators for one package), with deps resolving to `//gcs/v1:*` labels that do not
  exist. buf is the only protobuf generator; Bazel compiles its output as ordinary Go
- `MODULE.bazel.lock` committed; `make bazel-tidy` runs go mod tidy → gazelle →
  bazel mod tidy in that order
- Generated stubs committed (`internal/gen`, `frontend/src/gen`) — Tier 3 executed
- **Frontend under Bazel** — `aspect_rules_js` 3.4.0 + `aspect_rules_ts` 3.10.0.
  `npm_translate_lock` reads `frontend/pnpm-lock.yaml`, so the lockfile stays the source
  of truth for JS versions the way go.mod is for Go. `ts_project` typechecks src
  including the generated Connect types; `vitest` runs as a Bazel test.
  `bazel test //...` is now one signal across both languages
- Bazel 7.4.1 → **8.7.0** — forced by `aspect_rules_js`'s
  `bazel_compatibility = [">=7.6.0"]`
- `tsBuildInfoFile` moved out of `node_modules/.tmp/` — Bazel materialises that tree
  from the lockfile and cannot also own outputs in it
- Opted out of `aspect_tools_telemetry` in `.bazelrc`; it arrived transitively with
  `aspect_rules_js` and announced it would begin collecting usage data
- Added `frontend/src/codegen.smoke.test.ts` — round-trips a generated `TelemetryEvent`
  through binary. Typechecking proves the generated code compiles; this proves it loads
  and works at runtime, and gives the vitest target something real to run
- **ADR-0006 — Toolchain Version Coupling and Dependency Risk.** Records the five-step
  forced-upgrade chain, the coupling table (rules_go ↔ Go SDK ↔ gazelle,
  aspect_rules_js ↔ Bazel, npm_translate_lock ↔ pnpm lockfile format, protobuf-es major
  ↔ buf plugin set), and the rule that a gate is not trusted until it has been made to
  fail on purpose — two gates here passed while checking nothing

**Build:**
- `proto/buf.gen.yaml` added — v2 `remote:` plugins, no `connectrpc/es` (protobuf-es v2
  emits service descriptors itself), `--template` required
- `buf.yaml` lint exceptions scoped per file rather than module-wide
- `MODULE.bazel` wires gazelle's `go_deps` from `go.mod`; root `BUILD.bazel` adds a
  gazelle target
- vitest, `@bufbuild/protobuf`, `@connectrpc/connect{,-web}` added to the frontend;
  `@/` alias configured in both `vite.config.ts` and `tsconfig.app.json`
- 7.9 MB compiled `gcs` binary untracked and gitignored
- `docs/runbooks/dev-setup.md` — toolchain install, gate commands, and the gotchas

**Docs:**
- Build sequence changed to **0 → 3 → (1 ∥ 2)** — codegen depends on nothing in Tiers 1–2,
  and running it late means hand-maintaining a proto type stub for two tiers
- Tier 0, 1 and 3 rewritten; Tier 2 and 4 substantially revised. Every gomavlib symbol
  verified against v3.3.5: `EndpointCustomConn` does not exist, `NodeConf`/`NewNode` are
  deprecated, sequence is `GetSequenceNumber()`, and there is no `WriteMessage`
- Heartbeat decision corrected — gomavlib already sends one (5s, MAV_TYPE_GCS) unless
  disabled; the planned hand-rolled per-vehicle ticker would have double-emitted
- Stall gate rekeyed on flight regime rather than vehicle type, so a hovering VTOL keeps
  its predicted track
- Unit normalisation fixed to happen exactly once, at the proto boundary; the planned
  `TelemetrySample` used raw wire units against already-normalised protos
- `port-plan.md` scoped to algorithms and file mapping; superseded tooling statements
  marked inline and all five Open Questions closed

### Added
- Go module (`yalb.gcs`) with `cmd/gcs` entrypoint — minimal HTTP server on `:8080` with `/healthz`
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
7. ~~**CI pipeline**~~ — done: `.github/workflows/ci.yml` runs buf lint, the Tier 0
   contract gate, buf breaking on PRs, Go build/vet/test/lint, and frontend
   typecheck/test. `bazel test //...` becomes the canonical signal once BUILD files are
   generated (`make bazel-tidy`)
