# Tier 7 — Read Protocol Transactions

## Overview

Read before write. Prove parameter and mission download as pure transaction folds, connect to live SITL, and surface data in the UI. Nothing in this tier sends a write that changes vehicle configuration. The fold-first pattern ensures that a SITL session can be replayed from a recording without producing any outbound bytes — a property that is verified as part of the exit gate.

## Dependencies

- Tier 6 Connect services live
- Tier 5 UDP transport and Redis wired
- At least one parameter exists in the vehicle (ArduCopter always has ~500)

## Chapters

---

### Chapter 1: Parameter Read Fold (pure)

**Goal:** A pure fold that drives the MAVLink parameter read protocol — both single-parameter reads and full-list downloads.

**File:** `internal/transactions/parameter.go`

**Protocol: single read**
- Send: `PARAM_REQUEST_READ` with `param_id` or `param_index`
- Receive: `PARAM_VALUE` echo with matching `param_id`
- GCS identity: sysId=255, compId=190 (explicit constructor arg, not env var)

**Protocol: full list download**
- Send: `PARAM_REQUEST_LIST`
- Receive: `PARAM_VALUE` stream; each message carries `param_index` and `param_count`
- Handle out-of-order delivery (vehicles may send params out of sequence)
- Gap detection: bitmask of received indices; after quiescence (500ms no new params), re-request missing indices via `PARAM_REQUEST_READ`
- Duplicate suppression: `param_index` already received → drop silently
- Retry on route-loss: if 3s elapses with no `PARAM_VALUE` after requesting, emit TRANSACTION_FAILED

**On list completion:**
- Write `HSET params:<sysId> <param_id> <value>` for all params in one pipeline call
- Then `EXPIRE params:<sysId> 3600`

**Fold signature:**
```go
func ParameterListFold(state ParamListState, event *TelemetryEvent, nowMs int64) (ParamListState, []TransactionEvent)
```

**`time.Now()` must not appear.** All time decisions use `nowMs`.

---

### Chapter 2: Mission Download Fold (pure)

**Goal:** A pure fold that drives the MAVLink mission download protocol.

**File:** `internal/transactions/mission.go`

**Protocol:**
```
GCS sends:  MISSION_REQUEST_LIST
Vehicle:    MISSION_COUNT (n items)
GCS sends:  MISSION_REQUEST_INT for item 0
Vehicle:    MISSION_ITEM_INT (item 0)
GCS sends:  MISSION_REQUEST_INT for item 1
...
Vehicle:    MISSION_ITEM_INT (item n-1)
GCS sends:  MISSION_ACK (MAV_MISSION_ACCEPTED)
```

**Per-item retry:** 2s timeout per `MISSION_REQUEST_INT`. Retry 3 times before TRANSACTION_FAILED.

**Gap detection:** track received item indices; if MISSION_ITEM_INT arrives out of order, buffer it and request the missing item.

**On complete:** return list of `MissionItem` protos in sequence order.

---

### Chapter 3: ParameterService

**Goal:** Expose parameter read/list over Connect.

**File:** `internal/services/parameter.go`

**`ListParameters(vehicleId)`:**
- Initiate `PARAM_REQUEST_LIST` fold
- Stream `ParameterValue` protos as each `PARAM_VALUE` arrives
- On completion: write to Redis (Chapter 1)

**`GetParameter(vehicleId, paramId)`:**
- Initiate `PARAM_REQUEST_READ` fold for single param
- Return `ParameterValue` proto

**`GetCachedParameters(vehicleId)`:**
- `HGETALL params:<sysId>` → return all cached params
- Used for initial panel load without triggering a new vehicle request

---

### Chapter 4: MissionService (read)

**Goal:** Expose mission download over Connect.

**File:** `internal/services/mission.go`

**`DownloadMission(vehicleId)`:**
- Initiate `MISSION_REQUEST_LIST` → drive mission download fold
- Stream `MissionItem` protos as items arrive
- On `MISSION_ACK`: close stream

---

### Chapter 5: ParametersPanel (read-only)

**Goal:** A searchable list of vehicle parameters. Write capability is added in Tier 8b.

**File:** `frontend/src/components/ParametersPanel.tsx`

**Features:**
- Search by `param_id` (case-insensitive prefix match)
- Type-aware value display: `MavParamType` from `PARAM_VALUE` determines int vs float rendering
  - `MAV_PARAM_TYPE_INT8`, `INT16`, `INT32` → display as integer
  - `MAV_PARAM_TYPE_REAL32` → display with 4 significant figures
- Column: `param_id | value | type | index / count`
- Load from `GetCachedParameters` on mount if cache available; offer "Refresh" button to re-download

---

### Chapter 6: MissionPanel

**Goal:** Display the downloaded mission as a list and overlay waypoints on the map.

**File:** `frontend/src/components/MissionPanel.tsx`

**List view:** columns: `seq | command (decoded MAV_CMD enum name) | lat | lon | alt | param1–4`

**Map overlay:** waypoints rendered as markers on MapPanel when MapPanel is available. MissionPanel does not depend on MapPanel being complete — the list view is sufficient for Tier 7.

**`MAV_CMD` decoding:** derive human-readable name from the uint16 command ID using the proto enum or a local lookup table.

---

## Tier Exit Gate

**Parameters:**
- Capability matrix rows all `complete`: PARAM-LIST-COMPLETE, PARAM-LIST-IDLE, PARAM-READ-NAME, PARAM-READ-INDEX, PARAM-ROUTE-LOSS, PARAM-REPLAY, PARAM-INVALID-TARGET
- Live SITL: `ARMING_CHECK` readable; value present in `redis-cli HGET params:1 ARMING_CHECK`
- Replay of recorded parameter session produces zero outbound bytes

**Mission:**
- Live SITL: 5-waypoint QGC mission → yalb-gcs download → identical item list in order
- Round-trip: download → JSON → field-by-field comparison with QGC `.plan` format passes
- Per-item retry test: simulate missing item 2 → verify re-request sent after 2s quiescence

**UI:**
- ParametersPanel loads with cached params and shows correct value types
- "Refresh" triggers new download and updates panel
- MissionPanel shows waypoints in sequence with decoded command names
