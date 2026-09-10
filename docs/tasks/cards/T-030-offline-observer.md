---
id: T-030
title: Demonstrate observer startup without internet
status: ready
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

- [ ] Application loads without internet or a warm browser cache.
- [ ] Local telemetry, mission list and position/track/mission overlays remain usable when tile fetches fail.
- [ ] Recording/replay works locally; missing imagery does not imply missing telemetry.

## Verification

Using existing local stack and SITL, disconnect external networking while preserving localhost/container connectivity; exercise E2E-03 browser/recording portions and document actual results. Run frontend checks appropriate to any fixes.

## Open questions

Whether the operator later requires preloaded imagery; this does not block proving explicit imagery-free behavior.

## Notes

Planned, not executed. Dependencies sequence outcomes, not proof of hardware
compatibility. Refine implementation commands and scenario bounds before claim.
