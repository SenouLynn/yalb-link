---
id: T-012
title: Bring the commanded mission into the map viewport
status: done
priority: 1
owner: unassigned
depends_on: T-011
---

## Motivation and evidence

Observed in a browser at `/?source=mock` after T-011: the fixture mission spans
roughly 220 m north/south and 350 m east/west, and its markers `1`, `3` and `4`
sat clipped against the edges of the map panel while marker `0` sat under the
vehicle. Only the part of the route near the vehicle was legible.

`MapPanel` centres on the vehicle and jumps to `zoom 16` on first fix
(`MapPanel.tsx`, the `position` effect), then re-centres on the vehicle
whenever the map is not being moved. Nothing ever considers the mission extent.
A real ArduPilot mission is usually larger than the fixture, so on live SITL the
commanded route will often be entirely outside the viewport — the operator
downloads a mission, the list fills, and the map appears not to have changed.

Map viewport behaviour was explicitly out of scope for T-007 and T-009, so this
is deferred work rather than a regression.

## Outcome

A downloaded mission is visible on the map without the operator panning to find
it, and the vehicle-following behaviour that exists today is not lost.

## Scope

- In: fitting the viewport to a newly downloaded mission's extent, the
  interaction between that and vehicle-following re-centring, and the
  degenerate cases of an empty and a single-point mission.
- Out: mission mutation, viewport persistence across reloads, terrain, offline
  imagery, and automated browser verification.

## Acceptance criteria

- [x] Completing a download frames the mission's positional items, including
      when they lie outside the current viewport.
- [x] An empty mission and a single-point mission do not produce an invalid or
      degenerate viewport.
- [x] Vehicle position updates do not immediately undo the mission framing, and
      an operator's own pan is still not fought by the map.
- [x] The behaviour is decided by pure, tested logic; only the camera call
      itself is untested.

## Verification

```sh
cd frontend && pnpm typecheck && pnpm lint
cd frontend && pnpm vitest run src/map src/ui
cd frontend && pnpm dev --port 3001   # then /?source=mock, download, observe
```

## Open questions

- Whether framing the mission should suspend vehicle-following until the
  operator acts, or re-centre after a delay. Settle before implementation;
  the two produce noticeably different behaviour on a moving vehicle.

## Notes

Found while getting first eyes on the mission UI, not by a failing test. The
pure-geometry tests pass either way because the defect is entirely in the
camera.


Completed: a completed snapshot frames its positional items once. Mission framing
and user pan suspend following until Follow vehicle is clicked; Fit mission is
also available explicitly. Empty missions do not move the camera, single points
use zoom <=16, and date-line bounds use the shortest longitude arc.
Typecheck/lint and 91 map/UI tests pass. Browser at 1440×900 confirmed all four
mock mission markers are inside the viewport after download, with following off.
Evidence: `docs/temp/evidence/t012/framing.json` and `mission.png`.
