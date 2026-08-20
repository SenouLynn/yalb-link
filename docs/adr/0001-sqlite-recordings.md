# 0001: Use SQLite for initial telemetry recordings

Status: Accepted

## Context

Map history, instrument replay, and durable validation fixtures need bounded,
queryable recordings. The current event hub retains only the latest in-memory
state, while loose capture files require build-time cleanup and can grow
without a clear lifecycle. The backend is currently a single process, and
remote multi-user access is not yet implemented.

## Decision

Use SQLite as the initial durable store for explicitly started and stopped
recordings. Recording lifecycle, ordering, retention, replay, and fixture
export are application concerns; persistence remains separate from the live
event hub. Redis is not a system of record.

Keep the design portable enough to replace the store with Supabase/PostgreSQL
when remote durability, multiple writers, or authenticated shared access makes
that operational complexity worthwhile. Authentication and RBAC are not part
of the initial persistence slice.

## Consequences

- Local development needs no managed service or separate database process.
- Writes must be serialized and bounded so persistence cannot stall MAVLink
  ingestion indefinitely.
- Retention, database compaction, and recording gaps must be observable runtime
  behavior rather than build cleanup.
- A future PostgreSQL migration will require a new store implementation and
  data migration; SQLite-specific behavior must not leak into recording
  semantics.

## Verification

The implementing slice must test recording lifecycle, deterministic replay,
bounded retention, restart durability, and persistence-failure behavior.
