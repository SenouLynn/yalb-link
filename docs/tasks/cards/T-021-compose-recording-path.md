---
id: T-021
title: Make the documented Compose recording launch writable
status: done
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

T-017 executed `GCS_RECORDING_ENABLED=true docker compose up -d gcs-backend`.
The non-root backend exited with SQLite error 14 (`unable to open database file`)
for its default `./recordings.db`; dependent simulators were health-gated off.

## Outcome

The documented Compose recording command starts with a writable durable store.

## Scope

- In: Compose storage configuration, lifecycle documentation and restart check.
- Out: changing retention policy or recording event scope.

## Acceptance criteria

- [x] Enabling recording in Compose starts a healthy backend without an override.
- [x] A recorded session survives container recreation using named storage.
- [x] Document reset and deletion explicitly; preserve recording-disabled startup.

## Verification

```sh
docker compose config --quiet
./scripts/check-containers.sh
GCS_RECORDING_ENABLED=true docker compose up -d --build --wait gcs-backend
# Follow docs/runbooks/validation/t021.md for the SITL recording,
# recreation, replay comparison and recording-disabled acceptance.
./scripts/kanban check
```

## Open questions

Settled: `/var/lib/gcs`, initialized in the image with UID/GID 10001 ownership,
mounted as the Compose `recordings` named volume.

## Notes

T-017 used an explicit `/tmp/t017-recordings.db` override for acceptance and
exported the database. That workaround survives process restart, not recreation.


Completed 2026-09-09: default Compose launch creates a healthy non-root backend
with recording enabled, without a path override. A new named volume received
UID/GID 10001 ownership; SQLite database, WAL and SHM files were writable.
Recorded 651 events from stationary Copter 4.7.0 SITL, stopped the session,
recreated only the backend, and compared all replay events and metadata for exact
JSON equality. Recording-disabled recreation was healthy and recording routes
returned 404. Compose config, container checks and diff checks passed.

[Acceptance evidence](../../runbooks/validation/t021.md) includes full replay
before/after and the comparison result. The stack was shut down without deleting
`yalb-gcs_recordings`; recording 1 remains available. Runbook documents ordinary
shutdown, recreation, disabling recording, single-session deletion and full reset.
