# Order of Operations: ligma-gcs Buildout
**Iteration 2 — Adversarial synthesis from three sources + independent review**

---

## What This Document Is

A forward-looking build order, superseding the Iteration 1 draft. Derived from:
1. Walking the `flight-path-hud` commit history and Go bridge handoff doc
2. The first agent's review (MAVLink gaps, Redis schema, safety concerns)
3. The second agent's block-sequencing recommendation
4. Independent adversarial review of all three

`port-plan-v1.md` in this directory maps what to port and where it lands.
This document answers *in what order* to build, *why*, and *what must be true before moving on*.

ADRs are law. If this document contradicts an ADR, the ADR wins.

---

## Adversarial Findings: What Both Prior Drafts Got Wrong

These are not preferences. They are correctness issues that would cause silent failures,
blocked tiers, or re-work if not resolved before code is written.

### 1. GCS heartbeat is missing and its absence is a production failure mode

Neither the v1 order-of-operations nor the port plan mentions this. ArduPilot
`FS_GCS_ENABL=1` (enabled by default in Copter 4.x) triggers a failsafe if
no GCS HEARTBEAT arrives for ~5 seconds. The GCS must send `HEARTBEAT`
(sysId=255, compId=190, type=6/GCS, autopilot=0/GENERIC) at 1 Hz from the
moment a vehicle is discovered.

This is not optional. Guided reposition commands will succeed then silently
time out if this is absent. Add it to Tier 5 (transport live) — not the
transport layer (which is a byte pipe), but a dedicated goroutine in
the vehicle model that ticks at 1 Hz and pushes outbound bytes into the
transport write channel.

### 2. Hand-rolling the codec is wrong for this project

flight-path-hud hand-rolled its Go bridge, but that was a **read-only** slice
covering 14 message families for decode only. ligma-gcs needs:

**Outbound encoding** (write path):
- `COMMAND_LONG` (#76) — arm, mode, takeoff, land
- `SET_POSITION_TARGET_GLOBAL_INT` (#86) — guided reposition
- `PARAM_SET` (#23) — parameter write
- `PARAM_REQUEST_LIST` (#21) — parameter list
- `PARAM_REQUEST_READ` (#20) — parameter read
- `MISSION_COUNT` (#44) — mission upload initiation
- `MISSION_ITEM_INT` (#73) — mission item upload
- `MISSION_REQUEST_INT` (#51) — mission item request
- `MISSION_ACK` (#47) — mission complete
- `MAV_CMD_SET_MESSAGE_INTERVAL` (#511) — telemetry rate control

That is 10 outbound message types, each needing correct little-endian packing,
CRC_EXTRA computation, and sequence management. Hand-rolling encoding for all
of these with correct CRC_EXTRA seeds is weeks of work and a source of
hard-to-debug bugs (wrong byte order is CRC-silent until SITL rejects it).

**Use `gomavlib`** with `DialectCommon` + `DialectArdupilotmega`. It handles:
- CRC_EXTRA automatically per message ID
- Correct little-endian struct layout
- MAVLink v1/v2 framing for both read and write
- All 15 priority receive message families
- All 10 outbound message families above

The "minimal dependency surface" argument from v1 is wrong here. A codec
library is load-bearing infrastructure, not an optional convenience. The risk
of an incorrect hand-rolled encoder is higher than the risk of a maintained
library dependency.

**Retained from v1:** golden byte test vectors from `flight-path-hud/contracts/mavlink/`
still validate the codec end-to-end regardless of which implementation is used.

### 3. ACK correlation needs a concrete design before Tier 8 code is written

The port plan says "wait for COMMAND_ACK" as if ACKs are trivially matchable.
They are not. `COMMAND_ACK` (message #77) contains only:
- `command` (uint16): the command ID that was ACKed
- `result` (uint8): accepted/denied/etc.
- `target_system`, `target_component`: who it's for

There is no request correlation ID. With concurrent commands (arm + mode
change), `COMMAND_ACK` for `MAV_CMD_COMPONENT_ARM_DISARM` is ambiguous if
two are in-flight.

**Design: in-flight command registry**

```go
type inFlightCommand struct {
    ch      chan mavlink2.COMMAND_ACK
    cancel  context.CancelFunc
}

type commandRegistry struct {
    mu  sync.Mutex
    reg map[commandKey]*inFlightCommand
}

type commandKey struct {
    targetSysID  uint8
    targetCompID uint8
    commandID    uint16
}
```

Register before send; deregister on ACK or timeout (5s). If a duplicate key
arrives (same vehicle, same command, concurrent), reject the second call at the
service layer — the UI dual-gate prevents this for arm, but mode changes and
interval sets could race. Document this as a known limitation. Per-command
timeouts use `context.WithTimeout` on the registry entry.

### 4. Redis schema requires TTLs and MAXLEN from the start

Not retroactive — set these when the keys are first written:

| Key | Policy |
|---|---|
| `vehicle:state:<sysId>` HSET | `EXPIRE vehicle:state:<id> 300` on every heartbeat (rolling TTL); single `EXPIRE` on VEHICLE_LOST with 60s drain window |
| `audit:events` stream | `XADD audit:events MAXLEN ~ 50000 *` (approximate trimming; cheap) |
| `params:<sysId>:<param_id>` HSET | `EXPIRE params:<id>:<param> 3600` — stale after 1h; reload on next vehicle connect |
| `track:events` stream | `XADD track:events MAXLEN ~ 10000 *` — map only needs recent history |

Not setting these is an OOM risk in long dev sessions and leaves zombie
`vehicle:state:*` keys after SITL restarts.

### 5. The track layer must be scaffolded before Tier 8, not after

v1 places the track layer in Tier 9 (after all write transactions). The problem:
guided reposition (Tier 8f) requires the operator to click a position on the map.
The map is `MapPanel` which depends on `TrackService.WatchTracks` which depends
on the track layer. You cannot validate guided reposition without the map.

Move track layer scaffolding to Tier 6 alongside the first Connect services.
ADS-B and Meshtastic adapters remain deferred (Tier 9). The MAVLink-sourced
track path (`VehicleSnapshot` → `Track` → `track:events`) ships in Tier 6.

### 6. TLS is in ADR-0005 but missing from this document

ADR-0005 says TLS is on the Connect endpoint and the plan needs to account for
it. The v1 "out of scope" list silently contradicts ADR-0005 by not mentioning
TLS at all. Resolution: the Connect server is TLS-terminated by a reverse proxy
(nginx or Caddy in Docker Compose) from Tier 5. The backend itself listens on
HTTP/1.1 only. This is a one-day Docker Compose addition that satisfies ADR-0005
without touching Go service code.

### 7. ConnectLink auto-bind vs. operator-initiated is unresolved

Both documents leave this ambiguous. Resolution (explicit, authoritative):
- The backend **auto-binds `UDP:14550` at startup** and begins accepting SITL
  traffic immediately. No user action required. This is the primary operational path.
- `FleetService.ConnectLink` is for **runtime-added supplementary links** only
  (e.g., a secondary radio link on a different port, or a TCP serial bridge).
- The auto-bind config lives in `cmd/gcs/main.go` as an environment variable
  (`GCS_MAVLINK_UDP_BIND`, default `0.0.0.0:14550`).
- Setting `GCS_MAVLINK_UDP_BIND=""` disables auto-bind (useful for tests that
  don't want a real socket).

### 8. The second agent's block sequencing is too waterfall

The other agent sequences pure Go code → Docker → services → frontend as strict
sequential blocks. This prevents parallel work. The correct model:

Pure Go codec and pure TS resolvers are **fully independent**. Both can start
at Tier 1 simultaneously. Docker Compose scaffolding can be prepared in parallel
with Tier 1-4 pure work — it just doesn't need to work until Tier 5's gate.

### 9. Multi-SITL Docker Compose profile from Tier 5, not retrofit

Both documents defer multi-SITL to the Phase 5 verification. This creates a
painful retrofit: single-SITL assumptions bake into environment variable names,
port mappings, and test expectations. Scaffold three ArduCopter instances in
Docker Compose from the start, using Docker Compose profiles so `--profile sitl`
brings up all three. Each gets a distinct `SYSID_THISMAV` (1, 2, 3) and distinct
UDP output port (14550, 14560, 14570). The backend single-socket binds 14550 and
multiplexes by sysId — this is the correct single-socket design.

### 10. `force` field should not exist in the proto

`SetArmedRequest.force` should be removed from the proto definition, not just
documented as "always 0." A field that exists can be set by callers who bypass
the documented convention. The service cannot enforce a proto field to a fixed
value — it can only refuse requests that set it to 1. Removing the field is
the only safe choice. This requires a proto edit before Tier 3 codegen.

---

## Resolved Design Decisions

These are now closed. Do not re-open without an ADR.

| Decision | Resolution |
|---|---|
| gomavlib vs hand-roll | **gomavlib** with DialectCommon + DialectArdupilotmega. See §2 above. |
| Map tile provider | MapLibre GL + PMTiles (self-hosted). No API key. Field deployment = local mount. |
| Phase ordering: params vs commands | Params (read) before commands (write). Read-only proof before any writes. |
| Storybook vs Vite MSW | Vitest for pure logic. MSW in Tier 6 when ConnectAdapter needs a mock server. Storybook deferred to Tier 10 or beyond. |
| rules_buf vs buf generate | Commit generated files. Use `buf generate` as a Makefile target. Wire `rules_buf` into Bazel after Bzlmod support stabilizes. |
| Single socket vs per-vehicle sockets | **Single socket** at `0.0.0.0:14550`. Router dispatches by sysId. Multi-vehicle = multiple sysIds over one socket. |
| ConnectLink auto-bind vs operator-initiated | **Auto-bind at startup**. ConnectLink is for runtime supplementary links only. See §7 above. |
| GCS heartbeat placement | Dedicated goroutine in vehicle model, not transport layer. Ticks at 1 Hz per discovered vehicle. |
| ACK correlation design | In-flight command registry keyed by `(targetSysID, targetCompID, commandID)`. 5s timeout per entry. See §3 above. |
| Redis Streams vs Pub/Sub | `telemetry:<sysId>` → Pub/Sub (ephemeral, high-rate). `fleet:events`, `track:events`, `audit:events` → Streams (persistent, cold-start recovery). |
| TLS placement | Terminated at reverse proxy in Docker Compose. Backend speaks plain HTTP. See §6 above. |
| MAVLink signing key storage | Environment variable `GCS_MAVLINK_SIGNING_KEY` (hex). Empty = signing disabled (dev/SITL). Document in `docker-compose.yml` as optional. |
| Pub/Sub consumer groups | Use `XREADGROUP` with group `gcs-backend` from day one on all Streams. Cost is two lines of setup; benefit is zero schema change if replicas are added. |

---

## Principles

These are unchanged from v1 — they are correct.

1. **Evidence before claims** — nothing is "done" without a test vector, schema, or SITL trace.
2. **Offline before online** — pure folds first, then injected clock, then live sockets.
3. **Read before write** — read-only proven before any outbound capability.
4. **Trust as process** — trust is established through resolved value → known source → fallback chain → freshness → known-answer test.
5. **Contracts are the portable unit** — protos + golden bytes + semantic traces. Generated code is an adapter.
6. **UI composition is deferred** — components are not scheduled until their data is proven at logic + service layer.
7. **ADRs are law; this document is not.**

---

## Build Order

Parallel tracks are marked **[parallel ok]**. Sequential dependencies are marked
with their gate condition.

---

### Tier 0 — Proto Contracts + Proto Fix (do first)

**Status:** Mostly complete. One required edit before codegen.

Fix required now, before Tier 3 generates stubs:
- Remove `force` field from `SetArmedRequest` in `proto/gcs/v1/commands.proto`
- Confirm `buf lint` still passes after removal

**What exists:** 14 proto files, buf-linted, complete domain schema.
**What is missing:** `buf.gen.yaml`, `rules_buf`, generated code.
**Exit condition:** `buf lint` passes; `SetArmedRequest.force` is gone.

---

### Tier 1 — Pure Domain Logic [parallel ok: Go + TS simultaneously]

*No sockets. No goroutines. No Redis. No Docker.*

**Go: MAVLink codec via gomavlib** (`internal/codec/`)

- `frame.go` — wrap gomavlib's `FrameDecoder`; output `Frame{Version, SysID, CompID, MsgID, Seq, Payload}`. Do not re-implement framing.
- `message.go` — dispatch on `MsgID` → deserialize into typed struct via gomavlib; map to `TelemetryEvent` proto oneof variant.
- Priority receive set (15 families): `HEARTBEAT`, `ATTITUDE`, `GLOBAL_POSITION_INT`, `VFR_HUD`, `GPS_RAW_INT`, `SYS_STATUS`, `RADIO_STATUS`, `STATUSTEXT`, `MISSION_CURRENT`, `COMMAND_ACK`, `PARAM_VALUE`, `HOME_POSITION`, `NAV_CONTROLLER_OUTPUT`, `MISSION_COUNT`, `MISSION_ITEM_INT`.
- Priority send set (10 families, also via gomavlib marshal): `COMMAND_LONG`, `SET_POSITION_TARGET_GLOBAL_INT`, `PARAM_SET`, `PARAM_REQUEST_LIST`, `PARAM_REQUEST_READ`, `MISSION_COUNT`, `MISSION_ITEM_INT`, `MISSION_REQUEST_INT`, `MISSION_ACK`, `HEARTBEAT`.

**TypeScript: pure resolvers** (`frontend/src/logic/`)

- `attitude.ts` — `resolveAttitude(event) → {pitchDeg, rollDeg, source}`
- `heading.ts` — fallback chain VFR_HUD.heading → ATTITUDE.yaw → GLOBAL_POSITION_INT.hdg; reject 65535; `isFallback` on output
- `flightPath.ts` — `resolveFlightPath2d(event) → {trackDeg, groundSpeedMps, climbMps, fpaRad}`; `climbMps = -(vzCms / 100)` (NED sign flip, ADR-0005)
- `trajectory.ts` — body-rate Euler: `ψ̇ = (sin φ·q + cos φ·r)/cos θ`; stall gate below 14 m/s; 5s horizon; 10 points
- `position.ts` — GPS primary; velocity-integration fallback
- `track.ts` — `accumulateTrack(prev, event) → ENU[]`; ring buffer; ENU projection
- `freshness.ts` — `isFresh(lastSeenMs, ttlMs) → boolean`

Write `sampleFromEvent(e: TelemetryEvent): TelemetrySample` shim — all resolver logic operates on the flat shape; only the shim knows proto structure.

**Exit conditions:**
- `go test ./internal/codec/...` — no sockets, no goroutines, passes with golden byte vectors
- `vitest ./frontend/src/logic/...` — known-answer vectors match; NaN never returned from any resolver
- Every resolver output carries a `source` field naming the MAVLink message used
- `go test -race ./internal/codec/...` passes

---

### Tier 2 — Parity Apparatus + Test Vectors [parallel ok with Tier 1]

Establish the comparison apparatus. This is the harness that proves Tier 1 is
correct and measures everything built afterward.

**Go capability matrix** (`docs/wip/codec-capability-matrix.md`):
- One row per normalized message family (15 receive + 10 send)
- One row per framing case (v1 accept, unsigned-v2 accept, bad CRC reject, truncation reject)
- Columns: `Case ID | Behavior | Evidence artifact | Status | Notes`
- Copy golden byte fixtures from `flight-path-hud/contracts/mavlink/` → `contracts/mavlink/` in this repo

**TS known-answer vectors:**
- Each resolver: table of `{input, expectedOutput}` pairs
- Inputs from SITL captures; outputs hand-computed
- Reference named fixture files, not inline magic numbers

**Exit conditions:**
- Matrix exists with all case IDs; none `unstarted`
- TS resolver tests all reference named fixtures
- `go test -race ./internal/codec/...` passes (Tier 1 is race-free)

---

### Tier 3 — Codegen Infrastructure

Wire protobuf codegen so generated stubs are available for all subsequent tiers.

**`proto/buf.gen.yaml`:**
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

Commit generated files. Use `buf generate` as a Makefile target.
Wire `rules_buf` into Bazel `genrule` after Bzlmod support proves stable.
Do not let codegen tooling block Tier 4+.

**Exit conditions:**
- `buf generate` produces Go stubs in `internal/gen/gcs/v1/`
- `buf generate` produces TS client in `frontend/src/gen/gcs/v1/`
- `buf lint` passes
- `buf breaking --against HEAD~1` wired as CI gate

---

### Tier 4 — Bridge Core (pure fold, injected clock)

The vehicle model as a **pure fold** — no goroutines, no Redis, no sockets.

**Docker Compose scaffolding [parallel ok — prepare while Tier 4 pure code is written]:**

Prepare `docker-compose.yml` while pure fold code is being written.
It does not need to work until Tier 5's gate.

```
ardupilot-sitl-1   ArduCopter 4.6.x; SYSID_THISMAV=1; UDP:14550→host; TCP:5760
ardupilot-sitl-2   ArduCopter 4.6.x; SYSID_THISMAV=2; UDP:14560→host; TCP:5761  (profile: multi-sitl)
ardupilot-sitl-3   ArduCopter 4.6.x; SYSID_THISMAV=3; UDP:14570→host; TCP:5762  (profile: multi-sitl)
redis              :6379; AOF persistence enabled; no auth in dev
gcs-backend        :8080 (HTTP plain); env: GCS_MAVLINK_UDP_BIND=0.0.0.0:14550
gcs-frontend       Vite :3000; proxies /api → backend :8080
nginx              TLS termination on :443; reverse proxy to :8080 (satisfies ADR-0005)
```

SITL instances 2 and 3 are in a Docker Compose profile (`--profile multi-sitl`).
Default `docker-compose up` starts only SITL-1. Multi-vehicle validation uses the profile.

**`internal/vehicle/fold.go`:**
- `Fold(state VehicleState, msg Message, nowMs int64) (VehicleState, []Event)`
- `VehicleState`: current `VehicleSnapshot` proto fields, health counters, per-message-family rate buckets, last-seen timestamps per sysId
- `HeartbeatState` extraction; armed/mode flag expansion
- TTL expiry: `nowMs - lastSeenMs > 60000` → emit `FleetEvent{VEHICLE_LOST}`
- Source-conflict detection: same sysId from two different adapters emits warning event

**`internal/routes/table.go`:**
- Exact `sysId:compId` route table with explicit expiry timestamps
- `Lookup(sysId, compId uint8, nowMs int64) (Route, bool)`
- Expiry evicted on lookup; no background goroutine

**Exit conditions:**
- Pure fold tested with fixed-clock inputs; `time.Now()` never called in fold or route code
- `go test -race ./internal/vehicle/... ./internal/routes/...` passes
- Sequence wrap/fallback tested; source-conflict emits exactly one warning, not a panic
- Docker Compose file exists and `docker-compose config` validates (services don't need to start yet)

---

### Tier 5 — Transport Adapter + Live Bridge

Connect the pure fold to a live UDP socket. Wire Redis. Wire GCS heartbeat.
First goroutines in the codebase.

**`internal/transport/udp.go`:**
- `net.PacketConn` on `0.0.0.0:14550` (configurable via `GCS_MAVLINK_UDP_BIND`)
- Read loop → `chan []byte` (buffer: 512); write path: `chan []byte` (buffer: 64) → `WriteTo`
- Context cancellation triggers close
- Byte slice ownership: copy before retaining past buffer lifetime
- Setting `GCS_MAVLINK_UDP_BIND=""` skips socket creation (test mode)

**GCS heartbeat goroutine** (`internal/vehicle/heartbeat.go`):
- Launched by vehicle model once first `HEARTBEAT` is received from a vehicle
- Ticks at 1 Hz; encodes `HEARTBEAT` via gomavlib (sysId=255, compId=190, type=GCS, autopilot=GENERIC)
- Pushes encoded bytes into transport write channel
- Goroutine exits when vehicle goroutine's context is cancelled
- **This is not optional. Without it, ArduPilot guided commands time out silently.**

**`internal/redis/client.go`:**
- `Publish(channel, proto.Message)` — Pub/Sub
- `Subscribe(channel) chan proto.Message` — Pub/Sub
- `XAdd(stream, fields map[string]string)` — Stream append with MAXLEN enforcement
- `XREADGROUP` setup for consumer group `gcs-backend` on all Streams at init
- `HSet(key, proto.Message)`, `HGet(key) proto.Message`

**Redis key schema (authoritative):**
```
fleet:events                    XADD (Stream); MAXLEN ~ 10000; XREADGROUP group=gcs-backend
telemetry:<sysId>               PUBLISH (Pub/Sub); ephemeral; no persistence needed
track:events                    XADD (Stream); MAXLEN ~ 10000; XREADGROUP group=gcs-backend
audit:events                    XADD (Stream); MAXLEN ~ 50000; XREADGROUP group=gcs-backend
vehicle:state:<sysId>           HSET; rolling EXPIRE 300s on each heartbeat
params:<sysId>:<param_id>       HSET; EXPIRE 3600s
```

**Goroutine supervision** (`cmd/gcs/main.go`):
- Pipeline: transport → frame codec → message codec → router → vehicle fold → Redis
- Each boundary: typed channel with explicit buffer size
- `errgroup` for coordinated shutdown; zero goroutine leaks past context cancellation
- `/healthz` extended to return: `{"redis":"ok","sitl_link":"up|down","last_heartbeat_ms":N}`

**Exit conditions:**
- `docker-compose up` → SITL-1 heartbeat visible in backend logs with `VEHICLE_DISCOVERED`
- `redis-cli SUBSCRIBE telemetry:1` shows live `TelemetryEvent` proto bytes
- `redis-cli XREAD COUNT 10 STREAMS fleet:events 0` shows `VEHICLE_DISCOVERED` event
- GCS heartbeat confirmed transmitted: ArduPilot SITL accepts a guided command without GCS failsafe (send CMD 400 without heartbeat first → confirm FS triggers; add heartbeat → confirm FS does not trigger)
- `go test -race ./...` passes
- Goroutine count stable over 60-second SITL run (no unbounded growth)

---

### Tier 6 — First Connect Services + Track Layer + Telemetry Log

Parallel tracks: Go services and frontend adapter/log can proceed simultaneously
once Tier 5 gate is closed.

**Go: services** (`internal/services/`):

- `FleetService.WatchFleet` — XREADGROUP `fleet:events`, stream `FleetEvent` protos
- `FleetService.ListVehicles` — HGETALL `vehicle:state:*` → `ListVehiclesResponse`
- `TelemetryService.StreamTelemetry` — Subscribe `telemetry:<sysId>`, stream `TelemetryEvent`
- `TelemetryService.GetSnapshot` — HGET `vehicle:state:<sysId>` → `VehicleSnapshot`

**Go: track layer** (`internal/adapters/track.go`):

Wire MAVLink track path now. ADS-B and Meshtastic adapters land in Tier 9.
- Normalize each `VehicleSnapshot` → `Track` proto with `MavlinkDetail`
- `XADD track:events` on every snapshot update
- `TrackService.WatchTracks` — XREADGROUP `track:events`, stream `TrackEvent` protos

Track layer is here (not Tier 9) because `MapPanel` is needed in Tier 8 to
validate guided reposition. Without the track layer, there is no map.

Wire Connect server into `cmd/gcs/main.go` using Tier 3 generated stubs.

**Frontend: adapter context + telemetry log component:**

```
frontend/src/adapters/
  types.ts         GcsAdapter interface
  connect.ts       ConnectAdapter — wraps generated @connectrpc/connect-web client
  mock.ts          MockAdapter — static TelemetryEvent fixtures; no network; MSW not needed yet
  context.tsx      AdapterProvider + useAdapter() hook
```

`frontend/src/components/debug/TelemetryLog.tsx`:
- Receives `TelemetryEvent` stream via `useVehicleTelemetry(vehicleId)`
- Flat table: `variable | value | unit | source | age`
- Source color: primary (green), fallback (yellow), missing (grey)
- Freshness indicator: last-seen delta in ms
- No SVG. No math. Just the data.

**Exit conditions:**
- Browser shows live telemetry log for ArduCopter SITL
- Each row names its MAVLink source message
- Freshness goes grey when SITL is stopped
- MockAdapter renders the same component with fixture data (no network)
- `TrackEvent` stream includes MAVLink vehicle tracks
- Map-layer consumer can render without importing any MAVLink package

---

### Tier 7 — Read Protocol Transactions

Read before write. Prove parameter and mission read as pure transaction folds;
connect to live SITL; build UI display. No writes yet.

**Go: `internal/transactions/parameter.go`** (pure):
- `ParameterReadFold(state, event, nowMs) (state, []Event)`
- `ParameterListFold(state, event, nowMs) (state, []Event)`
- Handles: out-of-order `PARAM_VALUE`, duplicates, gap detection via bitmask,
  retry on quiescence (500ms no new params), route-loss failure
- GCS identity (255:190) is explicit constructor input, not env var
- Request encoding via gomavlib `PARAM_REQUEST_LIST` and `PARAM_REQUEST_READ`

**Go: `internal/transactions/mission.go`** (pure):
- `MISSION_REQUEST_LIST` → `MISSION_COUNT` → `MISSION_REQUEST_INT` × n → `MISSION_ACK`
- Per-item retry with 1s timeout (QGroundControl `MissionManager.cc` pattern)
- Gap detection: track received item indices; re-request missing after quiescence

**Services:**
- `ParameterService.ListParameters` and `GetParameter` — stream via `params:<sysId>:<param_id>` channels
- `MissionService.DownloadMission` — stream `MissionItem` protos

**Frontend:**
- `ParametersPanel` (read-only initially): searchable list, type-aware value display
- `MissionPanel`: seq, command, position; waypoints overlaid on map

**Exit conditions (parameters):**
- Capability matrix rows: PARAM-VEC-NAME, PARAM-VEC-INDEX, PARAM-VEC-LIST,
  PARAM-READ-NAME, PARAM-READ-INDEX, PARAM-LIST-COMPLETE, PARAM-LIST-IDLE,
  PARAM-ROUTE-LOSS, PARAM-REPLAY, PARAM-INVALID-TARGET — all `complete`
- SITL: `ARMING_CHECK` readable from live ArduCopter
- Replay of recorded parameter session is deterministic; produces no outbound bytes

**Exit conditions (mission download):**
- SITL: 5-waypoint QGC mission → ligma-gcs download → identical item list
- Round-trip: download → JSON → compare with QGC `.plan` format

---

### Tier 8 — Write Protocol Transactions (ordered by operational risk)

Before writing any Tier 8 code, wire the command registry. This is infrastructure,
not a service — it exists in `internal/command/registry.go` and is shared by
all write handlers.

**Command registry (first, before any write handlers):**
- In-flight map keyed by `commandKey{targetSysID, targetCompID, commandID}`
- Register before encoding + sending; deregister on COMMAND_ACK or 5s timeout
- Duplicate key (same vehicle + same command already in-flight) → return error to service
- `errgroup` with per-entry `context.WithTimeout(5 * time.Second)`

**Dual-gate contract (all writes):**
- Service-side gate: role ≥ `OPERATOR` from `OperatorContext` (JWT claim in dev; NoopAuth returns OPERATOR always in SITL mode)
- Service-side gate: vehicle eligible state (checked against `VehicleSnapshot`)
- UI-side gate: button disabled when eligibility conditions unmet
- Zero force-arm: `force` field removed from proto (Tier 0); not parameterizable
- Audit: `AuditEvent` XADD to `audit:events` after action; never before; never blocking

**8a. Message interval** (lowest risk — MAVLink #511):
- Outbound `MAV_CMD_SET_MESSAGE_INTERVAL` via COMMAND_LONG
- Reversible; affects only telemetry rate; no vehicle state change
- `CommandService.SetMessageInterval`

**8b. Parameter write** (low consequence, disarmed gate):
- `PARAM_SET` → wait for `PARAM_VALUE` echo with same `param_id` (not by ACK — PARAM_SET uses echo-back not COMMAND_ACK)
- Start with one low-consequence parameter (`LOG_BITMASK`)
- Gate: disarmed + operator role
- Extend `ParameterService.SetParameter`; read-back verification; audit log
- Extend `ParametersPanel` with write mode

**8c. Mission upload**:
- Client streams `MissionItem` protos; `MissionService.UploadMission`
- Handshake: `MISSION_COUNT` → `MISSION_REQUEST_INT` × n (vehicle drives item requests) → `MISSION_ACK`
- Per-item retry: 1s timeout per `MISSION_REQUEST_INT`; 3 retries before abort
- Round-trip test: upload → download → assert equal item list
- Reference: ADR-0030 (UDP replies routed by MAVLink system, not sender recency)

**8d. Mode change** (disarmed-only gate):
- Vehicle-type allowlist (ArduCopter and ArduPlane have different valid mode sets)
- ACK via command registry; `HEARTBEAT` post-condition verification
- `CommandService.SetMode`; gate: disarmed + operator role + mode in allowlist

**8e. Arm / Disarm**:
- `COMMAND_LONG` CMD 400 param1=1 param2=0 (no force arm; field removed from proto)
- Disarm: CMD 400 param1=0
- Gate for arm: mode is `GUIDED`, not already armed, operator role
- Exactly once per user action; no automatic retry; user must confirm failure
- Post-condition: `HEARTBEAT.base_mode & MAV_MODE_FLAG_SAFETY_ARMED` observed
- `CommandService.SetArmed`

**Map must be working before 8f. Build MapPanel during 8d/8e:**
- `MapPanel.tsx` — MapLibre GL; vehicle markers; track polyline from `TrackService.WatchTracks`
- `FleetView.tsx` — roster + basemap; vehicle cards color-coded by freshness
- The map is needed to validate 8f; build it here, not in Tier 10

**8f. Guided reposition** (highest risk):
- `SET_POSITION_TARGET_GLOBAL_INT` (#86): `type_mask=0b0000111111111000`
- **Server-side bounds validation** (not just UI): target >500m from home rejected at service layer; altitude <2m AGL rejected at service layer. UI gate alone is bypassable by direct Connect call.
- Gate: armed + GUIDED mode + operator role
- Read-back: poll `VehicleSnapshot.position` until within 2m or timeout (30s)
- Golden byte vectors from `flight-path-hud/contracts/mavlink/`
- `CommandService.SetPositionTargetGlobal`

**8g. Guided workflow lifecycle** (takeoff, land):
- ARM → GUIDED → TAKEOFF → (fly) → LAND → DISARM ordered state machine
- Port `flight-path-hud/packages/gcs-core/src/guidedWorkflow.ts` as pure fold
- Lifecycle semantic trace from `flight-path-hud/contracts/semantics/`
- `GuidedWorkflowPanel.tsx`; `GuidedRepositionPanel.tsx`

**Exit conditions per write family:**
- Pure transaction fold unit tested with ordered semantic trace
- Golden MAVLink byte vector matches SITL capture
- SITL acceptance test passes (Copter + Plane where applicable)
- Replay of recorded session produces no outbound bytes
- AuditEvent written to Redis Stream with operator identity
- Guided reposition: arm copter → switch GUIDED → map click → copter moves to point

---

### Tier 9 — Additional Protocol Adapters

MAVLink track layer is already live (Tier 6). This tier adds the remaining adapters.

**ADS-B** (when hardware available):
- Source 1: MAVLink `ADSB_VEHICLE` (#246) from onboard detect-and-avoid
- Source 2: `dump1090`/Beast over TCP from ground-side receiver
- Both normalize to `Track` with `AdsbDetail`

**Meshtastic** (when hardware available):
- Demultiplex one MeshPacket connection into three paths:
  - PortNum 1 → `ChatService`
  - PortNum 3 → `TrackService` (position)
  - PortNum N (custom/MAVLink) → MAVLink transport adapter
- `Track` with `MeshtasticDetail`; `is_mavlink_relay` flag for relay nodes

**Exit conditions:**
- Protocol-agnostic map renders all track sources without importing MAVLink packages
- Adding a new protocol = new adapter + new proto detail variant; zero map code changes

---

### Tier 10 — UI Composition (deferred)

Instrument composition after telemetry log establishes variable trust.
Module-first: smallest trusted atomic unit, then instruments, then composed layouts.

See **UI Information Architecture** section below.

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
| `FlightStateDisplay` | `resolveFlightPath2d` | NED sign flip tested |
| `PredictiveTrajectory` | `resolvePredictiveTrajectory` | CTRV model + stall gate tested |
| `FlightPathRecorder` | `accumulateTrack` | ENU projection tested |
| `AltitudeTape` | `resolvePosition.altM` | Position resolver with fallback tested |

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
| gRPC-native browser transport | ADR-0035 explicitly defers; Connect serves as bridge |
| Meshtastic relay implementation | Adapter boundary defined (ADR-0004); waits on Tier 9 + hardware |
| mTLS between services | TLS termination at proxy satisfies ADR-0005 for now; mTLS is a hardening step |
