# Order of Operations: ligma-gcs Buildout
**Iteration 3 — Fact-checked against MAVLink spec, ArduPilot source, gomavlib API, and current proto files**

---

## What This Document Is

A forward-looking build order. Derived from:
1. Walking the `flight-path-hud` commit history and Go bridge handoff doc
2. Adversarial review: MAVLink gaps, Redis schema, safety concerns
3. Block-sequencing review
4. Independent adversarial review of all three
5. **Iteration 3 additions:** line-by-line fact-check against MAVLink common.xml / ardupilotmega.xml, ArduPilot SITL source, gomavlib v3 API, Redis documentation, and current proto files in `proto/gcs/v1/`

`port-plan-v1.md` in this directory maps what to port and where it lands.
This document answers *in what order* to build, *why*, and *what must be true before moving on*.

ADRs are law. If this document contradicts an ADR, the ADR wins.

---

## Adversarial Findings: What Prior Drafts Got Wrong

These are correctness issues that would cause silent failures,
blocked tiers, or re-work if not resolved before code is written.

### 1. GCS heartbeat is missing and its absence is a production failure mode

Neither the v1 order-of-operations nor the port plan mentions this. ArduPilot
`FS_GCS_ENABLE=1` (enabled by default in Copter 4.x) triggers a failsafe if
no GCS HEARTBEAT arrives for ~5 seconds. The GCS must send `HEARTBEAT`
at 1 Hz from the moment a vehicle is discovered.

**Wire encoding for GCS HEARTBEAT:**
```
type            = 6     (MAV_TYPE_GCS)
autopilot       = 0     (MAV_AUTOPILOT_GENERIC)
base_mode       = 0
custom_mode     = 0
system_status   = 4     (MAV_STATE_ACTIVE)
mavlink_version = 3     (set automatically by gomavlib)
```
sysId=255, compId=190 in the frame header. All six fields must be set correctly;
an incomplete HEARTBEAT (e.g., system_status=0/UNINIT) may not reset the failsafe
timer depending on firmware version.

This is not optional. Guided reposition commands will succeed then silently
time out if this is absent. Add it to Tier 5 (transport live) — not the
transport layer (which is a byte pipe), but a dedicated goroutine in
the vehicle model that ticks at 1 Hz and pushes outbound bytes into the
transport write channel. Guard startup with `sync.Once` — two rapid-arrival
HEARTBEATs from the same vehicle must not spawn two goroutines.

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
- `HEARTBEAT` (#0) — GCS presence keepalive
- `MAV_CMD_SET_MESSAGE_INTERVAL` (#511 as command ID in COMMAND_LONG) — telemetry rate control

That is 11 outbound message types, each needing correct little-endian packing,
CRC_EXTRA computation, and sequence management. Hand-rolling encoding for all
of these with correct CRC_EXTRA seeds is weeks of work and a source of
hard-to-debug bugs (wrong byte order is CRC-silent until SITL rejects it).

**Use `gomavlib` (github.com/bluenviron/gomavlib/v3)** with the `ardupilotmega` dialect.
The ardupilotmega dialect includes all common messages as a superset. It handles:
- CRC_EXTRA automatically per message ID
- Correct little-endian struct layout
- MAVLink v1/v2 framing for both read and write
- All priority receive message families
- All outbound message families above

**gomavlib v3 API shape (do not fabricate type names):**
```go
import (
    "github.com/bluenviron/gomavlib/v3"
    "github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
    "github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
)

node, err := gomavlib.NewNode(gomavlib.NodeConf{
    Endpoints:     []gomavlib.EndpointConf{...},
    Dialect:       ardupilotmega.Dialect,
    OutVersion:    gomavlib.V2,
    OutSystemID:   255,
    OutComponentID: 190,
})

// Receive loop
for evt := range node.Events() {
    switch e := evt.(type) {
    case *gomavlib.EventFrame:
        switch msg := e.Message().(type) {
        case *common.MessageHeartbeat:
            // ...
        case *common.MessageCommandAck:  // NOT "mavlink2.COMMAND_ACK"
            // ...
        case *ardupilotmega.MessageEkfStatusReport:
            // ardupilot-dialect messages live in ardupilotmega package
        }
    }
}
```

The "minimal dependency surface" argument from v1 is wrong here. A codec
library is load-bearing infrastructure, not an optional convenience. The risk
of an incorrect hand-rolled encoder is higher than the risk of a maintained
library dependency.

**Retained from v1:** golden byte test vectors from `flight-path-hud/contracts/mavlink/`
still validate the codec end-to-end regardless of which implementation is used.

### 3. ACK correlation needs a concrete design before Tier 8 code is written

The port plan says "wait for COMMAND_ACK" as if ACKs are trivially matchable.
They are not. `COMMAND_ACK` (#77) contains only:
- `command` (uint16): the command ID that was ACKed
- `result` (uint8): MAV_RESULT
- `target_system`, `target_component`: who it's for (MAVLink 2 extensions)

There is no request correlation ID. With concurrent commands (arm + mode
change), `COMMAND_ACK` for `MAV_CMD_COMPONENT_ARM_DISARM` is ambiguous if
two are in-flight.

**Design: in-flight command registry**

```go
type inFlightCommand struct {
    ch      chan *common.MessageCommandAck  // gomavlib type, not "mavlink2.COMMAND_ACK"
    cancel  context.CancelFunc
}

type commandRegistry struct {
    mu  sync.Mutex
    reg map[commandKey]*inFlightCommand
}

type commandKey struct {
    vehicleSysID  uint8  // MAVLink frame source sysId (vehicle that will ACK)
    vehicleCompID uint8  // MAVLink frame source compId
    commandID     uint16 // MAV_CMD value
}
```

Register before send; deregister on ACK or 5s timeout. Matching uses the
MAVLink frame header's sysId/compId (sender of the ACK) plus the command ID
from the ACK message body. If a duplicate key arrives (same vehicle + same
command already in-flight), reject the second call at the service layer.
Per-command timeouts use `context.WithTimeout` on the registry entry.

### 4. Redis schema requires TTLs, MAXLEN, and throttled EXPIRE from the start

Not retroactive — set these when the keys are first written.

**Problem with the v1 schema:** `params:<sysId>:<param_id>` as separate HSET keys
creates 500+ Redis keys per vehicle (ArduCopter has ~500 parameters). Each
key needs its own `EXPIRE` call, meaning a full parameter download triggers
500+ `EXPIRE` calls. This is both slow and wasteful.

**Corrected param schema:** one hash per vehicle.

```
Key                          Type    Policy
---                          ----    ------
vehicle:state:<sysId>        HASH    EXPIRE 300s, throttled to once per 30s per vehicle
fleet:events                 STREAM  XADD MAXLEN ~ 10000; XREADGROUP group=gcs-backend
telemetry:<sysId>            PUBSUB  PUBLISH (ephemeral); no persistence
track:events                 STREAM  XADD MAXLEN ~ 10000; XREADGROUP group=gcs-backend
audit:events                 STREAM  XADD MAXLEN ~ 50000; XREADGROUP group=gcs-backend
params:<sysId>               HASH    HSET params:<sysId> <param_id> <value> ...
                                     Single EXPIRE 3600s on the entire hash after list completes
track:state:<nodeId>         HASH    HSET; EXPIRE 300s rolling
```

**EXPIRE throttling for vehicle:state:** calling `EXPIRE vehicle:state:<id> 300`
on every heartbeat at 1 Hz generates one Redis round-trip per second per vehicle.
Throttle: record `lastExpireMs` per vehicle; only call EXPIRE if
`nowMs - lastExpireMs > 30000`. On VEHICLE_LOST, call EXPIRE 60s explicitly for
the drain window.

**Consumer group initialization** (not optional — must happen at service startup
before any XREADGROUP call, or the call returns NOGROUP error):
```
XGROUP CREATE fleet:events  gcs-backend $ MKSTREAM
XGROUP CREATE track:events  gcs-backend $ MKSTREAM
XGROUP CREATE audit:events  gcs-backend $ MKSTREAM
```
`MKSTREAM` creates the stream if it doesn't exist. Use `XGROUP CREATECONSUMER`
to pre-register the consumer if desired. Alternatively, catch NOGROUP error and
create on demand — but do this in the Redis client init, not per-request.

### 5. The track layer must be scaffolded before Tier 8, not after

v1 places the track layer in Tier 9 (after all write transactions). The problem:
guided reposition (Tier 8f) requires the operator to click a position on the map.
The map is `MapPanel` which depends on `TrackService.WatchTracks` which depends
on the track layer. You cannot validate guided reposition without the map.

Move track layer scaffolding to Tier 6 alongside the first Connect services.
ADS-B and Meshtastic adapters remain deferred (Tier 9). The MAVLink-sourced
track path (`VehicleSnapshot` → `Track` → `track:events`) ships in Tier 6.

### 6. TLS is in ADR-0005 but missing from this document

ADR-0005 says TLS is on the Connect endpoint. Resolution: the Connect server
is TLS-terminated by a reverse proxy (nginx or Caddy in Docker Compose) from
Tier 5. The backend itself listens on HTTP/1.1 only. This is a one-day Docker
Compose addition that satisfies ADR-0005 without touching Go service code.

### 7. ConnectLink auto-bind vs. operator-initiated is unresolved

Resolution (explicit, authoritative):
- The backend **auto-binds `UDP:14550` at startup** and begins accepting SITL
  traffic immediately. No user action required. This is the primary operational path.
- `FleetService.ConnectLink` is for **runtime-added supplementary links** only
  (e.g., a secondary radio link on a different port, or a TCP serial bridge).
- The auto-bind config lives in `cmd/gcs/main.go` as an environment variable
  (`GCS_MAVLINK_UDP_BIND`, default `0.0.0.0:14550`).
- Setting `GCS_MAVLINK_UDP_BIND=""` disables auto-bind (useful for tests that
  don't want a real socket).

### 8. The second agent's block sequencing is too waterfall

Pure Go codec and pure TS resolvers are **fully independent**. Both can start
at Tier 1 simultaneously. Docker Compose scaffolding can be prepared in parallel
with Tier 1-4 pure work — it just doesn't need to work until Tier 5's gate.

### 9. Multi-SITL Docker Compose from Tier 4 — routing must be explicit

Both earlier documents defer multi-SITL and leave port routing ambiguous.
This creates a painful retrofit and a specific implementation error: the v1
compose table assigns UDP:14560 and UDP:14570 to instances 2 and 3, but
the backend single-socket binds only 14550. If instances 2/3 send to different
ports, the single socket never receives them.

**Correct multi-SITL routing:** all three instances are configured to send their
MAVLink output to `gcs-backend:14550`. Each has a distinct `SYSID_THISMAV`
(1, 2, 3). The single GCS socket receives frames from all three, discriminated
by sysId in the MAVLink header. The backend router tracks
`(sysId → (srcIP, srcPort))` from the first received UDP packet to know where
to send commands back to each vehicle.

The Docker Compose port notation `UDP:14560→host` and `UDP:14570→host` means
**host-published ports for external tooling access** (mavproxy, mavlogdump,
debugging from the host). They are not the GCS receive ports.

**ArduPilot SITL Docker image strategy:** `ardupilot/ardupilot-dev-jammy` is
a _development environment_, not a pre-built SITL binary. The compose file
must either:
1. Use a multi-stage Dockerfile: build stage clones ArduPilot at a pinned tag,
   builds `ArduCopter.elf` via `waf`, copies the binary to a minimal runtime image.
   Build is slow (10–20 min) but produces a reproducible image. Cache the
   build layer.
2. Or download a pre-built SITL binary from ArduPilot's CI artifacts for the
   pinned commit SHA and package it in a minimal image (Ubuntu 22.04 + libraries).

Either way, pin the ArduPilot commit SHA in the Dockerfile, not a floating
branch tag. `ArduCopter 4.6.x` is not a Docker tag — it needs to be a specific
commit or release tag from https://github.com/ArduPilot/ardupilot.

SITL launch flags per instance:
```
ArduCopter -S --model=+ --speedup=1 \
    --sysid=<N> \
    --out=udp:gcs-backend:14550 \
    --home=<lat,lon,alt,hdg>
```
`--out=udp:gcs-backend:14550` routes all three instances to the same GCS socket.

### 10. `force` field should not exist in the proto

`SetArmedRequest.force` (field 3 in `proto/gcs/v1/commands.proto`) must be
removed. It is currently present. A field that exists can be set by callers
who bypass the documented "always 0" convention. The service cannot enforce a
proto field to a fixed value — it can only refuse requests that set it to 1.
Removing the field is the only safe choice. This requires a proto edit before
Tier 3 codegen.

### 11. Priority receive set is incomplete

Tier 1 previously listed 15 families. The current proto (`telemetry.proto`)
defines 16 message types in `TelemetryEvent.oneof`, of which 17 MAVLink
message families need to be decoded to populate them:

| # | MAVLink message | ID | Dialect |
|---|---|---|---|
| 1 | HEARTBEAT | 0 | common |
| 2 | SYS_STATUS | 1 | common |
| 3 | PARAM_VALUE | 22 | common |
| 4 | GPS_RAW_INT | 24 | common |
| 5 | ATTITUDE | 30 | common |
| 6 | GLOBAL_POSITION_INT | 33 | common |
| 7 | MISSION_CURRENT | 42 | common |
| 8 | MISSION_COUNT | 44 | common |
| 9 | MISSION_ACK | 47 | common |
| 10 | MISSION_ITEM_INT | 73 | common |
| 11 | VFR_HUD | 74 | common |
| 12 | COMMAND_ACK | 77 | common |
| 13 | RADIO_STATUS | 109 | common |
| 14 | BATTERY_STATUS | 147 | common |
| 15 | HOME_POSITION | 242 | common |
| 16 | STATUSTEXT | 253 | common |
| 17 | NAV_CONTROLLER_OUTPUT | 62 | common |
| 18 | EKF_STATUS_REPORT | 193 | ardupilotmega |

`BATTERY_STATUS (#147)` and `EKF_STATUS_REPORT (#193)` were absent from the v2
list but are defined in the proto and essential for operations:
- `BATTERY_STATUS` is the preferred battery source in ArduPilot 4.x; the
  legacy `SYS_STATUS` battery fields are still populated but per-cell voltages
  and multi-battery support require `BATTERY_STATUS`.
- `EKF_STATUS_REPORT` EKF_UNINITIALIZED (bit 10) and EKF_ATTITUDE (bit 0) are
  needed to gate the arm button — a vehicle that hasn't completed EKF
  initialization should not be armable from the GCS.

`MISSION_ACK` is also added as a receive message (needed for mission upload
confirmation, separate from encoding it as outbound).

### 12. `type_mask` for SET_POSITION_TARGET_GLOBAL_INT

The v2 document states `type_mask=0b0000111111111000`. This includes bit 9
(`POSITION_TARGET_TYPEMASK_FORCE_SET = 512`), which means "interpret af* fields
as force (Newtons) not acceleration (m/s²)." Since bits 6–8 (ignore afx/afy/afz)
are also set, it has no observable effect in ArduPilot — but it's incorrect
documentation for "position-only."

**Correct position-only mask:**
```
Bits set (= fields to ignore):
  bit 3  (8):    IGNORE VX
  bit 4  (16):   IGNORE VY
  bit 5  (32):   IGNORE VZ
  bit 6  (64):   IGNORE AFX
  bit 7  (128):  IGNORE AFY
  bit 8  (256):  IGNORE AFZ
  bit 10 (1024): IGNORE YAW
  bit 11 (2048): IGNORE YAW_RATE

Sum = 8+16+32+64+128+256+1024+2048 = 3576 = 0xDF8 = 0b110111111000
```
Bit 9 (FORCE_SET) is NOT set. Named constant: `POSITION_ONLY_TYPE_MASK = 0xDF8`.

### 13. WatchFleet bootstrap requires HSET scan + synthetic events

`services.proto` `WatchFleet` docstring says "immediately emits VEHICLE_DISCOVERED
for each currently known vehicle so clients bootstrap fleet state without a separate
list call." This cannot be implemented by reading the `fleet:events` Stream from
the beginning (would replay all historical joins/losses).

**Correct implementation:**
1. `HGETALL vehicle:state:*` (scan pattern or maintain an index set `fleet:active`) → emit synthetic `FLEET_EVENT_TYPE_VEHICLE_DISCOVERED` for each live vehicle
2. Switch to `XREADGROUP fleet:events gcs-backend <consumer> > COUNT 100` for live events
3. The switch must be atomic from the client's perspective — buffer stream events
   received during the HGETALL scan and deliver them after the synthetic batch

Maintain a Redis SET `fleet:active` (SADD/SREM on VEHICLE_DISCOVERED/VEHICLE_LOST)
to avoid a SCAN over vehicle:state:* keys. The `HGETALL` pattern for
`vehicle:state:*` requires SCAN which is O(N) over all keys. An explicit
membership set is O(1).

### 14. Stall gate in trajectory resolver must be vehicle-type-conditional

The trajectory resolver applies a stall gate below 14 m/s:
`forward_progress = max(0, speed - stall_speed)`. This is the correct model for
fixed-wing aircraft. It must NOT be applied to multicopters.

Multicopters (MAV_TYPE_QUADROTOR = 2, MAV_TYPE_HEXAROTOR = 13, etc.) have no
aerodynamic stall speed. Applying the gate to a copter clips forward trajectory
prediction to zero whenever ground speed < 14 m/s — which is most of the
hover/low-speed envelope.

The resolver must accept `vehicleType: MavType` and branch:
```typescript
const stallSpeed = isFixedWing(vehicleType) ? 14 : 0;
const forwardProgress = Math.max(0, speed - stallSpeed);
```

`isFixedWing` should match MAV_TYPE_FIXED_WING (1), MAV_TYPE_VTOL_* family,
and explicitly exclude MAV_TYPE_QUADROTOR, MAV_TYPE_HEXAROTOR,
MAV_TYPE_OCTOROTOR, MAV_TYPE_TRICOPTER, MAV_TYPE_COAXIAL.

---

## Resolved Design Decisions

These are now closed. Do not re-open without an ADR.

| Decision | Resolution |
|---|---|
| gomavlib vs hand-roll | **gomavlib v3** with ardupilotmega.Dialect (superset of common). Import `github.com/bluenviron/gomavlib/v3`. |
| Map tile provider | MapLibre GL + PMTiles (self-hosted). No API key. Field deployment = local mount. |
| Phase ordering: params vs commands | Params (read) before commands (write). Read-only proof before any writes. |
| Storybook vs Vite MSW | Vitest for pure logic. MSW in Tier 6 when ConnectAdapter needs a mock server. Storybook deferred to Tier 10 or beyond. |
| rules_buf vs buf generate | Commit generated files. Use `buf generate` as a Makefile target. Wire `rules_buf` into Bazel after Bzlmod support stabilizes. |
| Single socket vs per-vehicle sockets | **Single socket** at `0.0.0.0:14550`. Router dispatches by sysId. Multi-vehicle = multiple sysIds over one socket. All SITL instances target gcs-backend:14550. |
| ConnectLink auto-bind vs operator-initiated | **Auto-bind at startup**. ConnectLink is for runtime supplementary links only. See §7 above. |
| GCS heartbeat placement | Dedicated goroutine in vehicle model, not transport layer. Ticks at 1 Hz per discovered vehicle. sync.Once guard on spawn. |
| GCS heartbeat HEARTBEAT fields | type=MAV_TYPE_GCS(6), autopilot=MAV_AUTOPILOT_GENERIC(0), base_mode=0, custom_mode=0, system_status=MAV_STATE_ACTIVE(4). |
| ACK correlation design | In-flight command registry keyed by (vehicleSysID, vehicleCompID, commandID) from MAVLink frame header + ACK body. 5s timeout per entry. See §3 above. |
| Redis Streams vs Pub/Sub | `telemetry:<sysId>` → Pub/Sub (ephemeral, high-rate). `fleet:events`, `track:events`, `audit:events` → Streams (persistent, cold-start recovery). |
| TLS placement | Terminated at reverse proxy in Docker Compose. Backend speaks plain HTTP. See §6 above. |
| MAVLink signing key storage | Environment variable `GCS_MAVLINK_SIGNING_KEY` (hex). Empty = signing disabled (dev/SITL). Document in `docker-compose.yml` as optional. |
| Pub/Sub consumer groups | Use `XREADGROUP` with group `gcs-backend` from day one on all Streams. Requires `XGROUP CREATE` at service startup with `MKSTREAM`. |
| Redis param key schema | `HSET params:<sysId>` with param_id as field names. One hash per vehicle. Single `EXPIRE 3600s` after full list completes. |
| EXPIRE throttling | `vehicle:state:<sysId>` EXPIRE at most once per 30s per vehicle, not once per heartbeat. |
| WatchFleet bootstrap | Scan `fleet:active` SET → emit synthetic VEHICLE_DISCOVERED batch → then XREADGROUP from stream. |
| type_mask for position-only | 0xDF8 = 3576 = 0b110111111000. FORCE_SET bit (9) not set. |
| ArduPilot SITL image | Multi-stage Dockerfile, pinned ArduPilot commit SHA. `ardupilot/ardupilot-dev-jammy` is dev env only, not a SITL runtime. |
| Stall gate | Conditional on vehicle type. Applies only to fixed-wing (MAV_TYPE_FIXED_WING and VTOL family). Zero for all multirotor types. |
| buf breaking gate | `buf breaking --against origin/main` (CI gate against merge target). `HEAD~1` is too strict for feature development. |

---

## Principles

These are unchanged — they are correct.

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
- Remove `force` field (field 3) from `SetArmedRequest` in `proto/gcs/v1/commands.proto`
- Run `buf lint` — must pass
- Run `buf breaking --against origin/main` — must pass (field removal before first codegen is not a breaking change since no generated code exists yet)

**What exists:** 14 proto files, buf-linted, complete domain schema including
`BatteryStatus`, `EkfStatusReport`, and `TelemetryEvent` oneof.
**What is missing:** `buf.gen.yaml`, generated code.
**Exit condition:** `buf lint` passes; `SetArmedRequest.force` is removed.

---

### Tier 1 — Pure Domain Logic [parallel ok: Go + TS simultaneously]

*No sockets. No goroutines. No Redis. No Docker.*

**Go: MAVLink codec via gomavlib** (`internal/codec/`)

- `frame.go` — wrap gomavlib's `Node` in read-only decode mode (or use
  `FrameProcessor` for testability). Output `Frame{Version, SysID, CompID, MsgID, Seq, Payload}`.
  Do not re-implement framing.
- `message.go` — switch on gomavlib message type → deserialize into `TelemetryEvent`
  proto oneof variant via gomavlib's strongly-typed message structs.
- **Priority receive set — 18 families across common + ardupilotmega dialects:**
  `HEARTBEAT` (#0), `SYS_STATUS` (#1), `PARAM_VALUE` (#22), `GPS_RAW_INT` (#24),
  `ATTITUDE` (#30), `GLOBAL_POSITION_INT` (#33), `MISSION_CURRENT` (#42),
  `MISSION_COUNT` (#44), `MISSION_ACK` (#47), `MISSION_ITEM_INT` (#73),
  `VFR_HUD` (#74), `COMMAND_ACK` (#77), `RADIO_STATUS` (#109),
  `BATTERY_STATUS` (#147), `HOME_POSITION` (#242), `STATUSTEXT` (#253),
  `NAV_CONTROLLER_OUTPUT` (#62), `EKF_STATUS_REPORT` (#193, ardupilotmega).
- **Priority send set — 11 families:**
  `COMMAND_LONG` (#76), `SET_POSITION_TARGET_GLOBAL_INT` (#86), `PARAM_SET` (#23),
  `PARAM_REQUEST_LIST` (#21), `PARAM_REQUEST_READ` (#20), `MISSION_COUNT` (#44),
  `MISSION_ITEM_INT` (#73), `MISSION_REQUEST_INT` (#51), `MISSION_ACK` (#47),
  `HEARTBEAT` (#0 — GCS presence), `MISSION_CLEAR_ALL` (#45).
- Use `ardupilotmega.Dialect` for the gomavlib Node — it is a superset of common
  and handles both common and ArduPilot-specific messages without a separate
  common dialect.

**TypeScript: pure resolvers** (`frontend/src/logic/`)

- `attitude.ts` — `resolveAttitude(event) → {pitchDeg, rollDeg, source}`
- `heading.ts` — fallback chain VFR_HUD.heading → ATTITUDE.yaw → GLOBAL_POSITION_INT.hdg;
  reject 65535; `isFallback` on output
- `flightPath.ts` — `resolveFlightPath2d(event) → {trackDeg, groundSpeedMps, climbMps, fpaRad}`;
  `climbMps = -(global_position.vz_m_s)` — NED sign flip (positive down → negative climb).
  VFR_HUD.climb_m_s is already positive-up and does NOT need the sign flip.
- `trajectory.ts` — body-rate Euler: `ψ̇ = (sin φ·q + cos φ·r)/cos θ`; stall gate
  conditional on vehicle type (see §14 above); 5s horizon; 10 points.
  Function signature: `resolvePredictiveTrajectory(event, vehicleType: MavType)`
- `position.ts` — GPS primary; velocity-integration fallback
- `track.ts` — `accumulateTrack(prev, event) → ENU[]`; ring buffer; ENU projection
  `north_m = Δlat_deg × 111319.49; east_m = Δlon_deg × 111319.49 × cos(lat0_rad)`
- `freshness.ts` — `isFresh(lastSeenMs, ttlMs) → boolean`

Write `sampleFromEvent(e: TelemetryEvent): TelemetrySample` shim — all resolver
logic operates on the flat shape; only the shim knows proto structure.

**Exit conditions:**
- `go test ./internal/codec/...` — no sockets, no goroutines, passes with golden byte vectors
- `vitest ./frontend/src/logic/...` — known-answer vectors match; NaN never returned
  from any resolver; each output carries a `source` field naming the MAVLink message
- `go test -race ./internal/codec/...` passes
- Stall gate test: copter vehicle type at 10 m/s produces nonzero forward trajectory;
  plane vehicle type at 10 m/s produces zero forward trajectory

---

### Tier 2 — Parity Apparatus + Test Vectors [parallel ok with Tier 1]

Establish the comparison apparatus.

**Go capability matrix** (`docs/wip/codec-capability-matrix.md`):
- One row per normalized message family (18 receive + 11 send)
- One row per framing case (v1 accept, unsigned-v2 accept, bad CRC reject, truncation reject)
- Columns: `Case ID | Behavior | Evidence artifact | Status | Notes`
- Copy golden byte fixtures from `flight-path-hud/contracts/mavlink/` → `contracts/mavlink/`

**TS known-answer vectors:**
- Each resolver: table of `{input, expectedOutput}` pairs
- Inputs from SITL captures; outputs hand-computed
- Reference named fixture files, not inline magic numbers

**Exit conditions:**
- Matrix exists with all case IDs; none `unstarted`
- TS resolver tests all reference named fixtures
- `go test -race ./internal/codec/...` passes

---

### Tier 3 — Codegen Infrastructure

Wire protobuf codegen so generated stubs are available for all subsequent tiers.

**`proto/buf.gen.yaml`** (paths relative to `proto/` directory where `buf generate` is run):
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

**buf CI gates:**
- `buf lint` — always
- `buf breaking --against origin/main` — required on PRs to main; not `HEAD~1`
  which would fail on any feature branch with iterative proto changes

**Exit conditions:**
- `buf generate` produces Go stubs in `internal/gen/gcs/v1/`
- `buf generate` produces TS client in `frontend/src/gen/gcs/v1/`
- `buf lint` passes
- `buf breaking` gate wired in CI

---

### Tier 4 — Bridge Core (pure fold, injected clock)

The vehicle model as a **pure fold** — no goroutines, no Redis, no sockets.

**Docker Compose scaffolding [parallel ok — prepare while Tier 4 pure code is written]:**

```
Service             Image/Build                         Ports (host:container)       Notes
---                 ---                                 ---                          ---
ardupilot-sitl-1    build: ./docker/sitl (pinned SHA)   14550:14550/udp, 5760:5760   SYSID=1; --out=udp:gcs-backend:14550
ardupilot-sitl-2    same image                          14560:14550/udp, 5761:5760   SYSID=2; --out=udp:gcs-backend:14550 (profile: multi-sitl)
ardupilot-sitl-3    same image                          14570:14550/udp, 5762:5760   SYSID=3; --out=udp:gcs-backend:14550 (profile: multi-sitl)
redis               redis:7-alpine                      6379:6379                    AOF persistence; no auth in dev
gcs-backend         build: .                            8080:8080                    GCS_MAVLINK_UDP_BIND=0.0.0.0:14550
gcs-frontend        build: ./frontend                   3000:3000                    Vite; proxies /api → backend:8080
nginx               nginx:alpine                        443:443, 80:80               TLS termination; reverse proxy → backend:8080
```

Host-published ports 14560/14570 are for external tooling (mavproxy, wireshark)
on the host. Both SITL instances 2 and 3 still send MAVLink to `gcs-backend:14550`
inside the Docker network. The single GCS socket at 14550 receives all three.

SITL instances 2 and 3 require `--profile multi-sitl` to start.
Default `docker-compose up` starts only SITL-1.

SITL Dockerfile: multi-stage. Builder stage clones ArduPilot at pinned commit SHA,
builds `ArduCopter.elf`. Runtime stage is `ubuntu:22.04` with only required libs.
Pin to a specific release tag (e.g., `ArduCopter-4.6.0`) to avoid float builds.

**`internal/vehicle/fold.go`:**
- `Fold(state VehicleState, msg Message, nowMs int64) (VehicleState, []Event)`
- `VehicleState`: current `VehicleSnapshot` proto fields, health counters,
  per-message-family rate buckets, last-seen timestamps per sysId,
  `lastExpireMs int64` for EXPIRE throttling
- `HeartbeatState` extraction; armed/mode flag expansion
- TTL expiry: `nowMs - lastSeenMs > 60000` → emit `FleetEvent{VEHICLE_LOST}`
- Source-conflict detection: same sysId from two different adapters emits warning event

**`internal/routes/table.go`:**
- Route table entries: `(sysId, compId) → (srcIP, srcPort, lastSeenMs)`
- UDP send path: look up `(sysId, compId)` → send to recorded `(srcIP, srcPort)`
  (recorded from the first received UDP packet from that vehicle)
- Expiry evicted on lookup; no background goroutine

**`fleet:active` Redis SET:**
- SADD `fleet:active <sysId>` on VEHICLE_DISCOVERED
- SREM `fleet:active <sysId>` on VEHICLE_LOST
- Used by WatchFleet bootstrap to enumerate live vehicles without SCAN

**Exit conditions:**
- Pure fold tested with fixed-clock inputs; `time.Now()` never called in fold or route code
- `go test -race ./internal/vehicle/... ./internal/routes/...` passes
- Sequence wrap/fallback tested; source-conflict emits exactly one warning
- Docker Compose file exists and `docker-compose config` validates
- Multi-stage SITL Dockerfile builds successfully (slow gate — CI only)

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
- Record sender address on each received packet → populate route table entry

**GCS heartbeat goroutine** (`internal/vehicle/heartbeat.go`):
- `sync.Once` guard per vehicle model — two rapid HEARTBEATs must not spawn two goroutines
- Launched once first `HEARTBEAT` is received from a vehicle
- Ticks at 1 Hz; encodes gomavlib `common.MessageHeartbeat{Type: 6, Autopilot: 0,
  BaseMode: 0, CustomMode: 0, SystemStatus: 4}` with sysId=255, compId=190
- Pushes encoded bytes into transport write channel
- Goroutine exits when vehicle goroutine's context is cancelled
- **This is not optional. Without it, ArduPilot guided commands time out silently.**

**`internal/redis/client.go`:**
- `Publish(channel, proto.Message)` — Pub/Sub
- `Subscribe(channel) chan proto.Message` — Pub/Sub
- `XAdd(stream, fields map[string]string)` — Stream append with MAXLEN enforcement
- `XGroupCreateMkStream(stream, group string)` — called at init; idempotent via BUSYGROUP error check
- `XREADGROUP` setup for consumer group `gcs-backend` on all Streams at init
- `HSet(key string, fields map[string]interface{})`, `HGetAll(key) map[string]string`
- `SAdd(key, member)`, `SMembers(key) []string` — for `fleet:active`

**Redis init sequence (startup, before serving requests):**
```go
for _, stream := range []string{"fleet:events", "track:events", "audit:events"} {
    err := redis.XGroupCreateMkStream(stream, "gcs-backend", "$")
    if err != nil && !isBusyGroupError(err) {
        return err  // fatal
    }
}
```

**EXPIRE throttling in vehicle model:**
```go
if nowMs - state.lastExpireMs > 30_000 {
    redis.Expire("vehicle:state:" + sysId, 300)
    state.lastExpireMs = nowMs
}
```

**Goroutine supervision** (`cmd/gcs/main.go`):
- Pipeline: transport → frame codec → message codec → router → vehicle fold → Redis
- Each boundary: typed channel with explicit buffer size
- `errgroup` for coordinated shutdown; zero goroutine leaks past context cancellation
- `/healthz` returns: `{"redis":"ok","sitl_link":"up|down","last_heartbeat_ms":N}`

**Exit conditions:**
- `docker-compose up` → SITL-1 heartbeat visible in backend logs with `VEHICLE_DISCOVERED`
- `redis-cli SUBSCRIBE telemetry:1` shows live `TelemetryEvent` proto bytes
- `redis-cli XREAD COUNT 10 STREAMS fleet:events 0` shows `VEHICLE_DISCOVERED` event
- `redis-cli SMEMBERS fleet:active` returns `["1"]`
- GCS heartbeat confirmed: ArduPilot SITL accepts a guided command without GCS failsafe.
  Validation procedure: (a) stop heartbeat goroutine → send CMD 400 → confirm STATUSTEXT
  "GCS Failsafe" or COMMAND_ACK DENIED; (b) restart heartbeat → confirm command accepted
- `go test -race ./...` passes
- Goroutine count stable over 60-second SITL run

---

### Tier 6 — First Connect Services + Track Layer + Telemetry Log

Parallel tracks: Go services and frontend adapter/log can proceed simultaneously
once Tier 5 gate is closed.

**Go: services** (`internal/services/`):

- `FleetService.WatchFleet` — bootstrap: SMEMBERS `fleet:active` → scan `vehicle:state:<id>`
  → emit synthetic VEHICLE_DISCOVERED batch → XREADGROUP `fleet:events` from `$` for live events
- `FleetService.ListVehicles` — SMEMBERS `fleet:active` → HGETALL `vehicle:state:<id>` per member
- `TelemetryService.StreamTelemetry` — Subscribe `telemetry:<sysId>`, stream `TelemetryEvent`
- `TelemetryService.GetSnapshot` — HGET `vehicle:state:<sysId>` → `VehicleSnapshot`

**Go: track layer** (`internal/adapters/track.go`):

Wire MAVLink track path now. ADS-B and Meshtastic adapters land in Tier 9.
- Normalize each `VehicleSnapshot` → `Track` proto with `MavlinkDetail`
- `XADD track:events` on every snapshot update; `HSET track:state:<nodeId>`; EXPIRE 300s
- `TrackService.WatchTracks` — bootstrap from `track:state:*`, then XREADGROUP `track:events`

Track layer is here (not Tier 9) because `MapPanel` is needed in Tier 8 to
validate guided reposition. Without the track layer, there is no map.

Wire Connect server into `cmd/gcs/main.go` using Tier 3 generated stubs.

**Frontend: adapter context + telemetry log component:**

```
frontend/src/adapters/
  types.ts         GcsAdapter interface
  connect.ts       ConnectAdapter — wraps generated @connectrpc/connect-web client
  mock.ts          MockAdapter — static TelemetryEvent fixtures; no network
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
- Each row names its MAVLink source message and dialect (common vs ardupilotmega)
- Battery voltage and EKF flags visible as rows
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
- GCS identity (sysId=255, compId=190) is explicit constructor input, not env var
- Request encoding via gomavlib `common.MessageParamRequestList` and
  `common.MessageParamRequestRead`
- On list completion: write `HSET params:<sysId>` with all param_id:value pairs;
  then `EXPIRE params:<sysId> 3600`

**Go: `internal/transactions/mission.go`** (pure):
- `MISSION_REQUEST_LIST` → `MISSION_COUNT` → `MISSION_REQUEST_INT` × n → `MISSION_ACK`
- Per-item retry with 2s timeout (1s is tight for radio links; 2s sufficient for
  SITL and accommodates marginal radio links)
- Gap detection: track received item indices; re-request missing after quiescence

**Services:**
- `ParameterService.ListParameters` — triggers list fold, streams `ParameterValue` protos
- `ParameterService.GetParameter` — triggers read fold, returns single `ParameterValue`
- `MissionService.DownloadMission` — streams `MissionItem` protos

**Frontend:**
- `ParametersPanel` (read-only initially): searchable list, type-aware value display;
  `MavParamType` from PARAM_VALUE determines int/float rendering
- `MissionPanel`: seq, command (decoded from `MavCmd` enum), position; waypoints overlaid on map

**Exit conditions (parameters):**
- Capability matrix rows: PARAM-VEC-NAME, PARAM-VEC-INDEX, PARAM-VEC-LIST,
  PARAM-READ-NAME, PARAM-READ-INDEX, PARAM-LIST-COMPLETE, PARAM-LIST-IDLE,
  PARAM-ROUTE-LOSS, PARAM-REPLAY, PARAM-INVALID-TARGET — all `complete`
- SITL: `ARMING_CHECK` readable from live ArduCopter; value present in `params:1` HSET
- Replay of recorded parameter session is deterministic; produces no outbound bytes

**Exit conditions (mission download):**
- SITL: 5-waypoint QGC mission → ligma-gcs download → identical item list
- Round-trip: download → JSON → compare with QGC `.plan` format

---

### Tier 8 — Write Protocol Transactions (ordered by operational risk)

Before writing any Tier 8 code, wire the command registry. This is infrastructure,
not a service — it lives in `internal/command/registry.go` and is shared by
all write handlers.

**Command registry (first, before any write handlers):**
- In-flight map keyed by `commandKey{vehicleSysID uint8, vehicleCompID uint8, commandID uint16}`
- `vehicleSysID`/`vehicleCompID` are the MAVLink frame source fields from the vehicle's
  COMMAND_ACK (i.e., the vehicle we commanded)
- Register before encoding + sending; deregister on `common.MessageCommandAck` match or 5s timeout
- Duplicate key (same vehicle + same command already in-flight) → return error to service
- `errgroup` with per-entry `context.WithTimeout(5 * time.Second)`

**Dual-gate contract (all writes):**
- Service-side gate: role ≥ `OPERATOR` from `OperatorContext` (JWT claim; `NoopAuthProvider`
  returns OPERATOR in SITL mode)
- Service-side gate: vehicle eligible state checked against `VehicleSnapshot`
- UI-side gate: button disabled when eligibility conditions unmet
- Zero force-arm: `force` field removed from proto (Tier 0); not parameterizable
- Audit: `AuditEvent` XADD to `audit:events` after action; never before; never blocking

**8a. Message interval** (lowest risk):
- Outbound `MAV_CMD_SET_MESSAGE_INTERVAL` (command ID 511) via `COMMAND_LONG` (#76)
- Reversible; affects only telemetry rate; no vehicle state change
- `CommandService.SetMessageInterval` (or as a subcase of `SendCommand`)

**8b. Parameter write** (low consequence, disarmed gate):
- `PARAM_SET` (#23) → wait for `PARAM_VALUE` echo with same `param_id`
  (PARAM_SET uses echo-back, NOT COMMAND_ACK — no registry entry needed)
- Start with one low-consequence parameter (`LOG_BITMASK`)
- Gate: disarmed + operator role
- Extend `ParameterService.SetParameter`; read-back verification via echo;
  update `params:<sysId>` HSET on confirmed echo; audit log
- Extend `ParametersPanel` with write mode

**8c. Mission upload**:
- Client streams `MissionItem` protos; `MissionService.UploadMission`
- Handshake: GCS sends `MISSION_COUNT` → vehicle drives item requests via
  `MISSION_REQUEST_INT` × n → GCS sends `MISSION_ITEM_INT` per request → vehicle sends `MISSION_ACK`
- Per-item retry: 2s timeout per `MISSION_REQUEST_INT`; 3 retries before abort
- On `MISSION_ACK`, check `MAV_MISSION_RESULT` — `MAV_MISSION_ACCEPTED = 0` is success;
  handle `MAV_MISSION_ERROR`, `MAV_MISSION_UNSUPPORTED_FRAME`, etc. as typed errors
- Round-trip test: upload → download → assert equal item list

**8d. Mode change** (disarmed-only gate):
- Vehicle-type allowlist (ArduCopter and ArduPlane have different `custom_mode` values;
  copter GUIDED=4, plane GUIDED=15)
- `MAV_CMD_DO_SET_MODE` (#176) via `COMMAND_LONG`; `base_mode` must include
  `MAV_MODE_FLAG_CUSTOM_MODE_ENABLED` (128) when setting `custom_mode`
- ACK via command registry; `HEARTBEAT.custom_mode` post-condition verification
- `CommandService.SetMode`; gate: disarmed + operator role + mode in vehicle-type allowlist

**8e. Arm / Disarm**:
- `COMMAND_LONG` (#76) with `MAV_CMD_COMPONENT_ARM_DISARM` (400), param1=1.0 (arm) / 0.0 (disarm),
  param2=0 (no force; `force` field removed from proto)
- Gate for arm: `custom_mode == GUIDED`, not already armed, operator role
- EKF gate: `EkfStatusReport.flags & EKF_ATTITUDE` must be set before allowing arm
  (`EKF_UNINITIALIZED` flag must be clear)
- Exactly once per user action; no automatic retry; user must confirm failure
- Post-condition: `HEARTBEAT.base_mode & MAV_MODE_FLAG_SAFETY_ARMED` (128) observed
- `CommandService.SetArmed`

**Map must be working before 8f. Build MapPanel during 8d/8e:**
- `MapPanel.tsx` — MapLibre GL; vehicle markers; track polyline from `TrackService.WatchTracks`
- `FleetView.tsx` — roster + basemap; vehicle cards color-coded by freshness
- The map is needed to validate 8f; build it here, not in Tier 10

**8f. Guided reposition** (highest risk):
- `SET_POSITION_TARGET_GLOBAL_INT` (#86): `type_mask=0xDF8` (3576 — position only;
  see §12 for derivation)
- Frame: `MAV_FRAME_GLOBAL_RELATIVE_ALT_INT` (6) — altitude is above home, not MSL
- lat/lon sent as integer degE7 (multiply degrees × 1e7, cast to int32)
- **Server-side bounds validation** (not just UI): target >500m from home rejected at
  service layer; altitude <2m AGL rejected at service layer. UI gate alone is
  bypassable by direct Connect call.
- Gate: armed + `custom_mode == GUIDED` + operator role
- Read-back: poll `VehicleSnapshot.position` until within 2m or 30s timeout
- `CommandService.SetPositionTargetGlobal`

**8g. Guided workflow lifecycle** (takeoff, land):
- ARM → GUIDED → TAKEOFF (CMD 22 = `MAV_CMD_NAV_TAKEOFF`, param7 = altitude AGL) →
  (fly) → LAND (CMD 21 = `MAV_CMD_NAV_LAND` or mode=LAND) → DISARM ordered state machine
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
- Deduplication by ICAO address required when both sources report the same aircraft

**Meshtastic** (when hardware available):
- Demultiplex one MeshPacket connection into three paths:
  - PortNum 1 (`TEXT_MESSAGE_APP`) → `ChatService`
  - PortNum 3 (`POSITION_APP`) → `TrackService` (position)
  - PortNum 67 (`TELEMETRY_APP`) → `TrackService` (device telemetry / MeshtasticDetail)
  - Custom PortNum (256+) → MAVLink transport adapter (negotiated per deployment)
- `Track` with `MeshtasticDetail`; `is_mavlink_relay` flag for relay nodes

**Exit conditions:**
- Protocol-agnostic map renders all track sources without importing MAVLink packages
- Adding a new protocol = new adapter + new proto detail variant; zero map code changes

---

### Tier 10 — UI Composition (deferred)

Instrument composition after telemetry log establishes variable trust.
Module-first: smallest trusted atomic unit, then instruments, then composed layouts.

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
| `FlightStateDisplay` | `resolveFlightPath2d` | NED sign flip tested; VFR_HUD path exempt from sign flip |
| `PredictiveTrajectory` | `resolvePredictiveTrajectory(event, vehicleType)` | CTRV model + vehicle-type-conditional stall gate tested |
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
| gRPC-native browser transport | ADR defers; Connect serves as bridge |
| Meshtastic relay implementation | Adapter boundary defined (ADR-0004); waits on Tier 9 + hardware |
| mTLS between services | TLS termination at proxy satisfies ADR-0005 for now; mTLS is a hardening step |
| MISSION_CLEAR_ALL UI | Protocol defined; wire command in Tier 8; surface in mission panel post-Tier 10 |
