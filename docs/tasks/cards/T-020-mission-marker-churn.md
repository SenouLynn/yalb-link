---
id: T-020
title: Retain mission markers across unchanged telemetry renders
status: ready
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

- [ ] Download a mission and count marker creation/removal across freshness ticks;
      unchanged snapshots do not rebuild markers.
- [ ] Refresh and vehicle switches update/remove the correct markers.
- [ ] Freshness still invalidates trajectory and active-sequence posture.

## Verification

Frontend typecheck, lint, Vitest; browser MutationObserver on `.mission-marker`
while a downloaded mission remains unchanged for at least 2 seconds.

## Open questions

None. Measure the allocation impact before broadening the fix.

## Notes

Observed during T-017 source review; confirmed by the explicit fresh return
object and `[mission]` effect. No claim of a measured frame-rate regression.

Browser MutationObserver confirmed 108 marker additions and 108 removals
over 2000 ms for an unchanged 4-point mock mission during telemetry.
See `docs/runbooks/evidence/t017/marker-churn.json`. No frame-rate claim.
