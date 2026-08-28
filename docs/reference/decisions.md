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
- Arm/disarm has strict addressed ACK correlation; telemetry rate requests have
  no correlation, and no command is retried. Success of acquisition is judged
  by the telemetry that arrives.

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

## Operator commands

- Commands are off unless `GCS_COMMANDS_ENABLED` is true. The sole endpoint is
  `POST /api/commands/arm`; there is no generic dispatcher.
- An ACK matches the commanded vehicle by frame sender and this GCS by payload
  target `(255,190)`. Zero-target and foreign ACKs cannot settle a transaction.
- Timeout, cancellation, or an uncertain link-write failure permanently poison
  that vehicle/command key until backend restart. MAVLink provides no invocation
  identity with which to prove a later ACK is not stale.
- The POST returns synchronously. Command SSE is live-only (not bootstrap state),
  while recordings preserve pending and terminal snapshots.
- Origin/content-type/fetch-metadata checks protect local development from
  casual CSRF; they are not authentication.

## Browser event stream

- `GET /api/events` carries `fleet`, `telemetry`, and live-only `command`
  protobuf-JSON events.
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

## Recording replay

- `GET /api/recordings/{id}/events` pages by sequence number, not by offset:
  sequence numbers are dense, immutable, and already the ordering key, while an
  offset would shift under a writer still committing batches. `next_seq` is the
  cursor and `null` ends the walk.
- Events are rendered with `protojson`, spliced into the envelope as raw JSON.
  `encoding/json` would mangle enums and well-known types. The browser parses a
  replayed event with `parseStreamJson`, the same reader the live SSE path uses
  through `parseStreamEvent`.
- The row lookup, not the event query, decides 404. An unknown recording and one
  with no events yet are different answers.
- An `active` recording serves its committed prefix. Reading a recording must
  not require stopping it.
- `occurred_at_ms` is the fold's `observed_at` for telemetry and `occurred_at`
  for fleet events, so playback and freshness run on one timeline.
- Pacing is the browser's. The backend serves pages; `logic/playback` owns the
  clock, so scrubbing and speed are local state rather than a reconnect, and
  the ordering rules are testable without a DOM or a network.
- `ReplayEventSource` implements `TelemetryStream.now()`; live and mock leave it
  unset and keep the wall clock. Recorded telemetry carries the timestamps of
  the flight that produced it, so wall-clock freshness would blank the display
  entirely.
- Events are emitted in recorded order. `dueCount` stops at the first event that
  is not yet due even when a later one is, because recorded order is delivery
  order rather than timestamp order.
- Seeking backwards emits a `reset` and replays from the buffer's start. The map
  track only appends, so a rewind rebuilds it rather than leaving a tail from a
  future the operator has seeked away from. An explicit vehicle selection
  survives the reset; an automatic one is picked again.
- The browser holds at most `MAX_BUFFERED_EVENTS` (50,000) of a recording's
  200,000-event cap and reports the truncation rather than showing a partial
  flight as a whole one.

## Recording retention

- Aggregate defaults retain at most 4 GiB of live SQLite pages, 30 days from a
  recording's `started_at`, and 200 recording rows. The oldest stopped
  recording is removed first while any bound is exceeded; startup performs the
  same sweep as the five-minute writer timer.
- Aggregate size means `(page_count - freelist_count) * page_size`, not the
  database file length. SQLite keeps freed pages for reuse, so file length does
  not fall after ordinary deletion and cannot drive a convergent sweep.
- The live-page byte bound differs deliberately from ADR 0002's payload-byte
  bound for one recording. The former includes SQLite structures; the latter
  measures encoded protobuf payloads.
- Active recordings are never swept or explicitly deleted. Deletion is ordered
  through the writer and removes event rows in chunks before removing metadata.
- No replay lease exists. Deleting a recording during paged replay makes the
  browser's next page fail visibly. No `VACUUM` runs; the database reuses free
  pages but retains its filesystem high-water mark.

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
- `frontend/src/map/MapPanel.tsx` is the renderer boundary. UI components pass
  plain position, track, and tile-source data; MapLibre types do not leak into
  fleet or display state.
- Live SSE is the default. `?source=mock` reaches fixtures and
  `?source=replay&recording=<id>` reaches a recording; both are opt-in, and the
  Source chip names them in amber. The rule runs both ways — a page asked for a
  replay it cannot load stays in replay mode and says so rather than quietly
  showing live telemetry instead.

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

- The system is read-only unless the command gate is enabled. With it enabled,
  only guarded arm/disarm is exposed; no generic command or configuration
  surface exists.
- Recording is opt-in and off by default; with it disabled the backend restart
  still loses all retained state. Retention bounds live database pages, age,
  and count, but does not compact the SQLite file or return its high-water-mark
  allocation to the filesystem.
- Transaction responses are decoded and logged, but there is no request
  registry or RPC surface consuming them.
- Inbound MAVLink frames are unauthenticated; no signing configuration exists.
- `/api/events` has no authentication and no origin restriction.
- The UI has instruments, a single-selected-vehicle map, and recording replay,
  but no mission editor or telemetry inspector. Map imagery is fetched directly by the
  browser from a public tile host, an external network dependency beyond the
  localhost/SITL/backend path. The local tile-source seam is not wired into
  the UI yet.
- Vitest runs without a DOM or WebGL context, so it covers map prop wiring and
  the pure track fold but not MapLibre's canvas lifecycle. Live browser
  acceptance remains a manual/CI-capable gate.
- The fixed Compose subnet is development-only and must not overlap a host or
  VPN route.
