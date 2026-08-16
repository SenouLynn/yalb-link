# Order of Operations: yalb-gcs Buildout

A canonical build order for yalb-gcs. Defines what to build, in what sequence, why, and what must be true before moving on.

`port-plan.md` in this directory maps what to port and where it lands.
Tier files are flat in `docs/roadmap/` alongside this document.

ADRs are law. If this document contradicts an ADR, the ADR wins.

---

## Resolved Design Decisions

These are closed. Do not re-open without an ADR.

| Decision | Resolution |
|---|---|
| gomavlib vs hand-roll | **gomavlib v3** with ardupilotmega.Dialect (superset of common). Import `github.com/bluenviron/gomavlib/v3`. |
| Map tile provider | MapLibre GL + PMTiles (self-hosted). No API key. Field deployment = local mount. |
| Phase ordering: params vs commands | Params (read) before commands (write). Read-only proof before any writes. |
| Storybook vs Vite MSW | Vitest for pure logic. MSW in Tier 6 when ConnectAdapter needs a mock server. Storybook deferred to Tier 10 or beyond. |
| rules_buf vs buf generate | Commit generated files. `make proto-gen` is the developer command; CI validates freshness, never generates. Wire `rules_buf` into Bazel after Bzlmod support stabilizes. Bazel resolves Go deps from `go.mod` via gazelle's `go_deps` extension — `go.mod` stays the single source of truth for versions. |
| Single socket vs per-vehicle sockets | **Single socket** at `0.0.0.0:14550`. Router dispatches by sysId. Multi-vehicle = multiple sysIds over one socket. All SITL instances target gcs-backend:14550. |
| ConnectLink auto-bind vs operator-initiated | **Auto-bind at startup**. ConnectLink is for runtime supplementary links only. `GCS_MAVLINK_UDP_BIND` env var; default `0.0.0.0:14550`; empty string = disabled. |
| GCS heartbeat placement | **gomavlib's built-in heartbeat**, configured — not hand-rolled. `NodeConf.HeartbeatDisable` defaults to *false*, so a node already emits HEARTBEAT every 5s with type=MAV_TYPE_GCS(6) and autopilot=MAV_AUTOPILOT_GENERIC(0). Set `HeartbeatPeriod: time.Second`. A second hand-rolled ticker would double-emit. Verified against gomavlib v3.3.5 `node.go`. |
| GCS heartbeat cardinality | **Per link, not per vehicle.** HEARTBEAT is a node-level broadcast on a channel; "1 Hz per discovered vehicle" over the single shared socket sends N identical frames/s from (255,190). On a 57.6 kbps SiK link that is ~400 B/s of duplication in the scarce direction and inflates `RADIO_STATUS.txbuf`, the back-pressure signal we rely on. gomavlib's per-channel heartbeat is already correct. |
| GCS heartbeat fields | type=MAV_TYPE_GCS(6), autopilot=MAV_AUTOPILOT_GENERIC(0), base_mode=0, custom_mode=0, system_status=MAV_STATE_ACTIVE(4). sysId=255, compId=190. |
| ACK correlation design | In-flight command registry keyed by (vehicleSysID, vehicleCompID, commandID) from MAVLink frame header + ACK body. 5s timeout per entry. |
| Redis Streams vs Pub/Sub | `telemetry:<sysId>` → Pub/Sub (ephemeral, high-rate). `fleet:events`, `track:events`, `audit:events` → Streams (persistent, cold-start recovery). |
| TLS placement | Terminated at reverse proxy in Docker Compose. Backend speaks plain HTTP. |
| MAVLink signing key storage | Environment variable `GCS_MAVLINK_SIGNING_KEY` (hex). Empty = signing disabled (dev/SITL). |
| Pub/Sub consumer groups | `XREADGROUP` with group `gcs-backend` from day one on all Streams. `XGROUP CREATE` at startup with `MKSTREAM`. |
| Redis param key schema | `HSET params:<sysId>` — one hash per vehicle, param_id as field names. Single `EXPIRE 3600s` after full list completes. |
| EXPIRE throttling | `vehicle:state:<sysId>` EXPIRE at most once per 30s per vehicle, not once per heartbeat. |
| WatchFleet bootstrap | SMEMBERS `fleet:active` SET → emit synthetic VEHICLE_DISCOVERED batch → XREADGROUP from stream. |
| type_mask for position-only | `0xDF8` = 3576 = `0b110111111000`. FORCE_SET bit (9) not set. |
| ArduPilot SITL image | Multi-stage Dockerfile, pinned ArduPilot commit SHA. `ardupilot/ardupilot-dev-jammy` is dev env only, not a SITL runtime. |
| Stall gate | **Conditional on flight regime, not vehicle type.** Apply the stall floor only when `airspeed_m_s` is present and below the stall speed; otherwise use groundspeed with no floor. Keying on MAV_TYPE alone zeroes the predicted track for a VTOL hovering or translating in multicopter mode — the phase where an operator most wants it. Stall speed is a parameter (`ARSPD_FBW_MIN`), so `resolvePredictiveTrajectory` takes it as an argument from day one rather than hardcoding 14 m/s. |
| Vehicle-type classification | Derived from the generated `MavType` constants in `types.proto`, never from a retyped integer list. Upstream renumbered the VTOL types (MAV_TYPE_VTOL_DUOROTOR became MAV_TYPE_VTOL_TAILSITTER_DUOROTOR); a hand-copied pre-2019 table maps ROCKET(9) and GROUND_ROVER(10) onto KITE and FLAPPING_WING. `types.proto` now carries the full MAV_TYPE list (0–49). |
| buf breaking gate | `buf breaking proto --against '.git#branch=origin/main,subdir=proto'`, run **from the repo root**, with `fetch-depth: 0` in CI checkout. All three parts are load-bearing and were verified by execution: the `.git` input resolves relative to the working directory (running from `proto/` looks for `proto/.git` and fails), `subdir=proto` is required or imports do not resolve in the ref, and a shallow clone leaves the ref unresolvable. Encoded once in `make proto-breaking`; do not retype it. |
| Track layer placement | MAVLink track path in Tier 6 alongside first Connect services. ADS-B and Meshtastic adapters in Tier 9. |
| SetArmedRequest.force field | Removed from proto in Tier 0 — a field that exists can be set. **The removal is not the control.** `CommandService.SendCommand` accepts any `MavCmd` plus a `raw_command` passthrough, so force-arm (cmd 400, param2=21196) is reachable one RPC over. The control is server-side validation in SendCommand: reject 400/21196, deny any command not on an explicit allowlist including via `raw_command`, and check the per-command role first. Stated on `CommandLong` in the proto; enforced by the Tier 8 command registry. No write path is exposed before then. |
| SITL multi-instance routing | All instances send to `gcs-backend:14550`. Host-published ports 14560/14570 are for external tooling only. Discriminated by sysId in MAVLink header. |
| Priority receive set | 18 families, split by destination — they are not one envelope. **Streaming telemetry (13) → `TelemetryEvent`:** HEARTBEAT(0), SYS_STATUS(1), GPS_RAW_INT(24), ATTITUDE(30), GLOBAL_POSITION_INT(33), MISSION_CURRENT(42), NAV_CONTROLLER_OUTPUT(62), VFR_HUD(74), RADIO_STATUS(109), BATTERY_STATUS(147), HOME_POSITION(242), STATUSTEXT(253), EKF_STATUS_REPORT(193/ardupilotmega). **Transaction responses (5) → `ProtocolEvent`:** PARAM_VALUE(22), MISSION_COUNT(44), MISSION_ACK(47), MISSION_ITEM_INT(73), COMMAND_ACK(77). HEARTBEAT additionally drives the fleet fold. |
| Codec output type | The codec emits exactly one of `TelemetryEvent` or `ProtocolEvent` per decoded frame, with `(sysId, compId, seq)` from the frame header. Telemetry folds into per-vehicle state and fans out; a transaction response correlates against an in-flight request registry and completes a pending RPC. A single conflated type leaves the mission and parameter protocols nowhere to land — discovered at Tier 7, after three tiers of tests are written against the wrong signature. |
| Outbound targeting | **Address every write to a link; never broadcast.** gomavlib has no `WriteMessage(msg)` — the API is `WriteMessageTo(*Channel, msg)` / `WriteMessageAll` / `WriteMessageExcept`. A port shaped `WriteMessage(msg)` can only be satisfied by `WriteMessageAll`, which transmits to every channel: N× uplink bandwidth, a command for sysid 2 physically sent over vehicle 1's radio, and two vehicles sharing a sysid across links both acting. The transport port is `WriteTo(link LinkID, msg message.Message)`; sysid→channel is populated from inbound frames; an unaddressable target is **rejected, not broadcast**. |
| Unit normalisation | **Exactly once, in the Go codec, at the proto boundary.** Protos carry SI units and degrees (`lat_deg` double, `alt_msl_m` float, `vz_m_s` float) — not raw wire units. No consumer downstream divides by 1e7, 1000 or 100. A `TelemetrySample` shaped in wire units (`latDegE7`, `altMslMm`, `vxCms`) is a flight-path-hud artifact and double-converts against this repo's protos. |
| Vehicle identity placement | On the envelope only. `TelemetryEvent.vehicle_id` and `ProtocolEvent.vehicle_id` are authoritative; telemetry payload messages carry no `vehicle_id`. Duplicating it per payload doubles wire cost at telemetry rates and creates two sources of truth that can disagree. Enforced by `scripts/check-tier-0.sh`. |
| Authorization in the contract | Every RPC declares `option (gcs.v1.required_role)` (see `options.proto`). Middleware reads the role off the method descriptor, so the check cannot drift from a hand-maintained method-name table. **Absent option = deny**: a new RPC without an annotation is unreachable, not public. An explicit `OPERATOR_ROLE_UNSPECIFIED` marks the one deliberately public RPC (`AuthService.ValidateSession`). Enforced by `scripts/check-tier-0.sh`. |
| buf lint posture | `use: STANDARD` with **per-file** `ignore_only` exceptions, never module-wide. MAVLink mirror enums in `types.proto` and the two protocol mirrors in `track.proto` are exempt from `ENUM_VALUE_PREFIX`/`ENUM_ZERO_VALUE_SUFFIX`; `services.proto` is exempt from the three RPC naming rules. Everything else keeps full checking — scoping this way is what surfaced `TrackAffiliation`, a GCS-native enum that was genuinely missing its `_UNSPECIFIED` zero value. |
| Browser streaming limits | **No client-streaming or bidirectional RPCs.** `@connectrpc/connect-web` supports unary and server-streaming only; a client-streaming RPC is uncallable from the frontend. `MissionService.UploadMission` is unary and takes `UploadMissionRequest { target, mission_type, repeated items }`. Enforced by `scripts/check-tier-0.sh`. |
| Project name | `yalb-gcs`. Go module `yalb.gcs`; Bazel module `yalb_gcs`; `go_package` = `yalb.gcs/internal/gen/gcs/v1;gcsv1`; buf module `buf.build/yalb-gcs/proto`. The five-name drift (ligma/yalb) is closed — generated imports do not resolve if `go_package` and the module path disagree. |
| Priority send set | 11 families: COMMAND_LONG(76), SET_POSITION_TARGET_GLOBAL_INT(86), PARAM_SET(23), PARAM_REQUEST_LIST(21), PARAM_REQUEST_READ(20), MISSION_COUNT(44), MISSION_ITEM_INT(73), MISSION_REQUEST_INT(51), MISSION_ACK(47), HEARTBEAT(0), MISSION_CLEAR_ALL(45). |

---

## Principles

1. **Evidence before claims** — nothing is "done" without a test vector, schema, or SITL trace.
2. **Offline before online** — pure folds first, then injected clock, then live sockets.
3. **Read before write** — read-only proven before any outbound capability.
4. **Trust as process** — trust is established through resolved value → known source → fallback chain → freshness → known-answer test.
5. **Contracts are the portable unit** — protos + golden bytes + semantic traces. Generated code is an adapter.
6. **UI composition is deferred** — components are not scheduled until their data is proven at logic + service layer.
7. **ADRs are law; this document is not.**

---

## Build Order

**Build sequence: 0 → 3 → (1 ∥ 2) → 4 → 5 → 6 → 7 → 8 → 9 → 10.**

Tier numbers are stable identifiers, not positions in the queue. Codegen (Tier 3)
runs immediately after contracts (Tier 0) because Tier 1's TS shim needs the
generated `TelemetryEvent` types. Deferring codegen means hand-writing a proto
type stub and keeping it in sync by hand for two tiers, then deleting it — a
drift generator with no upside, since `buf generate` depends on nothing in
Tier 1 or 2. Every tier below states its own gate; follow those rather than the
numbering.

Parallel tracks are marked **[parallel ok]**. Sequential dependencies are marked with their gate condition.

Every exit gate is a `make` target. A gate that cannot be expressed as a command
that exits non-zero is not yet a gate.

Tier files: `docs/roadmap/tier-N-*.md`

---

### Tier 0 — Proto Contracts + Proto Fix  [first]

**Detail:** `tier-0-proto-contracts.md`

Close every contract decision that is cheap now and a breaking change after
codegen: identity placement, the telemetry/transaction split, browser streaming
limits, role annotations, the command allowlist statement, `go_package`, and the
lint posture. Not a formality — this is the only free moment for all of it.

**Exit gate:** `make gate-tier-0` — `buf lint` clean, `buf breaking` clean against
`origin/main`, and `scripts/check-tier-0.sh` passing (no `force` field, no
client-streaming RPCs, every RPC role-annotated, identity declared once,
`go_package` matching the module path).

---

### Tier 1 — Pure Domain Logic  [after Tier 3; parallel ok: Go + TS simultaneously, and with Tier 2]

**Detail:** `tier-1-pure-domain-logic.md`

Go: gomavlib codec wrapper + dispatch table decoding the 13 telemetry families to
`TelemetryEvent` and the 5 transaction families to `ProtocolEvent`.
TS: all resolver functions + `sampleFromEvent` shim over the generated types.
No UDP socket, no Redis, no Docker.

**Gate:** Tier 3 complete — the shim imports generated types, never a hand-written stub.

**Exit gate:** `make gate-tier-1` — `go test -race ./internal/codec/...` passes
against golden byte vectors; `vitest src/logic` passes against named fixtures;
NaN never returned from any resolver; every node created in a test is closed
(`goleak`).

---

### Tier 2 — Parity Apparatus + Test Vectors  [parallel ok with Tier 1]

**Detail:** `tier-2-parity-apparatus.md`

Go capability matrix (18 receive + 11 send + framing cases), derived from the
codec's dispatch table rather than maintained by hand. TS known-answer fixture
tables. Golden byte fixtures generated by a committed `pymavlink` script.

**Exit gate:** `make gate-tier-2` — `scripts/check-matrix.sh` finds no unstarted
rows, and `TestMatrixCoverage` asserts the matrix rows equal the dispatch table
keys. TS resolver tests reference named fixture files; no inline magic numbers.

---

### Tier 3 — Codegen Infrastructure  [second — immediately after Tier 0]

**Detail:** `tier-3-codegen.md`

`buf.gen.yaml` → Go stubs in `internal/gen/`, TS types in `frontend/src/gen/`.
CI gates. Commit generated files.

**Gate:** Tier 0 complete. Nothing else — this does not depend on Tier 1 or 2,
which is why it runs here rather than after them.

**Exit gate:** `make gate-tier-3` — generated Go compiles (`go build
./internal/gen/...`) and generated TS typechecks (`pnpm typecheck`).

---

### Tier 4 — Bridge Core (pure fold, injected clock)

**Gate:** Tiers 1 and 2 complete.

**Detail:** `tier-4-bridge-core.md`

Vehicle fold, route table, fleet:active SET. Docker Compose scaffolding with multi-stage SITL Dockerfile **[parallel ok with pure code]**.

**Exit gate:** Pure fold tests pass with fixed-clock inputs; `time.Now()` never called in fold; Docker Compose validates; SITL Dockerfile builds.

---

### Tier 5 — Transport Adapter + Live Bridge

**Detail:** `tier-5-transport-live.md`

UDP transport, Redis client, GCS heartbeat configured on the gomavlib node (not hand-rolled), goroutine supervision, /healthz. First live SITL connection.

**Gate:** Tier 4 fold + Docker Compose complete.

**Exit gate:** `docker-compose up` → VEHICLE_DISCOVERED in logs; GCS heartbeat confirmed via failsafe test; goroutine count stable over 60s.

---

### Tier 6 — First Connect Services + Track Layer + Telemetry Log

**Detail:** `tier-6-connect-services.md`

FleetService, TelemetryService, track layer (MAVLink path), TrackService. Frontend adapter context + TelemetryLog component.

**Gate:** Tier 5 live bridge complete.

**Exit gate:** Browser shows live telemetry log; each row names its MAVLink source; MockAdapter renders same component with fixture data; TrackEvent stream has MAVLink vehicle tracks.

---

### Tier 7 — Read Protocol Transactions

**Detail:** `tier-7-read-transactions.md`

Parameter read/list folds + mission download fold (pure). ParameterService, MissionService. ParametersPanel (read-only), MissionPanel.

**Gate:** Tier 6 services live.

**Exit gate:** ARMING_CHECK readable from live ArduCopter; 5-waypoint mission round-trips cleanly; all capability matrix rows for read transactions complete.

---

### Tier 8 — Write Protocol Transactions

**Detail:** `tier-8-write-transactions.md`

Command registry first. Then ordered by risk: 8a message interval → 8b param write → 8c mission upload → 8d mode change → 8e arm/disarm → MapPanel (required before 8f) → 8f guided reposition → 8g guided workflow lifecycle.

**Gate:** Tier 7 read transactions complete; command registry wired.

**Exit gate:** Armed copter in GUIDED, map click → copter moves to target; full guided workflow lifecycle runs against SITL.

---

### Tier 9 — Additional Protocol Adapters

**Detail:** `tier-9-protocol-adapters.md`

ADS-B (MAVLink ADSB_VEHICLE + dump1090/Beast) and Meshtastic adapters. Both normalize to Track proto. Map renders all sources without importing MAVLink.

**Gate:** Tier 6 track layer live; hardware available.

---

### Tier 10 — UI Composition

**Detail:** `tier-10-ui-composition.md`

Base component modules → trusted instrument tier → composed layout tier. Instruments not scheduled until resolvers proven at logic + service layer.

**Gate:** All Tier 8 write transactions complete; all instruments have live data sources.

---

## UI Information Architecture (designed now, built in Tier 10)

### Base component modules

| Component | Data | Trust requirement |
|---|---|---|
| `TelemetryValue` | name, value, unit, source, age | Resolver returns non-null with named source |
| `TelemetryLog` | list of `TelemetryValue` rows | Tier 6 complete |
| `VehicleChip` | sysId, type icon, armed/mode, freshness dot | `HeartbeatState` from Tier 5 |
| `CommandButton` | label, eligibility gate, loading state, result | Write transaction fold from Tier 8 |
| `SourceBadge` | primary / fallback / missing + message name | Resolver `source` field |
| `FreshnessRing` | last-seen age as fill or color | `freshness.ts` from Tier 1 |

### Trusted instrument tier (one instrument per resolver)

| Instrument | Resolver | Prerequisite |
|---|---|---|
| `AttitudeIndicator` | `resolveAttitude` | pitch ladder + roll arc |
| `HeadingIndicator` | `resolveHeading` | fallback chain proven |
| `FlightStateDisplay` | `resolveFlightPath2d` | NED sign flip tested as `-vz_m_s` (protos are already SI — no /100); VFR_HUD path exempt from the flip |
| `PredictiveTrajectory` | `resolvePredictiveTrajectory(sample, stallSpeedMps)` | CTRV model tested; stall gate keyed on airspeed presence, not vehicle type; VTOL hover renders a track |
| `FlightPathRecorder` | `accumulateTrack` | ENU projection tested |
| `AltitudeTape` | `resolvePosition.altM` + `altRef` | Position resolver tested; tape refuses a datum it was not configured for (GPS_RAW fallback is MSL, GLOBAL_POSITION_INT is above home) |

### Composed layout tier

| Layout | Composes | Schedule |
|---|---|---|
| `PrimaryFlightDisplay` | `AttitudeIndicator` + `HeadingIndicator` | After both instruments pass SITL |
| `HudPanel` | PFD + `FlightStateDisplay` + `PredictiveTrajectory` | After all instruments validated |
| `OperatorPanel` | `VehicleChip` + `CommandButton` set + `ParametersPanel` | After Tier 8 write transactions |
| `NodeView` | `HudPanel` + `OperatorPanel` + `MapPanel` | After all above |

Note: `MapPanel` and `FleetView` are built during Tier 8 (before 8f), not here.

---

## What Is Deliberately Out of Scope

| Item | Reason |
|---|---|
| Supabase / OIDC live auth | `NoopAuthProvider` for SITL; real auth introduced when first external operator onboards |
| WebRTC / WFB-NG video | First-class sidecar; does not block any navigation tier |
| Calibration workflows | Requires command-driven state machines + physical prompts + reboot handling; post vehicle-config milestone |
| PID tuning | Depends on vehicle configuration milestone |
| Electron wrapper | Browser-first per ADR-0002 |
| gRPC-native browser transport | ADR defers; Connect serves as bridge |
| Meshtastic relay implementation | Adapter boundary defined (ADR-0004); waits on Tier 9 + hardware |
| mTLS between services | TLS termination at proxy satisfies ADR-0005 for now; mTLS is a hardening step |
| MISSION_CLEAR_ALL UI | Protocol defined; wire command in Tier 8; surface in mission panel post-Tier 10 |
