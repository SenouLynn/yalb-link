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

## Vehicle state

- `vehicle.Fold` and `vehicle.Expire` are pure with an injected millisecond
  clock. `TestNoWallClock` prevents wall-clock reads in the package.
- One bridge loop owns the state map and preserves frame order. `routes.Table`
  supports concurrent readers and writers, as its tests verify.
- Sink failures stop the bridge. Continuing would report a healthy receive path
  while discarding observations.

## Current limits

- The HTTP server exposes liveness only; it has no telemetry API.
- No persistence adapter or persistence configuration exists.
- Transaction responses are decoded, but there is no request registry or RPC
  surface consuming them.
- Inbound MAVLink frames are unauthenticated; no signing configuration exists.
