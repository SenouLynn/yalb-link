---
id: T-006
title: Expose read-only mission download over HTTP
status: done
priority: 0
owner: unassigned
depends_on: T-004, T-005
---

## Motivation and evidence

The browser has no way to invoke the planned download coordinator. Existing
HTTP command handling establishes local origin checks, bounded synchronous
transactions, structured errors, and full `(system_id, component_id)` routing,
but mission reads must remain available without enabling operator commands.

## Outcome

The browser can request the selected vehicle's ordinary mission through a
read-only HTTP endpoint and receive either a complete snapshot or an explicit
terminal error.

## Scope

- In: `GET /api/vehicles/{system_id}/{component_id}/mission`, protobuf-JSON
  success response, stable error codes/statuses, request cancellation,
  route-table lookup, configuration wiring, and HTTP tests.
- Out: mutation endpoints, generic MAVLink dispatch, cached or persisted
  missions, server push, fence/rally selection, authentication, and UI.

## Acceptance criteria

- [x] `GET /api/vehicles/{system_id}/{component_id}/mission` returns the
      coordinator's complete ordinary-mission snapshot as protobuf JSON.
- [x] Unknown vehicle, download already in progress, timeout, cancellation,
      rejected mission, and link-write failure have tested, distinct responses.
- [x] The endpoint works while `GCS_COMMANDS_ENABLED` is false and exposes no
      upload, clear, start, or set-current operation.
- [x] A disconnected HTTP client cancels its live download without leaking a
      registry entry or goroutine.

## Verification

```sh
go test -race ./internal/mission/... ./internal/routes/... ./cmd/gcs/...
make test-go
```

## Open questions

None. The first transport is an on-demand synchronous read; caching and SSE
would add consistency semantics that this milestone does not need.

## Notes

Use the same local-development request posture as existing APIs, but do not put
a read-only vehicle query behind the command enable gate.

Implementation notes:

- The route is always mounted, including when the MAVLink socket is disabled;
  without a live route it returns the same structured `mission_no_route` error
  as any other unknown vehicle. `GCS_COMMANDS_ENABLED` gates only command
  registry construction and command routes.
- Terminal transport mappings are stable JSON codes: no route (`404`), an
  existing download (`409`), timeout (`504`), request cancellation (`499`), a
  vehicle rejection (`422`, including its MAVLink result), and link-write
  failure (`502`). Successful snapshots use protobuf JSON.
- The request context is passed directly into `Coordinator.Download`. The HTTP
  cancellation test waits for the coordinator slot to be claimed, cancels the
  request, asserts the handler returns promptly, and verifies the slot is
  released; package-level `goleak` coverage checks the goroutine side.
- Verified with `go test -race ./internal/mission/... ./internal/routes/...
  ./cmd/gcs/...`, `make test-go`, `go vet ./internal/mission/... ./cmd/gcs/...`,
  `git diff --check`, and `./scripts/kanban check`. `golangci-lint` is not
  installed. Bazel verification could not run because the installed Bazel is
  8.3.1 while `.bazelversion` requires 8.7.0; the BUILD targets were updated
  manually.
