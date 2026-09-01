---
id: T-001
title: Demonstrate predicted trajectory with live Copter SITL
status: done
priority: 0
owner: unassigned
depends_on: none
---

## Motivation and evidence

The root README names a live Copter/browser acceptance run as part of the next
demonstrable outcome. The trajectory currently has deterministic logic and UI
tests, but those do not establish its behavior against live SITL telemetry.

## Outcome

The browser's predicted trajectory is exercised against live Copter SITL and
the repeatable acceptance procedure and result are captured in repository
evidence.

## Scope

- In: live Copter telemetry, map track and prediction behavior, and defects that
  prevent the acceptance outcome.
- Out: Plane acceptance, offline imagery, 3D tiles, and vehicle commanding.

## Acceptance criteria

- [x] Copter SITL telemetry drives a visible live position track and prediction.
- [x] The prediction responds plausibly to a heading or velocity change.
- [x] Any defect found is fixed or represented by a new board card.
- [x] The repeatable procedure and observed result are recorded in the relevant
      runbook or executable check.

## Verification

```sh
docker compose --profile ui up
make test
```

Manual: open the live UI, fly or reposition Copter, and observe the current
position, track, and five-second prediction.

## Open questions

None. Two Guided position targets on opposite north/south legs make the vector
change unambiguous without requiring a mission upload.

## Notes

Accepted on 2026-09-01 against the Compose ArduCopter 4.7.0 SITL. Browser
inspection observed a live 500-point track and an 11-coordinate prediction;
the in-motion sample was 9.9 m/s on a 180-degree heading and projected about
50 m over the configured five-second horizon. No defect was found.
