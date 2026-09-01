---
id: T-001
title: Demonstrate predicted trajectory with live Copter SITL
status: ready
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

- [ ] Copter SITL telemetry drives a visible live position track and prediction.
- [ ] The prediction responds plausibly to a heading or velocity change.
- [ ] Any defect found is fixed or represented by a new board card.
- [ ] The repeatable procedure and observed result are recorded in the relevant
      runbook or executable check.

## Verification

```sh
docker compose --profile ui up
make test
```

Manual: open the live UI, fly or reposition Copter, and observe the current
position, track, and five-second prediction.

## Open questions

- Which maneuver gives the shortest repeatable evidence of a changing vector?

## Notes

None.
