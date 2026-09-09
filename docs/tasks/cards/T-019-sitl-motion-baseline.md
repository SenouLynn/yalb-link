---
id: T-019
title: Establish the Copter motion baseline evidence chain
status: backlog
priority: 0
owner: unassigned
depends_on: none
---

## Motivation and evidence

First independently reviewable deliverable of [T-018](T-018-sitl-motion-scenarios.md).
Compound routes must follow a measured baseline.

## Outcome

A repeatable external Guided Copter driver captures stationary, straight and
single-turn autopilot telemetry and supports end-to-end evidence collection.

## Scope

- In: existing Compose Copter 4.7.0 (+), external pymavlink driver, bounded
  waits, raw MAVLink and timestamped decoded evidence, interruption/recovery.
- Out: application command endpoints and compound route acceptance.

## Acceptance criteria

- [ ] Run at home 37.7749,-122.4194 (10 m MSL), take off to 20 m relative
  altitude, hold 10 s, fly 100 m north then 100 m east at requested 5 m/s.
- [ ] Require each endpoint within 3 m horizontal and 2 m relative altitude
  in 90 s; land and observe disarm within 90 s. Never force arm.
- [ ] Capture received binary MAVLink and decoded messages with host timestamps;
  preserve commands separately and identify firmware from AUTOPILOT_VERSION.
- [ ] Compare received position, altitude datum and time with backend events;
  capture browser stationary/straight/turn evidence and five-second prediction
  errors (straight <=10 m p95; turns reported separately, <=30 m p95).
- [ ] Withhold position for 8 s using simulator message-rate input; verify UI
  stale after 5.25 s and recovery within 2 s of restoring 5 Hz.
- [ ] Capture and replay a backend recording and document live differences.
- [ ] Exercise runner timeout/failure handling and task-board validation.

## Verification

```sh
python3 -m unittest discover -s scripts -p 'test_sitl_motion.py'
python3 scripts/sitl_motion.py --output /private/tmp/yalb-baseline
./scripts/kanban check
```

Browser and recording checks follow docs/runbooks/dev-setup.md. Bounds above
are fixed before the first run; actual measurements must be reported separately.

## Open questions

None for baseline vehicle and input choice. Compound route geometry remains T-018.

## Notes

Split from T-018 during execution at the user's request. Baseline completion
is a prerequisite to building the compound patterns.

Execution deferred when the user corrected the requested plan to T-017. No
flight commands issued; external pymavlink 2.4.49 installed in a temporary venv.
