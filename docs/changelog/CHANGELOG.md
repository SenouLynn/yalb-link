# Changelog

All notable changes follow [Keep a Changelog](https://keepachangelog.com) conventions.
Format: `## [version] - YYYY-MM-DD`. Unreleased changes accumulate at the top.

---

## [Unreleased]

### Added — Tier 4 bridge core (2026-08-17)

**Vehicle fold (`internal/vehicle`):**
- `state.go` — `State` accumulates one vehicle: proto snapshot, per-family freshness
  map, liveness timestamps, EKF flags, source address. Copied by value; the two
  reference fields are cloned by the fold, so a `State` handed in is never modified
- `fold.go` — `Fold(state, Inbound, nowMs) (State, []Event)`. Discovery, loss,
  recovery, heartbeat-change suppression, source-conflict detection, snapshot
  aggregation, telemetry pass-through. No goroutines, no Redis, no sockets
- `event.go` — `Event` carries exactly one of a `FleetEvent`, a `TelemetryEvent` or a
  `Warning`; `occurred_at` is stamped from the injected clock
- `registry.go` — `FleetRegistry` (SADD/SREM on `fleet:active`, implemented in Tier 5)
  plus a `NopFleetRegistry` whose zero value is usable
- 18 tests, `-race`, `goleak`. `TestNoWallClock` parses the package's own AST and
  fails on `time.Now` or `time.Since` — a grep would have failed on the prose
  explaining the rule

**Route table (`internal/routes`, 11 tests):** `Upsert` / `Lookup` / `Resolve` / `Evict` /
`ForgetLink`, `sync.RWMutex`, no background goroutine, lazy eviction on the key being
looked at. `Resolve` returns `ErrNoRoute` rather than a zero value, because the
alternative gomavlib offers is `WriteMessageAll`.

**Containers:** `docker-compose.yml` (SITL ×3 behind a `multi-sitl` profile, Redis
with AOF, backend, Vite dev server, nginx), multi-stage `Dockerfile` for the backend
and `docker/sitl/Dockerfile` for ArduPilot SITL pinned to `ArduCopter-4.6.0`,
`docker/nginx/nginx.conf` with streaming-safe proxy settings.

**Gates:** `make gate-tier-4` (fold + routes under `-race`, `scripts/check-tier-4.sh`,
`docker compose config`, backend image build, Redis actually answering `PING`), plus
`gate-tier-4-sitl` as the slow gate. CI gains a `containers` job and a `sitl-image`
job restricted to pushes and manual dispatch.

**Nine places the plan did not survive contact:**
- **`codec.DecodedMessage` does not exist and should not.** The codec decodes bytes
  and knows nothing about source addresses. `vehicle.Inbound` carries `codec.Decoded`
  plus the transport facts (`SrcAddr`, `MsgID`) and the separately-decoded
  `HeartbeatState`, at the layer that has all three
- **The fold could never declare a silent vehicle lost.** `Fold` only runs when a
  message arrives, so a vehicle that stops transmitting stops being evaluated. Added
  `Expire(state, nowMs)` for a sweeper, emitting `VEHICLE_LOST` at most once per
  outage. `Fold` also emits `VEHICLE_RECOVERED`, which the proto has and the plan's
  event table omitted
- **The two liveness timeouts were an exact tie.** gomavlib's `IdleTimeout` defaults
  to 60s and the plan's fold TTL is 60s, so "vehicle lost" and "channel closed"
  race. `codec.HeartbeatTTL` is now authoritative and `codec.LinkIdleTimeout` is
  derived as 3× it, set explicitly on the node
- **`VehicleState.BaseMode` has no source.** The codec expands `base_mode` into
  discrete bools by design; storing the raw byte would mean re-deriving it and
  keeping a second answer to "is it armed"
- **The route table stores a `codec.LinkID`, not a `*gomavlib.Channel`.** The codec
  already owns the channel and exposes `WriteTo(LinkID, msg)`. The distinction the
  plan cared about survives — the address is the link, never the IP — with one owner
  of the socket. Component ID is part of the key: a gimbal and an autopilot can be
  reachable on different links
- **`WarningEvent` is a Go type, not a proto.** Nothing streams it to a client yet,
  and adding a message commits the wire contract to `buf breaking` forever for a
  shape Tier 5 has not had to use
- **`--out=udp:gcs-backend:14550` is `sim_vehicle.py` syntax, not a flag the SITL
  binary has.** The image runs the autopilot directly, so the equivalent is
  `--serial0=udpclient:...`; waf emits `arducopter` lowercase; `SYSID_THISMAV` is set
  through a parameter overlay, which works on every release
- **The compose table published UDP 14550 on a SITL container.** SITL dials out and
  binds nothing there; the backend is the listener, so that is where the publish
  belongs. nginx moved behind a `tls` profile — it cannot start without certificates,
  and a service that fails on a fresh clone teaches everyone to ignore failures
- **The backend image restates the Go version**, which ADR-0006 says is how versions
  disagree. `scripts/check-tier-4.sh` fails if `Dockerfile`, `go.mod` and
  `MODULE.bazel` do not agree, and if the compose file and the SITL image disagree
  about the ArduPilot tag

**Transport threat model, now executable:** `codec.PostureWarning` names the bind
address and the missing signing key, and `cmd/gcs` prints it at startup. Verified in
the running container. `codec.ResolveBind` keeps unset (default `0.0.0.0:14550`)
distinct from explicitly empty (socket disabled) — `os.Getenv` alone collapses the
two and silently disables the bridge for everyone who never set the variable.

**Not verified locally:** the SITL image has not been built on this machine (10-20
minute cold clone and compile); it is wired as a CI job and remains unproven until
that job runs. Everything else in the gate was executed: the backend image builds,
Redis answers `PING`, and the backend container reports healthy on `/healthz` with
the posture warning in its log.


### Added — Tier 1 codec + Tier 2 parity apparatus (2026-08-17)

**Go codec (`internal/codec`) — Tier 1 chapters 1–3:**
- `frame.go` — `FrameSource` port over gomavlib. Non-deprecated struct config +
  `Initialize()` (SA1019 is on), `HeartbeatPeriod` 1s over gomavlib's own heartbeat
  rather than a hand-rolled ticker, `StreamRequestEnable` left false with the
  hardware-bringup caveat recorded. `WriteTo(LinkID, message.Message)` with a routing
  table built only from inbound frames; an unaddressable target returns
  `ErrUnknownLink` and is never broadcast
- `message.go` — dispatch tables, not a type switch. 13 telemetry families →
  `TelemetryEvent`, 5 transaction families → `ProtocolEvent`. All unit normalisation
  happens here, exactly once
- `encode.go` — the 11 priority send encoders. Pure (`message.Message`, never bytes),
  and none of them chooses a destination
- 44 tests, `-race`, `goleak` in `TestMain`

**Tier 2 evidentiary layer:**
- `scripts/gen_mavlink_fixtures.py` + 33 golden fixtures in `contracts/mavlink/`.
  Committed, deterministic (verified byte-identical across regeneration), generated
  from this repo rather than sourced from an unpinned sibling checkout. Each `.json`
  carries wire-unit `fields` plus an `expected_proto` block, so a fixture is evidence
  for a NORM-* row and not just a decode test
- `docs/wip/codec-capability-matrix.md` — 13 + 5 receive, 11 send, 6 normalisation,
  7 framing rows
- `TestMatrixCoverage` — asserts matrix rows equal
  `telemetryDecoders ∪ protocolDecoders ∪ SendFamilies`. Canaried both directions: a
  removed row with a live decoder fails, and a removed decoder with a live row fails

**Three defects found by execution, not by reading:**
- **`pump` could wedge the node permanently.** Events were forwarded with a bare send
  on an unbuffered channel, so a consumer that stopped reading blocked the goroutine —
  and `Close` waits on it, making shutdown impossible. Found via a 600s hang whose
  goroutine dump showed seven pumps blocked in `chan send`. The send is now guarded by
  a `select` on a `closing` channel and `TestCloseReturnsWithNoEventConsumer` pins it
- **The `bad_crc` fixture was not testing a bad CRC.** It corrupted byte 8, which is
  inside the v2 msgid field (the header is 10 bytes), producing a self-consistent frame
  with an *unknown message ID* that gomavlib surfaces as `MessageRaw`. Now corrupts
  index 10, the first payload byte
- **`FRAME-TRUNCATED` does not emit a parse error**, contrary to the Tier 2 plan.
  gomavlib's parser is stream-oriented: a frame cut mid-payload leaves it waiting for
  the remainder. The observable contract is only "no frame surfaces, no panic"

**Two contract corrections against the generated protos:**
- HEARTBEAT has no `TelemetryEvent` oneof variant — it decodes to `HeartbeatState` and
  drives the fleet fold. Its dispatch-table entry is nil so the ID is still claimed and
  still appears in the matrix
- `hdg_cdeg`, `cog_cdeg` and `vel_cm_s` deliberately keep wire units *and* wire-unit
  names. The 65535 unknown sentinel is defined in those units and a divide turns it
  into 655.35, which nothing downstream can recognise

**Frontend logic (`frontend/src/logic`) — Tier 1 chapters 4–5:**
- `sample.ts` shim + `attitude`, `heading`, `flightPath`, `position`, `trajectory`,
  `track`, `freshness` resolvers; 60 tests over named fixture tables
- Stall gate keys on flight regime, not vehicle type — a hovering VTOL keeps its
  predicted track
- `finite.ts` `isNum` type predicate removes the `as number` casts that
  `Number.isFinite` forces

**Build:**
- `go.mod` gains `gomavlib v3.3.5` and `goleak v1.3.0`
- `contracts/mavlink/BUILD.bazel` filegroup + `data` on the codec test with `# keep`.
  Bazel sandboxes tests, so the fixtures were absent on the first hermetic run while
  plain `go test` passed — a reminder that `bazel test //...` is the signal
- `bazel test //...` green over 3 targets, both languages

**Gate status:** `gate-tier-1`'s Go half previously passed while reporting
`[no test files]` — a gate checking nothing, in the ADR-0006 sense. It now runs 44
tests. `gate-tier-2` is fully wired.

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
