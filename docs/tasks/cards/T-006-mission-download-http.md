---
id: T-006
title: Expose read-only mission download over HTTP
status: backlog
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

- [ ] `GET /api/vehicles/{system_id}/{component_id}/mission` returns the
      coordinator's complete ordinary-mission snapshot as protobuf JSON.
- [ ] Unknown vehicle, download already in progress, timeout, cancellation,
      rejected mission, and link-write failure have tested, distinct responses.
- [ ] The endpoint works while `GCS_COMMANDS_ENABLED` is false and exposes no
      upload, clear, start, or set-current operation.
- [ ] A disconnected HTTP client cancels its live download without leaking a
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
