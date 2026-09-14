---
id: T-023
title: Re-request telemetry rates when a vehicle returns on a new link
status: done
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

Observed during [T-015](T-015-render-discarded-telemetry.md) acceptance on
2026-09-09, with a Compose Copter and the backend log at `GCS_LOG_LEVEL=info`.

Restarting `ardupilot-sitl-copter-1` makes the autopilot rebind its UDP client
socket, so it returns on a new source port. The backend logs:

```text
link open        link=udp:172.30.250.2:48241
vehicle warning  warning="SOURCE_CONFLICT sysid=1 compid=1
                 previous=\"udp:172.30.250.2:52838\" current=\"udp:172.30.250.2:48241\""
fleet event      type=FLEET_EVENT_TYPE_HEARTBEAT_UPDATED sysid=1 compid=1
```

The vehicle is never marked lost — its heartbeats never stopped — so no
`VEHICLE_DISCOVERED` or recovery event fires, and `RateRequester` never re-runs.
`DefaultRates` was addressed to the old link and the vehicle now answers on a
new one, so **every requested family goes silent**. Measured over a 10 s window
after the restart, the SSE stream carried exactly one of each family: the
retained-state bootstrap, and nothing live.

This is not specific to the family T-015 added. Attitude, position, VFR_HUD,
GPS, battery, EKF and MISSION_CURRENT all stop. The display shows a vehicle that
is present and heard, with every instrument dashed. Restarting the backend
restores everything, which is the workaround used to finish T-015 acceptance —
see the [T-015 evidence](../../runbooks/validation/t015.md).

`SOURCE_CONFLICT` is already detected and logged; nothing acts on it.

## Outcome

A vehicle that returns on a different link resumes streaming the requested
families without restarting the backend.

## Scope

- In: treating an addressed-link change as a trigger for the existing rate
  policy, and a regression test that fails if a source change leaves the rates
  addressed to the previous link.
- Out: changing `DefaultRates` itself, the freshness TTL, the vehicle
  loss/recovery thresholds, and the display's stale posture.

## Acceptance criteria

- [x] A vehicle whose route changes to a new link has `DefaultRates` re-issued,
      addressed to the new link.
- [x] A test fails if a source change does not re-request, following
      `TestRateRequestsAddressTheObservedLink` and the recovery test beside it.
- [x] Re-requesting is bounded: a flapping source cannot issue rate requests
      faster than the existing recovery path would.
- [x] Demonstrated on Compose by restarting the SITL container alone and
      observing the families resume, with no backend restart.

## Verification

```sh
go test -race ./internal/bridge/... ./internal/vehicle/... ./internal/routes/...
docker compose up -d --build gcs-backend ardupilot-sitl-copter-1
docker compose restart ardupilot-sitl-copter-1
curl -sN --max-time 10 http://localhost:8080/api/events   # families must resume
```

## Open questions (resolved)

- Trigger source: `RateRequester.trigger` now reacts directly to the existing
  `vehicle.Warning{Type: WarningSourceConflict}` event the fold already emits,
  rather than adding a route-table hook or a new `FleetEventType`. The warning
  already carries the vehicle identity `sourceConflict` computed, and by the
  time it reaches the sink the route table has already been upserted to the
  new link (`bridge.frame` upserts before folding), so `Routes.Resolve` returns
  the new link with no additional plumbing. This is narrower than a new fleet
  event: the browser and recordings still cannot see that a link moved. That
  remains open for a follow-on card if operator-visible link-change history is
  wanted; it is not required to stop telemetry going silent.
- `SOURCE_CONFLICT` stays a `Warning` (still logged at `WARN`), not promoted to
  an ordinary fleet transition. Nothing about its meaning changed — it is still
  reporting an anomaly (the same identity answering from a different address);
  this card only makes the requester act on it. Revisit if a later card wants
  it surfaced to the operator.

## Notes

Found while accepting T-015, and deliberately not fixed there: it predates that
card, affects all eight previously requested families equally, and is a backend
acquisition concern rather than a display one.

## Resolution — 2026-09-14

`RateRequester.trigger` (internal/bridge/rates.go) now also fires on a
`WarningSourceConflict` event for the autopilot component, subject to a
per-vehicle `MinConflictInterval` (defaults to `codec.HeartbeatTTL`) so a
flapping address cannot re-issue the policy faster than loss/recovery already
could. `isAutopilot` factors out the existing non-autopilot-component guard so
both trigger paths use it.

New tests in `internal/bridge/rates_test.go`: `TestRateRequestsOnSourceConflict`,
`TestRateRequestsBoundAgainstFlappingSource`,
`TestNoRateRequestsForSourceConflictOnNonAutopilotComponent`. Existing tests
(`TestNoRateRequestsForOrdinaryEvents`'s warning case, `TestRateRequestsOnRecovery`,
etc.) pass unmodified in behavior; only the ordinary-events case was renamed to
an explicit `WarningUnspecified` warning to keep asserting the true negative.

`go test -race ./internal/bridge/... ./internal/vehicle/... ./internal/routes/...`
and `go test ./...` pass; `go vet ./...` is clean.

Compose demonstration run and captured at
`docs/temp/evidence/T-023/2026-09-14/`: discovery on
`udp:172.30.250.2:45724`, then `docker compose restart ardupilot-sitl-copter-1`
alone (backend never restarted) rebinds to `udp:172.30.250.2:37935`; the
backend logs `SOURCE_CONFLICT` immediately followed by
`telemetry rates requested ... trigger=SOURCE_CONFLICT` and 9 accepted ACKs,
and a 10 s post-restart SSE sample carried 271 live telemetry events plus the
next heartbeat update — full recovery with no backend restart.

Limits: Copter SITL only, per [SITL-first validation](../README.md). The
underlying real-hardware motivation (Plane 4.6.x QuadPlane over USB/radio,
[operator usage plan](../operator-usage-plan.md)) is unrelated to this specific
defect and remains separately unexecuted.
