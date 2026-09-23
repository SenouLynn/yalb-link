---
id: T-046
title: Discover and open local serial devices through the backend
status: done
priority: 1
owner: unassigned
depends_on: T-045
---

## Motivation and evidence

The executable currently opens UDP only; an attached USB controller or radio cannot be selected through the application.

Required by [ADR 0006](../../adr/0006-operator-connection-readiness.md).

## Outcome

The local backend exposes device inventory and controlled serial acquisition using the agreed connection contract.

## Scope

OS device enumeration and metadata, refresh/removal, port settings, open/close,
MAVLink ingress and addressed acquisition traffic. Implement the inventory,
status, connect/disconnect HTTP endpoints and bootstrapped `acquisition` SSE
contract so the result is exercisable before frontend controls exist. Preserve
the UDP path. No profile persistence, frontend controls or automatic recovery
policy (T-047/T-048/T-049 own these).

Initial controlled verification targets macOS on the development host; Linux
support requires its own executed checks. Start by evaluating the already pinned
`go.bug.st/serial` dependency and gomavlib transport adapters. Library choice
and dynamic source composition are implementation experiments, not external
approval dependencies. Keep one authoritative fleet fold and address writes
only to the connection that supplied the vehicle route. A source multiplexer
feeding the existing bridge is an option alongside the contract's alternatives.

Start the bridge even when UDP is disabled: currently `cmd/gcs/main.go` creates
it only inside the nonempty UDP-bind branch. Serial-only startup must retain
mission downloads, recording and the normal telemetry stream.

Follow [ADR 0007's platform/CGO boundary](../../adr/0007-native-serial-acquisition.md):
shared application source, separate OS/architecture builds, CGO permitted for
macOS detailed discovery and a CGO-free Linux production build. Use native
serial support initially; raw USB/libusb is not required by this workflow.

## Acceptance criteria

- [x] Native macOS builds with CGO and Linux production builds without CGO; controlled serial checks run on both platforms with OS/architecture and limitations recorded.

- [x] Inventory reports available device identity metadata without claiming a generic serial device is a radio.
- [x] Connect opens the selected device/settings and routes MAVLink through the normal bridge; disconnect releases it.
- [x] Missing, inaccessible, busy and silent devices produce distinct supported states/errors without inventing a cause.
- [x] Acquisition allows only the intended observer traffic when operator commands are disabled.
- [x] A controlled serial test source exercises framing, port closure, error handling and multiple vehicle identities; hardware limitations are recorded.
- [x] Inventory/status/connect/disconnect are exercisable over HTTP with operator commands disabled; a late SSE subscriber receives the current acquisition state.
- [x] A serial-only backend with UDP disabled discovers vehicles and streams telemetry through the existing bridge, routes and recording pipeline.
- [x] Closing/reopening a serial connection neither terminates the shared fleet pipeline nor leaves stale addressed-write routes; repeated identical connect is idempotent.

## Verification

Verify native macOS discovery/build with `CGO_ENABLED=1 go build ./...` and
Linux production compilation with `CGO_ENABLED=0 GOOS=linux go build ./...`.
Record the actual architecture/toolchain for each; test each architecture
claimed by packaging. Run the controlled serial harness below on both macOS
and Linux, recording inventory metadata availability, access errors and port
release behavior. A cross-build alone does not satisfy Linux runtime evidence.
Keep race tests CGO-enabled, independently of the production binary policy.
Executed results and limitations are recorded below and in the T-046 runbook.

Implement controlled transport tests in `internal/connection` (new package)
and the affected codec/bridge/HTTP packages, then run:

```sh
go test -race -timeout=2m ./internal/connection/... ./internal/codec/... ./internal/bridge/... ./internal/routes/... ./internal/stream/... ./internal/mission/... ./cmd/gcs/...
go build ./...
make bazel-tidy
bazel test //internal/... //cmd/gcs/...
./scripts/kanban check
```

Add a repeatable pseudo-terminal harness and a T-046 validation runbook. The
harness must launch the real backend with UDP and operator commands disabled,
select its test port through the HTTP API, and feed fragmented MAVLink frames
for two system IDs. Assert both identities in SSE and independently decode
outbound traffic. Allow GCS heartbeat and addressed telemetry-rate requests;
allow mission-read protocol traffic only when a download is requested. Reject
arming, parameter writes and mission writes. Exercise malformed bytes, silence,
close/reopen, duplicate connect, late SSE subscription and shutdown while a
consumer is stalled. Use injected OS errors for permission/busy cases and label
those as injected, not hardware observations. Assert that explicit disconnect
releases the port for a second opener and that no outbound traffic follows.

Repeat applicable Plane 4.6.3 QuadPlane ingestion from the T-027 runbook through
the controlled serial path before hardware acceptance. Record exact commands,
results and platform limits; executed evidence is recorded below.

## Open questions

Serial library/adapter behavior and OS enumeration/permission handling are
resolved by this experiment; actual device acceptance remains T-028/T-031.
Resolved: REPORTING requires valid frames, independent of heartbeat liveness;
INTERRUPTED means no valid frames for more than 60 seconds. Repeated identical
connect returns 200 with the same open session. Both are tested and reconciled
in the contract. I/O failure without removal evidence is TRANSPORT_FAILED.

## Notes

Readiness review 2026-09-16: T-045 is done; no hardware identity, radio baud or
packaging decision prevents this controlled implementation. Promoted after
pinning scope and verification. This is software readiness, not USB/radio
acceptance. ADR 0007 stays proposed until its implementing experiment passes.


Implementation and verification completed 2026-09-23:

- Native `go.bug.st/serial` inventory/open, 8N1 settings, observed OS errors,
  inventory removal, explicit release/reopen and a one-second evidence sweep.
  A one-shot gomavlib custom client exposes synchronous open failures without
  taking ownership of future retry policy. One manager multiplexes all sources
  into the existing bridge/fleet fold; generation-specific serial labels and
  ordered channel retirement prevent stale writes after close/reopen.
- Inventory/status/connect/disconnect HTTP routes operate with commands disabled.
  Acquisition status is retained and bootstrapped over SSE. The bridge, mission
  coordinator and recording sink run even with UDP disabled. Fixed UDP remains
  supported and can be released/reopened by its configured ID.
- Disconnect racing a discovery-triggered rate request now cancels that request
  without stopping other connections. Tests cover source handoff, pending close
  events, stalled consumers, shutdown and goroutine leaks.
- Required race suite passed; native macOS CGO and Linux CGO-free builds passed;
  `make bazel-tidy` completed and all 10 Bazel backend test targets passed using
  the repository-pinned Bazel 8.7.0. `./scripts/kanban check` passed.
- Real-backend PTY harness passed on macOS 26.6.2 arm64 and Linux
  6.12.76-linuxkit aarch64 with Go 1.25.0, including full 60-second silence,
  two identities, malformed/fragmented input, late SSE subscription, mission
  read, recording, duplicate connect, release to a second opener and reopen.
  Both independently decoded outbound streams contained only allowed traffic.
- Plane 4.6.3 QuadPlane ingestion passed through the native serial path, with
  independent firmware/Q_ENABLE/Q_FRAME_CLASS readback and real stream,
  mission-download and recording checks. ADR 0007 is now accepted.
- Reproduction commands, observed counts/timing, harness corrections and exact
  platform limitations: [T-046 runbook](../../runbooks/validation/t046.md).
  Physical USB metadata, drivers, permission/ownership, baud throughput and
  radio behavior are not claimed; permission/busy tests use injected OS errors.
  UI controls, saved profiles and automatic reconnect remain T-048/T-047/T-049.
