# Tier 1 — Pure Domain Logic

## Overview

The two most important vertical slices of the system — the Go MAVLink codec and the TypeScript resolver functions — built in isolation from all infrastructure. No sockets, no goroutines, no Redis, no Docker. Both tracks can proceed simultaneously once Tier 0 is merged. The exit gate is comprehensive known-answer testing; nothing moves to Tier 2+ until both pass.

## Dependencies

- Tier 0 complete (`buf lint` passing, `force` field removed)
- `gomavlib v3` added to `go.mod`
- `@bufbuild/protobuf` and `@connectrpc/connect-web` in `frontend/package.json` (or at minimum the TS types for proto messages available via path alias)

## Chapters

---

### Chapter 1: Go — gomavlib Node Wrapper

**Goal:** Wrap gomavlib's `Node` type in a testable interface that hides the real socket behind a channel. This allows Tier 1 tests to feed raw bytes without opening a UDP port.

**File:** `internal/codec/frame.go`

**Interface:**
```go
type FrameSource interface {
    Events() <-chan gomavlib.Event
    WriteMessage(msg interface{}) error
    Close() error
}
```

**Real implementation:** `UDPFrameSource` wraps a `gomavlib.Node` configured with the ardupilotmega dialect.

**Test implementation:** `ChanFrameSource` wraps a `gomavlib.Node` configured with `gomavlib.EndpointCustomConn` (in-memory io.Pipe) — raw bytes are written to the pipe, gomavlib decodes them through its full frame parser (CRC validation, signing check, dialect struct population), and the resulting `gomavlib.Event` objects emerge from `Events()`. This exercises the real decode path, including CRC_EXTRA. Do not hand-craft `gomavlib.EventFrame` structs directly — that would bypass the codec under test.

**gomavlib Node config:**
```go
gomavlib.NodeConf{
    Endpoints:      []gomavlib.EndpointConf{...}, // injected by caller
    Dialect:        ardupilotmega.Dialect,
    OutVersion:     gomavlib.V2,
    OutSystemID:    255,
    OutComponentID: 190,
}
```

**Key constraint:** The codec package must not import anything above `internal/codec/`. It has no knowledge of Redis, services, or vehicles.

---

### Chapter 2: Go — Message Decode → TelemetryEvent

**Goal:** Switch on gomavlib message type and map each of the 18 priority receive families to a `TelemetryEvent` proto oneof variant.

**File:** `internal/codec/message.go`

**18 receive families to handle:**

| Family | gomavlib type | Proto oneof field |
|---|---|---|
| HEARTBEAT (0) | `common.MessageHeartbeat` | `heartbeat` |
| SYS_STATUS (1) | `common.MessageSysStatus` | `sys_status` |
| PARAM_VALUE (22) | `common.MessageParamValue` | `param_value` |
| GPS_RAW_INT (24) | `common.MessageGpsRawInt` | `gps_raw_int` |
| ATTITUDE (30) | `common.MessageAttitude` | `attitude` |
| GLOBAL_POSITION_INT (33) | `common.MessageGlobalPositionInt` | `global_position_int` |
| MISSION_CURRENT (42) | `common.MessageMissionCurrent` | `mission_current` |
| MISSION_COUNT (44) | `common.MessageMissionCount` | `mission_count` |
| MISSION_ACK (47) | `common.MessageMissionAck` | `mission_ack` |
| NAV_CONTROLLER_OUTPUT (62) | `common.MessageNavControllerOutput` | `nav_controller_output` |
| MISSION_ITEM_INT (73) | `common.MessageMissionItemInt` | `mission_item_int` |
| VFR_HUD (74) | `common.MessageVfrHud` | `vfr_hud` |
| COMMAND_ACK (77) | `common.MessageCommandAck` | `command_ack` |
| RADIO_STATUS (109) | `common.MessageRadioStatus` | `radio_status` |
| BATTERY_STATUS (147) | `common.MessageBatteryStatus` | `battery_status` |
| HOME_POSITION (242) | `common.MessageHomePosition` | `home_position` |
| STATUSTEXT (253) | `common.MessageStatustext` | `statustext` |
| EKF_STATUS_REPORT (193) | `ardupilotmega.MessageEkfStatusReport` | `ekf_status_report` |

**Pattern:**
```go
func Decode(evt gomavlib.Event) (*TelemetryEvent, error) {
    switch e := evt.(type) {
    case *gomavlib.EventParseError:
        // Log with error metrics — distinguish from unrecognized message
        log.Printf("codec parse error: %v", e.Error)
        return nil, nil
    case *gomavlib.EventFrame:
        switch msg := e.Message().(type) {
        case *common.MessageHeartbeat:
            return &TelemetryEvent{Payload: &TelemetryEvent_Heartbeat{...}}, nil
        // ...
        case *ardupilotmega.MessageEkfStatusReport:
            return &TelemetryEvent{Payload: &TelemetryEvent_EkfStatusReport{...}}, nil
        default:
            return nil, nil // unrecognized message family — drop silently
        }
    default:
        return nil, nil
    }
}
```

**Parse error vs unrecognized message:** `EventParseError` (bad CRC, truncated frame, signing failure) must be distinguishable from a message ID we don't handle. Both result in a nil return, but parse errors should increment a counter (`codec_parse_errors_total`) for observability. Silent drop of unrecognized message IDs is correct; silent drop of parse errors hides hardware noise and replay tampering.

**Frame metadata to preserve:** sysId, compId, sequence number from `e.SystemID()`, `e.ComponentID()`, `e.Frame.GetSequence()`. Verify these API names against the actual gomavlib v3 tagged release before committing — they changed from v2. In v3: system ID is on the frame via `e.Frame.GetSystemID()` for raw access; check the v3 API surface in `pkg/frame/frame.go` before assuming the method names shown here.

**sysId architecture note:** sysId/compId travel with the event and are used by the Tier 4 fold for per-vehicle routing. They are NOT part of `TelemetrySample`. The boundary is: codec emits `(sysId, TelemetryEvent)` pairs; the fold dispatches by sysId; resolvers operate on `TelemetrySample` without vehicle identity. This is intentional — resolvers are pure sensor projections, not vehicle-aware. Document this explicitly in `internal/codec/message.go` so no one adds sysId to TelemetrySample thinking it belongs there.

---

### Chapter 3: Go — 11 Priority Send Encoders

**Goal:** One encode function per outbound message family. Each takes a typed struct, returns gomavlib-ready message.

**File:** `internal/codec/encode.go`

**11 send families:**

| Function | gomavlib type | Notes |
|---|---|---|
| `EncodeHeartbeat()` | `common.MessageHeartbeat` | GCS keepalive; fixed fields |
| `EncodeCommandLong(...)` | `common.MessageCommandLong` | arm, mode, takeoff, land, MAV_CMD_SET_MESSAGE_INTERVAL |
| `EncodeSetPositionTargetGlobalInt(...)` | `common.MessageSetPositionTargetGlobalInt` | type_mask=0xDF8; frame=MAV_FRAME_GLOBAL_RELATIVE_ALT_INT(6) |
| `EncodeParamSet(...)` | `common.MessageParamSet` | param write |
| `EncodeParamRequestList(...)` | `common.MessageParamRequestList` | triggers full param download |
| `EncodeParamRequestRead(...)` | `common.MessageParamRequestRead` | single param read |
| `EncodeMissionCount(...)` | `common.MessageMissionCount` | mission upload initiation |
| `EncodeMissionItemInt(...)` | `common.MessageMissionItemInt` | mission item upload |
| `EncodeMissionRequestInt(...)` | `common.MessageMissionRequestInt` | request specific item |
| `EncodeMissionAck(...)` | `common.MessageMissionAck` | upload complete confirmation |
| `EncodeMissionClearAll(...)` | `common.MessageMissionClearAll` | wipe vehicle mission |

**Constraint:** Encoder functions are pure — they produce message structs, not bytes. gomavlib handles CRC_EXTRA and framing when the message is passed to `node.WriteMessage()`. Do not compute CRC_EXTRA manually.

---

### Chapter 4: TypeScript — sampleFromEvent Shim

**Goal:** A single function that flattens a `TelemetryEvent` proto oneof into a flat `TelemetrySample` shape. All resolver logic operates on this flat shape; only the shim knows proto structure.

**File:** `frontend/src/logic/sample.ts`

**TelemetrySample shape (illustrative — derive from what resolvers actually need):**
```typescript
interface TelemetrySample {
  // Attitude
  rollRad?: number;
  pitchRad?: number;
  yawRad?: number;
  // Angular rates (from ATTITUDE message — required for full CTRV trajectory model)
  rollspeedRadS?: number;
  pitchspeedRadS?: number;
  yawspeedRadS?: number;
  // Position
  latDegE7?: number;
  lonDegE7?: number;
  altMslMm?: number;
  relativeAltMm?: number;
  // Velocity (NED, cm/s)
  vxCms?: number;
  vyCms?: number;
  vzCms?: number; // positive down (NED)
  // VFR
  groundspeedMps?: number;
  airspeedMps?: number;
  headingDeg?: number; // 0–359
  climbMps?: number;   // positive up (already flipped in VFR_HUD)
  // Vehicle identity
  vehicleType?: number; // MAV_TYPE enum value
  customMode?: number;
  baseMode?: number;
  systemStatus?: number;
  // EKF
  ekfFlags?: number;
  // Battery
  batteryVoltagesMv?: number[];
  batteryCurrentCa?: number;
  batteryRemainingPct?: number;
  // Source tracking
  sourceMessage: string; // e.g. "ATTITUDE", "GLOBAL_POSITION_INT"
  receivedAtMs: number;
}
```

**Rule:** The shim is the only place that touches proto-generated field names. Resolvers take `TelemetrySample` — they are not coupled to proto codegen output.

**Accumulation pattern — critical:** `TelemetryEvent` is a oneof: each event carries exactly one message family. `TelemetrySample` spans all families. The shim returns a partial sample per event (most fields undefined). The caller — the per-vehicle fold in Tier 4 — maintains a running `TelemetrySample` and merges each partial into it with a shallow spread:

```typescript
// Each event arrives separately — shim returns partial
const partial = sampleFromEvent(event);
// Caller merges into accumulated state (per vehicle)
accumulated = { ...accumulated, ...partial };
```

The shim does NOT maintain internal state. The shim does NOT accumulate across events. Callers that try to pass a single-event partial directly to resolvers expecting a full sample will get null returns — which is correct behavior, not a bug. Document this in the shim's JSDoc.

**Tier 1 / Tier 3 parallelism note:** The shim imports proto-generated types for the `TelemetryEvent` oneof. If Tier 3 codegen is not yet complete when Tier 1 TS work begins, stub the types by hand in `frontend/src/logic/_proto_stubs.ts` and replace with the generated import once Tier 3 merges. Do not let the stub diverge — match field names exactly to what buf will generate. Delete the stub file at Tier 3 merge; do not let it coexist with generated types.

---

### Chapter 5: TypeScript — Resolver Functions

**Goal:** Six pure resolver functions, each returning a typed result with a `source` field.

**File per resolver:**

**`frontend/src/logic/attitude.ts`**
- Input: `TelemetrySample`
- Output: `{ pitchDeg: number; rollDeg: number; source: 'ATTITUDE' } | null`
- Source: always ATTITUDE; return null if attitude fields absent

**`frontend/src/logic/heading.ts`**
- Input: `TelemetrySample`
- Output: `{ headingDeg: number; source: 'VFR_HUD' | 'ATTITUDE' | 'GLOBAL_POSITION_INT'; isFallback: boolean } | null`
- Fallback chain: VFR_HUD.heading (reject if 65535) → ATTITUDE.yaw (convert rad→deg, normalize 0–360) → GLOBAL_POSITION_INT.hdg (÷100)
- `isFallback: true` when not using primary source

**`frontend/src/logic/flightPath.ts`**
- Input: `TelemetrySample`
- Output: `{ trackDeg: number; groundSpeedMps: number; climbMps: number; fpaRad: number; source: string } | null`
- **Climb source preference:** Use VFR_HUD.climbMps as primary when present (positive-up, no flip needed, more accurate for airspeed-derived flight path). Fall back to GPI.vzCms when climbMps is absent — and apply the NED sign flip: `climbMps = -(sample.vzCms / 100)`. Never mix both sources in the same output. The `source` field must name the winner: `'VFR_HUD'` or `'GLOBAL_POSITION_INT'`.
- **NED sign flip:** `climbMps = -(sample.vzCms / 100)` when sourced from GLOBAL_POSITION_INT. VFR_HUD.climbMps is already positive-up — do NOT flip.
- `fpaRad = Math.atan2(climbMps, groundSpeedMps)`

**`frontend/src/logic/trajectory.ts`**
- Input: `TelemetrySample`, `vehicleType: number` (MAV_TYPE enum value)
- Output: `Array<{ northM: number; eastM: number }>` (10 points, 5s horizon)
- **Turn rate model:** Use the full CTRV body-rate formula when angular rates are present: `ψ̇ = (sinφ·q + cosφ·r) / cosθ` (requires `rollspeedRadS`, `pitchspeedRadS`, `yawspeedRadS` from ATTITUDE message — now in TelemetrySample). Fall back to bank-angle approximation `ψ̇ = g·tan(φ) / V` when angular rates are absent. The fallback produces accurate results for coordinated flight; use it as the degraded path, not the primary.
- Stall gate: `const stallSpeed = isFixedWing(vehicleType) ? 14 : 0; const forwardProgress = Math.max(0, speed - stallSpeed);`
- **`isFixedWing` — exact MAV_TYPE values:** Return `true` for: MAV_TYPE_FIXED_WING(1), MAV_TYPE_KITE(9), MAV_TYPE_FLAPPING_WING(10), MAV_TYPE_VTOL_DUOROTOR(19), MAV_TYPE_VTOL_QUADROTOR(20), MAV_TYPE_VTOL_TILTROTOR(21), MAV_TYPE_VTOL_TAILSITTER_DUOROTOR(22), and VTOL reserved types 23–25. Return `false` for: MAV_TYPE_QUADROTOR(2), HEXAROTOR(13), OCTOROTOR(14), TRICOPTER(15), COAXIAL(8), and all others not listed above. Do not use a range check — enumerate the list explicitly so additions require a deliberate edit. Test each fixed-wing and rotor type individually in Tier 2.

**`frontend/src/logic/position.ts`**
- Input: `TelemetrySample`
- Output: `{ latDeg: number; lonDeg: number; altM: number; source: string } | null`
- Primary: GLOBAL_POSITION_INT lat/lon (÷1e7), alt (relativeAltMm÷1000)
- Fallback: GPS_RAW_INT (when GLOBAL_POSITION_INT absent)

**`frontend/src/logic/track.ts`**
- Input: `prev: ENU[]`, `TelemetrySample`, `origin: { latDeg: number; lonDeg: number }`
- Output: `ENU[]` — ring buffer, max 500 points
- ENU projection:
  ```
  northM = Δlat_deg × 111319.49
  eastM  = Δlon_deg × 111319.49 × cos(lat0_rad)
  ```

**`frontend/src/logic/freshness.ts`**
- `isFresh(lastSeenMs: number, nowMs: number, ttlMs: number): boolean`
- `ageMs(lastSeenMs: number, nowMs: number): number`

---

## Tier Exit Gate

- `go test -race ./internal/codec/...` passes — no sockets, no goroutines
- Golden byte vectors from `flight-path-hud/contracts/mavlink/` validate decode path
- `vitest ./frontend/src/logic/...` passes with known-answer vectors
- NaN never returned from any resolver (assert in tests)
- Each resolver output carries a `source` field naming the MAVLink message used
- Stall gate test: copter (MAV_TYPE=2) at 10 m/s → nonzero forward trajectory points; plane (MAV_TYPE=1) at 10 m/s → zero forward trajectory
- NED sign flip test: `vzCms = +500` (descending) → `climbMps = -5.0`; VFR_HUD climbMps passthrough unchanged
