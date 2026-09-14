---
id: T-043
title: Expose reference map selection and manual navigation controls
status: done
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

The operator requested another comparison with flight-hud-hud and porting map
controls and overlays. The documented reference is the sibling flight-path-hud,
apps/gcs/src/map/MapControls.tsx and MapPanel.tsx. YALB already has a basemap
catalogue, track, prediction and mission overlays, but exposes only Fit/Follow.

## Outcome

Operators can select a basemap and manually zoom the fleet and vehicle maps.
Vehicle overlays remain intact across basemap changes.

## Scope

- In: basemap selector, manual zoom, north reset, reference camera tilt/reset controls.
- Out: new telemetry contracts, terrain DEM, buildings and guided commands.

## Acceptance criteria

- [x] Fleet and vehicle maps expose labelled basemap selection and zoom controls.
- [x] Vehicle map exposes tilt and reset view without interfering with Follow.
- [x] Changing styles restores current track, prediction and mission geometry.
- [x] Existing local tile configurations remain selectable.

## Verification

```sh
pnpm --dir frontend test -- src/map
pnpm --dir frontend typecheck
pnpm --dir frontend lint
./scripts/kanban check
```

## Open questions

- Resolved: operator confirmed terrain, roads, satellite and the future 3D view; keep all ported controls.

## Notes

This is presentation and camera work; no new trajectory accuracy or SITL claim.
Existing edits to UI primitives, Provenance and system.css belong to other work.

Map regression tests: 11 passed, including rapid style changes with telemetry
updates and manual zoom. Full suite run: 388 passed, four failures in App and
FlightDisplay tests expecting the prior vehicle selector and battery precision;
concurrent VehiclePane/UI edits change those surfaces. No browser/WebGL or SITL
acceptance was performed. 3D is camera tilt only, matching the reference stub;
no DEM or hosted 3D tiles are configured. Existing Fit/Follow remain available.

**2026-09-14 follow-up:** the basemap selector this task added was removed
from both maps in a later UI pass (button/layout polish on `MapControls`) —
it never earned its space against the operator's own judgement of ground
imagery. The config-driven restyle path (`MapPanel`'s `tileSource` prop) is
unaffected and still covered by `MapPanel.lifecycle.test.tsx`; only the
operator-facing `<select>` and its `onSourceChange` plumbing are gone. The
first three acceptance boxes above no longer hold for the selector itself —
left checked as the historical record of what this task shipped, not as a
claim about the current UI.
