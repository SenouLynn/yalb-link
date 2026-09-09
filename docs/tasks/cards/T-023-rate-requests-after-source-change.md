---
id: T-023
title: Re-request telemetry rates when a vehicle returns on a new link
status: ready
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
see the [T-015 evidence](../../runbooks/evidence/t015/README.md).

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

- [ ] A vehicle whose route changes to a new link has `DefaultRates` re-issued,
      addressed to the new link.
- [ ] A test fails if a source change does not re-request, following
      `TestRateRequestsAddressTheObservedLink` and the recovery test beside it.
- [ ] Re-requesting is bounded: a flapping source cannot issue rate requests
      faster than the existing recovery path would.
- [ ] Demonstrated on Compose by restarting the SITL container alone and
      observing the families resume, with no backend restart.

## Verification

```sh
go test -race ./internal/bridge/... ./internal/vehicle/... ./internal/routes/...
docker compose up -d --build gcs-backend ardupilot-sitl-copter-1
docker compose restart ardupilot-sitl-copter-1
curl -sN --max-time 10 http://localhost:8080/api/events   # families must resume
```

## Open questions

- Whether the trigger belongs on the route table's source change or on a new
  fleet event. A fleet event is visible to the frontend and to recordings; a
  route-table hook is narrower. This decides whether the browser can also show
  that the link moved.
- Whether `SOURCE_CONFLICT` should stay a warning once it is acted on, or become
  an ordinary state transition. It currently reads as an anomaly, and after this
  card an ordinary SITL restart produces it.

## Notes

Found while accepting T-015, and deliberately not fixed there: it predates that
card, affects all eight previously requested families equally, and is a backend
acquisition concern rather than a display one.
