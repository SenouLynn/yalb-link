# 0002: Bound recording resources and shutdown

Status: Proposed

## Context

A bounded event channel prevents telemetry ingestion from allocating memory
without limit, but it does not make the recording control plane bounded. A
full writer queue, a stalled SQLite operation, or shutdown waiting on that
writer can otherwise keep a backend or validation process alive indefinitely.
An agent or operator must be able to detect and stop a broken recording even
when persistence itself is the broken component.

These are safety bounds, not a complete historical-data retention policy.
Aggregate database age, recording count, deletion, and compaction remain
undecided.

## Proposed decision

Apply provisional, configurable bounds at every layer:

- `Recorder.Publish` remains non-blocking and its queue remains capped at 1,024
  events. Rejected events consume sequence numbers so loss is observable.
- One recording is capped at 200,000 events, 256 MiB of protobuf payloads, and
  30 minutes. The duration deadline fires independently of telemetry arrival.
- SQLite operations and lifecycle commands default to a five-second deadline.
  A timed-out write immediately makes the in-memory recording terminal; other
  write failures become terminal after five consecutive failed batches.
- `Store.Close` defaults to a ten-second deadline. Queueing its control command,
  draining the writer, and closing the database all observe that deadline.
  `Store.Shutdown(ctx)` lets the composition root or tests impose a tighter one.
- Native Go tests use a two-minute outer process timeout, and the Bazel
  recording test has a short test timeout.

All values are defaults in `recording.Config`, not permanent operational
policy. Change them from measurements as recording volume and deployment
conditions become known.

## Consequences

- Persistence failure cannot apply backpressure to the MAVLink receive loop.
- Shutdown returns an error when its deadline is exhausted instead of waiting
  forever. A process supervisor remains the final bound for a kernel or driver
  call that ignores Go context cancellation.
- Events already accepted may be lost after a writer deadline or terminal
  failure. Sequence gaps and runtime counters expose that loss.
- If SQLite is unavailable when terminal status is written, the row can remain
  `active` on disk. The next successful open converts it to
  `error/process_restart`.
- Batch-boundary event and byte limits may exceed their threshold by the final
  committed batch. Memory remains bounded by the queue.

## Verification

`internal/recording` tests inject a full control queue, an independently fired
duration deadline, and a write function that blocks until its context expires.
They assert that publish remains non-blocking, stop and close return within
their deadlines, blocked writes become terminal, shutdown completes, and the
event, byte, duration, and restart paths retain observable terminal reasons.
