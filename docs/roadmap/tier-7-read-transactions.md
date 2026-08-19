# Tier 7 — Read Protocol Transactions

## Overview

Read before write. Prove parameter and mission download as pure transaction folds, connect to live SITL, and surface data in the UI. Nothing in this tier sends a write that changes vehicle configuration — that sentence, not Principle 3's older "any outbound capability" wording, is the operative rule, and ADR-0010 makes it the general one.

**This tier does send.** `PARAM_REQUEST_READ`, `PARAM_REQUEST_LIST`, `MISSION_REQUEST_LIST` and `MISSION_REQUEST_INT` all go on the wire; they are requests for data, which is read-side. The zero-outbound-bytes property below is scoped to **replay** — a recorded session re-fed through the pure fold produces no bytes, because the fold is where the decisions live. It is not a claim that the live tier is silent.

## Dependencies

- Tier 6 Connect services live
- Tier 5 UDP transport and Redis wired
- At least one parameter exists in the vehicle (ArduCopter always has ~500)
- **`AUTOPILOT_VERSION` folded into vehicle state by Tier 5 Chapter 7.** `GetParameterMetadata`
  selects a vendored set by `flight_sw_version` and ADR-0009 §2's cache predicate is keyed
  on `uid`/`uid2`; both come from #148, which ArduPilot sends only when asked. Until that
  round trip exists, capabilities are permanently unknown and the only reachable exit line
  is the degraded one — "Metadata absent entirely → panel degrades to the raw four-column
  view". Scheduled nowhere before ADR-0010; see `../adr/0010-requests-for-data-are-not-writes.md`

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

> **Open — `MISSION_REQUEST_LIST` (#43) has no encoder and is in no send set.** The protocol
> below opens with it, but it is absent from the eleven-family priority send set in
> `order-of-operations.md`, from the Tier 1 encoder list, and from the capability matrix.
> Adding it moves `TestMatrixFamilyCounts`' `len(SendFamilies) == 11` pin, so it is a
> deliberate change and not an oversight to patch quietly. Raised 2026-08-19; decide before
> this chapter starts.

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
- **Not declared in `services.proto`.** `ParameterService` has `ListParameters`,
  `GetParameter`, `SetParameter` and `GetParameterMetadata` — there is no
  `GetCachedParameters` RPC, so this cannot be called from the frontend as written.
  Resolve one of two ways before Chapter 5: add the RPC (a Tier 0-class contract change,
  now post-codegen and therefore a `buf breaking` event), or serve the cached read as a
  mode of `ListParameters` and delete this entry. The second is preferred — the cache is
  an implementation detail of the same question, and the panel does not need to know which
  path answered it.

**`GetParameterMetadata(vehicleId, paramIds)`:**
- Select the `ParameterMetadataSet` matching the vehicle's reported
  `VehicleCapabilities.flight_sw_version`: nearest vendored version ≤ actual
- If capabilities are not yet known, **fail rather than guess a version** — the metadata
  set is wrong for the vehicle and a parameter editor built on it claims authority it does
  not have (ADR-0007 §5)
- Set `is_exact_match = false` whenever the vendored version is not the vehicle's own
- `param_ids` empty = the whole set; a full Copter set is ~4800 entries, so a panel should
  request the page it is showing

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

The panel is **generated from metadata, not hardcoded**. `parameters.proto` says why the
metadata entered the contract early: *"Without it a ParametersPanel can only show name /
value / type, which is what Tier 7 would otherwise write its tests against."* This chapter
is that test surface, so it consumes `GetParameterMetadata` from day one.

**Data sources:** `ParameterValue` for the live value, `ParameterMetadata` for everything
about what the value *means*, joined client-side on `param_id`. Metadata may be absent for
any given parameter — every field below degrades to the raw display.

**Features:**
- Search by `param_id` (case-insensitive prefix match); also match `human_name`
- Columns: `param_id | human_name | value + units | index / count`
- Type-aware value display: `MavParamType` from `PARAM_VALUE` determines int vs float
  rendering
  - `MAV_PARAM_TYPE_INT8`, `INT16`, `INT32` → display as integer
  - `MAV_PARAM_TYPE_REAL32` → display with 4 significant figures
  - **Integer parameters require the declared encoding.** The float→int reinterpretation
    is a function of `PARAM_ENCODE_BYTEWISE` (16) vs `PARAM_ENCODE_C_CAST` (131072). If
    neither bit is set, render "encoding not declared" — never a number (ADR-0007 §4)
- Units from `units` / `unit_text`; these are the parameter's own units and are **not**
  normalised — the normalise-once rule covers telemetry fields with a known wire unit, and
  a parameter's unit is data, not schema
- `documentation` and `human_name` in a detail view
- **Presence before value on every `optional` numeric.** `range_low`, `range_high` and
  `increment` are proto3 `optional` because 0 is a legal value for all three. Only ~half of
  ArduPilot's parameters declare a range at all, so absent is the common case — a slider
  that reads an unset range as `[0, 0]` clamps the parameter to zero
- `values` map → enum dropdown (label from the map, raw code shown alongside)
- `bitmask` map → bit editor. **Keys are bit indices, not bit values** — render
  `1 << key`. Treating the key as a mask puts every bit one position off
- `user_level` (`"Standard"` / `"Advanced"`) gates an advanced-parameters toggle. Verbatim
  string, not parsed into an enum — it is upstream's vocabulary and upstream may extend it
- Badges, read-only in this tier but the editor in Tier 8b depends on them:
  `reboot_required` (value takes effect only after reboot), `read_only` (no editor at all),
  `volatile_value` (changes on its own — do not cache, do not diff), `calibration`
  (written by a routine, not by hand)
- **Staleness banner when `ParameterMetadataSet.is_exact_match == false`**, naming
  `firmware_version_label` and the vehicle's actual version. This is the field's entire
  reason for existing: Cockpit describes a 4.6 vehicle with 4.3 metadata and nothing
  anywhere says so
- No metadata set at all (capabilities unknown) → fall back to `param_id | value | type |
  index / count` and say the panel is unannotated. Degrade visibly, never silently
- Initial load without triggering a new vehicle request — see the cached-read note in
  Chapter 3; offer a "Refresh" button to re-download

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
- `GetParameterMetadata` returns a set whose `firmware_version` is the nearest vendored
  version ≤ the vehicle's `flight_sw_version`, with `is_exact_match` set correctly for both
  the hit and the miss case
- Capabilities unknown → `GetParameterMetadata` fails rather than returning a guessed set

**Mission:**
- Live SITL: 5-waypoint QGC mission → yalb-gcs download → identical item list in order
- Round-trip: download → JSON → field-by-field comparison with QGC `.plan` format passes,
  against the mapping in `../reference/mission-interchange-formats.md` §1.4. **Scope: the
  `mission` section, `SimpleItem`s only.** A `ComplexItem` round-trip is lossy by design
  (§1.6) — a plan containing one either fails the gate or asserts the loss explicitly; it
  must not pass silently
- The two traps in §1.5 are asserted, not assumed: `null` in `params[0..3]` round-trips as
  NaN and never as 0; `.plan` degrees are **not** put through the wire `degE7` conversion
- The `doJumpId` ↔ `seq` offset (§4.1) is pinned by a real `.plan` file committed as a
  fixture, not by reading QGC's source
- Per-item retry test: simulate missing item 2 → verify re-request sent after 2s quiescence

**UI:**
- ParametersPanel loads with cached params and shows correct value types
- ParametersPanel renders from metadata: an enumerated parameter shows a labelled dropdown,
  a bitmask parameter shows bit labels at the correct positions (`1 << key`), and a
  parameter with no declared range shows no slider
- `is_exact_match == false` surfaces a staleness banner naming both versions
- Metadata absent entirely → panel degrades to the raw four-column view and says so
- "Refresh" triggers new download and updates panel
- MissionPanel shows waypoints in sequence with decoded command names
