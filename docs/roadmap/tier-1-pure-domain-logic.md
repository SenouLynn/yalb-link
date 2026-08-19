# Tier 1 — Pure Domain Logic

## Overview

The two most important vertical slices — the Go MAVLink codec and the TypeScript
resolvers — built in isolation from infrastructure. No UDP socket, no Redis, no
Docker. Both tracks proceed simultaneously, and in parallel with Tier 2. The exit
gate is comprehensive known-answer testing.

Every gomavlib symbol in this document was verified against
`github.com/bluenviron/gomavlib/v3@v3.3.5`. Where a name here differs from what you
remember, this document is the one that was checked.

## Dependencies

- **Tier 3 complete.** The TS shim imports generated `TelemetryEvent` types. Codegen
  runs immediately after Tier 0 precisely so this tier never needs a hand-written
  proto stub kept in sync by hand.
- `github.com/bluenviron/gomavlib/v3` in `go.mod` (then `bazel mod tidy`).
- `@bufbuild/protobuf` and `@connectrpc/connect-web` in `frontend/package.json`.

## Chapters

---

### Chapter 1: Go — gomavlib Node Wrapper

**Goal:** Wrap gomavlib's `Node` behind a port so tests feed raw bytes without
opening a UDP port.

**File:** `internal/codec/frame.go`

**Port:**

```go
type FrameSource interface {
    Events() <-chan gomavlib.Event
    WriteTo(link LinkID, msg message.Message) error
    Close() error
}
```

**`WriteTo`, not `WriteMessage`.** gomavlib has no `WriteMessage(msg)`; the API is
`WriteMessageTo(*Channel, message.Message)`, `WriteMessageAll`, and
`WriteMessageExcept`. A port shaped `WriteMessage(msg)` can only be satisfied by
`WriteMessageAll`, which transmits to every channel — every SITL instance, every
radio link. That means N× uplink bandwidth on the constrained direction, a
COMMAND_LONG for sysid 2 physically sent over vehicle 1's radio, and two vehicles
sharing a sysid on different links both acting on it. The destination is part of
the signature. An unaddressable target is rejected, never broadcast.

Also note the argument type: `message.Message`, not `interface{}`. The typed port is
the point.

**Node configuration.** `gomavlib.NodeConf` and `gomavlib.NewNode` are both marked
`Deprecated` in v3 ("configuration has been moved inside Node"). `staticcheck`
SA1019 is enabled in `.golangci.yml`, so the deprecated path fails our own lint
gate. Configure the struct and call `Initialize()`:

```go
node := &gomavlib.Node{
    Endpoints:        []gomavlib.EndpointConf{...}, // injected by caller
    Dialect:          ardupilotmega.Dialect,
    OutVersion:       gomavlib.V2,
    OutSystemID:      255,
    OutComponentID:   190,

    // gomavlib sends its own HEARTBEAT unless disabled — see below.
    HeartbeatDisable: false,
    HeartbeatPeriod:  time.Second,
}
if err := node.Initialize(); err != nil { ... }
defer node.Close()
```

**The heartbeat is already handled.** `HeartbeatDisable` defaults to *false*, and
the defaults are `HeartbeatPeriod = 5s`, `HeartbeatSystemType = 6` (MAV_TYPE_GCS),
`HeartbeatAutopilotType = 0` (MAV_AUTOPILOT_GENERIC) — which is exactly the GCS
heartbeat we want, at the wrong rate. Set the period and use it. Do **not** add a
hand-rolled per-vehicle ticker: it double-emits, and heartbeat is a per-link
broadcast rather than a per-peer message (see the heartbeat cardinality decision in
`order-of-operations.md`).

**`StreamRequestEnable`** defaults to false. **Leave it false** — but not for the reason
this paragraph gave until 2026-08-19.

The original claim was that SITL over UDP streams telemetry anyway, so Tiers 5–6 would look
healthy while only a real link stayed quiet. **That is false.** A six-minute run against the
pinned `Copter-4.7.0` SITL produced heartbeats and zero telemetry events. Nothing streams
until asked, in SITL or in the field — which is the good case, because it means SITL is a
faithful test of the thing that was in doubt.

The flag stays false because gomavlib's built-in path sends the deprecated
`REQUEST_DATA_STREAM` (#66) for seven coarse stream groups: a distinct message family that
would move the `len(SendFamilies) == 11` pin permanently, with no per-message control.
Asking for telemetry is instead an explicit `MAV_CMD_SET_MESSAGE_INTERVAL` at **Tier 5
Chapter 7**, not Tier 8a — see ADR-0010, which classifies a request for data as read-side.

**Test endpoint.** Use `EndpointCustomClient` with `net.Pipe()`. (`EndpointCustom`
exists but is deprecated in favour of it; `EndpointCustomConn` does not exist.)

```go
harness, nodeSide := net.Pipe()
ep := gomavlib.EndpointCustomClient{
    Connect: func(context.Context) (net.Conn, error) { return nodeSide, nil },
    Label:   "test",
}
// write golden .bin bytes to `harness`; gomavlib's parser handles STX detection,
// length extraction, CRC_EXTRA validation and dialect struct population before
// any test logic sees an event.
```

Do not hand-craft `gomavlib.EventFrame` structs — that bypasses the codec under test.

**Key constraint:** `internal/codec/` imports nothing above itself. No Redis, no
services, no vehicle model.

---

### Chapter 2: Go — Message Decode

**Goal:** Map each priority receive family onto its proto envelope.

**File:** `internal/codec/message.go`

**Two envelopes, not one.** Of the 18 priority receive families, 13 are streaming
telemetry and 5 are transaction responses that must correlate against an in-flight
request registry. They go to different places:

```go
type Decoded struct {
    SysID, CompID, Seq uint8
    Telemetry   *gcsv1.TelemetryEvent // 13 streaming families
    Transaction *gcsv1.ProtocolEvent  // PARAM_VALUE, MISSION_*, COMMAND_ACK
}
```

**Streaming telemetry → `TelemetryEvent`:** HEARTBEAT(0), SYS_STATUS(1),
GPS_RAW_INT(24), ATTITUDE(30), GLOBAL_POSITION_INT(33), MISSION_CURRENT(42),
NAV_CONTROLLER_OUTPUT(62), VFR_HUD(74), RADIO_STATUS(109), BATTERY_STATUS(147),
HOME_POSITION(242), STATUSTEXT(253), EKF_STATUS_REPORT(193/ardupilotmega).

**Transaction responses → `ProtocolEvent`:** PARAM_VALUE(22), MISSION_COUNT(44),
MISSION_ITEM_INT(73), MISSION_ACK(47), COMMAND_ACK(77).

**Dispatch table, not a switch.** An 18-case type switch with a proto construction
per case runs 90–120 lines at cyclomatic complexity ~20, and fails three limits in
our own `.golangci.yml` (`funlen` 80 lines / 50 statements, `cyclop` 15,
`gocognit` 20). Use a table:

```go
var telemetryDecoders = map[uint32]func(message.Message) *gcsv1.TelemetryEvent{
    30: decodeAttitude,       // ATTITUDE
    33: decodeGlobalPosition, // GLOBAL_POSITION_INT
    // ...
}

var protocolDecoders = map[uint32]func(message.Message) *gcsv1.ProtocolEvent{
    22: decodeParamValue,     // PARAM_VALUE
    // ...
}
```

One small converter per family, each independently testable and each well under
every limit. The table is also what makes the Tier 2 capability matrix mechanically
verifiable: a test asserts `keys(telemetryDecoders) ∪ keys(protocolDecoders)` equals
the matrix rows, so the matrix stops being prose.

**Normalisation happens here and only here.** The protos are SI: `lat_deg` is
degrees (double), `alt_msl_m` is metres, `vz_m_s` is m/s. The wire is not — E7
degrees, millimetres, cm/s. Every conversion lives in these converter functions.
Nothing downstream divides by 1e7, 1000 or 100.

**Parse error vs unrecognised message.** `EventParseError` (bad CRC, truncated
frame, signing failure) must be distinguishable from a message ID we do not handle.
Both yield no envelope, but parse errors increment `codec_parse_errors_total`.
Silently dropping unknown IDs is correct; silently dropping parse errors hides
hardware noise and replay tampering.

**Frame metadata.** `*gomavlib.EventFrame` exposes `SystemID()`, `ComponentID()` and
`Message()`. The sequence number is on the frame: `e.Frame.GetSequenceNumber()` —
not `GetSequence()`.

**sysId boundary.** `(sysId, compId)` travel on the `Decoded` struct and are set on
the envelope's `vehicle_id`. They are **not** part of `TelemetrySample`. The fold
dispatches by sysId; resolvers are pure sensor projections with no vehicle identity.
Telemetry payload messages carry no `vehicle_id` — identity is on the envelope.
Document this in `message.go` so nobody re-adds it.

---

### Chapter 3: Go — 11 Priority Send Encoders

**File:** `internal/codec/encode.go`

One encode function per outbound family, each returning a typed message struct:
`EncodeHeartbeat`, `EncodeCommandLong`, `EncodeSetPositionTargetGlobalInt`
(type_mask `0xDF8`, frame `MAV_FRAME_GLOBAL_RELATIVE_ALT_INT(6)`), `EncodeParamSet`,
`EncodeParamRequestList`, `EncodeParamRequestRead`, `EncodeMissionCount`,
`EncodeMissionItemInt`, `EncodeMissionRequestInt`, `EncodeMissionAck`,
`EncodeMissionClearAll`.

**Constraint:** encoders are pure — they produce `message.Message` values, not
bytes. gomavlib handles CRC_EXTRA and framing at write time. Never compute
CRC_EXTRA by hand.

**Constraint:** encoders do not choose a destination. Addressing is the transport
port's job (`WriteTo`), which keeps the "never broadcast" rule in one place.

---

### Chapter 4: TypeScript — `sampleFromEvent` Shim

**Goal:** Flatten a `TelemetryEvent` oneof into a flat `TelemetrySample`. Only the
shim knows proto structure.

**File:** `frontend/src/logic/sample.ts`

**Units are SI and degrees — the protos already normalised.** This is the single
most important thing in the file. The predecessor's `TelemetrySample` used raw wire
units (`latDegE7`, `altMslMm`, `vxCms`) because that repo had no normalising proto
boundary. This one does. Carrying wire-unit field names here reintroduces the
division a second time: position 1e7 too small, altitude 1000× too small, climb
100× too small. Every value stays finite, so a "NaN never returned" gate passes
happily, and Tier 2 then locks the wrong numbers in as known answers.

```typescript
interface TelemetrySample {
  // Attitude (radians)
  rollRad?: number;
  pitchRad?: number;
  yawRad?: number;
  rollspeedRadS?: number;
  pitchspeedRadS?: number;
  yawspeedRadS?: number;

  // Position — degrees and metres, already normalised by the codec
  latDeg?: number;
  lonDeg?: number;
  altMslM?: number;         // above mean sea level
  altRelativeM?: number;    // above home/takeoff

  // Velocity — NED, m/s
  vxMs?: number;
  vyMs?: number;
  vzMs?: number;            // positive down (NED)

  // VFR
  groundspeedMps?: number;
  airspeedMps?: number;
  headingDeg?: number;      // may arrive negative; normalise
  climbMps?: number;        // positive up (VFR_HUD is already positive-up)

  // Vehicle identity/state
  vehicleType?: number;     // MavType enum value
  customMode?: number;
  baseMode?: number;
  systemStatus?: number;

  ekfFlags?: number;
  batteryVoltagesMv?: number[];
  batteryCurrentA?: number;
  batteryRemainingPct?: number;

  sourceMessage: string;    // e.g. "ATTITUDE"
  receivedAtMs: number;
}
```

**Accumulation.** `TelemetryEvent` is a oneof: one family per event. `TelemetrySample`
spans families. The shim returns a *partial* sample per event and holds no state.
Something downstream must merge partials:

> **Open — this accumulator has no owner.** The snippet below is TypeScript, and the text
> assigned it to "the Tier 4 per-vehicle fold". Tier 4 is Go-only; it contains no
> TypeScript. Tier 6's `useVehicleTelemetry` only buffers the last N events, and a
> single-event partial passed straight to a resolver yields null — so Tier 6's exit
> criterion "browser shows live TelemetryLog" depends on code no tier schedules. Raised
> 2026-08-19; the likely home is Tier 6 ch.6 alongside the adapter, but that is a decision,
> not a default.

```typescript
accumulated = { ...accumulated, ...sampleFromEvent(event) };
```

Passing a single-event partial straight to a resolver yields null. That is correct
behaviour, not a bug. Say so in the shim's JSDoc.

---

### Chapter 5: TypeScript — Resolver Functions

Each resolver is pure and returns a `source` naming the MAVLink message it used.

**`attitude.ts`** — `{ pitchDeg, rollDeg, source: 'ATTITUDE' } | null`. Null when
the attitude fields are absent.

**`heading.ts`** — `{ headingDeg, source, isFallback } | null`.

Fallback chain: `VFR_HUD.heading` → `ATTITUDE.yaw` (rad→deg, normalised 0–360) →
`GLOBAL_POSITION_INT.hdg` (÷100).

**The 65535 sentinel belongs to GLOBAL_POSITION_INT, not VFR_HUD.** `VFR_HUD.heading`
is `int16_t` on the wire (verified in gomavlib's generated `common` dialect:
`Heading int16`), so it cannot hold 65535 — testing for it tests an impossible
input. UINT16_MAX-as-unknown applies to `GlobalPosition.hdg_cdeg` and
`GpsRaw.cog_cdeg`, and the reject belongs there.

What is actually needed on the VFR_HUD path is normalisation: ArduPilot may report
heading as a negative int16. Use `((h % 360) + 360) % 360`.

**`flightPath.ts`** — `{ trackDeg, groundSpeedMps, climbMps, fpaRad, source } | null`.

Climb source preference: `VFR_HUD.climbMps` when present (already positive-up — do
not flip). Otherwise `GLOBAL_POSITION_INT`, where the NED flip is `climbMps = -vzMs`
— note **no /100**, because the proto already carries m/s. Never mix sources in one
output; `source` names the winner. `fpaRad = Math.atan2(climbMps, groundSpeedMps)`.

**`position.ts`** — `{ latDeg, lonDeg, altM, altRef, source } | null`.

Primary: GLOBAL_POSITION_INT. Fallback: GPS_RAW_INT.

**Carry the altitude reference frame.** GLOBAL_POSITION_INT gives altitude above
home; GPS_RAW_INT only gives MSL. They differ by field elevation — routinely
hundreds of metres — so a fallback that drops both into `altM` silently changes what
the number means, and `source` naming the message does not convey that. Return
`altRef: 'RELATIVE' | 'MSL'` and let `AltitudeTape` refuse a datum it was not
configured for.

**`trajectory.ts`** — `resolvePredictiveTrajectory(sample, stallSpeedMps)` →
`Array<{ northM, eastM }>` (10 points, 5s horizon).

Turn rate: full CTRV body-rate formula when angular rates are present,
`ψ̇ = (sinφ·q + cosφ·r) / cosθ`. Degraded path when they are absent: bank-angle
approximation `ψ̇ = g·tan(φ) / V`, accurate for coordinated flight.

**Stall gate keys on flight regime, not vehicle type.** Apply the floor only when
`airspeedMps` is present and below `stallSpeedMps`; otherwise use groundspeed with
no floor. Gating on MAV_TYPE means a VTOL hovering or translating at 10 m/s in
multicopter mode shows no predicted track — the phase where an operator most wants
one. `stallSpeedMps` is a parameter (`ARSPD_FBW_MIN`), passed in from day one so
Tier 7's parameter read can supply it rather than forcing a signature change later.

**If a vehicle-class predicate is still needed**, derive it from the generated
`MavType` constants — never from a retyped integer list:

```typescript
import { MavType } from '@/gen/gcs/v1/types_pb';

function isFixedWingAirframe(t: MavType): boolean {
  switch (t) {
    case MavType.FIXED_WING:
    case MavType.FLAPPING_WING:
    case MavType.KITE:
    case MavType.VTOL_TAILSITTER_DUOROTOR:
    // ... remaining VTOL values
      return true;
    default:
      return false;
  }
}
```

A hand-copied table from an older MAVLink revision maps ROCKET(9) and
GROUND_ROVER(10) onto KITE and FLAPPING_WING, and COAXIAL onto FREE_BALLOON —
upstream renumbered these when the VTOL types were renamed. Importing the generated
constants makes a proto edit a compile error instead of silent drift.

**`track.ts`** — `accumulateTrack(prev: ENU[], sample, origin)` → `ENU[]`, ring
buffer capped at 500.

```
northM = Δlat_deg × 111319.49
eastM  = Δlon_deg × 111319.49 × cos(lat0_rad)
```

**`freshness.ts`** — `isFresh(lastSeenMs, nowMs, ttlMs)`, `ageMs(lastSeenMs, nowMs)`.
Convention: fresh when `nowMs - lastSeenMs < ttlMs` (strictly less). Exactly at TTL
is stale. Documented in the file so `FreshnessRing` does not invent its own boundary.

---

### Chapter 6: Go — Firmware Variance Resolution

**Added after ADR-0007.** A Go chapter appended after the TypeScript ones because it
was scheduled later, not because it belongs to that track. Both functions below are
pure functions over generated constants with no I/O — this tier's charter exactly.

**File:** `internal/codec/firmware.go`

#### 6a — Mode name resolution, three ordered layers

**This supersedes the adversarial review's suggestion** at
`tier-0-3-adversarial-review.md:607-611` — *"schedule a minimal ArduCopter mode table in
Tier 1 (it is a map literal)"* — and the identical note at
`qgc-missionplanner-analysis.md:25-31`. The diagnosis was right and the remedy is not
needed:

| Layer | Source | Available when |
|---|---|---|
| (a) | `AVAILABLE_MODES.mode_name` (#435) | vehicle supplies it — ArduPilot ≥ 4.7.0 |
| (b) | Generated dialect enum `String()`, selected by `(MavAutopilot, MavType)` | always, for known vehicle families |
| (c) | empty string | neither |

Layer (b) needs no table written. `gomavlib v3.3.5` already generates `COPTER_MODE`,
`PLANE_MODE`, `ROVER_MODE`, `SUB_MODE` and `TRACKER_MODE` in `pkg/dialects/ardupilotmega`,
each with a `String()` method (`enum_copter_mode.go:151`, `enum_plane_mode.go:147`,
`enum_rover_mode.go:99`, `enum_sub_mode.go:87`, `enum_tracker_mode.go:71`).

```go
// AvailableMode is one decoded AVAILABLE_MODES (#435) entry.
//
// A Go type, not a proto — deliberately, and for the reason this repo already
// wrote down at internal/vehicle/event.go:26-33 about Warning: nothing streams
// it to a client. It feeds flight_mode_name, which is already on the wire.
// Adding a proto message commits the wire contract to buf breaking forever for a
// shape no client has had to render. Promote it if a mode picker ever needs the
// Properties bits client-side.
type AvailableMode struct {
    CustomMode uint32
    ModeName   string
    Properties uint32 // MavModeProperty bitmask; NOT_USER_SELECTABLE, ADVANCED
}

// ResolveFlightModeName returns the human-readable mode name, or "" when it
// cannot be resolved. It never fabricates a name.
func ResolveFlightModeName(
    autopilot gcsv1.MavAutopilot,
    vehicleType gcsv1.MavType,
    customMode uint32,
    available []AvailableMode, // layer (a); nil when the vehicle sent none
) string
```

`MavModeProperty` *is* mirrored in `types.proto`, because the bits gate UI behaviour and
will cross the wire once a mode picker exists. The container around them does not need to
yet.

**Why a map literal is the wrong shape**, stated so it does not get re-proposed: this repo
already closed the identical question for vehicle classification —
`order-of-operations.md`, *"Derived from the generated `MavType` constants … never from a
retyped integer list"* — after a hand-copied pre-2019 table mapped `ROCKET(9)` and
`GROUND_ROVER(10)` onto `KITE` and `FLAPPING_WING`. A typed mode table is the same artefact
one enum over. Importing the generated constants makes a dialect bump a compile error
rather than a silent mis-mapping.

**Layer (c) returns empty and that is a designed outcome**, not a failure path. Cockpit's
`PX4.mode()` returns `MANUAL` unconditionally because the subclass never overrode it — a
plausible wrong answer the type system cannot catch. An empty string is checkable; a
fabricated name is not.

**Required fixtures:**

| Case | Expectation |
|---|---|
| `AVAILABLE_MODES` present, `custom_mode` matches an entry | that entry's `mode_name` |
| `AVAILABLE_MODES` present, `custom_mode` matches nothing | fall through to (b), **not** empty |
| No `AVAILABLE_MODES`, ARDUPILOTMEGA + QUADROTOR, `custom_mode` 4 | `"GUIDED"` via `COPTER_MODE.String()` |
| No `AVAILABLE_MODES`, ARDUPILOTMEGA + FIXED_WING, `custom_mode` 15 | `"GUIDED"` via `PLANE_MODE.String()` |
| No `AVAILABLE_MODES`, `MAV_AUTOPILOT_PX4` | `""` — we have no PX4 mode enum and must not guess |
| No `AVAILABLE_MODES`, unknown `custom_mode` for a known family | `""`, never the integer rendered as text |
| `NOT_USER_SELECTABLE` set on a mode | name still resolves; the flag is a UI gate, not a name gate |

The PX4 row is the regression guard for the failure in §8 of
`cockpit-blueos-analysis-ii.md`. Assert empty, and assert it deliberately.

#### 6b — Capability-derived parameter cast

`PARAM_VALUE.param_value` is a float on the wire. How an integer parameter is packed into
it is declared, not fixed: `MAV_PROTOCOL_CAPABILITY_PARAM_ENCODE_BYTEWISE` (16) and
`PARAM_ENCODE_C_CAST` (131072) are mutually exclusive, and exactly one should be set.

```go
// DecodeParamValue reinterprets the wire float per the declared encoding.
// Returns ErrUnknownParamEncoding when neither capability bit is set.
func DecodeParamValue(
    raw float32,
    paramType gcsv1.MavParamType,
    caps uint64, // VehicleCapabilities.capability_flags
) (float64, error)
```

**Unknown is an error, not a default.** Decoding BYTEWISE data under a C_CAST assumption
yields a finite, plausible, wrong number — a bit pattern reinterpreted as a magnitude — so
nothing downstream can detect it and this tier's own "NaN never returned" gate passes.
`LOG_BITMASK`, Tier 8b's first write target
(`tier-8-write-transactions.md:81-98`), goes straight through this path.

**Required fixtures:** BYTEWISE with an `INT32` value; C_CAST with the same value, asserting
a *different* result; `REAL32` under both, asserting an *identical* result (floats are
unaffected by the encoding, and a test that does not pin this will not notice a codec that
casts everything); neither bit set → `ErrUnknownParamEncoding`; both bits set → error, not
a preference (a vehicle declaring both is malformed and picking one hides it).

**Exit-gate additions:** both functions table-tested; the PX4-empty case and the
neither-bit-set error case asserted explicitly rather than falling out of a happy path.

---

## Tier Exit Gate

```
make gate-tier-1
```

- `go test -race ./internal/codec/...` passes against golden byte vectors
- `pnpm vitest run src/logic` passes against named fixtures
- NaN never returned from any resolver (asserted in tests)
- Every resolver output carries a `source` naming the MAVLink message used
- Position output carries `altRef`; a GPS_RAW fallback is not reported as relative altitude
- Stall gate: copter at 10 m/s with no airspeed → nonzero forward trajectory;
  fixed-wing at 10 m/s with airspeed 10 and stall 14 → zero forward progress;
  **VTOL hovering with no airspeed → nonzero trajectory** (the regression the
  vehicle-type gate caused)
- NED flip: `vzMs = +5.0` (descending) → `climbMps = -5.0`; VFR_HUD climb passes through
- No goroutine leaks: every node created in a test is closed (`goleak`). Note that
  `Node.Initialize()` starts three goroutines, so "no goroutines" is not the claim —
  "none leaked" is.
