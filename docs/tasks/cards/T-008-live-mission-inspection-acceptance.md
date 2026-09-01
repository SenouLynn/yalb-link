---
id: T-008
title: Accept live mission inspection with Copter and Plane
status: backlog
priority: 0
owner: unassigned
depends_on: T-006, T-007
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

- [ ] Copter and Plane each return the exact count and ordered values of a
      known externally loaded mission.
- [ ] Each mission's supported positional items appear in both the ordered list
      and the correctly ordered map geometry for the selected vehicle.
- [ ] A completed zero-item mission is visibly empty rather than failed or
      indefinitely loading.
- [ ] A controlled timeout or rejected transfer produces an explicit failure
      and a subsequent request can succeed without restarting the backend.
- [ ] Any defect found is fixed or represented by a new board card.
- [ ] The repeatable procedure and observed results are recorded in the
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

None.
