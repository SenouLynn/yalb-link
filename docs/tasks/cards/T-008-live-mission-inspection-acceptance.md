---
id: T-008
title: Accept live mission inspection with Copter and Plane
status: done
priority: 0
owner: unassigned
depends_on: T-006, T-007, T-010
---

## Motivation and evidence

Contract, coordinator, HTTP, and fixture tests do not establish interoperability
with ArduCopter and ArduPlane or prove that the browser presents a real onboard
mission correctly.

## Outcome

The complete read-only mission inspection path is demonstrated against both
Compose SITL vehicle types, with a repeatable procedure and observed results in
repository evidence.

## Scope

- In: preloading ordinary missions through an external ground station, live
  download through YALB-GCS, browser list/map inspection, empty mission posture,
  one controlled transfer failure, and defects required for acceptance.
- Out: using YALB-GCS to mutate a mission, fences, rallies, offline imagery, and
  automated browser infrastructure.

## Acceptance criteria

- [x] Copter and Plane each return the exact count and ordered values of a
      known externally loaded mission.
- [x] Each mission's supported positional items appear in both the ordered list
      and the correctly ordered map geometry for the selected vehicle.
- [x] A completed zero-item mission is visibly empty rather than failed or
      indefinitely loading.
- [x] A controlled timeout or rejected transfer produces an explicit failure
      and a subsequent request can succeed without restarting the backend.
- [x] Any defect found is fixed or represented by a new board card.
- [x] The repeatable procedure and observed results are recorded in the
      relevant runbook or executable check.

## Verification

```sh
docker compose --profile multi-sitl --profile ui up
make test
```

Manual: load known missions into Copter and Plane with an external ground
station, request each from the live UI, and compare counts, ordered item values,
map geometry, active-sequence posture, and empty/error states.

## Open questions

None. Use ordinary missions only; fence and rally protocols remain separate
future slices even though they share MAVLink message families.

## Notes

Promoted from backlog once T-010 (defects only a live link or a real browser
can show) and T-011 (the same UI, demonstrable with no backend) were done. The
mission UI has now been seen rendering — with fixtures, in a browser — so this
card is about interoperability with ArduPilot, not about first contact with the
display.

Carry into the session:

- ArduPilot reports the home position as mission item `seq 0`, and counts it.
  A mission of N waypoints loaded by an external ground station is expected to
  download as N+1 items with item 0 at home. Confirm this against the actual
  vehicles before recording a count mismatch as a defect, and say which
  convention the recorded procedure uses.
- Downloads are single-attempt by design (T-005): one dropped datagram fails
  the transfer on the 5 s per-response timeout. Over Compose loopback this
  should be rare, but the criterion about a subsequent request succeeding is
  the one most likely to expose it. If it does show up, that is evidence for
  the bounded retry policy T-005 deferred, and belongs in a new card rather
  than in this one.
- The map does not yet frame a downloaded mission (T-012). Expect to pan or
  zoom out to see the route, and do not record that as a new defect.
- `MISSION_CURRENT` is now requested at 1 Hz by the rate policy, so the active
  item highlight should appear without external setup. It is worth checking
  explicitly, because it was unreachable before T-010 and no test covers the
  live path.

T-017 session evidence (2026-09-09): both six-item downloads, selected route
and active seq 0, interrupted request switching, explicit timeout and subsequent
success were captured under T-017, since pruned. Independent ordered
value comparison and zero-item completion were subsequently accepted after
claiming this card; see the T-008 follow-through in the evidence README.

Final acceptance: both external MAVLink/HTTP comparisons and live empty UI
checks passed. The first external Copter clear timed out, then a separate
retry succeeded; no application retry policy changed. See the durable
`mission-comparison.json` and `zero-missions.json` artifacts.
