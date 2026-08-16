# Tier 0 — Proto Contracts + Proto Fix

## Overview

Every decision in this tier is free now and a breaking change after codegen. That
is the entire reason the tier exists, and it is why it is not the formality it
was originally scoped as. Generated Go and TS carry these names and shapes into
`internal/gen/`, `frontend/src/gen/`, service handlers, and every test written
against them; changing any of it afterwards costs a proto break plus a
regeneration cycle plus a rewrite of whatever was built on top.

## Dependencies

None. This is the first thing that happens.

## Chapters

---

### Chapter 1: Audit Existing Proto Files

**Goal:** Establish what the contracts actually say before changing them.

**Files:** `proto/gcs/v1/` (all `.proto`), `proto/buf.yaml`.

**Checks:**
- All proto files present and named consistently.
- `TelemetryEvent` covers the **13 streaming families** — not 18. PARAM_VALUE(22),
  MISSION_COUNT(44), MISSION_ITEM_INT(73), MISSION_ACK(47) and COMMAND_ACK(77) are
  transaction responses and belong on `ProtocolEvent` (Chapter 3). A check for "all
  18 in the oneof" cannot pass and should not.
- `Attitude` carries `rollspeed_rad_s`, `pitchspeed_rad_s`, `yawspeed_rad_s`
  (angular rates feeding the CTRV trajectory model).
- Units are SI at the proto boundary: `lat_deg` double degrees, `alt_msl_m` /
  `alt_relative_m` float metres, `vx_m_s` float m/s. This is the contract that
  makes the "normalise exactly once" rule enforceable — see Tier 1.
- `commands.proto` `SetArmedRequest` has no `force` field.
- No floating imports or missing dependencies.

**Note:** `buf.yaml` needs no `deps:` entry for `google/protobuf/*`. The well-known
types are built into buf. `buf.build/googleapis/googleapis` covers `google.api.*`
and `google.rpc.*`, which nothing here uses — adding it buys a BSR dependency and
a `buf.lock` for nothing, and works against the air-gap story in Tier 3.

---

### Chapter 2: Remove `SetArmedRequest.force`, and state the real control

**File:** `proto/gcs/v1/commands.proto`

Remove `bool force`. No `reserved` statement — the field never reached generated
code, so there is no wire-compatibility concern.

**What the removal does and does not do.** It states intent and closes the obvious
footgun. It does not close force-arm. In the same file, `CommandService.SendCommand`
accepts any `MavCmd` with free `param1`–`param7` plus a `raw_command` passthrough
for IDs outside the enum. Force-arm is `MAV_CMD_COMPONENT_ARM_DISARM (400)` with
`param2 = 21196`, so any caller able to reach SendCommand can force-arm regardless
of this field.

Claiming the property without the enforcement is worse than not claiming it. The
control is server-side validation in SendCommand, which must:

1. reject command 400 with `param2 == 21196`;
2. reject any command not on an explicit allowlist, including via `raw_command` —
   unrecognised IDs are denied, not passed through;
3. check the per-command role before dispatch.

Written into the `CommandLong` proto comment in this tier. Enforced by the command
registry in Tier 8. No write path is exposed before then.

---

### Chapter 3: Split telemetry from transaction responses

**Goal:** Decide the codec's output type while it costs nothing.

Five of the 18 priority receive families answer a request the GCS made. Their
consumer is an in-flight request registry that completes a pending RPC, not the
per-vehicle telemetry fold. `TelemetryEvent` has no variant for them and should
not gain one.

**Add `proto/gcs/v1/protocol.proto`:**

```proto
message ProtocolEvent {
  VehicleId vehicle_id = 1;
  oneof payload {
    ParameterValue param_value = 2;  // PARAM_VALUE (#22)
    MissionCount mission_count = 3;  // MISSION_COUNT (#44)
    MissionItem mission_item = 4;    // MISSION_ITEM_INT (#73)
    MissionAck mission_ack = 5;      // MISSION_ACK (#47)
    CommandAck command_ack = 6;      // COMMAND_ACK (#77)
  }
}
```

`MissionCount` did not exist as a message and is added in `missions.proto`.

The codec emits exactly one of `TelemetryEvent` or `ProtocolEvent` per decoded
frame. Deferring this to Tier 7 means re-cutting the codec's public signature
after three tiers of tests are written against it.

---

### Chapter 4: Vehicle identity on the envelope only

`TelemetryEvent` carried `vehicle_id`, and every payload message carried its own —
two sources of truth per event, wire cost duplicated at telemetry rates, and no
rule for what happens when they disagree.

Identity now lives on the envelope. The 15 telemetry payload messages carry no
`vehicle_id`; field numbers were renumbered from 1 (safe pre-codegen). The codec
reads `(system_id, component_id)` from the MAVLink frame header once and sets it
on the envelope.

Do **not** additionally add scalar `system_id` / `component_id` fields to
`TelemetryEvent` — `VehicleId` already carries exactly that pair, and a third copy
is a third thing to disagree.

Payloads that are also returned directly from RPCs (`ParameterValue`,
`MissionItem`, `MissionAck`) keep their own `vehicle_id` for standalone use; on
`ProtocolEvent` the envelope field is authoritative.

Enforced by `scripts/check-tier-0.sh`.

---

### Chapter 5: No client-streaming or bidirectional RPCs

`@connectrpc/connect-web` supports unary and server-streaming only. Client and
bidirectional streaming are unavailable to browser clients, so a client-streaming
RPC is uncallable from the frontend.

`MissionService.UploadMission` was `rpc UploadMission(stream MissionItem)`. It is
now unary over `UploadMissionRequest { target, mission_type, repeated items }`.
Missions are bounded — hundreds of items at most — so a single request costs
nothing, and the backend still runs the full MAVLink handshake internally.

`scripts/check-tier-0.sh` greps for `rpc X(stream ` so this cannot regress. Free
to catch here; at Tier 8 it is a proto break, a regen, and a service rewrite.

---

### Chapter 6: Typed subscription filter

`StreamTelemetryRequest.payload_types` was `repeated string` with example values in
a comment — no lint coverage, no breaking-change coverage, no autocomplete, and a
typo silently subscribes to nothing. It is now `repeated TelemetryPayloadType`, an
enum in `telemetry.proto` whose values correspond 1:1 to the oneof.

Same class of fix: `OperatorContext.roles` was `repeated string` while
`OperatorProfile.roles` was already `repeated OperatorRole`. Both are typed now.

---

### Chapter 7: Authorization in the contract

ADR-0005 requires a role check before every RPC handler. Without a link from the
proto, that mapping is a hand-maintained method-name table that silently goes
stale when an RPC is added.

**Add `proto/gcs/v1/options.proto`:**

```proto
extend google.protobuf.MethodOptions {
  OperatorRole required_role = 50001;
}
```

Every RPC in `services.proto` is annotated (37 of them):

```proto
rpc SetArmed(SetArmedRequest) returns (CommandResult) {
  option (gcs.v1.required_role) = OPERATOR_ROLE_OPERATOR;
}
```

Middleware reads the role off the method descriptor. **Absent option = deny** — a
new RPC without an annotation is unreachable, not public. An explicit
`OPERATOR_ROLE_UNSPECIFIED` marks the one deliberately public RPC,
`AuthService.ValidateSession`, which must answer callers whose token is invalid.

Adding this later means a new options file, a full regeneration, and touching all
eight services.

---

### Chapter 8: `go_package` and the project name

`go_package` was `ligma-gcs/proto/gcs/v1;gcsv1`, matching neither the Go module nor
the Tier 3 output path. With `paths=source_relative` the files land correctly while
the import path baked into generated code points at a module that does not exist.

Settled: the project is `yalb-gcs`. Go module `yalb.gcs`, Bazel module `yalb_gcs`,
`go_package = "yalb.gcs/internal/gen/gcs/v1;gcsv1"`, buf module
`buf.build/yalb-gcs/proto`. Verified: generated Connect stubs import
`yalb.gcs/internal/gen/gcs/v1` and compile.

This must happen **before** the breaking baseline — `buf.yaml` sets
`breaking.use: [FILE]`, which flags `go_package` changes, so a later fix trips the
gate permanently.

---

### Chapter 9: buf lint posture

`buf.yaml` is `version: v2`, where the category is `STANDARD`. `DEFAULT` is the v1
spelling and is not valid here.

Under `STANDARD` the MAVLink mirror protos fail four rules. They are not mistakes:
MAVLink's own names do not follow buf conventions (`MAV_MISSION_RESULT` holds
`MAV_MISSION_ACCEPTED`), and `MAV_TYPE_GENERIC = 0` is a meaningful value rather
than a placeholder. Renaming ~40 messages and enums would break the value-for-value
correspondence the codec depends on.

Resolution: keep `STANDARD` and add **per-file** `ignore_only` exceptions, never
module-wide.

```yaml
lint:
  use: [STANDARD]
  ignore_only:
    ENUM_VALUE_PREFIX:        [gcs/v1/types.proto]
    ENUM_ZERO_VALUE_SUFFIX:   [gcs/v1/types.proto, gcs/v1/track.proto]
    RPC_REQUEST_STANDARD_NAME:   [gcs/v1/services.proto]
    RPC_RESPONSE_STANDARD_NAME:  [gcs/v1/services.proto]
    RPC_REQUEST_RESPONSE_UNIQUE: [gcs/v1/services.proto]
```

Scoping is what makes this honest rather than a blanket silence. It surfaced
`TrackAffiliation`, a GCS-native enum whose zero value was `TRACK_AFFILIATION_UNKNOWN`
— "unknown affiliation" is a real classification and had to stay distinguishable
from an unpopulated field, so it now carries an explicit `_UNSPECIFIED = 0`.

The blanket `FIELD_LOWER_SNAKE_CASE` exception was removed: every field in the tree
is already lower_snake_case, so it was silencing nothing.

**Verified:** `buf lint proto` exits 0.

---

### Chapter 10: buf breaking baseline

**Command:**

```
buf breaking proto --against '.git#branch=origin/main,subdir=proto'
```

Run **from the repo root**. Three details, all verified by execution, all of which
produce a silently-passing gate when wrong:

- The `.git` input resolves relative to the working directory. `cd proto && buf
  breaking --against '.git#...'` looks for `proto/.git` and fails outright.
- Without `subdir=proto`, buf reads the module from the ref's root and the imports
  do not resolve (`imported file does not exist`).
- In GitHub Actions there is no local `main` branch even at `fetch-depth: 0` — the
  checkout is a detached HEAD with remote-tracking refs. Use `origin/main`, and set
  `fetch-depth: 0` or the ref is unresolvable.

Encoded once in `make proto-breaking` and in `.github/workflows/ci.yml`. Do not
retype it elsewhere.

---

## Tier Exit Gate

```
make gate-tier-0
```

- `buf lint proto` exits 0
- `buf breaking` clean against `origin/main`
- `scripts/check-tier-0.sh` passes all five contract checks:
  - `SetArmedRequest` has no `force` field
  - no client-streaming or bidirectional RPCs
  - all 37 RPCs declare `required_role`
  - `telemetry.proto` declares `vehicle_id` exactly once, on the envelope
  - `go_package` matches the `go.mod` module path
