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
operator -> guarded arm POST -> addressed COMMAND_LONG -> ACK registry
                                      |-> command SSE / recording / response
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
- opt-in, bounded SQLite recording with explicit start/stop/delete lifecycle,
  aggregate age/count/live-size retention, and
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
- an opt-in, addressed arm/disarm transaction with acknowledgement, timeout,
  cancellation, and delivery-uncertain failure states.

It is read-only unless `GCS_COMMANDS_ENABLED=true`; even then, the only operator
command is guarded arm/disarm. The project does not have a generic command or
mission surface, authentication, or MAVLink signing. SQLite reuses pages freed
by recording retention, but the database file does not shrink automatically.

No project license has been selected or committed.

### Next demonstrable outcome

Authenticated operator sessions and an explicit workflow for resolving an
ambiguous timed-out command, without expanding to a generic command surface.

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
