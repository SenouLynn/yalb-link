# Tier 8 — Write Protocol Transactions

## Overview

Write capability, ordered by operational risk. The command registry is wired first — it is load-bearing infrastructure shared by every write handler. The sequence (8a → 8b → 8c → 8d → 8e → 8f → 8g) reflects increasing risk and increasing state dependency. MapPanel is built during 8d/8e because guided reposition (8f) requires the operator to click on a map.

## Dependencies

- Tier 7 read transactions complete
- Tier 6 track layer live (required for MapPanel)
- Command registry not yet built (built as Chapter 1 of this tier)

## Chapters

---

### Chapter 1: Command Registry (prerequisite for all writes)

**Goal:** In-flight command tracking. All write handlers register a command before encoding it and deregister on ACK or timeout.

**File:** `internal/command/registry.go`

**Key type:**
```go
type commandKey struct {
    vehicleSysID  uint8
    vehicleCompID uint8
    commandID     uint16
}
```

`vehicleSysID` and `vehicleCompID` are from the MAVLink frame header of the vehicle that will ACK (the vehicle we are commanding, not the GCS).

**Entry:**
```go
type inFlightCommand struct {
    ch     chan *common.MessageCommandAck
    cancel context.CancelFunc
}
```

**Operations:**
- `Register(key, ctx) (<-chan *MessageCommandAck, error)` — returns error if key already in-flight (duplicate command)
- `Deliver(ack *MessageCommandAck, sysID, compID uint8)` — called from the message receive loop on every COMMAND_ACK; matches by frame header sysId/compId + ack.Command field
- `Timeout` handled via `context.WithTimeout(5 * time.Second)` in the caller

**Duplicate key policy:** if a command is already in-flight for the same (vehicle, commandID), return an error to the service layer. The service returns a RESOURCE_EXHAUSTED Connect status. Do not queue; the UI should disable the button while a command is in-flight.

---

### Chapter 2: Dual-Gate Contract

**Goal:** All write handlers enforce two independent gates. Define the pattern once; apply to every write.

**Gates:**
1. **Service-side role gate:** `role >= OPERATOR` from `OperatorContext` (JWT claim). `NoopAuthProvider` returns OPERATOR unconditionally in SITL mode.
2. **Service-side eligibility gate:** vehicle state checked against `VehicleSnapshot` (armed status, mode, EKF flags as appropriate per command)

**UI gate:** button disabled when eligibility conditions are unmet. This is defense-in-depth; the server gate is authoritative.

**Audit:** `AuditEvent` written to `audit:events` Stream after action completes. Never before. Never blocking — write asynchronously after returning the response to the client.

**Audit entry fields:** `operator_id`, `vehicle_id`, `command`, `timestamp_ms`, `result`.

---

### Chapter 3: 8a — Message Interval (lowest risk)

**Goal:** Allow an **operator** to change the rate of a specific MAVLink message on a live
vehicle.

**Scope narrowed by ADR-0010 §4.** This chapter no longer owns *first* telemetry. The
discovery-time request that makes a link produce any telemetry at all is Tier 5 Chapter 7 —
it is read-side, and gating it on the command registry here is what left Tier 6 and Tier 7
with exit criteria they could not meet. What stays here is the deliberate operator action:
the RPC, the registry entry, ACK correlation, the role check and the audit event.

**Protocol:** `COMMAND_LONG` (#76) with `command = MAV_CMD_SET_MESSAGE_INTERVAL (511)`, `param1 = message_id`, `param2 = interval_us (-1 = disable, 0 = default rate)`.

**Service:** `CommandService.SetMessageInterval(vehicleId, messageId, intervalUs)`

**Risk profile:** Reversible. Affects only telemetry rate. No vehicle state change. No eligibility gate needed beyond operator role.

**ACK correlation:** uses command registry with `commandID = 511`.

---

### Chapter 4: 8b — Parameter Write

**Goal:** Allow the GCS to write a parameter value to the vehicle. Start with one low-consequence parameter.

**Protocol:** `PARAM_SET` (#23) → vehicle echoes `PARAM_VALUE` with updated value. `PARAM_SET` uses echo-back confirmation, **not** `COMMAND_ACK`. No registry entry needed.

**Confirmation:** wait for `PARAM_VALUE` echo where `param_id` matches the sent value. Timeout: 3s. If no matching echo arrives, report failure.

**Service:** `ParameterService.SetParameter(vehicleId, paramId, value, type)`

**Gate:** disarmed + operator role. Do not allow parameter writes while armed.

**First parameter to target:** `LOG_BITMASK` (low consequence, survives power cycle harmlessly).

**Post-write:** update `params:<sysId>` HSET entry for the written param. Write audit event.

**UI:** extend ParametersPanel with an edit mode toggle. Edit a single cell at a time. Inline confirmation dialog showing old value → new value before sending.

---

### Chapter 5: 8c — Mission Upload

**Goal:** Allow the GCS to upload a mission to the vehicle.

**Protocol (GCS-driven):**
```
GCS sends:   MISSION_COUNT
Vehicle:     MISSION_REQUEST_INT (item 0)
GCS sends:   MISSION_ITEM_INT (item 0)
Vehicle:     MISSION_REQUEST_INT (item 1)
...
GCS sends:   MISSION_ITEM_INT (item n-1)
Vehicle:     MISSION_ACK
```

**Service:** `MissionService.UploadMission(UploadMissionRequest) returns (MissionAck)` —
**unary**, taking `{ target, mission_type, repeated items }`.

This chapter said "client-streaming Connect RPC" until 2026-08-19, which had been false
since Tier 0. `@connectrpc/connect-web` supports unary and server-streaming only, so a
client-streaming RPC is uncallable from the browser; `proto/gcs/v1/services.proto:152` is
unary and `scripts/check-tier-0.sh` greps for `rpc X(stream ` so it cannot regress.
Missions are bounded — hundreds of items at most — so one request costs nothing, and the
backend still runs the full MAVLink handshake below. Per ADR-0008 §3 the proto was
authoritative and this chapter was the defect.

**Per-item retry:** 2s timeout per `MISSION_REQUEST_INT`; 3 retries before ABORTED.

**ACK result handling:** `MAV_MISSION_ACCEPTED (0)` = success; map other `MAV_MISSION_RESULT` values to descriptive Connect errors (`MAV_MISSION_ERROR`, `MAV_MISSION_UNSUPPORTED_FRAME`, etc.).

**Round-trip test:** upload → download → assert item list is identical (field-by-field).

---

### Chapter 6: 8d — Mode Change

**Goal:** Allow the GCS to change the vehicle's flight mode.

**Protocol:** `COMMAND_LONG` (#76) with `command = MAV_CMD_DO_SET_MODE (176)`, `param1 = base_mode | MAV_MODE_FLAG_CUSTOM_MODE_ENABLED (128)`, `param2 = custom_mode`.

**Mode values by vehicle type:**
- ArduCopter GUIDED = 4
- ArduCopter STABILIZE = 0, LOITER = 5, RTL = 6, LAND = 9
- ArduPlane GUIDED = 15

**Service:** `CommandService.SetMode(vehicleId, mode)`

**Gate:** disarmed + operator role + mode in vehicle-type allowlist.

**Post-condition verification:** poll `VehicleSnapshot.custom_mode` until it matches or 5s timeout elapses.

**ACK correlation:** command registry with `commandID = 176`.

---

### Chapter 7: MapPanel (required before 8f)

**Goal:** A live map showing vehicle positions and tracks. Required before guided reposition can be validated.

**File:** `frontend/src/components/MapPanel.tsx`

**Stack:** MapLibre GL JS + PMTiles (self-hosted tile source). No external API key.

**Features:**
- Vehicle markers at current lat/lon; color-coded by freshness
- Track polyline from `TrackService.WatchTracks` (accumulated positions)
- Click handler: emits a `{latDeg, lonDeg}` event consumed by GuidedRepositionPanel

**FleetView:**

**File:** `frontend/src/components/FleetView.tsx`
- Vehicle roster (list of VehicleChip components) + MapPanel basemap
- Vehicle cards show: sysId, type icon, armed status, mode name, battery %, freshness dot
- Selecting a vehicle in the roster focuses the map on that vehicle

**Data source:** `useAdapter().watchTracks()` — no MAVLink imports in map code.

---

### Chapter 8: 8e — Arm / Disarm

**Goal:** Allow the GCS to arm or disarm the vehicle.

**Protocol:** `COMMAND_LONG` (#76) with `command = MAV_CMD_COMPONENT_ARM_DISARM (400)`, `param1 = 1.0` (arm) or `0.0` (disarm), `param2 = 0` (no force — field was removed from proto in Tier 0).

**Arm gate (all must be true):**
- `custom_mode == GUIDED`
- `armed == false`
- operator role
- `EkfFlags & EKF_ATTITUDE (bit 0)` set
- `EkfFlags & EKF_UNINITIALIZED (bit 10)` clear

**Disarm gate:**
- operator role
- not already disarmed

**No automatic retry.** Exactly once per user action. On failure, user must confirm and retry manually.

**Post-condition:** `HEARTBEAT.base_mode & MAV_MODE_FLAG_SAFETY_ARMED (128)` observed after ACK.

**Inherited from Tier 5 Chapter 3 — the GCS failsafe assertion.** Tier 5 configures the GCS
heartbeat at 1 Hz and proves its *cardinality* (the count does not scale with vehicle
count). It cannot prove the heartbeat is doing its job, because that takes a command the
vehicle will refuse — and an arm command is the first one this build order permits. Run it
here, once, while the command already exists:

1. `HeartbeatDisable: true` → send `MAV_CMD_COMPONENT_ARM_DISARM` → confirm STATUSTEXT
   "GCS Failsafe" or COMMAND_ACK with result DENIED.
2. `HeartbeatDisable: false, HeartbeatPeriod: time.Second` → send the same command →
   confirm COMMAND_ACK ACCEPTED.

This is the one assertion in the roadmap that Principle 3 genuinely blocked from landing
earlier: arming changes vehicle state, unlike the Tier 5 requests ADR-0010 reclassified.

**UI:** CommandButton labeled "ARM" / "DISARM". Disabled when gate conditions unmet. Shows EKF status indicator when EKF gate is blocking.

---

### Chapter 9: 8f — Guided Reposition

**Goal:** Allow the operator to click a map point and reposition the vehicle there.

**Protocol:** `SET_POSITION_TARGET_GLOBAL_INT` (#86).

**Key fields:**
- `type_mask = 0xDF8` (3576) — position only; velocity/acceleration/force/yaw all ignored
- `coordinate_frame = MAV_FRAME_GLOBAL_RELATIVE_ALT_INT (6)` — altitude above home
- `lat_int = latDeg × 1e7` (int32)
- `lon_int = lonDeg × 1e7` (int32)
- `alt = altAgl` (float32, meters above home)

**Service:** `CommandService.SetPositionTargetGlobal(vehicleId, latDeg, lonDeg, altAgl)`

**Server-side bounds validation (not optional — UI gate alone is bypassable):**
- Target distance from home > 500m → INVALID_ARGUMENT
- Target altitude AGL < 2m → INVALID_ARGUMENT

**Gate:** armed + `custom_mode == GUIDED` + operator role.

**Read-back:** poll `VehicleSnapshot.position` until within 2m of target or 30s timeout → return SUCCESS or DEADLINE_EXCEEDED.

**ACK:** `SET_POSITION_TARGET_GLOBAL_INT` does not generate a `COMMAND_ACK`. Acceptance is inferred from the vehicle beginning to move toward the target. Read-back via position polling is the confirmation mechanism.

---

### Chapter 10: 8g — Guided Workflow Lifecycle

**Goal:** The full autonomous flight lifecycle as a pure fold.

**Sequence:** ARM → set GUIDED → TAKEOFF → (fly / reposition) → LAND → DISARM

**Commands:**
- TAKEOFF: `COMMAND_LONG` with `MAV_CMD_NAV_TAKEOFF (22)`, `param7 = altitudeAgl`
- LAND: `COMMAND_LONG` with `MAV_CMD_NAV_LAND (21)` or mode change to LAND

**Pure fold:** port `flight-path-hud/packages/gcs-core/src/guidedWorkflow.ts` to Go as `internal/transactions/guided_workflow.go`. Semantic trace from `flight-path-hud/contracts/semantics/` governs state transitions.

**UI:**
- `GuidedWorkflowPanel.tsx` — state machine display; buttons for each valid transition
- `GuidedRepositionPanel.tsx` — map click → target input → send button with read-back progress

---

## Tier Exit Gate

**Per write family:**
- Pure transaction fold unit tested with ordered semantic trace
- Golden MAVLink byte vector matches SITL capture
- SITL acceptance test passes (Copter + Plane where mode values differ)
- Replay of recorded session produces zero outbound bytes
- AuditEvent written to Redis Stream with operator identity

**Guided reposition end-to-end:**
- Arm copter → switch to GUIDED mode → map click → copter moves to target point
- Server rejects target > 500m from home with INVALID_ARGUMENT
- Server rejects target < 2m AGL with INVALID_ARGUMENT

**Guided workflow:**
- Full ARM → TAKEOFF → hover → LAND → DISARM sequence completes against SITL
