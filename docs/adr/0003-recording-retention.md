# 0003: Retain recordings within aggregate bounds

Status: Accepted

## Context

ADR 0002 bounds one recording, but a long-running backend can still create an
unbounded number of recordings. SQLite retains freed pages in its file for
reuse, so the database file's length is not a useful feedback signal for
deletion: deleting rows would not reduce it and a size-driven sweep could
delete every recording without satisfying its condition.

Operators also need to remove a stopped recording explicitly. Replay is paged
and stateless, so the backend cannot know whether another browser is currently
reading a recording.

## Decision

Retain recordings within three configurable defaults: 4 GiB of live SQLite
pages, 30 days measured from `started_at`, and 200 recording rows. Sweep at
startup and every five minutes, deleting eligible recordings oldest-first
until every configured bound holds. These values are defaults in
`recording.Config`, not permanent operational policy. A negative configuration
value disables its bound; zero retains the default, consistent with the other
numeric `Config` fields.

Measure aggregate size as
`(page_count - freelist_count) * page_size`. This is live-page bytes for the
whole database. It deliberately differs from ADR 0002's per-recording
`MaxBytes`, which counts protobuf payload bytes only.

Delete events in bounded transactions and their parent row last. Deletion is
serialized through the writer so a queued batch cannot recreate children after
their parent is removed. Active recordings are never swept and an explicit
attempt to delete one returns a conflict. `DELETE /api/recordings/{id}` removes
a stopped recording for an operator.

Do not run `VACUUM`. Freed pages are reused by later writes, so the file
plateaus at its high-water mark rather than shrinking. Returning allocated
space to the filesystem would require incremental auto-vacuum and conversion
of existing databases; that remains deferred.

## Consequences

- Recording storage converges to its configured age, count, and live-page
  bounds whenever stopped candidates are available.
- A bound can remain exceeded while only an active recording and SQLite's base
  schema remain; active flight data is never deleted to force convergence.
- Deletion is permanent, but interrupted chunk deletion is resumable because
  the parent remains until all child rows are gone.
- A replay already in progress is not protected. Its next page returns 404
  after deletion, and `ReplayEventSource` renders that as "Replay unavailable"
  rather than switching sources or blanking silently.
- The on-disk file need not shrink after retention. Its free pages reduce later
  file growth.

## Verification

Recording tests cover explicit deletion, active and unknown conflicts,
multi-chunk deletion, falling live-page bytes, oldest-first count retention,
and startup sweeping. HTTP tests cover 204, 400, 404, and 409 responses; browser
tests cover deletion requests and preserved error status. The package race and
goroutine-leak checks continue to run with the ordinary test suite.
