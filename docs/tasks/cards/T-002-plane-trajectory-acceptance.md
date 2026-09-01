---
id: T-002
title: Demonstrate predicted trajectory with live Plane SITL
status: done
priority: 0
owner: unassigned
depends_on: none
---

## Motivation and evidence

The root README names a live Plane/browser acceptance run as part of the next
demonstrable outcome. The trajectory currently has deterministic logic and UI
tests, but those do not establish its behavior against live SITL telemetry.

## Outcome

The browser's predicted trajectory is exercised against live Plane SITL and
the repeatable acceptance procedure and result are captured in repository
evidence.

## Scope

- In: live Plane telemetry, map track and prediction behavior, and defects that
  prevent the acceptance outcome.
- Out: Copter acceptance, offline imagery, 3D tiles, and vehicle commanding.

## Acceptance criteria

- [x] Plane SITL telemetry drives a visible live position track and prediction.
- [x] The prediction responds plausibly to a heading or velocity change.
- [x] Any defect found is fixed or represented by a new board card.
- [x] The repeatable procedure and observed result are recorded in the relevant
      runbook or executable check.

## Verification

```sh
docker compose --profile multi-sitl --profile ui up \
  gcs-backend ardupilot-sitl-plane-2 gcs-frontend
make test
```

Manual: open the live UI, fly or reposition Plane, and observe the current
position, track, and five-second prediction.

## Open questions

None. A Guided position target after autonomous takeoff produces a sustained
banked turn, making the changing prediction vector unambiguous without a
mission upload.

## Notes

Accepted on 2026-09-01 against the Compose ArduPlane 4.6.3 SITL. Headless
browser inspection observed live fixed-wing vehicle `2:1`, the map canvas and
vehicle marker, and the `5 S PREDICTION` overlay. During a Guided turn, browser
heading changed from 55 to 87 degrees at 22.1 m/s while the projection remained
visible and rotated with the aircraft, a roughly 110 m five-second horizon.
The preceding live MAVLink samples covered the full circuit and populated the
position track. No defect was found.
