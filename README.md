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
                                      \-> ambiguous outcome -> operator
                                          attestation -> bounded quarantine
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
- a guidance, home and radio-link inspection tier below the primary readings,
  projecting the three telemetry families the browser previously decoded and
  discarded, with `NAV_CONTROLLER_OUTPUT` added to the rate policy because a
  Copter sends none of it unasked;
- opt-in, bounded SQLite recording with explicit start/stop/delete lifecycle,
  aggregate age/count/live-size retention, and
  deterministic replay after a backend restart, served as paged JSON from
  `GET /api/recordings/{id}/events`;
- a responsive vehicle workspace with persistent panel visibility, a compact
  context sidebar, and independently scrolling instruments/map/mission panes —
  artificial horizon, heading tape,
  altitude with its datum, speed, climb, power, link health, and a live
  MapLibre position/track map with a freshness-gated five-second prediction —
  that also
  runs from deterministic fixtures at `?source=mock` and replays a recorded
  flight at `?source=replay&recording=<id>`, with pause, scrub, and speed;
- pure TypeScript attitude, heading, position, battery, track, freshness, and
  trajectory logic;
- Docker Compose definitions for Copter and Plane SITL;
- native Go/TypeScript tests and a Bazel checkpoint build.
- an opt-in, addressed arm/disarm transaction with acknowledgement, timeout,
  cancellation, and delivery-uncertain failure states;
- an operator-attested resolution for an ambiguous arm/disarm outcome, which
  records the observed armed state beside the unchanged terminal state and
  reopens commanding only after a bounded stale-ACK quarantine;
- a read-only onboard mission download — correlated by full vehicle identity
  and mission type, served as protobuf JSON from
  `GET /api/vehicles/{system_id}/{component_id}/mission`, and shown in the
  browser as an ordered item list and a distinct commanded route on the map,
  with a fixture mission at `?source=mock`. Copter and Plane downloads, exact ordered values, empty missions and
  timeout recovery were accepted in T-008.

It is read-only unless `GCS_COMMANDS_ENABLED=true`; even then, the only operator
commands are guarded arm/disarm and its resolution. `HOME_POSITION` is read but
never requested: ArduPilot sends it when home is set, so a backend started after
the vehicle set home will show home as not received rather than ask for it,
which would need `MAV_CMD_GET_HOME_POSITION`. Missions can be read but
never written: the project has no mission upload, clear, start, or set-current
operation, no generic command surface, no authentication, and no MAVLink
signing.
Transactions carry the fixed label `local-operator`, which records that a human
acted rather than who. SQLite reuses pages freed by recording retention, but the
database file does not shrink automatically.

No project license has been selected or committed.

### Next demonstrable outcome

Decoded telemetry inspection (T-015) is done: guidance, home and link render
below the instruments' primary readings, exercised against live SITL, mock
fixtures and recording replay — see
[T-015 evidence](docs/runbooks/evidence/t015/README.md). Status messages
(T-016) remain, in a dedicated main pane. The reference-informed shell is
implemented and exercised the same way; see
[T-017 evidence](docs/runbooks/evidence/t017/README.md) and the
[reference review](docs/reference/ui-reference-review.md). Mission framing and
styling remain T-012 and T-014, with full HUD parity in T-013.

T-015 acceptance surfaced one backend defect that predates it: after a vehicle
restarts and returns on a new UDP source port, the backend logs
`SOURCE_CONFLICT`, never marks it lost, and never re-issues its rate requests,
so every requested family goes silent until the backend restarts. That is
T-023.

Visual review with fixtures is only part of acceptance. Exercise applicable
workflows with live SITL and recording replay, including disconnect/reconnect,
stale or absent data, vehicle switches and failed requests. The completed
[T-008](docs/tasks/cards/T-008-live-mission-inspection-acceptance.md) accompanied
this phase to demonstrate mission inspection against both Copter and Plane.
Backend changes should address concrete workflow gaps discovered in these
slices; no broad infrastructure expansion is currently identified as a
prerequisite. Acceptance is limited to the scenarios recorded in the runbook.

Local/offline imagery and future 3D tile support are deliberately deferred. The
current public raster basemap remains a prototyping dependency; no tile storage
or serving architecture has been selected.

Operator authentication is deliberately deferred rather than pending. The system
runs in a trusted local environment with one operator, so a network identity
boundary waits for a deployment that needs one; multi-user and TAK identity wait
with it.

### SITL-first development

SITL is the default development environment until a specific validation need
requires hardware. Exercise simulated vehicles through the actual MAVLink UDP,
backend, SSE, UI and recording paths before beginning real flight-controller
ingestion. The aim is fidelity to the deployed data path: ArduPilot runs in
simulation while this application's normal infrastructure stays in the loop.
The current setup uses Docker Compose; this does not require replacing it with
a native host installation.

Develop against repeatable scenarios representing normal flight and failure
conditions: mission execution, vehicle selection, link loss/recovery, missing
or stale telemetry and rejected or timed-out operations, as applicable to the
feature. Record scenario setup, expected behavior and observed results. A
running simulator or a moving instrument alone is not feature acceptance.

Build trust through explicit claims and traceable evidence: begin with one
simple scenario, check autopilot response and captured telemetry against backend
state and UI output, then exercise failure and recovery. Expand to compound
patterns after that foundation works. Preserve scenario intent—crossings and
turn reversals, or sustained travel—while allowing the simulated aircraft to
respond naturally. Each acceptance result states what was proved, under which
conditions, and what remains unproven.

The browser's `?source=mock` uses synthetic fixtures and bypasses the backend;
it remains useful for fast UI iteration and isolated edge cases. It does not
replace SITL acceptance. Replay supplements SITL with reproducible captured
behavior. Name limitations that require hardware evidence, such as physical
radio behavior or device-specific timing, and move to hardware only for an
identified gap after the relevant SITL scenarios pass.

See the [development runbook](docs/runbooks/dev-setup.md#run-sitl) for the
existing stack and [task conventions](docs/tasks/README.md#sitl-first-validation)
for how to record scenario acceptance.

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
make tasks                       # agent task board and current ownership
docker compose --profile ui up   # SITL, backend, and the flight display
```

Then open <http://localhost:3000>, or
<http://localhost:3000/?source=mock> to run the display with no backend, or
<http://localhost:3000/?source=replay> to pick a recorded flight.

See [docs/runbooks/dev-setup.md](docs/runbooks/dev-setup.md) for prerequisites
and [docs/README.md](docs/README.md) for the documentation policy.
