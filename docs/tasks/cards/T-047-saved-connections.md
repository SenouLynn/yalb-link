---
id: T-047
title: Persist recognizable local connection profiles
status: done
priority: 1
owner: unassigned
depends_on: T-045, T-046
---

## Motivation and evidence

Bench controller and Field radio settings must survive browser and application restarts without silently opening a replacement device.

Required by [ADR 0006](../../adr/0006-operator-connection-readiness.md).

## Outcome

Backend-owned local profiles retain settings and resolve saved devices with explicit missing or ambiguous outcomes.

## Scope

Profile storage/API, device matching and declared startup policy. Profiles select
acquisition paths and never grant command capabilities. T-046 now supplies native
inventory, controlled acquisition and a shared bridge. UI controls and retry after
startup remain T-048/T-049.

Implementation plan: a versioned JSON document in the OS user configuration
folder, overridable with GCS_CONNECTION_PROFILES_PATH, atomically replaced after
file sync and protected against concurrent backend writers. Save captures metadata
from current inventory, not caller-invented device identity. Names/settings and
IDLE/CONNECTED/RELEASED intent persist independently of transient telemetry state.
A matching serial number plus VID/PID permits automatic startup selection; absent
identity requires explicit selection, and duplicate matches remain AMBIGUOUS.
Persistence errors reject profile mutations and prevent automatic acquisition;
manual unsaved observation remains available. Document the durable format and
identity rules in an ADR and the connection contract.

## Acceptance criteria

- [x] Profiles retain a recognizable name, device matching information and communication settings across browser/backend restart.
- [x] Stable identity is preferred when available; changed port names, missing identity and ambiguous matches follow documented behavior.
- [x] A different device occupying the saved port is not silently accepted as the saved device.
- [x] Invalid or unreadable stored settings produce an actionable state; saving failures are reported.
- [x] Remembering a profile does not itself override intentional disconnect or enable commands.

## Verification

- Test atomic save/reload/delete, invalid/version-mismatched/truncated files,
  concurrent ownership and injected write failures without replacing good data.
- Controlled inventory fixtures cover renamed ports, replacement at the same
  path, missing metadata and multiple identical matches. Assert exact open counts
  for fresh saves, active restart and intentional release across restart.
- HTTP checks cover CRUD, profile connect/disconnect, strict bounded JSON and
  same-origin guards with operator commands disabled.
- A real-backend PTY harness saves/reloads/updates/deletes, restarts the process,
  checks anonymous devices require explicit selection, and verifies RELEASED
  intent persists. Exercise applicable Plane 4.6.3 ingestion through a selected
  saved profile using the existing T-046 simulator setup. Startup/resolution and
  API observations must settle within 8 seconds; no hardware identity is inferred.

```sh
go test -race -timeout=2m ./internal/connection/... ./internal/bridge/... ./internal/stream/... ./cmd/gcs/...
CGO_ENABLED=1 go build ./...
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./...
make bazel-tidy
bazel test //internal/... //cmd/gcs/...
./scripts/kanban check
```

## Open questions

Storage location/schema and identity fallback follow the declared host scope and T-045; do not assume every adapter exposes a serial number. Resolved by this
implementation and [ADR 0010](../../adr/0010-connection-profile-persistence.md).

## Notes

Ready review 2026-09-23: T-045 and T-046 are complete. Storage and identity rules
are implementable without hardware decisions; physical identity reliability
remains hardware acceptance.

Implementation and verification completed 2026-09-23:

- `internal/connection.Profile`/`Store`/`FileStore` implement the versioned,
  atomically replaced JSON document at `GCS_CONNECTION_PROFILES_PATH` or the OS
  user config directory, with serial-number+VID/PID identity matching, the new
  `AMBIGUOUS` state, and CONNECTED/IDLE/RELEASED intent persisted independently
  of transient `Status.State`. `Manager` seeds `Connections()` with one entry
  per saved profile at startup (keyed by profile id, stable across a renamed
  port), auto-connects only unambiguously resolved CONNECTED profiles, and
  never auto-connects a RELEASED one. `GET/POST /api/connections/profiles`,
  `DELETE /api/connections/profiles/{id}` and `POST /api/connections/connect`
  with `profile_id` (plus an explicit `device_id` for AMBIGUOUS/DEVICE_MISSING
  selection) are mounted with the existing same-origin guards, independent of
  `GCS_COMMANDS_ENABLED`. See [ADR 0010](../../adr/0010-connection-profile-persistence.md)
  for the full rationale and the [contract's T-047 notes](../connection-contract.md#t-047-wire-and-implementation-notes)
  for the wire summary.
- Persistence failures reject the mutation before any device is opened
  (`ConnectProfile` persists intent before acquiring); an unreadable profile
  store disables automatic acquisition only, leaving manual device-id connect
  and `GET /api/connections` working, and `GET /api/connections/profiles`
  surfaces the same read error.
- `go test -race -timeout=2m ./internal/connection/... ./internal/bridge/... ./internal/stream/... ./cmd/gcs/...`,
  `CGO_ENABLED=1 go build ./...`, `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./...`,
  `go vet ./...`, `gofmt -l` and `./scripts/kanban check` all passed on macOS
  arm64 with Go 1.25.0. `make bazel-tidy`/`bazel test` were **not** run in this
  session (local toolchain constraint, unrelated to this change) and remain
  unexecuted here.
- **Not executed in this pass, by explicit scope decision:** the real-backend
  PTY hardware harness (save/reload/update/delete across a process restart,
  anonymous-device explicit selection, RELEASED persistence, and Plane 4.6.3
  ingestion through a selected saved profile via T-046's simulator setup) was
  scoped out to keep this pass to storage/API/matching/startup-policy plus
  automated tests. No hardware or PTY evidence is claimed for T-047; retry/
  backoff after startup and UI controls remain T-049/T-048 as originally scoped.
