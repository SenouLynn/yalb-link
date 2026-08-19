# Tier 2 — Parity Apparatus + Test Vectors

## Overview

Establish the comparison infrastructure that makes Tier 1 claims verifiable and all subsequent tiers auditable. This is not feature work — it is the evidentiary layer. A capability matrix without status is a wish list. A test that uses inline magic numbers is not reproducible. This tier closes both gaps.

## Dependencies

- Tier 3 complete (generated types available); Tier 1 in place or progressing in parallel
- `pymavlink` available to run the fixture generator once (`pip install pymavlink`)

**No dependency on a sibling checkout.** The original plan sourced fixtures from
`flight-path-hud/contracts/mavlink/`, a repo that is not present here, not pinned,
and not reachable by URL or SHA — leaving the evidentiary layer that every later
tier's claim rests on with an unresolvable dependency. Fixtures are generated from
a committed script instead (Chapter 1). If flight-path-hud fixtures are still
wanted as a cross-check, vendor them into `contracts/mavlink/` with the upstream
commit SHA recorded in a README.

## Chapters

---

### Chapter 1: Golden Byte Fixtures

**Goal:** Byte-level reference data for the decode path, reproducible from a
committed script rather than copied from a checkout that may not exist.

**Generator:** `scripts/gen_mavlink_fixtures.py` — constructs a known message per
family, encodes it via `pymavlink`'s `mavutil.mavlink.MAVLink`, and writes the bytes
plus an expected-fields JSON companion. Committed, deterministic, run deliberately —
never at build time. This is the primary source, not the fallback: a fixture
regenerable from a script in this repo is the only version that satisfies ADR-0001's
hermeticity argument.

**Destination:** `contracts/mavlink/`

**Fixture format:** One file per message family, binary `.bin` plus a companion `.json` describing the expected decoded fields. Example:
```
contracts/mavlink/
  heartbeat_v2_unsigned.bin
  heartbeat_v2_unsigned.json
  gps_raw_int_v2.bin
  gps_raw_int_v2.json
  attitude_v2.bin
  attitude_v2.json
  ...
```

**JSON companion format:**
```json
{
  "message_id": 30,
  "message_name": "ATTITUDE",
  "sys_id": 1,
  "comp_id": 1,
  "seq": 42,
  "fields": {
    "time_boot_ms": 12345,
    "roll": 0.0174533,
    "pitch": -0.0087266,
    "yaw": 1.5708
  }
}
```

**For families `pymavlink` cannot produce cleanly** (dialect-specific messages such
as EKF_STATUS_REPORT, if the installed dialect lacks them): capture from SITL during
Tier 5 with `tcpdump -i any -w capture.pcap udp port 14550`, then extract frames via
`tshark -r capture.pcap -T fields -e data > raw.hex`. Record which fixtures came from
capture rather than generation — a captured fixture is not reproducible and needs a
note saying so.

**Constraint:** Fixtures are committed to source control. They are reference data, not generated output. The generation script is also committed so fixtures can be regenerated if the MAVLink spec changes.

---

### Chapter 2: Go Capability Matrix

**Goal:** A structured document tracking decode + encode coverage. Every cell must have an evidence artifact before it can be marked `complete`.

**File:** `docs/reference/codec-capability-matrix.md`

**Structure:**

```markdown
# Codec Capability Matrix

## Receive — Telemetry (13 families → TelemetryEvent)

| Case ID | MAVLink Message | ID | Dialect | Behavior | Evidence Artifact | Status | Notes |
|---|---|---|---|---|---|---|---|
| RECV-HEARTBEAT | HEARTBEAT | 0 | common | Decode → HeartbeatState | contracts/mavlink/heartbeat_v2.bin | complete | |
| RECV-SYS-STATUS | SYS_STATUS | 1 | common | Decode → SystemStatus | ... | pending | |
...
| RECV-EKF | EKF_STATUS_REPORT | 193 | ardupilotmega | Decode → EkfStatusReport | ... | pending | |

## Receive — Transactions (5 families → ProtocolEvent)

These correlate against an in-flight request registry rather than folding into
vehicle state, so they decode to a different envelope and their evidence must show
the envelope as well as the fields.

| Case ID | MAVLink Message | ID | Behavior | Evidence Artifact | Status |
|---|---|---|---|---|---|
| RECV-PARAM-VALUE | PARAM_VALUE | 22 | Decode → ProtocolEvent.param_value | ... | pending |
| RECV-MISSION-COUNT | MISSION_COUNT | 44 | Decode → ProtocolEvent.mission_count | ... | pending |
| RECV-MISSION-ACK | MISSION_ACK | 47 | Decode → ProtocolEvent.mission_ack | ... | pending |
| RECV-MISSION-ITEM | MISSION_ITEM_INT | 73 | Decode → ProtocolEvent.mission_item | ... | pending |
| RECV-COMMAND-ACK | COMMAND_ACK | 77 | Decode → ProtocolEvent.command_ack | ... | pending |

## Unit Normalisation

One row per field that changes units between wire and proto. These are the cases a
field-by-field decode test would pass and a human would still get wrong.

| Case ID | Field | Wire | Proto | Status |
|---|---|---|---|---|
| NORM-LAT | GLOBAL_POSITION_INT.lat | int32 degE7 | double degrees | pending |
| NORM-ALT-REL | GLOBAL_POSITION_INT.relative_alt | int32 mm | float metres | pending |
| NORM-VEL | GLOBAL_POSITION_INT.vx/vy/vz | int16 cm/s | float m/s | pending |
| NORM-GPS-ALT | GPS_RAW_INT.alt | int32 mm MSL | float metres MSL | pending |

## Send (Encode)

| Case ID | MAVLink Message | ID | Behavior | Evidence Artifact | Status | Notes |
|---|---|---|---|---|---|---|---|
| SEND-HEARTBEAT | HEARTBEAT | 0 | Encode GCS keepalive; fixed fields | contracts/mavlink/heartbeat_gcs_out.bin | pending | |
| SEND-CMD-LONG | COMMAND_LONG | 76 | Encode arm/disarm | ... | pending | |
...

## Framing Cases

| Case ID | Behavior | Evidence Artifact | Status |
|---|---|---|---|
| FRAME-V1-ACCEPT | MAVLink v1 frame decoded without error; `gomavlib.Node` configured with `OutVersion: V2` still accepts inbound v1 — this is gomavlib's default receive behavior | contracts/mavlink/v1_heartbeat.bin | pending |
| FRAME-V2-UNSIGNED | MAVLink v2 unsigned frame decoded | contracts/mavlink/heartbeat_v2.bin | pending |
| FRAME-BAD-CRC | Bad CRC: gomavlib emits `EventParseError` internally, no `EventFrame` surfaces. Test asserts: (a) no TelemetryEvent emitted, (b) no panic, (c) codec parse error counter incremented. Do NOT assert an error return from `Decode()` — it returns `nil, nil` for parse errors too | contracts/mavlink/bad_crc.bin | pending |
| FRAME-TRUNCATED | Truncated frame: same behavior as FRAME-BAD-CRC — `EventParseError`, no panic, counter incremented | contracts/mavlink/truncated.bin | pending |

**Golden byte tests must run the real decoder.** Feed `.bin` bytes through gomavlib,
never hand-craft `EventFrame` structs — that tests a stub. The endpoint is
`gomavlib.EndpointCustomClient` with `net.Pipe()` (see Tier 1 Chapter 1;
`EndpointCustomConn` does not exist, and `EndpointCustom` is deprecated). Write raw
bytes to the harness side; gomavlib's parser handles STX detection, length
extraction, CRC_EXTRA validation and dialect struct population before any test logic
sees an event.
```

**Status values:** `unstarted` | `pending` (fixture exists, test not written) | `complete` (test passes with fixture) | `blocked` (dependency missing)

**Rule:** No case ID can be `unstarted` by the end of Tier 2. Every receive and send family must have at least `pending` status with a named fixture artifact.

**Enforcement — two layers.**

*Layer 1, `scripts/check-matrix.sh`:* fails the build when any row is `unstarted`.
The obvious one-liner does not work, and it is worth knowing why because it fails
in the most misleading way possible:

```makefile
# BROKEN — do not use
check-matrix:
	@grep -q 'unstarted' FILE && (echo "ERROR" && exit 1) || true
```

The trailing `|| true` swallows the subshell's `exit 1`, so make sees 0 and the
build passes after printing ERROR. (Verified: prints the error, exits 0.) And a bare
`unstarted` also matches the document's own status legend, so the check fires
whether or not any row is unstarted — simultaneously always-triggered and
never-failing. The script matches table cells (`| unstarted |`) and lets the exit
status be the result.

*Layer 2, `TestMatrixCoverage`:* a Go test asserting that the matrix's case IDs equal
the keys of the codec's dispatch tables (`telemetryDecoders` ∪ `protocolDecoders`,
plus the encoder table). This is what stops the matrix being aspirational — a new
decoder without a matrix row fails the build, and a matrix row without a decoder
fails too. Layer 1 alone only checks that someone typed a word.

Both run in `make gate-tier-2`.

---

### Chapter 3: TypeScript Known-Answer Fixture Tables

**Goal:** Replace any inline magic numbers in resolver tests with named fixture files. Each resolver must have a fixture table with at least three test vectors: a normal case, a fallback case, and a missing-data case.

**Location:** `frontend/src/logic/__fixtures__/`

**Format:** One `.ts` fixture file per resolver:

```typescript
// frontend/src/logic/__fixtures__/heading.fixtures.ts
export const headingFixtures = [
  {
    name: 'primary-vfr-hud',
    input: { headingDeg: 270, yawRad: undefined, hdgCdeg: undefined },
    expected: { headingDeg: 270, source: 'VFR_HUD', isFallback: false },
  },
  {
    // VFR_HUD.heading is int16 on the wire; ArduPilot may report it negative.
    name: 'primary-vfr-hud-negative-normalised',
    input: { headingDeg: -90, yawRad: undefined, hdgCdeg: undefined },
    expected: { headingDeg: 270, source: 'VFR_HUD', isFallback: false },
  },
  {
    name: 'fallback-attitude-yaw',
    input: { headingDeg: undefined, yawRad: 1.5708, hdgCdeg: undefined },
    expected: { headingDeg: 90, source: 'ATTITUDE', isFallback: true },
  },
  {
    // UINT16_MAX is the unknown sentinel on GLOBAL_POSITION_INT.hdg, not VFR_HUD.
    name: 'gpi-unknown-sentinel-rejected',
    input: { headingDeg: undefined, yawRad: undefined, hdgCdeg: 65535 },
    expected: null,
  },
  {
    name: 'missing-all-sources',
    input: { headingDeg: undefined, yawRad: undefined, hdgCdeg: undefined },
    expected: null,
  },
];
```

**Do not write a fixture with `headingDeg: 65535`.** `VFR_HUD.heading` is `int16_t`
on the wire and cannot hold that value; such a fixture tests an input that never
occurs while leaving the real sentinel path (GLOBAL_POSITION_INT.hdg) and the real
gap (negative headings) uncovered.

**Required fixture coverage per resolver:**

| Resolver | Required fixtures |
|---|---|
| `heading.ts` | primary VFR_HUD, negative heading normalised, fallback ATTITUDE, fallback GPI, GPI 65535 rejected, missing |
| `attitude.ts` | normal, missing fields |
| `flightPath.ts` | VFR_HUD primary climb (no flip), GPI fallback climb (`-vzMs`, no /100), source names the winner |
| `trajectory.ts` | copter at 10 m/s with angular rates (full CTRV); copter without angular rates (bank fallback); fixed-wing with airspeed 10 below stall 14 (zero forward progress); fixed-wing with airspeed 20 (nonzero); **VTOL hovering with no airspeed (nonzero — must not be gated by vehicle type)**; hover zero yaw rate |
| `position.ts` | primary GPI with `altRef: 'RELATIVE'`, fallback GPS_RAW_INT with `altRef: 'MSL'`, missing. Assert the datum, not just the value — a fallback that reports MSL as relative altitude is the failure this catches |
| `track.ts` | accumulate 3 points, ring buffer overflow at 500, ENU accuracy at 1km offset |
| `freshness.ts` | fresh, stale, exactly at TTL boundary (convention: `nowMs - lastSeenMs === ttlMs` is stale — document this in freshness.ts) |
| unit sanity | at least one fixture per resolver whose expected values are physically plausible (`latDeg: 47.6`, not `476000000`). A reviewer spots a 1e7 error instantly in a plausible number and never in a magic one |
| vehicle-class predicate | if `trajectory.ts` keeps one, drive it from the generated `MavType` constants and assert the values that were historically miscopied: ROCKET(9) and GROUND_ROVER(10) are **not** fixed-wing; KITE(17) and FLAPPING_WING(16) are; COAXIAL(3) is not |

**freshness TTL boundary convention:** `isFresh` returns `true` if `nowMs - lastSeenMs < ttlMs` (strictly less than). At exactly TTL, the value is stale. Document this convention in `freshness.ts` so FreshnessRing components don't implement their own boundary logic differently.

---

### Chapter 4: Test Harness Configuration

**Goal:** Ensure test infrastructure is wired so both Go and TS test suites run in CI from day one.

**Go:**
- `go test ./...` runs from repo root
- `-race` flag enabled by default in CI Makefile target
- Fixture files read from `contracts/mavlink/` using path relative to test file (or `testdata/` symlink)

**TypeScript:** already wired — `vitest` is a devDependency, `vite.config.ts` sets
`environment: 'node'` and `include: ['src/**/*.test.ts']`, and the `@/` alias
resolves to `frontend/src` in both `vite.config.ts` and `tsconfig.app.json` so
generated imports (`@/gen/gcs/v1/telemetry_pb`) work in tests and typecheck alike.
`pnpm test` and `pnpm typecheck` are the entry points.

**Makefile targets:** `make test-go`, `make test-ts`, `make test`, and the tier
gates `make gate-tier-0` … `gate-tier-3`. CI runs the same targets — see
`.github/workflows/ci.yml`.

---

## Tier Exit Gate

```
make gate-tier-2
```

- `scripts/check-matrix.sh` exits 0 — no `| unstarted |` rows
- `TestMatrixCoverage` passes — matrix case IDs equal the codec's dispatch table keys
- Golden byte fixtures committed to `contracts/mavlink/`, regenerable via
  `scripts/gen_mavlink_fixtures.py`
- TS resolver tests reference named fixture files — no inline magic numbers
- `go test -race ./internal/codec/...` passes
