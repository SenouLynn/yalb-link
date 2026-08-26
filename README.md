# YALB-GCS

YALB-GCS is an early-stage ground-control-system experiment for receiving
MAVLink telemetry, folding it into per-vehicle state, and presenting trusted
flight data in a React UI.

## Current state

The working path runs end to end:

```text
SITL -> MAVLink UDP -> codec -> per-vehicle fold -> event hub
                                               |-> SSE -> React flight display
                                               \-> bounded SQLite recording
                                                        |
                                             paged JSON replay -> same display
```

The repository currently has:

- MAVLink v1/v2 framing through `gomavlib` and golden-frame codec tests;
- decoding for 13 fleet/telemetry families and five transaction responses;
- addressed outbound encoders for 11 MAVLink message families;
- automatic `MAV_CMD_SET_MESSAGE_INTERVAL` rate requests on vehicle discovery
  and recovery, so a connected vehicle streams without external setup;
- deterministic vehicle discovery, loss, recovery, freshness, and route-table tests;
- an in-memory event hub and a `GET /api/events` server-sent-event stream that
  bootstraps each browser from retained state;
- opt-in, bounded SQLite recording with explicit start/stop lifecycle and
  deterministic replay after a backend restart, served as paged JSON from
  `GET /api/recordings/{id}/events`;
- a fleet-aware React flight display — artificial horizon, heading tape,
  altitude with its datum, speed, climb, power, link health, and a live
  MapLibre position/track map — that also
  runs from deterministic fixtures at `?source=mock` and replays a recorded
  flight at `?source=replay&recording=<id>`, with pause, scrub, and speed;
- pure TypeScript attitude, heading, position, battery, track, freshness, and
  trajectory logic;
- Docker Compose definitions for Copter and Plane SITL;
- native Go/TypeScript tests and a Bazel checkpoint build.

It is **read-only observation**. The project does not yet have a command or
mission surface, authentication, or MAVLink signing. Recordings accumulate
without an aggregate retention policy: nothing yet deletes or compacts them.

No project license has been selected or committed.

### Next demonstrable outcome

Aggregate recording retention — deleting a recording, and bounding total
database age and size. ADR 0002 bounds one recording; nothing yet bounds the
file they all share, so a long-running backend fills its disk.

### What the display refuses to do

An absent or stale reading renders as `- - -`; stale provenance remains amber
with its source and age, but stale data never drives a number or instrument.
Freshness is measured from the backend's `observed_at` rather than the moment
the browser received the event, so retained state replayed on reconnect reads
as old — which is what it is.

## Commands

```sh
make test                        # Go and TypeScript tests
make lint                        # golangci-lint and eslint
make bazel-test                  # hermetic checkpoint
docker compose --profile ui up   # SITL, backend, and the flight display
```

Then open <http://localhost:3000>, or
<http://localhost:3000/?source=mock> to run the display with no backend, or
<http://localhost:3000/?source=replay> to pick a recorded flight.

See [docs/runbooks/dev-setup.md](docs/runbooks/dev-setup.md) for prerequisites
and [docs/README.md](docs/README.md) for the documentation policy.
