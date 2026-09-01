---
id: T-007
title: Show a downloaded mission in the flight display
status: backlog
priority: 0
owner: unassigned
depends_on: T-004
---

## Motivation and evidence

The trajectory end-state established `MapPanel` as the renderer boundary and
states that its dashed amber line is a prediction, not a commanded route. The
map has no representation of the vehicle's onboard mission. The shared
snapshot contract from T-004 allows that distinct route view to be developed
against fixtures independently of the live HTTP implementation.

## Outcome

An operator can request and inspect the selected vehicle's ordinary mission as
an ordered list and a visually distinct map overlay without confusing it with
the flown track or five-second prediction.

## Scope

- In: a client for `GET /api/vehicles/{system_id}/{component_id}/mission`, a
  download action, loading/empty/error/complete postures, ordered item list,
  supported global waypoint map geometry, non-positional item presentation,
  active-sequence indication from fresh `MISSION_CURRENT`, deterministic
  fixtures, and component/pure-logic tests.
- Out: editing, drag handles, upload, clear, start, set-current, fence/rally
  views, terrain-relative conversion, and MapLibre browser automation.

## Acceptance criteria

- [ ] Mission loading is explicit and tied to the full selected vehicle
      identity; switching vehicles cannot display the prior vehicle's mission.
- [ ] Empty, loading, failed, and complete downloads are distinguishable and a
      failed refresh does not masquerade as a current snapshot.
- [ ] Ordered items retain command, frame, parameters, coordinates, altitude,
      and autocontinue values in an inspectable list.
- [ ] Supported global positional items render as a route and numbered points
      styled distinctly from the live track and predicted trajectory.
- [ ] Non-positional or unsupported-frame items remain visible in the list and
      are omitted from geometry with an explicit explanation.
- [ ] A fresh `MISSION_CURRENT` highlights the active sequence; absent or stale
      mission state does not claim an active item.

## Verification

```sh
cd frontend && pnpm vitest run src/mission src/map src/ui
cd frontend && pnpm typecheck
```

## Open questions

None. The UI consumes fixtures until T-006 supplies the same protobuf-JSON
contract over HTTP.

## Notes

The mission line is commanded intent, unlike the amber prediction. Labels and
styling must make that semantic difference visible without relying on color
alone.
