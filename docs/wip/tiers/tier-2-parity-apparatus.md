# Tier 2 — Parity Apparatus + Test Vectors

## Overview

Establish the comparison infrastructure that makes Tier 1 claims verifiable and all subsequent tiers auditable. This is not feature work — it is the evidentiary layer. A capability matrix without status is a wish list. A test that uses inline magic numbers is not reproducible. This tier closes both gaps.

## Dependencies

- Tier 1 codec and resolver logic in place (or progressing in parallel)
- Golden byte fixtures available from `flight-path-hud/contracts/mavlink/`
- `flight-path-hud` accessible locally or its contracts directory copied

## Chapters

---

### Chapter 1: Golden Byte Fixture Migration

**Goal:** Copy MAVLink golden byte fixtures from the flight-path-hud repo into this project so they are available without depending on a sibling repo at test time.

**Source:** `flight-path-hud/contracts/mavlink/`

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

**For message families without existing fixtures:** Two options, in preference order:
1. Generate deterministically using `pymavlink`: write a one-time script that constructs a known message, encodes it via `mavutil.mavlink.MAVLink`, and writes the bytes to a `.bin` file alongside the expected-fields `.json`. This is reproducible and doesn't require hardware. Script lives at `scripts/gen_mavlink_fixtures.py` and is committed — it is not run at build time.
2. Capture from SITL during Tier 5: `tcpdump -i any -w capture.pcap udp port 14550`, then extract individual frames with `tshark -r capture.pcap -T fields -e data > raw.hex`. Use only for families that `pymavlink` can't produce cleanly (e.g., EKF_STATUS_REPORT).

**Constraint:** Fixtures are committed to source control. They are reference data, not generated output. The generation script is also committed so fixtures can be regenerated if the MAVLink spec changes.

---

### Chapter 2: Go Capability Matrix

**Goal:** A structured document tracking decode + encode coverage. Every cell must have an evidence artifact before it can be marked `complete`.

**File:** `docs/wip/codec-capability-matrix.md`

**Structure:**

```markdown
# Codec Capability Matrix

## Receive (Decode)

| Case ID | MAVLink Message | ID | Dialect | Behavior | Evidence Artifact | Status | Notes |
|---|---|---|---|---|---|---|---|
| RECV-HEARTBEAT | HEARTBEAT | 0 | common | Decode → HeartbeatState | contracts/mavlink/heartbeat_v2.bin | complete | |
| RECV-SYS-STATUS | SYS_STATUS | 1 | common | Decode → SysStatus | ... | pending | |
...
| RECV-EKF | EKF_STATUS_REPORT | 193 | ardupilotmega | Decode → EkfStatusReport | ... | pending | |

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

**ChanFrameSource and golden byte tests:** The test infrastructure must feed `.bin` bytes through gomavlib's real decoder, not produce hand-crafted `EventFrame` structs. Use `gomavlib.EndpointCustomConn` (in-memory `io.Pipe`) as the endpoint in `ChanFrameSource`. Write raw `.bin` bytes to the pipe; gomavlib's frame parser handles STX detection, length extraction, CRC_EXTRA validation, and dialect struct population before any test logic sees the event. Tests that skip this path are testing a stub, not the codec.
```

**Status values:** `unstarted` | `pending` (fixture exists, test not written) | `complete` (test passes with fixture) | `blocked` (dependency missing)

**Rule:** No case ID can be `unstarted` by the end of Tier 2. Every receive and send family must have at least `pending` status with a named fixture artifact.

**Enforcement:** Without automation the matrix becomes aspirational within a week. Add a CI Makefile target that hard-fails on `unstarted` cells:

```makefile
check-matrix:
	@grep -q 'unstarted' docs/wip/codec-capability-matrix.md && \
	  (echo "ERROR: unstarted cells in capability matrix" && exit 1) || true
```

Wire `check-matrix` into the CI `test` job. Any new row must be at minimum `pending` before the PR merges.

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
    name: 'fallback-attitude-yaw',
    input: { headingDeg: 65535, yawRad: 1.5708, hdgCdeg: undefined },
    expected: { headingDeg: 90, source: 'ATTITUDE', isFallback: true },
  },
  {
    name: 'missing-all-sources',
    input: { headingDeg: undefined, yawRad: undefined, hdgCdeg: undefined },
    expected: null,
  },
];
```

**Required fixture coverage per resolver:**

| Resolver | Required fixtures |
|---|---|
| `heading.ts` | primary VFR_HUD, fallback ATTITUDE, fallback GPI, missing, 65535 rejection |
| `attitude.ts` | normal, missing fields |
| `flightPath.ts` | VFR_HUD primary climb (no flip), GPI fallback climb (flip applied), source field names correct winner |
| `trajectory.ts` | copter at 10 m/s with angular rates (full CTRV), copter at 10 m/s without angular rates (bank fallback), plane at 10 m/s below stall (zero forward progress), plane at 20 m/s above stall (nonzero), hover zero yaw rate |
| `position.ts` | primary GPI, fallback GPS_RAW_INT, missing |
| `track.ts` | accumulate 3 points, ring buffer overflow at 500, ENU accuracy at 1km offset |
| `freshness.ts` | fresh, stale, exactly at TTL boundary (convention: `nowMs - lastSeenMs === ttlMs` is stale — document this in freshness.ts) |
| `isFixedWing` (trajectory) | each MAV_TYPE in the fixed-wing list returns true; QUADROTOR(2), HEXAROTOR(13), OCTOROTOR(14), TRICOPTER(15), COAXIAL(8) return false |

**freshness TTL boundary convention:** `isFresh` returns `true` if `nowMs - lastSeenMs < ttlMs` (strictly less than). At exactly TTL, the value is stale. Document this convention in `freshness.ts` so FreshnessRing components don't implement their own boundary logic differently.

---

### Chapter 4: Test Harness Configuration

**Goal:** Ensure test infrastructure is wired so both Go and TS test suites run in CI from day one.

**Go:**
- `go test ./...` runs from repo root
- `-race` flag enabled by default in CI Makefile target
- Fixture files read from `contracts/mavlink/` using path relative to test file (or `testdata/` symlink)

**TypeScript:**
- `vitest` configured in `frontend/vite.config.ts` or `vitest.config.ts`
- Test files match `src/logic/**/*.test.ts`
- No DOM environment needed for logic tests — use `environment: 'node'` in vitest config

**Makefile targets:**
```makefile
test-go:
	go test -race ./...

test-ts:
	cd frontend && pnpm vitest run

test: test-go test-ts
```

---

## Tier Exit Gate

- Capability matrix exists at `docs/wip/codec-capability-matrix.md` with all case IDs populated
- No case ID is `unstarted` — every receive/send family has a fixture named and a status
- Golden byte fixtures committed to `contracts/mavlink/`
- TS resolver tests use named fixture files — no inline magic numbers
- `make test` runs both suites from repo root
- `go test -race ./internal/codec/...` passes
