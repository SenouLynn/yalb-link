# Tier 0 — Proto Contracts + Proto Fix

## Overview

All protos must be lint-clean and the `SetArmedRequest.force` field must be removed before any codegen happens. This is a surgical pre-flight: the contracts are substantially complete, but one unsafe field must be excised and the lint gate must be confirmed passing before Tier 3 generates stubs that would lock in the mistake.

## Dependencies

None. This is the first thing that happens.

## Chapters

---

### Chapter 1: Audit Existing Proto Files

**Goal:** Confirm which files exist, which messages are defined, and that the domain schema is complete before touching anything.

**Files to audit:**
- `proto/gcs/v1/` — all `.proto` files
- `proto/buf.yaml` — module config
- `proto/buf.lock` — dependency pins

**Checks:**
- All 14+ proto files present and named consistently
- `TelemetryEvent` oneof covers all 18 receive families
- `TelemetryEvent` carries `uint32 system_id` and `uint32 component_id` as top-level fields alongside the oneof payload — these are the MAVLink frame header values, not inside the oneof variants. If they are absent, add them now before codegen; adding fields post-generation forces a regeneration cycle and risks drift with any hand-written stub from Tier 1.
- `BatteryStatus`, `EkfStatusReport` present as message types
- `Attitude` message type carries `rollspeed_rad_s`, `pitchspeed_rad_s`, `yawspeed_rad_s` fields (angular rates from MAVLink ATTITUDE message). These feed the CTRV trajectory model. Absent now = must add before Tier 1 encodes them.
- `commands.proto` contains `SetArmedRequest` with the offending `force` field (field 3)
- No floating imports or missing dependencies

**Output:** A written list of what exists and what needs to change before touching buf.

---

### Chapter 2: Remove `SetArmedRequest.force`

**Goal:** Eliminate the field that cannot be safely enforced at the proto layer.

**File:** `proto/gcs/v1/commands.proto`

**Change:** Remove field 3 (`bool force`) from `SetArmedRequest`. Do not add a reserved statement — the field never shipped in generated code, so no wire compatibility concern exists yet.

**Why this must happen now:** A field that exists can be set by any caller who bypasses service-layer documentation. The service cannot enforce a proto field to a fixed value; it can only reject requests where the field is set to 1. Removing it is the only safe choice.

**Constraint:** This is not a breaking change before first codegen — no generated stubs exist yet, so `buf breaking` will not flag it.

---

### Chapter 3: buf Workspace Configuration

**Goal:** Confirm `buf.yaml` module configuration is correct for the project layout.

**Checks:**
- `buf.yaml` `name:` field matches the project's buf.build module path (if using BSR) or is absent (if local-only)
- `buf.yaml` `deps:` includes `buf.build/googleapis/googleapis` if any proto uses google.protobuf types
- `buf.work.yaml` (if present) references the correct module directories
- `buf.lock` is committed and pins exact dependency SHAs

**If buf.gen.yaml is present:** Move codegen config to Tier 3 scope — it does not belong in Tier 0.

---

### Chapter 4: buf lint Gate

**Goal:** `buf lint` passes with zero warnings or errors.

**Run:** `cd proto && buf lint`

**Lint category:** Explicitly set the lint category in `buf.yaml` rather than relying on buf's default:

```yaml
lint:
  use:
    - DEFAULT
```

The `DEFAULT` set is stable across buf releases. Leaving `use:` unset ties behavior to buf's version-dependent default, which may tighten between upgrades and cause surprise CI failures.

**Common failures to resolve:**
- Field naming (should be snake_case)
- Enum value prefixes (ENUM_NAME_VALUE pattern)
- Package naming (should be `gcs.v1`)
- Missing `go_package` option in each file

**Note:** Do not silence lint rules with `breaking.ignore` or `lint.ignore` — fix the underlying issue.

---

### Chapter 5: buf breaking Baseline

**Goal:** Establish the breaking-change baseline against `origin/main` so subsequent PRs can be gated.

**Run (after merging Tier 0 changes to main):**
```
buf breaking --against '.git#branch=main'
```

**Not:** `--against HEAD~1` — that would fire on any feature branch with iterative proto changes.

This gate runs in CI on all PRs to main. Not required to pass locally during Tier 0 feature work; required before merging.

---

## Tier Exit Gate

- `buf lint` passes with zero errors
- `SetArmedRequest.force` field is absent from `commands.proto`
- All proto files committed and buf.lock updated
- CI buf lint job configured (even if not running yet — the config must exist)
