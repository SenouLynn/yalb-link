# Development setup

## Prerequisites

- Go 1.25.x
- Bazelisk (reads `.bazelversion`, currently Bazel 8.7.0)
- Node.js 22 and Corepack/pnpm
- buf 1.72.x
- Docker with Compose v2
- golangci-lint v2

Verify the checkout with the versions pinned in the executable files rather
than copying versions from this guide:

```sh
go version
bazel version
pnpm --version
buf --version
docker compose version
```

Install frontend dependencies once:

```sh
cd frontend
pnpm install --frozen-lockfile
```

## Test and build

From the repository root:

```sh
make test
make lint
make bazel-test
```

Useful narrower checks:

```sh
make check-codec
make check-matrix
make check-integration SKIP_SITL_BUILD=1
```

## Generate code

```sh
make proto-gen
make bazel-tidy
```

Generated Go and TypeScript files are committed. Do not edit them directly.

## Run SITL

```sh
docker compose up
docker compose --profile multi-sitl up
docker compose --profile ui up
```

The default stack starts Copter SITL and the backend. SITL sends MAVLink
to `gcs-backend:14550`; the backend publishes UDP 14550 for host-side tools.
Compose health-gates SITL startup and assigns the backend `172.30.250.10` on
the project network. The SITL entrypoint resolves the service name to that
numeric address because ArduPilot's `udpclient` parser does not accept it
reliably; the fixed address keeps the route valid when only the backend is
recreated.

## Watch a vehicle fly

```sh
docker compose --profile ui up
```

Then open <http://localhost:3000>. The display picks the lowest-numbered
active vehicle and shows attitude, heading, altitude, speed, and power.

Nothing needs to be configured for telemetry to arrive. When the backend
sees a vehicle's first heartbeat it requests the message rates itself, so no
MAVProxy session or parameter change is involved:

| Message | ID | Requested rate |
|---|---|---|
| ATTITUDE | 30 | 10 Hz |
| GLOBAL_POSITION_INT | 33 | 5 Hz |
| VFR_HUD | 74 | 5 Hz |
| GPS_RAW_INT | 24 | 1 Hz |
| SYS_STATUS | 1 | 1 Hz |
| BATTERY_STATUS | 147 | 1 Hz |
| EKF_STATUS_REPORT | 193 | 1 Hz |

The requests are `MAV_CMD_SET_MESSAGE_INTERVAL` (511) in a `COMMAND_LONG`,
addressed to component 1 only, sent on discovery and again on recovery. The
policy lives in `bridge.DefaultRates`. Each `COMMAND_ACK` is logged, so an
autopilot that refuses a rate says so:

```sh
docker compose logs -f gcs-backend | grep -E "rates requested|command ack"
```

## Demonstrate the path end to end

1. `docker compose --profile ui up`
2. Confirm the stream carries a heartbeat and every requested family:
   ```sh
   curl -sN http://localhost:8080/api/events | head -40
   ```
3. Open <http://localhost:3000> and confirm the horizon and heading move and
   the readouts are populated.
4. Stop the vehicle with `docker compose stop ardupilot-sitl-copter-1`. After
   the five-second telemetry TTL (allow one 250 ms display tick), every reading
   becomes `- - -` with amber, aged provenance. After the 60-second vehicle TTL
   (allow the next one-second backend sweep), the link chip reads `LOST`.
5. Open <http://localhost:3000/?source=mock> with the backend stopped and
   confirm the same display renders from fixtures.

## Run the UI without a backend

`?source=mock` replays a deterministic fixture flight from
`frontend/src/stream/fixtures.ts` — no network, no SITL, the same protobuf
messages the live path carries. Live SSE is the default and mock is opt-in:
a ground station that quietly fell back to synthetic data would show an
aircraft that does not exist.

## The browser event stream

`GET /api/events` is a `text/event-stream` with two event names:

| Event | Payload |
|---|---|
| `fleet` | `gcs.v1.FleetEvent` as protobuf JSON |
| `telemetry` | `gcs.v1.TelemetryEvent` as protobuf JSON |

Each connection is bootstrapped from retained state — the latest fleet event
per vehicle, then the latest telemetry per vehicle and family, in identity
order — and then followed live. There is no replay and no `Last-Event-ID`:
reconnecting gets a fresh bootstrap.

Vite proxies `/api` to the backend, so the browser only ever uses a relative
URL and the backend needs no CORS configuration. Outside Compose the proxy
targets `http://localhost:8080`; set `GCS_BACKEND_URL` to point it elsewhere.
