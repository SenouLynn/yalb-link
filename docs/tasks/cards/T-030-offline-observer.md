---
id: T-030
title: Demonstrate observer startup without internet
status: done
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

Public basemap imagery is a known network dependency; field observation must have an explicit offline contract. See the [operator usage plan](../operator-usage-plan.md) for current evidence and scenario definitions.

## Outcome

Demonstrate observer startup without internet with explicit evidence and remaining limits.

## Scope

Exercise local application startup with network disabled and empty browser cache; fix local asset dependencies and expose imagery-unavailable behavior. Mission editing and tile storage architecture are excluded.

## Acceptance criteria

- [x] Application loads without internet or a warm browser cache.
- [x] Local telemetry, mission list and position/track/mission overlays remain usable when tile fetches fail.
- [x] Recording/replay works locally; missing imagery does not imply missing telemetry.

## Verification

```sh
GCS_RECORDING_ENABLED=true docker compose up -d --build --wait gcs-backend ardupilot-sitl-copter-1
pnpm --dir frontend test
pnpm --dir frontend run lint
pnpm --dir frontend run typecheck
```

Full offline-browser and replay procedure: [T-030 runbook](../../runbooks/validation/t030.md).

## Open questions

Whether the operator later requires preloaded imagery; this does not block proving explicit imagery-free behavior. Settled as out of scope for this card — `localTileSource()` remains an unused, uninstantiated option.

## Notes

Completed 2026-09-14: MapPanel and FleetMap had no handling for a failed
basemap tile request — MapLibre reports it as a `map.on('error',
{sourceId: 'basemap'})` event, not a thrown exception, and nothing listened
for it. Both now show an explicit amber `Imagery unavailable` chip, cleared on
the next successful style load. A frontend dependency audit found no other
runtime network dependency (fonts, CDN scripts) beyond the three basemap tile
hosts, so blocking those three at the browser's `Network.setBlockedURLs` while
leaving `localhost` traffic untouched reproduces "internet disconnected, local
stack reachable" for this application without touching host networking.

Verified end-to-end with Docker Compose (backend + stationary Copter 4.7.0
SITL) and headless Chrome over CDP, fresh profile and empty disk cache: fleet
overview and vehicle workspace both loaded fully offline with live telemetry,
mission download worked and showed the commanded-mission overlay, and a
recording taken across the whole run replayed correctly offline with the same
explicit imagery-unavailable signal. `pnpm test`, `lint` and `typecheck` are
clean; the pre-existing 4 unrelated `FlightDisplay`/`App` test failures are
unchanged. Full procedure, results and limitations in the linked runbook.
