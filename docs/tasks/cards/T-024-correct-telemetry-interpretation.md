---
id: T-024
title: Correct guidance units and radio readout semantics
status: done
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

Branch review found reversed guidance sign labels, unhandled ArduPlane airspeed
error scaling, inverted RADIO_STATUS buffer meaning, and the wrong unknown
sentinel. MAVLink common.xml defines free buffer space and 255 as unknown;
ArduPlane sends target-minus-measured airspeed error multiplied by 100.

## Outcome

The inspection panel reports guidance and radio values with correct units,
direction labels, and unknown-value handling.

## Scope

Correct the four review findings and their contract comments and fixtures.
T-023 remains separate.

## Acceptance criteria

- [x] Guidance labels describe positive errors as below target.
- [x] ArduPlane/QuadPlane airspeed error is scaled to m/s using heartbeat identity;
      unknown identity does not produce an assumed airspeed error.
- [x] Radio buffer shows free space; signal and noise preserve 254 and omit 255.
- [x] Regression checks, frontend build/lint/typecheck/tests, and board check pass.

## Verification

```sh
(cd frontend && pnpm typecheck && pnpm lint && pnpm vitest run && pnpm build)
make proto-lint
make test-go
./scripts/kanban check
```

## Open questions

None.

## Notes

Automated regressions cover protocol interpretation; live SITL is not rerun for
these corrections. Existing T-015 captures do not prove nonzero Plane errors.


Implementation preserves the existing protobuf wire value so prior recordings
remain compatible, names it `aspdErrorRaw` in the browser, and normalizes in the
resolver using heartbeat autopilot and vehicle type. Rover and other autopilots
retain standard m/s. The radio fixture now decreases free buffer space as the
link degrades. Missing radio telemetry no longer asserts that hardware is absent.

Protocol references:
- [MAVLink RADIO_STATUS](https://github.com/mavlink/mavlink/blob/master/message_definitions/v1.0/common.xml)
- [Pinned Plane 4.6.3 sender](https://github.com/ArduPilot/ardupilot/blob/Plane-4.6.3/ArduPlane/GCS_Mavlink.cpp#L187)
- [Plane altitude error](https://github.com/ArduPilot/ardupilot/blob/Plane-4.6.3/ArduPlane/altitude.cpp)
- [Plane airspeed error](https://github.com/ArduPilot/ardupilot/blob/Plane-4.6.3/ArduPlane/navigation.cpp)

Verification: 342 frontend tests pass, including both heartbeat/telemetry arrival
orders, signed and zero Plane errors, QuadPlane, Rover, other autopilots, missing
identity, and signal/noise sentinel boundaries. Frontend lint, typecheck, production
build, protobuf lint, and diff whitespace checks pass. Protobuf bindings were
regenerated with the pinned generators; only comments changed.

The full Go race suite also passed. Bazel and live SITL were not rerun.
