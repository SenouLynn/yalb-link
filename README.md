# YALB-GCS

YALB-GCS is an early-stage ground-control-system experiment for receiving
MAVLink telemetry, folding it into per-vehicle state, and presenting trusted
flight data in a React UI.

## Current state

The working path is:

```text
MAVLink frame -> codec -> per-vehicle fold -> structured log sink
```

The repository currently has:

- MAVLink v1/v2 framing through `gomavlib` and golden-frame codec tests;
- decoding for 13 fleet/telemetry families and five transaction responses;
- addressed outbound encoders for 11 MAVLink message families;
- deterministic vehicle discovery, loss, recovery, freshness, and route-table tests;
- pure TypeScript attitude, heading, position, track, freshness, and trajectory logic;
- Docker Compose definitions for Copter and Plane SITL;
- native Go/TypeScript tests and a Bazel checkpoint build.

It does **not** yet have a backend-to-browser telemetry API, a flight-instrument
UI, persistence, automatic telemetry-rate requests, or MAVLink signing.

No project license has been selected or committed.

## Next working slice

The next milestone is intentionally narrow: start one SITL vehicle, observe live
telemetry in the backend, expose that same data shape to the browser, and render
a small instrument view that can also run from deterministic mocks. Work that
does not help prove that end-to-end path should wait.

## Commands

```sh
make test          # Go and TypeScript tests
make bazel-test    # hermetic checkpoint
docker compose up  # Copter SITL and backend
```

See [docs/runbooks/dev-setup.md](docs/runbooks/dev-setup.md) for prerequisites
and [docs/README.md](docs/README.md) for the documentation policy.
