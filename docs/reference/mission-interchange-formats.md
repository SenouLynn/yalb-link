# Mission Interchange Formats

**Purpose:** define the mapping the Tier 7 exit gate requires —

> Round-trip: download → JSON → field-by-field comparison with QGC `.plan` format passes
> (`../roadmap/tier-7-read-transactions.md`)

Before this document nothing in the repo defined that mapping, so the gate could not be
met. It is a written gate rather than a `make` target, so nothing was failing in CI; it was
simply unmeetable.

**Provenance.** Everything in the `.plan` sections was read from QGroundControl `master`
source on **2026-08-18** via the GitHub API, constant by constant — file and symbol named at
each claim. The `.waypoints` section is carried forward from the earlier Cockpit reference
analysis (Mission Planner source) and is **not re-verified here**; it is marked accordingly.

---

## 1. `.plan` — QGroundControl plan file

### 1.1 Envelope

Written by `JsonParsing::saveQGCJsonFileHeader()`
(`src/Utilities/Parsing/Json/JsonParsing.cc:286-291`), which sets three keys on the root
object. All three are required on load (`:333`, `:298`).

| Key | Value | Source |
|---|---|---|
| `groundStation` | `"QGroundControl"` | `JsonParsing.cc:198-199` |
| `fileType` | `"Plan"` | `PlanMasterController.h` — `kPlanFileType` |
| `version` | `1` | `PlanMasterController.h` — `kPlanFileVersion` |

Three section objects hang off the root (`PlanMasterController.h`):

| Key | Constant | Section version |
|---|---|---|
| `mission` | `kJsonMissionObjectKey` | `2` (`MissionController.h` — `_missionFileVersion`) |
| `geoFence` | `kJsonGeoFenceObjectKey` | `2` (`GeoFenceController.h` — `_jsonCurrentVersion`) |
| `rallyPoints` | `kJsonRallyPointsObjectKey` | `2` (`RallyPointController.h` — `_jsonCurrentVersion`) |

Each section object carries its own `version` key using the same
`JsonParsing::jsonVersionKey = "version"` (`JsonParsing.h:12`).

### 1.2 The `mission` section

Root keys and their required-ness, from `MissionController::load()`'s
`rootKeyInfoList`, with string values from `MissionController.h`:

| Key | Type | Required |
|---|---|---|
| `plannedHomePosition` | array | **yes** |
| `items` | array | **yes** |
| `firmwareType` | number | **yes** |
| `vehicleType` | number | no |
| `cruiseSpeed` | number | no |
| `hoverSpeed` | number | no |
| `globalPlanAltitudeMode` | number | no |

`plannedHomePosition` is `[lat, lon, alt]`. We have no equivalent — `HOME_POSITION` (#242)
is a telemetry family, not a mission item. A writer must synthesise it; a reader should not
fold it into `MissionItem`.

### 1.3 `SimpleItem`

From `MissionItem::save()` (`src/MissionManager/MissionItem.cc`), key strings from
`MissionItem.h` and `VisualMissionItem.h:217-219`:

```cpp
void MissionItem::save(QJsonObject& json) const
{
    json[VisualMissionItem::jsonTypeKey] = VisualMissionItem::jsonTypeSimpleItemValue;
    json[_jsonFrameKey]        = frame();
    json[_jsonCommandKey]      = command();
    json[_jsonAutoContinueKey] = autoContinue();
    json[_jsonDoJumpIdKey]     = _sequenceNumber;

    QJsonArray rgParams = { param1(), param2(), param3(), param4(),
                            param5(), param6(), param7() };
    json[_jsonParamsKey] = rgParams;
}
```

| Constant | String |
|---|---|
| `jsonTypeKey` | `"type"` |
| `jsonTypeSimpleItemValue` | `"SimpleItem"` |
| `jsonTypeComplexItemValue` | `"ComplexItem"` |
| `_jsonFrameKey` | `"frame"` |
| `_jsonCommandKey` | `"command"` |
| `_jsonAutoContinueKey` | `"autoContinue"` |
| `_jsonDoJumpIdKey` | `"doJumpId"` |
| `_jsonParamsKey` | `"params"` |

### 1.4 Field-by-field mapping onto `MissionItem`

`proto/gcs/v1/missions.proto`. **`params` is a 7-element array** — `param1`…`param7` — so
positions 5, 6, 7 hold what MAVLink calls `x`, `y`, `z`.

| `.plan` | `MissionItem` | Notes |
|---|---|---|
| `frame` | `frame` (`MavFrame`) | integer, direct |
| `command` | `command` (`MavCmd`) | integer, direct. Values outside our enum subset need a decision — see §4 |
| `autoContinue` | `autocontinue` | bool |
| `doJumpId` | `seq` | QGC writes `_sequenceNumber` here. **Verify the home-position offset against a real file** — QGC's item 0 is the planned home position, so plan indices and MAVLink `seq` may differ by one |
| `params[0]` | `param1` | `null` → NaN |
| `params[1]` | `param2` | `null` → NaN |
| `params[2]` | `param3` | `null` → NaN |
| `params[3]` | `param4` | `null` → NaN; yaw in **radians** in our proto |
| `params[4]` | `x` | latitude, **degrees** — see §1.5 |
| `params[5]` | `y` | longitude, **degrees** |
| `params[6]` | `z` | altitude, metres |
| *(section)* | `mission_type` | `mission` → `MISSION`, `geoFence` → `FENCE`, `rallyPoints` → `RALLY` |
| — | `current` | **No `.plan` equivalent.** Runtime state, not file state. Never round-trips |
| — | `vehicle_id` | Not in the file. Envelope identity only |

### 1.5 Two traps

**NaN is `null`, and only for params 1–4.** QGC uses
`JsonParsing::possibleNaNJsonValue()` on load and permits `QJsonValue::Double` *or*
`QJsonValue::Null` for those positions. A reader that treats `null` as `0` silently turns
"yaw unspecified" into "yaw north". Our proto already documents `param4` as
`NaN = not used`.

**`.plan` is already in degrees — do not normalise twice.** The codec converts
`degE7 → degrees` at the wire boundary, exactly once
(`order-of-operations.md`, unit normalisation). `.plan` stores `params[4]`/`params[5]` as
JSON doubles already in degrees, so a `.plan` reader must feed `MissionItem.x`/`.y`
directly. Applying the wire conversion here divides by 1e7 a second time and puts every
waypoint within a few metres of Null Island.

### 1.6 `ComplexItem` — unmapped, and deliberately so

`ComplexMissionItem.h` defines `jsonComplexItemTypeKey = "complexItemType"`. The survey
implementation sets `jsonComplexItemTypeValue = "survey"`
(`SurveyComplexItem.h`); corridor-scan and structure-scan carry their own values in their
own headers. Transect-style items nest a `TransectStyleComplexItem` object
(`TransectStyleComplexItem.h` — `_jsonTransectStyleComplexItemKey`) whose
`_jsonItemsKey = "Items"` holds the **pre-expanded** waypoints.

Note the capitalisation: `"items"` at the mission root, `"Items"` inside
`TransectStyleComplexItem`. They are different keys.

**We have no equivalent and this document does not invent one.** The pre-expanded `Items`
array can be read as ordinary `SimpleItem`s and mapped by §1.4, which is enough to *fly* a
survey downloaded from QGC. What is lost is the survey *definition* — polygon, spacing,
angle — so the mission is no longer re-editable as a survey. Both QGC and Cockpit concluded
the definition must be persisted somewhere to stay editable, and if it is ever a stored
artifact it is a message. Recorded as open in
`../research/cockpit-reference-reconciliation.md`.

**Consequence for the Tier 7 gate:** a round-trip through a plan containing a
`ComplexItem` is lossy by design. Scope the gate to `SimpleItem`s, or assert the loss
explicitly.

### 1.7 `geoFence` and `rallyPoints`

| Section | Keys | Constant |
|---|---|---|
| `geoFence` | `polygons`, `circles` | `GeoFenceController.h` — `_jsonPolygonsKey`, `_jsonCirclesKey` |
| `rallyPoints` | `points` | `RallyPointController.h` — `_jsonPointsKey` |

Both are version `2`. Neither maps onto `MissionItem` element-for-element: our fence and
rally support is the `MavMissionType` discriminator on the same flat `MissionItem`, so a
polygon becomes N `MAV_CMD_NAV_FENCE_POLYGON_VERTEX_INCLUSION` items rather than one
object. Out of scope for the Tier 7 gate, which covers the `mission` section.

---

## 2. `.waypoints` — ArduPilot / MAVProxy text format

**Not re-verified.** Carried forward from the earlier Cockpit reference analysis, which
sourced it from Mission Planner. Treat as a lead, not as settled, until someone checks it
against `mavwp` or a Mission Planner build.

- Header line must contain `QGC WPL`; Mission Planner writes `QGC WPL 110`.
- 12 columns, tab / space / comma separated:
  `seq  current  frame  command  param1  param2  param3  param4  x  y  z  autocontinue`
- `command` accepts **names or numbers** on read.
- `autocontinue` is written but **ignored on read**.
- The file is metric, and lat/lon are decimal degrees — so §1.5's
  do-not-normalise-twice trap applies here too.

De-facto interchange across the ArduPilot ecosystem (MAVProxy, `mavwp`), and considerably
cheaper to support than `.plan`. It carries no complex items at all, which makes it a
clean round-trip target: every column maps onto a `MissionItem` field with no loss.

---

## 3. tlog — telemetry log framing

**Not re-verified.** From the earlier analysis: byte-identical between QGroundControl and
Mission Planner — repeated `[8-byte big-endian microsecond timestamp][raw MAVLink frame]`,
with no container, index or header.

No consumer in this repo. Recorded here because it is the cheapest possible ecosystem
interop and because ADR-0007 §8's **event history** port is the natural home for it. Listed
as open, not scheduled.

---

## 4. Known gaps

1. **`doJumpId` ↔ `seq` offset.** QGC's item 0 is the planned home position. Whether
   `doJumpId` is 0-based or 1-based against MAVLink `seq` must be settled against a real
   `.plan` file before the gate is trusted. This is the single most likely off-by-one in the
   mapping.
2. **`command` values outside `MavCmd`.** `types.proto` mirrors a *subset* of `MAV_CMD`. A
   `.plan` naming a command we do not model has no enum value. `commands.proto` has a
   `raw_command` passthrough for the send path; `MissionItem.command` has no equivalent.
3. **`ComplexItem` definitions are lossy** — §1.6.
4. **`plannedHomePosition` has no home in our model** — §1.2.
5. **`.waypoints` and tlog are unverified** — §2, §3.
