---
id: T-021
title: Make the documented Compose recording launch writable
status: ready
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

- [ ] Enabling recording in Compose starts a healthy backend without an override.
- [ ] A recorded session survives container recreation using named storage.
- [ ] Document reset and deletion explicitly; preserve recording-disabled startup.

## Verification

Compose config, recording start/stop/list, recreate backend and replay the session.

## Open questions

Choose a named-volume location writable by UID 10001.

## Notes

T-017 used an explicit `/tmp/t017-recordings.db` override for acceptance and
exported the database. That workaround survives process restart, not recreation.
