# Implemented constraints

This is a compact map of behavior that is easy to misunderstand but already
exists. Executable artifacts are authoritative if this file drifts.

## Build and generation

- Bazel 8.7.0 is pinned in `.bazelversion`; Go 1.25.0 is pinned in `go.mod` and
  `MODULE.bazel`. `frontend/package.json` pins pnpm.
- `go.mod` and `frontend/pnpm-lock.yaml` are dependency sources of truth.
  `MODULE.bazel` reads them.
- Protobuf output is committed. `make proto-gen` regenerates it; an advisory CI
  job reports drift in the generated tree.
- `make test` is the native development loop. `bazel test //...` is the
  cross-language checkpoint.

## MAVLink receive path

- The backend listens on one UDP socket (`GCS_MAVLINK_UDP_BIND`, default
  `0.0.0.0:14550`) and distinguishes vehicles by frame identity.
- An explicitly empty bind disables the socket. `internal/codec/posture_test.go`
  checks the unset-versus-empty behavior.
- HEARTBEAT produces `HeartbeatState`; it is not a `TelemetryEvent` payload.
- Telemetry and transaction responses use separate envelopes. The dispatch
  tables and `TestMatrixCoverage` define the handled message families.
- Unit conversion occurs in `internal/codec/message.go`. Fields with MAVLink
  unknown sentinels retain wire units and wire-unit names.
- Writes require a known `LinkID`; `Node.WriteTo` rejects unknown links and never
  falls back to broadcast.
- The GCS heartbeat is gomavlib's built-in heartbeat, configured for one second.
  `LinkIdleTimeout` is derived from `HeartbeatTTL`.
- `StreamRequestEnable` stays off. Rates are requested above the codec with
  `MAV_CMD_SET_MESSAGE_INTERVAL`, not with the deprecated REQUEST_DATA_STREAM.

## Telemetry acquisition

- `bridge.RateRequester` requests `bridge.DefaultRates` on
  `VEHICLE_DISCOVERED` and `VEHICLE_RECOVERED` only. `HEARTBEAT_UPDATED` fires
  on every arm and mode change and must not re-request the policy.
- Requests go to component 1 alone. Other components on the same system ID
  answer their own message sets and would multiply link load.
- Rate is stated in Hz in the policy and converted to the wire's microsecond
  interval once, at encode time.
- A failed write is a sink failure and stops the bridge. A GCS that silently
  failed to ask for telemetry would show a healthy link carrying nothing.
- There is no ACK correlation or retry. `COMMAND_ACK` is decoded and logged;
  success is judged by the telemetry that arrives.

## Vehicle state

- `vehicle.Fold` and `vehicle.Expire` are pure with an injected millisecond
  clock. `TestNoWallClock` prevents wall-clock reads in the package.
- One bridge loop owns the state map and preserves frame order. `routes.Table`
  supports concurrent readers and writers, as its tests verify.
- Sink failures stop the bridge. Continuing would report a healthy receive path
  while discarding observations. `bridge.MultiSink` fans out in declaration
  order and stops at the first failure.
- The fold stamps `observed_at` on a *copy* of each telemetry payload, resolved
  through the descriptor rather than a type switch, so a new family cannot be
  added unstamped. `TelemetryEvent` and `ProtocolEvent` both reach the sinks.

## Browser event stream

- `GET /api/events` is the only public surface added. Two event names, `fleet`
  and `telemetry`, each carrying the existing protobuf message as protobuf JSON.
- The oneof payload establishes message presence. Ordinary proto3 scalar
  defaults therefore remain safe with standard `protojson`: an ATTITUDE
  payload containing zero roll is present and decodes as a level reading.
- Event data is compacted to one line. SSE terminates a data field at a
  newline, so a multi-line payload would arrive truncated.
- The hub retains the latest fleet event per `(sysid, compid)` and the latest
  telemetry per vehicle and family. Subscribe and bootstrap happen under one
  lock; bootstrap is ordered fleet-first, then telemetry, both by identity.
- Each subscriber has a bounded queue and is disconnected when it overruns.
  `Hub.Publish` never returns an error: no browser may stop the receive path.
- The HTTP server sets `BaseContext` from the process context. `Shutdown` waits
  for active requests but does not cancel them, and an event stream never ends
  on its own.
- Memory is ephemeral. Restart, reconnect, and duplicate bootstrap events are
  handled by keyed, idempotent state; there is no replay or `Last-Event-ID`.

## Flight display

- Freshness uses the backend's `observed_at`, not browser receipt time. The hub
  replays retained state on reconnect, and stamping arrival would show
  ten-minute-old telemetry as live. This assumes the backend and browser agree
  roughly on the wall clock, which holds for localhost and Compose.
- `TELEMETRY_TTL_MS` is 5 s, on `logic/freshness`'s strict boundary. Vehicle
  loss remains the backend's 60 s TTL — a separate, slower judgement.
- `ui/readings.ts` is the only place that decides whether a value may be shown
  as a number. Absent and stale readings render as `- - -`; stale provenance
  remains amber with its source and age.
- Overlapping position and power fields remain family-qualified in
  `TelemetrySample`; asynchronous GPS/global-position or battery/system-status
  arrivals cannot produce a mixed tuple carrying one source label.
- View state is keyed by the full `(sysid, compid)`. Selection distinguishes an
  operator's explicit pick, which is sticky, from an automatic one, which is
  revisited as the fleet changes.
- Live SSE is the default; `?source=mock` is the only way to reach fixtures.

## Development topology

- Compose health-gates SITL startup on the backend. ArduPilot's `udpclient`
  parser requires a numeric IPv4 destination, so the SITL entrypoint resolves
  `gcs-backend` before launching the autopilot.
- The project network is `172.30.250.0/24` and the backend is fixed at
  `172.30.250.10`. Resolution therefore remains valid when Compose recreates
  only the backend; the running SITLs reconnect, are rediscovered, and receive
  fresh message-rate requests without their containers restarting.
- Each SITL process receives its vehicle identity through ArduPilot's runtime
  `--sysid` flag, the authoritative per-process selector. Vehicle identity is
  not injected through the defaults file.

## Current limits

- The system is read-only. Rate requests are data acquisition, not an operator
  command surface; nothing arms, commands, or configures a vehicle.
- No persistence adapter or persistence configuration exists. Backend restart
  loses all retained state.
- Transaction responses are decoded and logged, but there is no request
  registry or RPC surface consuming them.
- Inbound MAVLink frames are unauthenticated; no signing configuration exists.
- `/api/events` has no authentication and no origin restriction.
- The UI is instrumentation only: no map, mission editor, or telemetry
  inspector.
- The fixed Compose subnet is development-only and must not overlap a host or
  VPN route.
