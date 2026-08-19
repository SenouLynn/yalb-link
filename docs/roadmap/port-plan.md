# yalb-gcs Port Plan: flight-path-hud → Refined Architecture
**Iteration 1 — Initial Port Roadmap**

> **Scope, and what this document is not authoritative for.**
> This is the canonical reference for **algorithms and file-to-file mapping** —
> what to port from flight-path-hud and where it lands.
>
> It is **not** authoritative for build tooling, codegen configuration, library
> choice, or sequencing. Those live in `order-of-operations.md` and the tier files,
> which supersede anything here that conflicts. Statements in this document that
> have been superseded are marked inline; the "Open Questions" section at the end is
> closed in full.
>
> Order of authority: ADRs → `order-of-operations.md` → tier files → this document.

---

## Context

`yalb-gcs` is a fully designed but entirely unimplemented GCS scaffold. It has:
- 14 proto files covering the complete domain (fleet, telemetry, commands, missions, track, mesh, chat, video, auth, security)
- 8 Connect RPC service definitions
- 5 ADRs documenting every architectural decision
- Strict TypeScript + Go linting; Bazel build system
- No running code beyond a `/healthz` endpoint

`flight-path-hud` is the predecessor — a mature, running GCS with proven algorithms, validated against real ArduPilot SITL. It contains:
- Pure TypeScript domain resolvers (attitude, heading, trajectory, flight path, position)
- MAVLink v1/v2 binary codec in Go (read-only parity slice, byte-for-byte accurate)
- Working HUD instruments (SVG-based: attitude, heading, predictive trajectory, 3D trail)
- Full GCS app (map, fleet roster, mission panel, guided workflow, parameters)
- Portable contracts library (JSON Schema + golden MAVLink bytes + behavioral traces)
- Dual-gate safety model for writes (bridge-side gate + UI-side eligibility check)
- Deterministic SITL replay (JSONL, injected clock, no system calls in domain logic)

**This plan's goal:** Port the proven implementations into yalb-gcs's cleaner, refined architecture — upgrading from ad-hoc JSON WebSocket to protobuf + Connect RPC, adding Redis as an integration bus, enabling multi-protocol track layer and full auth/security from day one.

**Canonical references throughout:**
- MAVLink 2.0 spec (common.xml, ardupilotmega.xml)
- ArduPilot SITL UDP:14550 — pinned at `Copter-4.7.0` and `Plane-4.6.3` (superseded 2026-08-19; this read "Copter 4.6.x, Plane 4.5.x"). The tags carry no `Ardu` prefix — that is the binary name. See the SITL firmware pins row in `order-of-operations.md`
- QGroundControl source (parameter handling, MAVLink command sequences)
- yalb-gcs ADRs 0001–0005 (highest-authority architectural guidance)
- flight-path-hud as algorithm/pattern source material, not as a style guide

---

## Principles for This Port

1. **yalb-gcs ADRs are law.** Where flight-path-hud and the ADRs diverge, ADRs win.
2. **Refine, don't copy.** flight-path-hud uses ad-hoc JSON; yalb-gcs uses proto types. Translate algorithms, not wire formats.
3. **Pure domain logic stays pure.** Resolvers (attitude, heading, trajectory) are framework-free functions; they port directly to TypeScript with no dependencies.
4. **Dual-gate safety model is non-negotiable.** Every write (arm/disarm, mode change, mission upload, guided reposition) requires a service-side gate and a UI-side eligibility check. No automatic writes.
5. **SITL before hardware.** Nothing is considered working until it passes a live ArduPilot SITL test.
6. **Contracts as evidence.** Every protocol boundary must have a schema or test vector. Claim nothing without proof.

---

## Architecture Mapping

| flight-path-hud | yalb-gcs equivalent | Refinement |
|---|---|---|
| Node.js WebSocket bridge | Go hexagonal pipeline | Goroutine-per-layer, typed channels |
| JSON wire envelope (messageName, payload) | `TelemetryEvent` proto oneof | Schema-enforced, backward-compatible |
| `TelemetrySample` (TS) | `TelemetryEvent` → normalized via resolvers | Proto as source of truth |
| `NodeIdentity` (sysId/compId) | `VehicleId` (system_id/component_id) + `NodeId` (track layer) | Dual-layer: MAVLink-specific + protocol-agnostic |
| `vehicles` map (TTL fold) | `Vehicle model` → Redis HSET + XADD | Distributed state, cold-start from Redis |
| WebSocket pub/sub | Redis pub/sub → Connect server streams | Multi-client fan-out, persistent history |
| MockAdapter / WebSocketAdapter | MockAdapter / WebSocketAdapter (ADR-0003) | Same pattern, different transport |
| Go bridge read-only parity slice | Go frame/message codec layer | UDP transport + SITL + live writing |
| JSONL replay | Redis Streams replay | Persistent, queryable, multi-consumer |
| JSON Schema contracts | Proto `.proto` files + buf linting | Breaking-change detection, generated clients |
| Guided workflow state machine | `CommandService.SetArmed/SetMode/SetPositionTargetGlobal` | Service-layer gates + React Query mutations |
| Safety gate (SITL-only) | `NoopAuthProvider` / SITL-only `ConnectLink` | Auth model enforces operator roles from day one |

---

## Phase 0: Infrastructure Foundation
**Goal:** `docker-compose up && bazel build //...` works; SITL heartbeat received by Go backend.

### 0a. Docker Compose
Create `docker-compose.yml` with four services:
```
ardupilot-sitl   — Copter-4.7.0 / Plane-4.6.3, MAVLink UDP:14550, TCP:5760 for SITL control
redis            — :6379, no auth in dev
gcs-backend      — Connect HTTP :8080, WebSocket (SITL harness) :8081
gcs-frontend     — Vite dev server :3000, proxies Connect to :8080
```
~~Source SITL image from `ardupilot/ardupilot-dev-jammy` or equivalent.~~ **Superseded:** that image is a dev environment, not a SITL runtime. `docker/sitl/Dockerfile` is a multi-stage build against a pinned tag — see Tier 4. Confirm MAVLink output with `mavproxy.py --master=udp:127.0.0.1:14550`.

### 0b. Connect Codegen (buf + Bazel)

> **Superseded by `tier-3-codegen.md`.** The config below is v1-era and does not
> work as written: the key is `remote:` not `plugin:`, `out:` paths are relative to
> the repo root, `buf.build/connectrpc/es` does not belong in the protobuf-es v2
> line, and `buf generate` needs `--template proto/buf.gen.yaml`. `rules_buf` is
> deferred (see `order-of-operations.md`); Bazel resolves Go deps from `go.mod` via
> gazelle's `go_deps` extension instead. Kept for the output-location intent only.

Add `buf.gen.yaml` to `proto/`:
```yaml
version: v2
plugins:
  - plugin: buf.build/protocolbuffers/go
    out: gen/go
    opt: paths=source_relative
  - plugin: buf.build/connectrpc/go
    out: gen/go
    opt: paths=source_relative
  - plugin: buf.build/bufbuild/es
    out: frontend/src/gen
  - plugin: buf.build/connectrpc/es
    out: frontend/src/gen
```
Wire `rules_buf` into `MODULE.bazel` and `proto/BUILD.bazel`. Generated Go stubs land in `internal/gen/gcs/v1/`; generated TS client land in `frontend/src/gen/gcs/v1/`.

### 0c. Go Hexagonal Skeleton
Stub out the pipeline in `internal/` as interfaces only — no implementation:
```
internal/
  transport/   adapter.go  (Transport interface: Recv() chan []byte, Close())
  codec/       frame.go    (FrameDecoder interface → chan Frame)
               message.go  (MessageDecoder interface → chan Message)
  router/      router.go   (Router: dispatch Message to service handlers)
  vehicle/     model.go    (VehicleModel: fold Message → VehicleSnapshot → Redis publish)
  services/    fleet.go    telemetry.go  command.go  track.go  (Connect handlers)
  redis/       client.go   (RedisClient: Publish, Subscribe, HSet, XAdd)
```
Each layer boundary is a typed Go channel. Context cancellation wires through every goroutine. Follows ADR-0002's pipeline diagram exactly.

**Verification:** `bazel build //...` passes. `docker-compose up` brings SITL + Redis + containers online.

---

## Phase 1: Go MAVLink Frame + Message Codec
**Goal:** Raw UDP bytes from SITL → typed `TelemetryEvent` proto message.

### 1a. Frame Codec (`internal/codec/frame.go`)

> **Superseded by `tier-1-pure-domain-logic.md`.** The gomavlib-vs-hand-roll question
> is closed in favour of **gomavlib v3**: STX detection, length extraction,
> CRC_EXTRA validation, v2 signing and dialect struct population are all library
> concerns. Do not port hand-rolled framing. The list below is retained as a
> description of what the library must be shown to do, i.e. the framing cases in the
> Tier 2 capability matrix.

Port from `flight-path-hud/apps/mavlink-bridge-go/` (MAVLink v1/v2 binary parsing):
- Start-of-frame detection (STX: 0xFE for v1, 0xFD for v2)
- Payload length extraction + complete-frame accumulation
- CRC-16/MCRF4XX with message-specific seed (from common.xml CRC_EXTRA table)
- MAVLink 2.0 signing verification (when `MavlinkSigningConfig.enabled`)
- Outputs `Frame` struct: `{Version, SystemID, ComponentID, MessageID, Payload []byte, Sequence uint8}`

Source: flight-path-hud Go bridge CRC/framing logic; cross-reference MAVLink 2.0 spec §6.

### 1b. Message Codec (`internal/codec/message.go`)
Deserialize `Frame.Payload` into typed Go structs (generated from common.xml + ardupilotmega.xml):
- Priority messages: `HEARTBEAT` (#0), `ATTITUDE` (#30), `GLOBAL_POSITION_INT` (#33), `VFR_HUD` (#74), `GPS_RAW_INT` (#24), `SYS_STATUS` (#1), `RADIO_STATUS` (#109), `STATUSTEXT` (#253), `MISSION_CURRENT` (#42), `COMMAND_ACK` (#77), `PARAM_VALUE` (#22), `HOME_POSITION` (#242), `NAV_CONTROLLER_OUTPUT` (#62), `EKF_STATUS_REPORT` (#193)
- ~~Use `gomavlib` or hand-coded structs (either acceptable)~~ → **gomavlib v3**, closed
- Map decoded message → `TelemetryEvent` (13 streaming families) **or** `ProtocolEvent`
  (PARAM_VALUE, MISSION_COUNT, MISSION_ITEM_INT, MISSION_ACK, COMMAND_ACK). Not one
  envelope — see `tier-0-proto-contracts.md` Chapter 3.

### 1c. UDP Transport Adapter (`internal/transport/udp.go`)
- `net.PacketConn` on 0.0.0.0:14550
- Read loop → `chan []byte`
- Send path (for commands): `chan []byte` → `WriteTo(addr)`
- Context cancellation triggers `Close()`
- Follows ADR-0002's transport adapter interface

### 1d. Vehicle Model (`internal/vehicle/model.go`)
Port freshness/TTL model from `flight-path-hud/packages/gcs-core/src/nodes.ts`:
- Per-vehicle goroutine (keyed by `VehicleId`)
- `HEARTBEAT` → update `HeartbeatState`, publish `FleetEvent{VEHICLE_DISCOVERED}` to Redis
- Telemetry messages → update `VehicleSnapshot` fields
- Atomic snapshot read via `sync/atomic.Value` or `sync.RWMutex`
- 60-second TTL: no heartbeat → `FleetEvent{VEHICLE_LOST}`, goroutine exits
- Publish to Redis channels: `track:events`, `telemetry:<system_id>`, `fleet:events`
- Redis HSET: `vehicle:state:<system_id>` (last-known `VehicleSnapshot`)

**Verification:**
- Go unit test: feed golden MAVLink bytes (from flight-path-hud `contracts/mavlink/`) → assert correct proto fields
- SITL integration: `docker-compose up` → `go test ./...` passes; observe Redis channels with `redis-cli SUBSCRIBE`

---

## Phase 2: Connect Service Layer (Backend)
**Goal:** Frontend can call `TelemetryService.StreamTelemetry` and receive live data from SITL.

### 2a. FleetService (`internal/services/fleet.go`)
- `WatchFleet`: Subscribe to `fleet:events` Redis channel, stream `FleetEvent` protos to client
- `ListVehicles`: HGETALL from `vehicle:state:*` Redis keys → `ListVehiclesResponse`
- `ConnectLink`: Accept `ConnectLinkRequest.uri`, spin up transport adapter (UDP for now), register link
- `DisconnectLink`: Cancel link context, publish `LINK_DISCONNECTED` audit event
- `WatchLinkStatus`: Stream `LinkStatus` from Redis

### 2b. TelemetryService (`internal/services/telemetry.go`)
- `StreamTelemetry`: Subscribe to `telemetry:<vehicle_id>` Redis channel per requested vehicle, fan-out `TelemetryEvent` stream
- `GetSnapshot`: HGET `vehicle:state:<system_id>` → deserialize `VehicleSnapshot` proto

### 2c. CommandService (`internal/services/command.go`)
Port dual-gate safety model from flight-path-hud:
- **Service-side gate**: Check `OperatorContext` role ≥ `OPERATOR` before any write
- **COMMAND_LONG / COMMAND_INT** encoding: port golden byte sequences from `flight-path-hud/contracts/mavlink/`
- `SetArmed`: encode MAVLink `COMMAND_LONG` (CMD 400, force=0 always), send via transport, wait for `COMMAND_ACK`, verify `HEARTBEAT.base_mode & MAV_MODE_FLAG_SAFETY_ARMED`
- `SetMode`: disarmed-only gate, vehicle-type allowlist (ArduCopter vs ArduPlane modes differ), ACK correlation, HEARTBEAT post-condition
- `SetPositionTargetGlobal`: encode `SET_POSITION_TARGET_GLOBAL_INT` (#86), fire-and-forget with position verification
- Write `AuditEvent` to Redis Stream `audit:events` (async, never blocks command path)

### 2d. TrackService (`internal/services/track.go`)
- Subscribe to `track:events` Redis channel
- Normalize `VehicleSnapshot` → `Track` with `MavlinkDetail` (ADR-0004)
- `WatchTracks`: stream `TrackEvent` protos to frontend map

**Verification:**
- Connect client test (httptest): call `StreamTelemetry`, confirm events arrive
- SITL: browser curl of Connect endpoint receives `TelemetryEvent` JSON (connect-go has REST bridge)

---

## Phase 3: Frontend — Adapter Pattern + Domain Logic
**Goal:** Components are testable with MockAdapter; pure resolvers pass all validation tests.

### 3a. Adapter Context (`frontend/src/adapters/`)
Port ADR-0003 adapter pattern:
```
src/adapters/
  types.ts         — GcsAdapter interface (useVehicleTelemetry, useFleet, useSendCommand, ...)
  connect.ts       — ConnectAdapter (wraps generated Connect TS client)
  mock.ts          — MockAdapter (static fixture data, used in tests + Storybook)
  replay.ts        — ReplayAdapter (JSONL or Redis Streams replay)
  context.tsx      — AdapterProvider + useAdapter() hook
```
`ConnectAdapter` subscribes to proto `TelemetryEvent` stream via `@connectrpc/connect-web`, feeds into `queryClient.setQueryData()` per vehicle. `MockAdapter` returns fixture `TelemetryEvent` protos — same shape, no network.

### 3b. Pure Domain Resolvers (`frontend/src/logic/`)
Port directly from `flight-path-hud/packages/hud-ui/src/logic/` and `packages/gcs-core/src/`:

**Adapt from `TelemetrySample` → proto `TelemetryEvent` input types.**

```
src/logic/
  attitude.ts      — resolveAttitude(event: TelemetryEvent) → { pitchDeg, rollDeg, source }
  heading.ts       — resolveHeading(event: TelemetryEvent) → { headingDeg, source, isFallback }
  flightPath.ts    — resolveFlightPath2d(event) → { trackDeg, groundSpeedMps, climbMps, fpaRad }
  trajectory.ts    — resolvePredictiveTrajectory(event) → { forwardPoints[], turnRate, stall }
  position.ts      — resolvePosition(event) → { latDeg, lonDeg, altM, source }
  track.ts         — accumulateTrack(prev: ENU[], event) → ENU[]  (ring buffer)
  freshness.ts     — isFresh(lastSeenMs, ttlMs) → boolean
```

**Key algorithms to preserve** — with the corrections that apply in this repo. The
algorithm shapes port; three of the constants and one of the units do not. See
`tier-1-pure-domain-logic.md` Chapter 5 for the authoritative statement.

- **Heading fallback chain**: `VFR_HUD.heading` → `ATTITUDE.yaw` → `GLOBAL_POSITION_INT.hdg`.
  The 65535 reject belongs to `GLOBAL_POSITION_INT.hdg` (uint16 cdeg), **not** VFR_HUD —
  `VFR_HUD.heading` is `int16_t` and cannot hold it. What the VFR_HUD path needs is
  normalisation of negative values.
- **NED sign flip**: `climbMps = -vzMs`. **No `/100`** — this repo's protos already
  carry m/s, unlike flight-path-hud's raw wire envelope.
- **Turn rate**: unchanged — `ψ̇ = (sin φ·q + cos φ·r)/cos θ`, falling back to `g·tan(roll)/V`.
- **Stall gate**: keyed on **flight regime, not vehicle type** — apply the floor only
  when airspeed is present and below stall. 14 m/s is not a constant; it is
  `ARSPD_FBW_MIN`, passed in as an argument.
- **ENU projection**: unchanged.

**Input type translation:** flight-path-hud reads from `TelemetrySample` (flat struct). yalb-gcs resolvers receive a `TelemetryEvent` proto with a `oneof payload`. Write a thin adapter `sampleFromEvent(e: TelemetryEvent): TelemetrySample` that maps proto fields to the same flat shape the resolver logic expects — isolating the translation to one file.

Strict TS compliance: all resolver return types are `exactOptionalPropertyTypes`-safe (no `| undefined` unless the field genuinely may be absent).

**Verification:**
- Port known-answer test vectors from `flight-path-hud/packages/hud-ui/src/logic/replay.ts`
- Run `vitest` against every resolver with the same hand-computed expected values
- Confirm NaN is never returned (sanitize at boundary, ADR-0002)

---

## Phase 4: HUD Instruments
**Goal:** Live SITL data renders in all five instrument panels.

Port from `flight-path-hud/packages/hud-ui/src/components/`:

```
frontend/src/components/hud/
  AttitudeIndicator.tsx      — SVG artificial horizon, pitch ladder, roll arc
  HeadingIndicator.tsx       — Scrolling compass tape (0–360°)
  PrimaryFlightDisplay.tsx   — Composes heading + attitude, heading tape overlay
  PredictiveTrajectory.tsx   — 3D perspective flight-path corridor:
                               • Pinhole camera: focal=240px·m, near-plane 6m, camera height 3.6m
                               • World rolls with aircraft (two-layer: ground grid + horizon)
                               • Corridor banks opposite to roll
                               • Sky/ground color by climb sign
  FlightPathRecorder.tsx     — Orbitable 3D breadcrumb trail (ENU frame, ground shadow + drop lines)
  FlightState.tsx            — Text telemetry readout (speed, altitude, climb, heading)
```

Each component:
- Receives `TelemetryEvent` or pre-resolved values from parent hook
- Renders SVG (no Canvas unless forced by performance)
- Testable in Storybook via MockAdapter fixture
- No logic inside component — logic lives in `src/logic/`

Wire via `useVehicleTelemetry(vehicleId)` hook (defined in adapter context), which calls the appropriate resolver and returns display-ready values.

**Verification:**
- Storybook: all five instruments render at 0°/0°/0° and at arbitrary fixture values
- SITL: instruments update live at ~10 Hz (ATTITUDE message rate)

---

## Phase 5: GCS Map + Fleet View
**Goal:** Multi-vehicle map displays all SITL vehicles; clicking vehicle opens NodeView.

```
frontend/src/components/
  FleetView.tsx              — Roster + basemap; vehicle cards color-coded by freshness
  NodeView.tsx               — Single-vehicle detail page (map + instruments + panels)
  MapPanel.tsx               — MapLibre GL:
                               • Vehicle markers (heading arrow, type icon)
                               • Flight track polyline (ENU → lat/lon unprojection)
                               • Mission waypoint overlay
                               • Follow/track-up/3D tilt modes (ref ADR-0025 from flight-path-hud)
```

**Track Layer** (`TrackService` → map):
- Subscribe to `TrackService.WatchTracks` via ConnectAdapter
- Each `TrackEvent` updates a `Map<NodeId, Track>` in React Query cache
- Map renders from cache; no per-message re-render (use `setQueryData` + `useSuspenseQuery`)
- Protocol-agnostic: when Meshtastic/ADS-B adapters land later, they appear here with zero map code changes

**Freshness indicators:**
- Port `flight-path-hud/packages/gcs-core/src/nodes.ts` freshness model
- `isFresh(lastSeenMs, 5000)` → green; `isFresh(lastSeenMs, 15000)` → yellow; stale → grey
- Vehicle card flips to "lost" after 60 s (mirrors vehicle model TTL)

**Verification:**
- SITL: open browser → FleetView shows ArduCopter vehicle → click → NodeView opens
- Three SITL instances: all three vehicles visible on map simultaneously

---

## Phase 6: Command Surfaces (Safety-Gated)

> **Superseded on sequencing — this is the last phase to build, not the sixth.**
> Phase numbers here are file-mapping identifiers, not a build order; this document is not
> authoritative for sequencing (see the header). Writes come *after* the reads in Phases 7
> and 8, per the resolved decision "params (read) before commands (write)" and Principle 3.
> The Open Questions list at the end records that resolution, but these headings were never
> annotated to match — corrected 2026-08-19. The real build order is Tier 8 in
> `order-of-operations.md`, which sequences these by risk: message interval → param write →
> mission upload → mode change → arm/disarm → guided reposition.

**Goal:** Operator can arm/disarm, change mode, and guided-reposition from UI.

### 6a. Guided Workflow Panel (`frontend/src/components/command/GuidedWorkflowPanel.tsx`)
Port state machine from `flight-path-hud/packages/gcs-core/src/guidedWorkflow.ts`:
- States: `IDLE → AWAITING_ACK → AWAITING_OBSERVATION → COMPLETE | FAILED`
- Arm: must be in `GUIDED` mode, `force=0` always (never force-arm)
- Takeoff: COMMAND_LONG CMD 22 (NAV_TAKEOFF), altitude from user input
- Land: COMMAND_LONG CMD 21 (NAV_LAND) or mode change to LAND
- Disarm: COMMAND_LONG CMD 400 param1=0

Each transition calls `CommandService.SetArmed` or `CommandService.SetMode` via React Query `useMutation`. Optimistic update shows "in progress"; rollback on `COMMAND_ACK.result != ACCEPTED`.

**UI gates:**
- "Arm" button disabled unless: `FlightMode == GUIDED`, not already armed, operator role
- "Disarm" button disabled unless: already armed, operator role
- No arm/disarm in automatic retry loop — exactly once, user must confirm failure

### 6b. Guided Reposition Panel (`frontend/src/components/command/GuidedRepositionPanel.tsx`)
Port from `flight-path-hud/packages/gcs-core/src/guidedReposition.ts`:
- User clicks map or enters lat/lon/alt
- Encode `SET_POSITION_TARGET_GLOBAL_INT` (#86): type_mask=0b0000111111111000 (position only)
- `CommandService.SetPositionTargetGlobal` via `useMutation`
- Read-back verification: poll `VehicleSnapshot.position` until within 2m or timeout

**Bounds validation (UI-side gate):**
- Reject target >500m from home (configurable)
- Reject altitude below 2m AGL
- Reject if vehicle not in GUIDED mode

**Verification:**
- SITL: arm copter → switch GUIDED → click map target → verify copter moves to point
- Replay: guided reposition replay does NOT re-execute commands (read-only replay mode)

---

## Phase 7: Mission Service

> **Sequencing:** the *download* half precedes Phase 6 (Tier 7); the *upload* half is a
> write and belongs with it (Tier 8c).

**Goal:** Download and display active ArduPilot mission; upload a mission from GCS.

### 7a. Mission Download (`MissionService.DownloadMission`)
Protocol (reference: QGroundControl source `MissionManager.cc`):
1. Send `MISSION_REQUEST_LIST` → receive `MISSION_COUNT`
2. For each item: send `MISSION_REQUEST_INT` → receive `MISSION_ITEM_INT`
3. Send `MISSION_ACK` when complete
Decode each item as `MissionItem` proto; stream to client via `DownloadMission` RPC.

### 7b. Mission Upload (`MissionService.UploadMission`)
Protocol:
1. Send `MISSION_COUNT`
2. Vehicle sends `MISSION_REQUEST_INT` for each item
3. Send `MISSION_ITEM_INT` for requested item
4. Receive final `MISSION_ACK`
Client streams `MissionItem` protos via bidirectional `UploadMission` RPC.

### 7c. Mission Panel (`frontend/src/components/mission/MissionPanel.tsx`)
Port from flight-path-hud `MissionPanel`:
- Display seq, command (decoded from `MavCmd` enum), position, param1–4
- Overlay waypoints on map (lat/lon markers, sequence numbers)
- `SetCurrentItem` highlights active item (MISSION_CURRENT message)

**Verification:**
- SITL: plan 5-waypoint mission in QGC → load in yalb-gcs → verify identical item list
- Upload round-trip: upload mission → download → assert identical `MissionItem` list

---

## Phase 8: Parameter Service

> **Sequencing:** 8a is a *read* and precedes Phase 6 (Tier 7); 8b is a write (Tier 8b).
> Note the collision: "8a" here means Parameter List/Get, while **Tier** 8a in
> `order-of-operations.md` means message interval. They are unrelated.

**Goal:** Read and display all vehicle parameters; write parameters with SITL gate.

### 8a. Parameter List/Get (`ParameterService.ListParameters`, `GetParameter`)
Port from flight-path-hud Go bridge parameter state machine:
- `PARAM_REQUEST_LIST` → stream `PARAM_VALUE` messages until `param_index == param_count - 1`
- Handle re-request on timeout (QGC pattern: retry each missing index after 1s)
- Convert `param_value` (float wire) to typed value using `MavParamType`
- Store in Redis HSET `params:<system_id>:<param_id>`

### 8b. Parameter Write (`ParameterService.SetParameter`)
- `PARAM_SET` → wait for `PARAM_VALUE` echo with same `param_id`
- Service-side gate: operator role required
- Audit log: `AuditEvent{PARAMETER_CHANGED}` to Redis Stream

### 8c. Parameters Panel (`frontend/src/components/params/ParametersPanel.tsx`)
- Searchable list of all parameters with type-aware value display
- Edit in-place → confirm → `SetParameter` mutation
- Read-only mode if `OperatorRole == OBSERVER`

**Verification:**
- SITL: list parameters → verify `ARMING_CHECK` present → modify value → read back → confirm change

---

## Critical Files to Modify / Create

| File | Action | Notes |
|---|---|---|
| `docker-compose.yml` | Create | SITL + Redis + backend + frontend |
| `proto/buf.gen.yaml` | Create | Connect codegen for Go + TS |
| `MODULE.bazel` | Edit | Add rules_buf |
| ~~`internal/transport/udp.go`~~ | **Do not create** | **Superseded.** gomavlib owns its socket through an `EndpointConf` and takes an endpoint, not a byte stream, so there is no seam. `codec.FrameSource` is the transport port; `cmd/gcs` builds `gomavlib.EndpointUDPServer` and hands it to `codec.NewNode`. See the transport package placement row in `order-of-operations.md` |
| `internal/codec/frame.go` | Create | MAVLink v1/v2 framing |
| `internal/codec/message.go` | Create | Message deserialization |
| `internal/vehicle/model.go` | Create | Vehicle state fold → Redis |
| `internal/services/fleet.go` | Create | FleetService Connect handler |
| `internal/services/telemetry.go` | Create | TelemetryService Connect handler |
| `internal/services/command.go` | Create | CommandService with dual-gate |
| `internal/services/track.go` | Create | TrackService Connect handler |
| `internal/redis/client.go` | Create | Redis pub/sub + HSET + XADD |
| `cmd/gcs/main.go` | Edit | Wire pipeline, start server |
| `frontend/src/adapters/*.ts` | Create | ConnectAdapter, MockAdapter, context |
| `frontend/src/logic/*.ts` | Create | Pure resolvers (ported from flight-path-hud) |
| `frontend/src/components/hud/*.tsx` | Create | 5 HUD instruments |
| `frontend/src/components/command/*.tsx` | Create | Guided workflow + reposition panels |
| `frontend/src/components/map/MapPanel.tsx` | Create | MapLibre GL map |
| `frontend/src/components/fleet/FleetView.tsx` | Create | Fleet roster |

**Source material files in flight-path-hud to reference:**
- `apps/mavlink-bridge-go/` — Go MAVLink codec (Phase 1)
- `packages/hud-ui/src/logic/*.ts` — Pure resolvers (Phase 3)
- `packages/hud-ui/src/components/*.tsx` — SVG HUD instruments (Phase 4)
- `packages/gcs-core/src/guidedWorkflow.ts` — Command state machine (Phase 6)
- `packages/gcs-core/src/guidedReposition.ts` — Reposition encoding (Phase 6)
- `packages/gcs-core/src/nodes.ts` — Multi-vehicle identity (Phase 5)
- `contracts/mavlink/*.json` — Golden MAVLink byte vectors (Phase 1 tests)
- `contracts/wire/*.schema.json` — Wire format schemas (useful for test vectors)

---

## Verification Strategy (End-to-End)

```
Phase 0 ─ docker-compose up → SITL heartbeat in logs
Phase 1 ─ go test ./internal/codec/... (golden bytes → proto)
Phase 2 ─ curl connect endpoint → TelemetryEvent JSON
Phase 3 ─ vitest ./frontend/src/logic/... (known-answer vectors)
Phase 4 ─ Storybook: all 5 HUD components render
Phase 5 ─ Browser: FleetView shows live SITL vehicle on map
Phase 7 ─ SITL: download mission uploaded by QGC → verify parity      (read)
Phase 8 ─ SITL: list params → read back                               (read)
Phase 8 ─ SITL: modify LOG_BITMASK → confirm PARAM_VALUE echo         (write)
Phase 6 ─ SITL: arm copter → guided reposition → observe flight       (write)
```

**The last three lines were reordered on 2026-08-19** to match "params (read) before
commands (write)". The write target also changed: this read "modify `ARMING_CHECK`", which
is a safety parameter. Tier 8b names `LOG_BITMASK` as the first write target specifically
because it is low-consequence and survives a power cycle harmlessly.

**Regression gate:** the tier gate for the phase must pass — `make gate-tier-0`
through `make gate-tier-3`, then `bazel test //...` once BUILD files are generated
(`make bazel-tidy`).

---

## Open Questions — all closed

Resolved in `order-of-operations.md` under "Resolved Design Decisions". Kept here so
nobody re-opens them from this document.

1. ~~Go MAVLink library choice~~ → **gomavlib v3** with `ardupilotmega.Dialect`.
2. ~~Map tile provider~~ → **MapLibre GL + PMTiles**, self-hosted, no API key.
3. ~~Phase ordering, params vs commands~~ → **params (read) before commands (write)**.
4. ~~Storybook vs Vite MSW~~ → **vitest** for pure logic; MSW at Tier 6; Storybook deferred.
5. ~~rules_buf maturity~~ → **commit generated files**, `make proto-gen` as the developer
   command; rules_buf deferred. Bazel resolves Go deps from `go.mod` via `go_deps`.
