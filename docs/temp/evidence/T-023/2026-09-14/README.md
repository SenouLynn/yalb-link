# T-023 evidence — 2026-09-14

Purpose: demonstrate the fix's acceptance criterion "Demonstrated on Compose by
restarting the SITL container alone and observing the families resume, with no
backend restart."

Task: [T-023](../../../tasks/cards/T-023-rate-requests-after-source-change.md)
Revision: working tree at the commit that added `RateRequester.trigger`'s
SOURCE_CONFLICT branch (internal/bridge/rates.go).

Environment: local Docker Compose (`docker-compose.yml`), `gcs-backend` and
`ardupilot-sitl-copter-1` only, `GCS_LOG_LEVEL=info`.

## Commands

```sh
GCS_LOG_LEVEL=info docker compose up -d --build gcs-backend ardupilot-sitl-copter-1
curl -sN --max-time 5 http://localhost:8080/api/events   # pre-restart: families streaming
docker compose restart ardupilot-sitl-copter-1            # backend itself never restarted
curl -sN --max-time 10 http://localhost:8080/api/events   # post-restart: families resumed
docker compose logs --no-log-prefix gcs-backend
```

## Observed

`backend.log` is the backend's stdout across the restart. It shows:

- Initial discovery on `udp:172.30.250.2:45724`, 9 families requested, 9
  COMMAND_ACKs `MAV_RESULT_ACCEPTED`.
- After `docker compose restart ardupilot-sitl-copter-1`, the autopilot rebinds
  to `udp:172.30.250.2:37935`. The backend logs `SOURCE_CONFLICT`
  (`previous="...45724" current="...37935"`) and, on the same line sequence,
  `telemetry rates requested ... trigger=SOURCE_CONFLICT`, followed by 9 more
  accepted ACKs — no `VEHICLE_LOST`/`VEHICLE_RECOVERED` pair, matching the
  card's observation that heartbeats never stopped.
- A 10 s post-restart SSE sample (`curl /api/events`) carried 271 `telemetry`
  events plus the `HEARTBEAT_UPDATED` fleet event, i.e. every previously
  requested family resumed live. The backend process was not restarted at any
  point (`docker compose restart` targeted only `ardupilot-sitl-copter-1`).

## Limitations

- Copter SITL only; the card's underlying real-hardware motivation (Plane
  4.6.x QuadPlane over USB/radio) is unrelated to this fix and remains
  unexecuted, per the [operator usage plan](../../../tasks/operator-usage-plan.md).
- Automated coverage of the same behavior lives in
  `internal/bridge/rates_test.go`
  (`TestRateRequestsOnSourceConflict`, `TestRateRequestsBoundAgainstFlappingSource`,
  `TestNoRateRequestsForSourceConflictOnNonAutopilotComponent`); this capture is
  the Compose-level demonstration the card also asks for.
