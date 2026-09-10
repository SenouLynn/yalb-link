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

This is the default development source under the
[SITL-first development policy](../../README.md#sitl-first-development).
Open the normal UI URL to consume simulator traffic through the backend;
`?source=mock` selects browser fixtures instead and bypasses that path.
Keep real flight controllers disconnected from the development feed during
this phase.

```sh
docker compose up
docker compose --profile multi-sitl up
docker compose --profile ui up
```

For the UI with the mixed fleet, combine the existing profiles:
`docker compose --profile multi-sitl --profile ui up`. Starting the stack
establishes connectivity; use the scenario procedures below and each task's
acceptance criteria to validate behavior.

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

## Accept the live Copter trajectory

1. Start `docker compose --profile ui up` and open
   <http://localhost:3000> before moving the vehicle so the browser accumulates
   the complete track.
2. Attach a MAVLink ground station to `tcp:127.0.0.1:5760`, wait for healthy
   GPS and EKF state, switch Copter to Guided mode, arm, and take off to 15 m.
3. Send a Guided position target at `37.77535, -122.41900, 15`, then a second
   target at `37.77450, -122.41900, 15`. These north/south legs are long enough
   to make both the velocity vector and heading reversal unambiguous at the
   default map zoom.
4. During each leg, confirm the cyan track follows the vehicle and the dashed
   amber `5 S PREDICTION` extends ahead of it. Confirm that the projection
   reverses when the vehicle changes from the northbound to southbound leg.

Acceptance result on 2026-09-01 with ArduCopter 4.7.0: the live browser tracked
vehicle `1:1`; heading changed from 274 degrees on the westbound setup leg to
180 degrees on the southbound acceptance leg. At 9.9 m/s the map held its
bounded 500-point track and rendered an 11-coordinate trajectory from
`37.7747935, -122.4189998` to `37.7743469, -122.4189969`, approximately 50 m
ahead over the five-second horizon. No trajectory defect was observed.

## Accept the live Plane trajectory

1. Start only Plane, the backend, and the browser UI so the default vehicle
   selection cannot select Copter instead:

   ```sh
   docker compose --profile multi-sitl --profile ui up \
     gcs-backend ardupilot-sitl-plane-2 gcs-frontend
   ```

2. Open <http://localhost:3000>, then attach a MAVLink ground station to
   `tcp:127.0.0.1:5761`. Wait for healthy GPS and EKF state.
3. Put Plane in Takeoff mode, arm it, and let the autonomous takeoff reach a
   stable circuit. Switch to Guided mode and send a position target roughly
   150--200 m from home at 35 m above home. A fixed-wing target produces a
   sustained banked turn, which is the shortest repeatable way to exercise a
   visibly changing prediction vector.
4. Confirm the cyan track follows the circuit and the dashed amber
   `5 S PREDICTION` remains ahead of the vehicle while rotating through the
   turn. Check that the displayed heading, direction of travel, and projection
   agree before and after at least a 30-degree heading change.

Acceptance result on 2026-09-01 with ArduPlane 4.6.3: the live browser showed
armed fixed-wing vehicle `2:1`, current position and VFR HUD readings, a map
canvas and vehicle marker, and `5 S PREDICTION`. During the Guided turn the
browser heading changed from 55 to 87 degrees while ground speed held at
22.1 m/s; the projection remained present and rotated with the aircraft. That
speed projects about 110 m over the five-second horizon. The preceding live
MAVLink samples covered the full circuit and populated the bounded position
track. No trajectory defect was observed.

## Accept trajectory freshness posture

Use a controlled MAVLink publisher on UDP 14550 so one required family can be
withheld while unrelated traffic stays live. The publisher should use a
distinct system ID and send `HEARTBEAT`, `ATTITUDE`, and `VFR_HUD` continuously
with a nonzero ground speed:

1. Withhold `GLOBAL_POSITION_INT` (message 33) from the initial stream. Open
   the live UI and confirm that heading and ground speed are current while the
   map has no vehicle marker or `5 S PREDICTION`.
2. Begin message 33 at 5 Hz. Confirm the marker and prediction appear without a
   page reload.
3. Stop only message 33 for more than the five-second telemetry TTL. Keep the
   heartbeat, attitude, and VFR HUD messages flowing. Confirm position shows
   stale provenance and the marker and prediction disappear, while heading and
   speed remain current.
4. Resume message 33 at 5 Hz. Confirm the marker and prediction return without
   a reload.

Against a full Copter SITL stream, message 33 can be isolated with
`MAV_CMD_SET_MESSAGE_INTERVAL` (command 511): use param 1 = `33` and param 2 =
`-1` to stop it, then param 2 = `200000` to restore 5 Hz.

Acceptance result on 2026-09-01: a controlled live vehicle `42:1` began with
fresh 10.0 m/s speed and 90-degree heading but no position; the browser exposed
zero track and trajectory points. Fresh position produced a marker, track, and
11-coordinate prediction. With position stopped for more than five seconds,
the browser retained live 0.1-second attitude and VFR HUD readings but removed
the marker and prediction and marked only position stale. Resuming position
restored the marker and 11-coordinate prediction in the same page.

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

## Tune components in Storybook

From the repository root:

```sh
cd frontend
corepack pnpm install --frozen-lockfile
corepack pnpm storybook
```

Open the URL printed by Storybook (normally <http://localhost:6006>). It may
choose another port if 6006 is occupied. No backend, Docker, or SITL is needed.
Use Corepack to select the pnpm version pinned in `package.json`.

The catalog is ordered from individual components up to the actual frontend:

- **Components**: provenance, readout, attitude, heading, vehicle selector,
  pane controls, and the arm control's disabled mock posture.
- **Panels**: instruments, inspection, vehicle status, mission, and the real map.
- **Composition / Layout shell**: the production workspace shell with interactive
  panes and a labelled map placeholder for focusing on layout alone.
- **Frontend / Current app**: the actual `FlightDisplay` composition used by
  `App`, including fleet/vehicle navigation, sidebar, instruments, map, mission
  download, and pane controls. Select Vehicle, Fleet, Narrow, Animated, Stale,
  Empty Fleet, or No Vehicle.

Select a story and use **Controls** to change its values, reading state, mission
length, or available width. Use the viewport toolbar for 390px and 1440px layouts.
Narrow/wide stories select those viewports automatically. Changes to production
components and `src/ui/display.css` hot reload here and in the app.

Frozen readings use a fixed clock and existing mock telemetry fixtures. Stale
and unavailable stories preserve the app's withheld-value behavior. The Animated
frontend story runs the same `MockEventSource` and fleet reducer as `?source=mock`;
its stream and timer stop when the story unmounts. The frontend stories reuse
production visual composition; only stream and clock wiring are supplied by the
story harness. Public basemap imagery requires internet, just as in the app.

Mission download buttons in isolated panel stories log to Actions. The layout
shell story has local clear/download controls. The real frontend uses the
production mock mission loader. Arm commands remain disabled in mock mode.
Connected command/replay scenarios and figure-eight/snake profiles are outside
this catalog.

Add stories under `frontend/stories/*.stories.tsx`; keep reusable helpers in
`stories/fixtures.ts`. Only export story definitions from story files, since
Storybook treats named exports as stories. Configuration is in `.storybook/`;
production CSS is imported by its preview. The preview restores document
scrolling for isolated panels. Map stories retain the production `pane--map`
ancestor, which gives their inner map shell its height. Stories live outside `src` so the
application and Bazel source targets remain separate from this workshop.

```sh
corepack pnpm typecheck       # includes stories and Storybook configuration
corepack pnpm lint
corepack pnpm build-storybook # standalone output in storybook-static/
```

For a browser smoke check, open each catalog story and check for render errors.
On Components → Attitude → Stale, confirm a blank horizon and withheld roll/pitch. On
Composition → Layout shell → Wide, toggle each pane, hide all panes, and restore them. Clear the
mission and download it again. On Narrow, verify the stacked layout and scrolling.
In Frontend → Current app, open the selected vehicle from Fleet, download its
mission, toggle panels, and return to Fleet. Confirm the real map canvas appears
and Animated updates readings. In Panels → Map, check that the canvas fills
the selected height and actually displays tiles and mission markers. These are component checks, not acceptance
evidence for live flight telemetry.

## The browser event stream

`GET /api/events` is a `text/event-stream` with three event names:

| Event | Payload |
|---|---|
| `fleet` | `gcs.v1.FleetEvent` as protobuf JSON |
| `telemetry` | `gcs.v1.TelemetryEvent` as protobuf JSON |
| `command` | `gcs.v1.CommandTransaction` as protobuf JSON (live only; not bootstrapped) |

Each connection is bootstrapped from retained state — the latest fleet event
per vehicle, then the latest telemetry per vehicle and family, in identity
order — and then followed live. There is no replay and no `Last-Event-ID`:
reconnecting gets a fresh bootstrap.

Vite proxies `/api` to the backend, so the browser only ever uses a relative
URL and the backend needs no CORS configuration. Outside Compose the proxy
targets `http://localhost:8080`; set `GCS_BACKEND_URL` to point it elsewhere.

## Record one local flight

Recording is disabled by default. Start the backend with a writable SQLite
path, then use the lifecycle endpoints around a SITL flight:

```sh
GCS_RECORDING_ENABLED=true \
GCS_RECORDING_DB_PATH=./recordings.db \
GCS_RECORDING_MAX_TOTAL_BYTES=4294967296 \
GCS_RECORDING_MAX_TOTAL_AGE=720h \
go run ./cmd/gcs
curl -X POST http://localhost:8080/api/recordings/start
curl -X POST http://localhost:8080/api/recordings/stop
curl http://localhost:8080/api/recordings
```

The start request may instead carry `{"name":"test flight"}` as JSON. The
backend flushes every event accepted before stop returns. The SQLite store
persists fleet, telemetry, and command protobuf events; protocol events and
local warnings remain outside recording scope.

Initial safety defaults cap one recording at 200,000 events, 256 MiB of
protobuf payloads, or 30 minutes. Database and lifecycle operations time out
after five seconds, and store shutdown after ten seconds. These are tunable
`recording.Config` defaults.

Aggregate retention defaults to 4 GiB of live SQLite pages, 30 days, and 200
recordings, swept every five minutes and at startup. The two environment
variables above override size and age; count and sweep interval are code-level
configuration. Set a value negative to disable that bound. A stopped recording
can also be removed explicitly:

```sh
curl -X DELETE -i http://localhost:8080/api/recordings/1
```

Deletion frees pages for SQLite to reuse but does not shrink the database file.

### Durable recordings in Compose

```sh
GCS_RECORDING_ENABLED=true docker compose up -d --build gcs-backend
curl -X POST http://localhost:8080/api/recordings/start
# Allow telemetry to arrive, then flush and stop the session.
curl -X POST http://localhost:8080/api/recordings/stop
curl http://localhost:8080/api/recordings
GCS_RECORDING_ENABLED=true docker compose up -d --force-recreate gcs-backend
```

Compose stores SQLite and its journal files at `/var/lib/gcs/recordings.db` in
the `recordings` named volume (`yalb-gcs_recordings` with the default project
name). The image creates that directory owned by the backend's UID 10001;
Docker initializes a new volume with those permissions. No host-directory
permission override is needed. Container recreation and `docker compose down`
preserve the recordings. Keep `GCS_RECORDING_ENABLED=true` on subsequent `up`
commands to expose the recording endpoints; recording remains disabled by default.
Disabling recording does not delete stored sessions.

Delete individual stopped sessions with the DELETE endpoint above. To reset
**all** recordings permanently, stop the stack and remove its named volume:

```sh
docker compose down
docker volume rm yalb-gcs_recordings
```

For a custom Compose project name, use `<project>_recordings`. The next `up`
creates an empty volume. `docker compose down --volumes` also deletes recordings
and any other Compose-managed volumes; omit `--volumes` for ordinary shutdown.
The named volume does not import databases from earlier `/tmp` overrides or
native `go run` sessions.

## Replay a recorded flight

Recordings are served as pages of protobuf-JSON events, cursored by sequence
number:

```sh
curl -s 'http://localhost:8080/api/recordings/1/events?limit=5' | jq .
```

Each page carries the recording's lifecycle metadata, its events, and
`next_seq` — the `from_seq` that continues it, or `null` at the end of what is
available. An active recording serves its committed prefix, so a recording can
be checked without stopping it.

To watch one, open the display with the recording selected:

```sh
open 'http://localhost:3000/?source=replay&recording=1'
```

`?source=replay` with no `recording` lists what is on the backend instead. The
Source chip reads `REPLAY` in amber for as long as the page is showing history.

Restarting the backend between recording and replay is the check worth running:
it is what distinguishes durable storage from a buffer that happened to still
be in memory.

## Arm or disarm one SITL vehicle

Operator commands are disabled by default. For local SITL, enable the gate and
optionally recording before starting the backend:

```sh
GCS_COMMANDS_ENABLED=true GCS_RECORDING_ENABLED=true docker compose --profile ui up
curl -H 'Content-Type: application/json' \
  -d '{"system_id":1,"arm":true}' http://localhost:8080/api/commands/arm
```

The display requires a second confirmation click. `ACCEPTED` describes the
command ACK; the ARMED/DISARMED chip continues to come only from HEARTBEAT.
There is no retry. A timeout, cancelled browser request, or uncertain write
blocks another arm/disarm for that vehicle for this process session; restart
`gcs-backend` to clear it. Zero-target ACKs are ignored.

The endpoint's origin, fetch-metadata, and JSON content-type checks are
local-development CSRF protection, not authentication. Do not expose it to an
untrusted network.

## Accept the vehicle workspace (T-017)

Executed 2026-09-09 against current host Vite on port 3001 and rebuilt backend,
Copter 4.7.0 and Plane 4.6.3. See [procedures, screenshots and observations](validation/t017.md).
All eight pane combinations at 1440×900 and 768×1024 preserved viewport bounds.
Mission scrolling, pane lifecycle, selected-vehicle isolation, live timeout and
recovery, stale posture and persisted replay passed. Backend restart exposed a
terminal EventSource failure; the fixed rerun recovered in 1010 ms.

Frontend typecheck, lint, 287 Vitest tests and production build passed. Go race
suite passed on unchanged backend code. Bazel remains unavailable at the required
8.7.0 pin (8.3.1 installed), and golangci-lint is absent. These are not passing
gates. The frontend build retains its existing large-chunk warning.

The workspace inventory belongs to `frontend/src/workspace/Workspace.tsx`.
Feature modules retain their own state and lifecycle; hidden panes stay mounted.
T-015 inspection extends the instruments scroll body below primary readings;
T-016 adds an independently scrolling messages pane in main. App retains sole
stream/freshness-clock ownership. All-hidden leaves the topbar controls reachable.

Recording-enabled Compose currently needs an explicit writable database path;
T-021 tracks fixing the default launch. The acceptance override and external
mission preloader are documented with the evidence. No YALB mission mutation
endpoint was added.

T-008 was completed in the same session after a separate claim: independent
MAVLink/HTTP ordered-value comparison and true zero-item mission downloads
passed for Copter and Plane. See the evidence README's T-008 follow-through.
