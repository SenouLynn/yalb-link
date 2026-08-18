# Codec Capability Matrix

Decode and encode coverage for `internal/codec`. Every cell names an evidence
artifact before it can be marked `complete`.

**Status values:** `unstarted` | `pending` (fixture exists, test not written) |
`complete` (test passes with fixture) | `blocked` (dependency missing)

**Enforcement is two layers**, and the second is the one that matters:

1. `scripts/check-matrix.sh` fails the build on any `unstarted` row. This only
   checks that somebody typed a word.
2. `TestMatrixCoverage` asserts the case IDs below equal the keys of
   `telemetryDecoders` ∪ `protocolDecoders` ∪ `SendFamilies`. A decoder without
   a row fails; a row without a decoder fails. This is what stops the matrix
   being aspirational.

Both run in `make gate-tier-2`.

Fixtures live in `contracts/mavlink/`, regenerable via
`scripts/gen_mavlink_fixtures.py`. Golden byte tests feed those bytes through a
real `gomavlib` node over `net.Pipe` — never a hand-crafted `EventFrame`, which
would bypass STX detection, length extraction, CRC_EXTRA validation and dialect
struct population, i.e. everything worth testing.

---

## Receive — Telemetry (13 families → TelemetryEvent)

| Case ID | MAVLink Message | ID | Dialect | Behavior | Evidence Artifact | Status | Notes |
|---|---|---|---|---|---|---|---|
| RECV-HEARTBEAT | HEARTBEAT | 0 | minimal | Decode → HeartbeatState (**not** TelemetryEvent) | contracts/mavlink/heartbeat_v2.bin | complete | No oneof variant exists; drives the fleet fold via `DecodeHeartbeat`. Armed = base_mode bit 7 |
| RECV-SYS-STATUS | SYS_STATUS | 1 | common | Decode → SystemStatus | contracts/mavlink/sys_status_v2.bin | complete | |
| RECV-GPS-RAW | GPS_RAW_INT | 24 | common | Decode → GpsRaw | contracts/mavlink/gps_raw_int_v2.bin | complete | Altitude is MSL, never relative |
| RECV-ATTITUDE | ATTITUDE | 30 | common | Decode → Attitude | contracts/mavlink/attitude_v2.bin | complete | Radians in, radians out — no conversion |
| RECV-GLOBAL-POSITION | GLOBAL_POSITION_INT | 33 | standard | Decode → GlobalPosition | contracts/mavlink/global_position_int_v2.bin | complete | vz positive down; hdg kept in centidegrees |
| RECV-MISSION-CURRENT | MISSION_CURRENT | 42 | common | Decode → MissionCurrent | contracts/mavlink/mission_current_v2.bin | complete | |
| RECV-NAV-CONTROLLER | NAV_CONTROLLER_OUTPUT | 62 | common | Decode → NavControllerOutput | contracts/mavlink/nav_controller_output_v2.bin | complete | |
| RECV-VFR-HUD | VFR_HUD | 74 | common | Decode → VfrHud | contracts/mavlink/vfr_hud_v2.bin | complete | heading is int16; climb already positive-up |
| RECV-RADIO-STATUS | RADIO_STATUS | 109 | common | Decode → RadioStatus | contracts/mavlink/radio_status_v2.bin | complete | txbuf is the uplink back-pressure signal |
| RECV-BATTERY-STATUS | BATTERY_STATUS | 147 | common | Decode → BatteryStatus | contracts/mavlink/battery_status_v2.bin | complete | 65535 cell slots dropped, not carried as 65.535 V |
| RECV-EKF | EKF_STATUS_REPORT | 193 | ardupilotmega | Decode → EkfStatusReport | contracts/mavlink/ekf_status_report_v2.bin | complete | Only family requiring the ardupilotmega dialect |
| RECV-HOME-POSITION | HOME_POSITION | 242 | common | Decode → HomePosition | contracts/mavlink/home_position_v2.bin | complete | |
| RECV-STATUSTEXT | STATUSTEXT | 253 | common | Decode → StatusText | contracts/mavlink/statustext_v2.bin | complete | |

## Receive — Transactions (5 families → ProtocolEvent)

These correlate against an in-flight request registry rather than folding into
vehicle state, so they decode to a different envelope and the evidence shows the
envelope as well as the fields.

| Case ID | MAVLink Message | ID | Behavior | Evidence Artifact | Status |
|---|---|---|---|---|---|
| RECV-PARAM-VALUE | PARAM_VALUE | 22 | Decode → ProtocolEvent.param_value | contracts/mavlink/param_value_v2.bin | complete |
| RECV-MISSION-COUNT | MISSION_COUNT | 44 | Decode → ProtocolEvent.mission_count | contracts/mavlink/mission_count_v2.bin | complete |
| RECV-MISSION-ACK | MISSION_ACK | 47 | Decode → ProtocolEvent.mission_ack | contracts/mavlink/mission_ack_v2.bin | complete |
| RECV-MISSION-ITEM | MISSION_ITEM_INT | 73 | Decode → ProtocolEvent.mission_item | contracts/mavlink/mission_item_int_v2.bin | complete |
| RECV-COMMAND-ACK | COMMAND_ACK | 77 | Decode → ProtocolEvent.command_ack | contracts/mavlink/command_ack_v2.bin | complete |

## Unit Normalisation

One row per field that changes units between wire and proto. These are the cases
a field-by-field decode test passes and a human still gets wrong. Each is
asserted against the `expected_proto` block of its fixture's `.json` companion.

| Case ID | Field | Wire | Proto | Status |
|---|---|---|---|---|
| NORM-LAT | GLOBAL_POSITION_INT.lat | int32 degE7 | double degrees | complete |
| NORM-ALT-REL | GLOBAL_POSITION_INT.relative_alt | int32 mm | float metres | complete |
| NORM-VEL | GLOBAL_POSITION_INT.vx/vy/vz | int16 cm/s | float m/s | complete |
| NORM-GPS-ALT | GPS_RAW_INT.alt | int32 mm MSL | float metres MSL | complete |
| NORM-MISSION-XY | MISSION_ITEM_INT.x/y | int32 degE7 | double degrees | complete |
| NORM-RETAINED | GLOBAL_POSITION_INT.hdg, GPS_RAW_INT.cog/vel | centidegrees, cm/s | **unchanged** | complete |
| NORM-PARAM-BYTEWISE | PARAM_VALUE.param_value, integer types | float bit pattern | int64 via byte reinterpretation | blocked |
| NORM-PARAM-C-CAST | PARAM_VALUE.param_value, integer types | float magnitude | int64 via numeric cast | blocked |

`NORM-RETAINED` is the deliberate non-conversion. The 65535 unknown sentinel is
defined in wire units; dividing by 100 turns it into 655.35, which no consumer
can recognise. These three fields keep both their units and their wire-unit
names all the way to the resolver that rejects the sentinel.

**The two parameter-encoding rows are one case, not two.** `NORM-PARAM-BYTEWISE` and
`NORM-PARAM-C-CAST` are mutually exclusive readings of the same four bytes, selected by
`VehicleCapabilities.capability_flags` (bits 16 and 131072). The test that matters feeds
the **same** wire value through both and asserts the results *differ* — a codec that
ignores the capability and casts unconditionally passes each row in isolation. A third
case asserts that neither bit set returns an error rather than a number. See ADR-0007 R3;
both are `blocked` on Tier 1 Chapter 6b, which owns the decoder.

## Send (Encode)

Encoders are pure — they return `message.Message`, never bytes, and never choose
a destination. Addressing is `Node.WriteTo`'s job, which keeps the
never-broadcast rule in one place.

| Case ID | MAVLink Message | ID | Behavior | Evidence Artifact | Status | Notes |
|---|---|---|---|---|---|---|
| SEND-HEARTBEAT | HEARTBEAT | 0 | GCS keepalive, fixed fields | contracts/mavlink/heartbeat_gcs_out.bin | complete | gomavlib emits this itself; encoder exists for completeness |
| SEND-PARAM-REQUEST-READ | PARAM_REQUEST_READ | 20 | Read one parameter | contracts/mavlink/param_request_read_out.bin | complete | index -1 = look up by name |
| SEND-PARAM-REQUEST-LIST | PARAM_REQUEST_LIST | 21 | Request full parameter set | contracts/mavlink/param_request_list_out.bin | complete | |
| SEND-PARAM-SET | PARAM_SET | 23 | Write one parameter | contracts/mavlink/param_set_out.bin | complete | |
| SEND-MISSION-COUNT | MISSION_COUNT | 44 | Open a mission upload | contracts/mavlink/mission_count_out.bin | complete | |
| SEND-MISSION-CLEAR-ALL | MISSION_CLEAR_ALL | 45 | Erase a mission | contracts/mavlink/mission_clear_all_out.bin | complete | Wired Tier 8; not surfaced in UI until after Tier 10 |
| SEND-MISSION-ACK | MISSION_ACK | 47 | Acknowledge a transfer | contracts/mavlink/mission_ack_out.bin | complete | |
| SEND-MISSION-REQUEST-INT | MISSION_REQUEST_INT | 51 | Request one item | contracts/mavlink/mission_request_int_out.bin | complete | |
| SEND-MISSION-ITEM-INT | MISSION_ITEM_INT | 73 | One mission item | contracts/mavlink/mission_item_int_out.bin | complete | x/y convert to degE7; z stays float metres |
| SEND-CMD-LONG | COMMAND_LONG | 76 | Arm/disarm, mode change | contracts/mavlink/command_long_arm_out.bin | complete | Pure passthrough — force-arm is rejected in Tier 8, not here |
| SEND-SET-POSITION-TARGET | SET_POSITION_TARGET_GLOBAL_INT | 86 | Guided reposition | contracts/mavlink/set_position_target_global_int_out.bin | complete | type_mask 0xDF8; FORCE_SET bit asserted clear |

## Framing Cases

| Case ID | Behavior | Evidence Artifact | Status |
|---|---|---|---|
| FRAME-V1-ACCEPT | MAVLink v1 frame decoded without error. `OutVersion: V2` governs what we transmit, not what we accept — gomavlib's parser handles v1 inbound. Asserted, not assumed | contracts/mavlink/v1_heartbeat.bin | complete |
| FRAME-V2-UNSIGNED | MAVLink v2 unsigned frame decoded; sequence number read via `GetSequenceNumber` | contracts/mavlink/heartbeat_v2.bin | complete |
| FRAME-BAD-CRC | Payload byte corrupted after framing. Asserts (a) no TelemetryEvent, (b) no panic, (c) parse-error counter incremented. Does **not** assert an error return from `Decode` — it returns a zero `Decoded` for parse errors and unhandled IDs alike | contracts/mavlink/bad_crc.bin | complete |
| FRAME-TRUNCATED | Frame cut mid-payload. **Behaves differently from FRAME-BAD-CRC** — see below | contracts/mavlink/truncated.bin | complete |
| FRAME-ROUTE-ON-RECEIVE | A vehicle becomes addressable only after a frame is heard from it | contracts/mavlink/heartbeat_v2.bin | complete |
| FRAME-REJECT-UNKNOWN-LINK | `WriteTo` an unopened link returns `ErrUnknownLink` and never falls back to a broadcast | (no fixture — pure API assertion) | complete |
| FRAME-CLOSE-WITHOUT-CONSUMER | `Close` returns even when nothing drains `Events()` | (no fixture — pure API assertion) | complete |

### Two corrections found by execution

**FRAME-TRUNCATED does not emit a parse error.** The Tier 2 plan asserted it
behaves like a bad CRC — `EventParseError`, counter incremented. It does not.
gomavlib's parser is stream-oriented, so a frame cut mid-payload with no
following bytes leaves it *waiting for the remainder*: no event, no error, no
counter movement. The observable contract is only "no frame surfaces, no panic".
The damaging case is a truncated frame followed by more traffic, where the parser
consumes the next frame's bytes as this one's remainder and both are lost — a
live-link concern for Tier 5, not something a static fixture can express.

**A corrupt byte must land in the payload, not the header.** The first version of
the bad-CRC fixture flipped byte 8, which is inside the v2 msgid field (the
header is 10 bytes: STX, len, incompat, compat, seq, sysid, compid, msgid×3).
That produced a self-consistent frame with an *unknown message ID*, which
gomavlib surfaces as a `MessageRaw` — a different path that exercises nothing
about CRC validation. The generator now corrupts index 10, the first payload
byte.

---

## Planned — Capability Negotiation (ADR-0007)

Families this codec will handle once ADR-0007's resolution path is built. **None has a
decoder yet**, which is exactly why this section exists separately.

> **Do not move these rows into the tables above, and do not give the ID column a bare
> number.** `TestMatrixCoverage` binds a row to a dispatch-table key by matching
> `| RECV-… | anything | <digits> |`, and it asserts the two sets are *equal* — a row with
> a numeric ID and no decoder fails the build just as a decoder with no row does. The
> status column is not consulted, so marking a premature row `pending` does not help.
> The ID column here is written `#148` / `cmd 512` deliberately: it does not match, so
> these rows document intent without asserting code that does not exist. Give a row its
> bare number in the table above on the commit that adds its decoder, not before.

| Case ID | MAVLink Message | ID | Behavior | Blocked on | Status |
|---|---|---|---|---|---|
| RECV-AUTOPILOT-VERSION | AUTOPILOT_VERSION | #148 | Decode → `VehicleCapabilities`; 21-flag bitmask carried raw plus decomposed | Tier 5 — requested at discovery, folded into `vehicle.State` | blocked |
| RECV-AVAILABLE-MODES | AVAILABLE_MODES | #435 | Decode → `[]codec.AvailableMode` (a Go type; see Tier 1 ch.6a for why it is not a proto) | Tier 1 ch.6a | blocked |
| RECV-CURRENT-MODE | CURRENT_MODE | #436 | Decode → current `custom_mode` + `intended_custom_mode`; the two differ when a failsafe overrode the operator | Tier 1 ch.6a | blocked |
| SEND-REQUEST-MESSAGE | COMMAND_LONG payload | cmd 512 | `MAV_CMD_REQUEST_MESSAGE` requesting #148 and #435 | Tier 5 | blocked |

**`SEND-REQUEST-MESSAGE` is not a new send family**, and the distinction is load-bearing
rather than pedantic. `MAV_CMD_REQUEST_MESSAGE = 512` is a *command ID* carried inside
`COMMAND_LONG` (#76), not a message ID. `SendFamilies` is keyed by message ID, and
`SEND-CMD-LONG` (76) is already `complete`, so the encoder already exists — what is missing
is a caller. A row asserting message family 512 would invent a family that must not exist
and would break `TestMatrixFamilyCounts`, which pins `len(SendFamilies) == 11`.

`AVAILABLE_MODES_MONITOR` (#437) is deliberately absent. It only matters when a vehicle's
mode set changes *after* boot; nothing we support does that yet, and a row for it would be
blocked on a condition rather than on work.

**Fixtures.** All four are generable by `scripts/gen_mavlink_fixtures.py` — every message
is in `common`, so no SITL capture is needed. `AUTOPILOT_VERSION` needs two fixtures, not
one: `uid2` is a MAVLink 2 extension field, so a frame from older firmware carries `uid`
only, and a decoder that reads `uid2` unconditionally gets 18 zero bytes and treats an
identifiable vehicle as anonymous.

---

## Goroutine Discipline

`Node.Initialize` starts three goroutines, so the claim under `goleak` is "none
leaked", not "none running". `TestMain` runs `goleak.VerifyTestMain`, so every
node a test creates must reach `Close`.

One defect this surfaced: `pump` forwarded events with a bare send on an
unbuffered channel, so a consumer that stopped reading wedged the goroutine —
and since `Close` waits on it, the node could never be shut down. The send is now
guarded by a `select` on a `closing` channel, and
`TestCloseReturnsWithNoEventConsumer` pins it.
