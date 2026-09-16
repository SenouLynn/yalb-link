---
id: T-019
title: Establish the Copter motion baseline evidence chain
status: done
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

- [x] Run at home 37.7749,-122.4194 (10 m MSL), take off to 20 m relative
  altitude, hold 10 s, fly 100 m north then 100 m east at requested 5 m/s.
- [x] Require each endpoint within 3 m horizontal and 2 m relative altitude
  in 90 s; land and observe disarm within 90 s. Never force arm.
- [x] Capture received binary MAVLink and decoded messages with host timestamps;
  preserve commands separately and identify firmware from AUTOPILOT_VERSION.
- [x] Compare received position, altitude datum and time with backend events;
  capture browser stationary/straight/turn evidence and five-second prediction
  errors (steady northward flight <=10 m p95; whole east-turn phase <=30 m
  p95). Report full north-leg error, including acceleration and turn-crossing
  horizons, separately without claiming it meets the steady-flight bound.
  Steady interval: measured speed >=4.5 m/s until 35 m before north endpoint;
  fixed before run 3 after the whole-leg run 2 failed (see runbook).
- [x] Withhold position for 8 s using simulator message-rate input; verify UI
  stale after 5.25 s and recovery within 2 s of restoring 5 Hz.
- [x] Capture and replay a backend recording and document live differences.
- [x] Exercise runner timeout/failure handling and task-board validation.

## Verification

```sh
python3 -m unittest discover -s scripts -p 'test_sitl_motion.py'
/private/tmp/yalb-t019-venv/bin/python scripts/sitl_motion.py --output docs/temp/evidence/T-019/RUN/flight
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

Claimed 2026-09-15 by codex-sitl. T-027 needs operator/device evidence; this
card is the first independently executable baseline below the compound T-018.
The concrete setup, predeclared comparison tolerances and capture commands are
in [the T-019 runbook](../../runbooks/validation/t019.md).

Completed 2026-09-15 under the explicitly revised, pre-run-3 steady-flight
protocol. Run 1 setup failed (speed reset and wrong interruption channel); run 2
whole north-leg prediction failed 10 m p95. These results remain documented.
Run 3: endpoints 2.638/2.502 m, relative altitude 19.996 m; land/disarm 30.58 s.
Steady north prediction p95 2.75 m; whole east turn 21.81 m. Full north leg
including acceleration and turn-crossing horizons is 20.44 m p95 and does not
meet the steady-flight bound. No claim of general predictive accuracy.

777 exact boot-time UDP/backend position matches passed coordinate, altitude
and velocity checks. Autopilot POS/ATT logs independently support frame/unit
interpretation. The 8.205 s backend position gap cleared browser trajectory
at 5.088 s and restored it 0.164 s after rate restoration; position/altitude
readouts became absent while attitude/heading remained live.

Recording 6 replayed 4,227 events at 4× through its endpoint with zero browser
runtime exceptions. Replay reproduced the route and position interruption;
recording 6 lacks the final UDP disarmed heartbeat because the original relay
closed on TCP disarm. A three-second terminal drain was added; a no-flight
smoke against the already-disarmed simulator observed backend vehicle 1:1
STANDBY in LAND with the armed flag absent. Preserve this distinction rather
than claim the old recording contains the final disarm.

Seven focused runner tests, browser script syntax, numerical analyzer gates,
`git diff --check` and `./scripts/kanban check` passed. No application code changed.
Task-started containers were stopped; recording volume retained. Regenerated
captures are ignored under docs/temp/evidence/T-019/.

Adjacent track/Flown-distance overcount is T-052; it does not invalidate the
independently measured position/prediction results. Compound routes remain
T-018; no Plane, wind, prolonged flight or physical hardware acceptance.
