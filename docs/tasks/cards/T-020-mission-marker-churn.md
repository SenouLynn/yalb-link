---
id: T-020
title: Retain mission markers across unchanged telemetry renders
status: done
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

T-017 confirmed `missionGeometry(mission.snapshot)` creates a new object on
all selected-display renders. MapPanel's `[mission]` marker effect removes and
recreates each DOM marker, including every 250 ms freshness tick. Geometry
source updates are separate from this marker churn.

## Outcome

An unchanged downloaded mission retains its marker instances as telemetry ages.

## Scope

- In: stable mission geometry and meaningful marker lifecycle regression check.
- Out: prediction caching, route framing (T-012), mission styling (T-014).

## Acceptance criteria

- [x] An unchanged snapshot retains the same mission geometry object across
      telemetry renders and 250 ms freshness ticks. Over a 2 s settled browser
      observation, an unchanged four-point mission causes zero mission-marker
      additions/removals (baseline: 108 of each), with no map remount.
- [x] Refreshing with changed items replaces the route/labels; an empty mission
      removes them. Switching A → B → A never shows A's old route for B or
      revives A's discarded snapshot. Existing request cancellation is preserved.
- [x] Stale position/heading/speed still remove prediction at the existing TTL;
      stale MISSION_CURRENT removes the active highlight without removing the
      downloaded mission. Restoring telemetry restores the appropriate display.
- [x] A focused regression fails on the existing allocation behavior and passes
      with the fix. Existing map geometry and mission lifecycle checks still pass.

## Verification

```sh
cd frontend && pnpm typecheck && pnpm lint && pnpm vitest run && pnpm build
./scripts/kanban check
```

Use the existing createRoot/happy-dom lifecycle test style to observe mission
prop identity or marker lifecycle during clock/telemetry changes, then changed
snapshot and selection transitions. Avoid testing only that a memo hook exists.

Browser: download the four-point mock mission, wait for completion, then count
`.mission-marker` child additions/removals with MutationObserver over 2 s.
Repeat on a known downloaded live SITL mission with the real backend and UI.
Change/clear that mission externally and refresh to verify invalidation; pause
telemetry >5 s and confirm freshness behavior. Record actual counts and artifacts
in the development runbook. This performance check does not require flight.

## Open questions

None for the bounded fix. Prefer stabilizing geometry at the snapshot owner;
only change the MapPanel reconciliation strategy if evidence requires it.
Prediction caching and route framing remain outside this chunk.

## Notes

Observed during T-017 source review; confirmed by the explicit fresh return
object and `[mission]` effect. No claim of a measured frame-rate regression.

Browser MutationObserver confirmed 108 marker additions and 108 removals
over 2000 ms for an unchanged 4-point mock mission during telemetry.
See `docs/runbooks/evidence/t017/marker-churn.json`. No frame-rate claim.


Selected next during the fleet-view planning pass. This is one reviewable fix:
`SelectedFlightDisplay` geometry ownership, a focused lifecycle regression, and
browser/SITL allocation evidence. No telemetry inspection fields, fleet UI,
transport changes or unrelated rendering optimization are bundled into it.
Keep status ready until implementation begins, then claim before editing code.
After this chunk, resume T-015; T-022 now holds the separately actionable fleet
navigation work. Sequencing is a recommendation, not a new dependency chain.


Completed: memoized geometry at its snapshot owner. A regression was first run
against the old allocation and failed on reference identity after a clock tick;
it now passes. All 347 frontend tests, typecheck, lint and build pass.
Browser MutationObserver measured zero additions and zero removals over 2 s
for both the four-point mock and four-point stationary Copter SITL mission.
Live external refresh to two items changed the markers to two; clearing removed
all markers. A 6.5 s simulator pause kept the downloaded mission while removing
its active highlight and making 14 readings stale; resume restored freshness and
highlight. Artifacts: `docs/runbooks/evidence/t020/{mock,live,invalidation}.json`.
No frame-rate or moving-flight prediction accuracy claim is made.
