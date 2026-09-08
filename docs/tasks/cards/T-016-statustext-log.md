---
id: T-016
title: Show a vehicle message log from STATUSTEXT
status: backlog
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

A message log is one of the things the flight-hud-trajectory reference had and
this display does not. `STATUSTEXT` (253) is decoded, fixture-tested, matrix-
complete, generated into TypeScript, and already recorded — `recorder.go:77`
stores telemetry generically by kind, so log messages persist and replay today
with no recording change. It reaches the browser and is discarded.

Unlike the families in T-015, this is not a projection line. Three things make
it real work:

- **The frontend state model does not fit it.** `VehicleView.sample` is *latest
  value per field wins*; a log is a sequence. Projecting `STATUSTEXT` into
  `TelemetrySample` yields one message that overwrites itself. The precedent to
  follow is `VehicleView.track`, a bounded accumulated array capped by
  `TRACK_CAPACITY = 500` (`frontend/src/logic/track.ts:21`).
- **Messages are chunked.** `StatusText` carries `id` and `chunk_seq` because a
  message longer than 50 characters arrives split across frames. Nothing in the
  codebase reassembles them today, so a naive log shows fragments.
- **The hub retains latest-per-family only** (`internal/stream/hub.go:63`). A
  browser reconnecting is bootstrapped with exactly one `STATUSTEXT` — possibly
  a mid-message chunk — not the history. Whether the log survives a reconnect is
  a backend question, not a UI one.

## Outcome

The operator can read what the vehicle has been saying, with severity, ordering,
and multi-chunk messages reassembled, and the reconnect behaviour is a stated
decision rather than an accident.

## Scope

- In: chunk reassembly, bounded accumulation, severity presentation, ordering,
  the log surface itself, and a decision on reconnect retention.
- Out: the last-mile families (T-015), layout and density rework (T-013),
  changes to what the recorder stores.

## Acceptance criteria

- [ ] A message split across chunks is reassembled into one entry by `id` and
      `chunk_seq`, and an incomplete or out-of-order chunk sequence cannot
      produce a corrupted or duplicated entry.
- [ ] Severity is visible and drives presentation, using the existing palette —
      amber for caution, and no new "good" colour.
- [ ] The log is bounded, following the `TRACK_CAPACITY` precedent, and cannot
      grow without limit on a long session.
- [ ] Reconnect behaviour is explicit: either backend log retention is added, or
      the gap is shown to the operator rather than presented as complete history.
- [ ] Replay shows the log for a recorded flight, since the events are already
      stored.
- [ ] Chunk reassembly is pure, tested logic — not something only observable in
      a browser.

## Open questions

- Whether the hub gains a bounded per-vehicle log buffer. This is the one part
  of the reference's log that is genuinely absent from the backend rather than
  merely unrendered, and it decides whether a reconnected browser sees history.
- Whether the log is a panel inside the display or its own surface. Ties to
  T-013 and should be settled with it.
- Whether the log is per-vehicle or fleet-wide with vehicle attribution.
- Retention depth, and whether severity affects it — dropping `INFO` before
  `CRITICAL` under pressure is a policy decision, not a default.

## Notes

Raised 2026-09-08 while assessing how much of the flight-hud-trajectory
reference is reachable on the current backend. This is the highest-value of the
unrendered families: it is the one the operator specifically remembered, and the
one certain to produce visible output in SITL, since ArduPilot emits `STATUSTEXT`
on arming, mode change, and prearm failures without any rate request.

Held in `backlog` rather than `ready` because of the reconnect-retention
question. An earlier estimate in conversation called this "a small feature";
that was before the chunking and hub-retention constraints were checked, and it
is better described as small in UI and real in state handling.
