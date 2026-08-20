# Resolved Decisions

Decisions that are closed. Each row states the decision and the fact that makes it
checkable. Reopening one takes an ADR.

Where a row and an executable artifact disagree, the artifact wins and this file is the
defect. The executable artifacts are `codec-capability-matrix.md` (enforced by
`TestMatrixCoverage`), `scripts/check-*.sh`, `contracts/mavlink/`, and the `.proto` files.

---

## Toolchain

| Decision | Fact |
|---|---|
| Build system | Bazel is the hermetic checkpoint signal (ADR-0001). `make test` is the fast local loop; `bazel test //...` is the one that has to be right. |
| Pinned versions | Bazel 8.7.0 (`.bazelversion`), rules_go 0.62.0, gazelle 0.52.2, aspect_rules_js 3.4.0, aspect_rules_ts 3.10.0, Go SDK 1.25.0. Coupled — bump as one batch. See ADR-0006. |
| rules_go ↔ Go SDK | rules_go must keep pace with the Go SDK. 0.52.0 passes `GOEXPERIMENT=coverageredesign`, removed in Go 1.25; the stdlib build fails outright. |
| Go SDK hermeticity | `go_sdk.download(version = "1.25.0")`, never `go_sdk.host()`. Keep in step with `go.mod`. |
| Dependency source of truth | `go.mod` for Go, `frontend/pnpm-lock.yaml` for JS. Bazel restates neither — gazelle's `go_deps` and `npm_translate_lock` read them. |
| Generated code | Committed to the tree. `make proto-gen` is the developer command; CI validates freshness and never generates. |
| Single protobuf generator | buf. `# gazelle:proto disable_global` in the root `BUILD.bazel` — left on, gazelle emits rules claiming the same `importpath` as the committed buf output, two generators racing for one package. |
| Frontend under Bazel | `aspect_rules_js` + `aspect_rules_ts`. `frontend/BUILD.bazel` is hand-written; `# gazelle:exclude frontend` keeps Go-only gazelle out. |
| Typecheck gate target | `//frontend:typecheck_typecheck_test`, **not** `bazel build //frontend:typecheck`. Typechecking runs in a separate action whose outputs live in the `typecheck` output group, so building default outputs passes with type errors in the tree. Verified by canary; ADR-0006 §4. |
| Ruleset telemetry | Opted out in `.bazelrc` via `common --repo_env=ASPECT_TOOLS_TELEMETRY_OPTOUT=1`, so it is a property of the checkout rather than of whoever runs the build. |
| `buf breaking` invocation | `buf breaking proto --against '.git#branch=origin/main,subdir=proto'`, run **from the repo root**, with `fetch-depth: 0` in CI. All three parts are load-bearing and were verified by execution. Encoded once in `make proto-breaking`; do not retype it. |
| `buf lint` posture | `use: STANDARD` with **per-file** `ignore_only`, never module-wide. Scoping this way is what surfaced `TrackAffiliation`, a GCS-native enum genuinely missing its `_UNSPECIFIED` zero value. |
| Project name | `yalb-gcs`. Go module `yalb.gcs`; Bazel module `yalb_gcs`; `go_package` = `yalb.gcs/internal/gen/gcs/v1;gcsv1`; buf module `buf.build/yalb-gcs/proto`. Generated imports do not resolve if `go_package` and the module path disagree. |

## Contracts

| Decision | Fact |
|---|---|
| Codec output type | Exactly one of `TelemetryEvent` or `ProtocolEvent` per decoded frame, with `(sysId, compId, seq)` from the frame header. Telemetry folds into per-vehicle state; a transaction response correlates against an in-flight request registry. A single conflated type leaves the mission and parameter protocols nowhere to land. |
| Vehicle identity placement | On the envelope only. `TelemetryEvent.vehicle_id` and `ProtocolEvent.vehicle_id` are authoritative; payload messages carry no `vehicle_id`. Enforced by `scripts/check-tier-0.sh`. |
| HEARTBEAT's destination | `HeartbeatState`, never a `TelemetryEvent` variant. The oneof has no heartbeat member and will not gain one. HEARTBEAT carries fleet identity — discovery, loss, recovery, armed state, mode — a different consumer with a different lifetime from a telemetry sample. |
| Authorization in the contract | Every RPC declares `option (gcs.v1.required_role)` (`options.proto`). Middleware reads the role off the method descriptor. **Absent option = deny.** An explicit `OPERATOR_ROLE_UNSPECIFIED` marks the one deliberately public RPC. Enforced by `scripts/check-tier-0.sh`, which compares the RPC count against the annotation count. |
| Browser streaming limits | **No client-streaming or bidirectional RPCs.** `@connectrpc/connect-web` supports unary and server-streaming only. `MissionService.UploadMission` is unary. Enforced by `scripts/check-tier-0.sh`. |
| `SetArmedRequest.force` | Removed from the proto — a field that exists can be set. **The removal is not the control.** Force-arm (cmd 400, param2=21196) stays reachable via `SendCommand`. The control is server-side validation: reject 400/21196, deny any command not on an explicit allowlist including via `raw_command`, check the per-command role first. |

## Codec

| Decision | Fact |
|---|---|
| MAVLink library | gomavlib v3 with `ardupilotmega.Dialect` (superset of common). |
| Unit normalisation | **Exactly once, in the Go codec, at the proto boundary.** Protos carry SI units and degrees. No consumer downstream divides by 1e7, 1000 or 100. **One exception:** fields whose MAVLink *unknown* sentinel is defined in wire units keep wire units and wire-unit names — `hdg_cdeg`, `cog_cdeg`, `vel_cm_s`, `voltage_battery_mv`, `current_battery_ca`, `current_consumed_mah`, `temperature_cdeg`. Dividing turns the `65535`/`INT16_MAX`/`-1` sentinel into a plausible magnitude no consumer can recognise as *unknown*. Evidence: the `NORM-RETAINED` row in `codec-capability-matrix.md`. |
| Vehicle-type classification | Derived from the generated `MavType` constants in `types.proto`, never a retyped integer list. Upstream renumbered the VTOL types; a hand-copied pre-2019 table maps ROCKET(9) and GROUND_ROVER(10) onto KITE and FLAPPING_WING. `types.proto` carries the full MAV_TYPE list (0–49). |
| Priority receive set | 18 families split **three** ways. **Streaming telemetry (12) → `TelemetryEvent`:** SYS_STATUS(1), GPS_RAW_INT(24), ATTITUDE(30), GLOBAL_POSITION_INT(33), MISSION_CURRENT(42), NAV_CONTROLLER_OUTPUT(62), VFR_HUD(74), RADIO_STATUS(109), BATTERY_STATUS(147), HOME_POSITION(242), STATUSTEXT(253), EKF_STATUS_REPORT(193). **Transaction responses (5) → `ProtocolEvent`:** PARAM_VALUE(22), MISSION_COUNT(44), MISSION_ACK(47), MISSION_ITEM_INT(73), COMMAND_ACK(77). **Fleet identity (1) → `HeartbeatState`:** HEARTBEAT(0). |
| Priority send set | 11 families: COMMAND_LONG(76), SET_POSITION_TARGET_GLOBAL_INT(86), PARAM_SET(23), PARAM_REQUEST_LIST(21), PARAM_REQUEST_READ(20), MISSION_COUNT(44), MISSION_ITEM_INT(73), MISSION_REQUEST_INT(51), MISSION_ACK(47), HEARTBEAT(0), MISSION_CLEAR_ALL(45). |
| `type_mask` for position-only | `0xDF8` = 3576 = `0b110111111000`. FORCE_SET bit (9) not set. |
| Stall gate | **Conditional on flight regime, not vehicle type.** Apply the stall floor only when `airspeed_m_s` is present and below stall speed; otherwise use groundspeed with no floor. Keying on MAV_TYPE zeroes the predicted track for a VTOL hovering in multicopter mode. Stall speed is a parameter (`ARSPD_FBW_MIN`), so `resolvePredictiveTrajectory` takes it as an argument. |

## Transport and routing

| Decision | Fact |
|---|---|
| Socket topology | **Single socket** at `0.0.0.0:14550`. Router dispatches by sysId. Multi-vehicle = multiple sysIds over one socket. |
| Bind configuration | Auto-bind at startup. `GCS_MAVLINK_UDP_BIND`, default `0.0.0.0:14550`; **empty string = disabled**, not defaulted. |
| Transport package | **There is no `internal/transport`.** gomavlib owns its socket through an `EndpointConf` and accepts an endpoint, not a byte stream — there is no seam for one. `cmd/gcs` builds `gomavlib.EndpointUDPServer` from `codec.ResolveBind`; **`codec.FrameSource` is the transport port.** `EndpointCustom` is not a workaround: it is a single stream and cannot represent N UDP peers on one bound port. |
| Outbound targeting | **Address every write to a link; never broadcast.** gomavlib has no `WriteMessage(msg)` — a port shaped that way can only be satisfied by `WriteMessageAll`, which sends a command for sysid 2 over vehicle 1's radio. The port is `WriteTo(link LinkID, msg message.Message)`; sysid→channel is populated from inbound frames; an unaddressable target is **rejected, not broadcast**. |
| Router concurrency | **One router loop, not one goroutine per vehicle.** `internal/bridge` holds `map[routes.Key]vehicle.State` and calls `Fold` sequentially. A `sync.Once` guard keyed on system ID would deny a returning vehicle its goroutine forever, and concurrent folds discard the deterministic ordering the pure fold was built for. `states` is single-owner and unguarded; `routes.Table` keeps its mutex for the send path. |
| Fold output destination | **A `bridge.Sink` interface, not a Redis publisher directly.** The pipeline had to be runnable before Redis existed, or every failure is ambiguous between fold and publisher. A sink error stops the bridge rather than being logged and continued — a bridge that folds while nothing records the result presents as healthy while losing fleet history. |
| GCS heartbeat placement | **gomavlib's built-in heartbeat**, configured, not hand-rolled. `NodeConf.HeartbeatDisable` defaults to *false*, so a node already emits HEARTBEAT every 5s. Set `HeartbeatPeriod: time.Second`. A second ticker double-emits. Verified against gomavlib v3.3.5 `node.go`. |
| GCS heartbeat cardinality | **Per link, not per vehicle.** HEARTBEAT is a node-level broadcast on a channel. "1 Hz per discovered vehicle" over a shared socket sends N identical frames/s — on a 57.6 kbps SiK link, ~400 B/s of duplication in the scarce direction, inflating `RADIO_STATUS.txbuf`, the back-pressure signal we rely on. |
| GCS heartbeat fields | type=MAV_TYPE_GCS(6), autopilot=MAV_AUTOPILOT_GENERIC(0), base_mode=0, custom_mode=0, system_status=MAV_STATE_ACTIVE(4). sysId=255, compId=190. |
| Liveness clocks | One derived from the other: `LinkIdleTimeout = 3 * HeartbeatTTL`. Two independent clocks disagree. |
| ACK correlation | In-flight registry keyed by (vehicleSysID, vehicleCompID, commandID) from the frame header and ACK body. 5s timeout per entry. |
| Read/write boundary | A frame is a **write** when it changes what the vehicle is or does — `PARAM_SET`, mission upload, `MISSION_CLEAR_ALL`, mode change, arm/disarm, guided reposition. A frame that only *asks* the vehicle to send data is read-side, including `COMMAND_LONG` cmd 511 and cmd 512. The envelope is not the classification. See ADR-0010. |
| MAVLink signing key | `GCS_MAVLINK_SIGNING_KEY` (hex). Empty = signing disabled (dev/SITL). |

## Persistence

| Decision | Fact |
|---|---|
| Redis Streams vs Pub/Sub | `telemetry:<sysId>` → Pub/Sub (ephemeral, high-rate). `fleet:events`, `audit:events` → Streams (persistent, cold-start recovery). |
| Consumer groups | `XREADGROUP` with group `gcs-backend` from day one on all Streams. `XGROUP CREATE` at startup with `MKSTREAM`. |
| Param key schema | `HSET params:<sysId>` — one hash per vehicle, param_id as field names. Single `EXPIRE 3600s` after a full list completes. |
| EXPIRE throttling | `vehicle:state:<sysId>` EXPIRE at most once per 30s per vehicle, not once per heartbeat. |
| WatchFleet bootstrap | `SMEMBERS fleet:active` → emit synthetic `VEHICLE_DISCOVERED` batch → `XREADGROUP` from the stream. |

## Vehicles and SITL

| Decision | Fact |
|---|---|
| Supported vehicle set | **Copter and Plane.** Everything vehicle-shaped is sized to this list: SITL services, vendored parameter-metadata sets, `scripts/check-tier-4.sh`'s supported-set assertion. Adding a type is three things — a `waf` target, a SITL service, a metadata set. |
| SITL firmware pins | **`Copter-4.7.0` and `Plane-4.6.3`** — deliberately different minor lines. Copter 4.7 is the first line implementing `AVAILABLE_MODES` (#435); Plane 4.6 has no such message and exercises the generated dialect enum fallback. One line for both leaves a layer permanently unexercised. Tags are `Copter-*`/`Plane-*` — the `Ardu` prefix is the *binary* name, not the tag. Pinning `ArduCopter-4.6.0` broke every image build for three months while the gate compared it only against itself. |
| SITL image | Multi-stage Dockerfile, pinned ArduPilot tag. `ardupilot/ardupilot-dev-jammy` is a dev environment, not a SITL runtime. |
| SITL routing | All instances send to `gcs-backend:14550`. Host-published ports are for external tooling only. Discriminated by sysId in the MAVLink header. |
| Stream rates | **ArduPilot delivers nothing until something requests a stream** — on SITL *and* hardware. Verified by a six-minute run against Copter-4.7.0: heartbeats and zero telemetry. Requesting is read-side, not a write. See ADR-0010. |

## Frontend

| Decision | Fact |
|---|---|
| Map tile provider | MapLibre GL + PMTiles, self-hosted. No API key. Field deployment = local mount. |
| Test tooling | Vitest for pure logic. MSW when the adapter needs a mock server. Storybook deferred. |
| TLS placement | Terminated at the reverse proxy in Docker Compose. The backend speaks plain HTTP. |
| Read before write | Parameter read before command write. A proven read path precedes anything that changes vehicle state. |
