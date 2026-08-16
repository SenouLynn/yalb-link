# Order of Operations: yalb-gcs Buildout

A canonical build order for yalb-gcs. Defines what to build, in what sequence, why, and what must be true before moving on.

`port-plan-v1.md` in this directory maps what to port and where it lands.
`tiers/` contains one file per tier with chapter-level task breakdown.

ADRs are law. If this document contradicts an ADR, the ADR wins.

---

## Resolved Design Decisions

These are closed. Do not re-open without an ADR.

| Decision | Resolution |
|---|---|
| gomavlib vs hand-roll | **gomavlib v3** with ardupilotmega.Dialect (superset of common). Import `github.com/bluenviron/gomavlib/v3`. |
| Map tile provider | MapLibre GL + PMTiles (self-hosted). No API key. Field deployment = local mount. |
| Phase ordering: params vs commands | Params (read) before commands (write). Read-only proof before any writes. |
| Storybook vs Vite MSW | Vitest for pure logic. MSW in Tier 6 when ConnectAdapter needs a mock server. Storybook deferred to Tier 10 or beyond. |
| rules_buf vs buf generate | Commit generated files. Use `buf generate` as a Makefile target. Wire `rules_buf` into Bazel after Bzlmod support stabilizes. |
| Single socket vs per-vehicle sockets | **Single socket** at `0.0.0.0:14550`. Router dispatches by sysId. Multi-vehicle = multiple sysIds over one socket. All SITL instances target gcs-backend:14550. |
| ConnectLink auto-bind vs operator-initiated | **Auto-bind at startup**. ConnectLink is for runtime supplementary links only. `GCS_MAVLINK_UDP_BIND` env var; default `0.0.0.0:14550`; empty string = disabled. |
| GCS heartbeat placement | Dedicated goroutine in vehicle model, not transport layer. Ticks at 1 Hz per discovered vehicle. sync.Once guard on spawn. |
| GCS heartbeat fields | type=MAV_TYPE_GCS(6), autopilot=MAV_AUTOPILOT_GENERIC(0), base_mode=0, custom_mode=0, system_status=MAV_STATE_ACTIVE(4). sysId=255, compId=190. |
| ACK correlation design | In-flight command registry keyed by (vehicleSysID, vehicleCompID, commandID) from MAVLink frame header + ACK body. 5s timeout per entry. |
| Redis Streams vs Pub/Sub | `telemetry:<sysId>` → Pub/Sub (ephemeral, high-rate). `fleet:events`, `track:events`, `audit:events` → Streams (persistent, cold-start recovery). |
| TLS placement | Terminated at reverse proxy in Docker Compose. Backend speaks plain HTTP. |
| MAVLink signing key storage | Environment variable `GCS_MAVLINK_SIGNING_KEY` (hex). Empty = signing disabled (dev/SITL). |
| Pub/Sub consumer groups | `XREADGROUP` with group `gcs-backend` from day one on all Streams. `XGROUP CREATE` at startup with `MKSTREAM`. |
| Redis param key schema | `HSET params:<sysId>` — one hash per vehicle, param_id as field names. Single `EXPIRE 3600s` after full list completes. |
| EXPIRE throttling | `vehicle:state:<sysId>` EXPIRE at most once per 30s per vehicle, not once per heartbeat. |
| WatchFleet bootstrap | SMEMBERS `fleet:active` SET → emit synthetic VEHICLE_DISCOVERED batch → XREADGROUP from stream. |
| type_mask for position-only | `0xDF8` = 3576 = `0b110111111000`. FORCE_SET bit (9) not set. |
| ArduPilot SITL image | Multi-stage Dockerfile, pinned ArduPilot commit SHA. `ardupilot/ardupilot-dev-jammy` is dev env only, not a SITL runtime. |
| Stall gate | Conditional on vehicle type. Applies only to fixed-wing (MAV_TYPE_FIXED_WING and VTOL family). Zero for all multirotor types. |
| buf breaking gate | `buf breaking --against origin/main`. Not `HEAD~1`. |
| Track layer placement | MAVLink track path in Tier 6 alongside first Connect services. ADS-B and Meshtastic adapters in Tier 9. |
| SetArmedRequest.force field | Removed from proto in Tier 0. A field that exists can be set; removing it is the only safe choice. |
| SITL multi-instance routing | All instances send to `gcs-backend:14550`. Host-published ports 14560/14570 are for external tooling only. Discriminated by sysId in MAVLink header. |
| Priority receive set | 18 families: HEARTBEAT(0), SYS_STATUS(1), PARAM_VALUE(22), GPS_RAW_INT(24), ATTITUDE(30), GLOBAL_POSITION_INT(33), MISSION_CURRENT(42), MISSION_COUNT(44), MISSION_ACK(47), MISSION_ITEM_INT(73), VFR_HUD(74), COMMAND_ACK(77), NAV_CONTROLLER_OUTPUT(62), RADIO_STATUS(109), BATTERY_STATUS(147), HOME_POSITION(242), STATUSTEXT(253), EKF_STATUS_REPORT(193/ardupilotmega). |
| Priority send set | 11 families: COMMAND_LONG(76), SET_POSITION_TARGET_GLOBAL_INT(86), PARAM_SET(23), PARAM_REQUEST_LIST(21), PARAM_REQUEST_READ(20), MISSION_COUNT(44), MISSION_ITEM_INT(73), MISSION_REQUEST_INT(51), MISSION_ACK(47), HEARTBEAT(0), MISSION_CLEAR_ALL(45). |

---

## Principles

1. **Evidence before claims** — nothing is "done" without a test vector, schema, or SITL trace.
2. **Offline before online** — pure folds first, then injected clock, then live sockets.
3. **Read before write** — read-only proven before any outbound capability.
4. **Trust as process** — trust is established through resolved value → known source → fallback chain → freshness → known-answer test.
5. **Contracts are the portable unit** — protos + golden bytes + semantic traces. Generated code is an adapter.
6. **UI composition is deferred** — components are not scheduled until their data is proven at logic + service layer.
7. **ADRs are law; this document is not.**

---

## Build Order

Parallel tracks are marked **[parallel ok]**. Sequential dependencies are marked with their gate condition.

Tier files: `docs/wip/tiers/tier-N-*.md`

---

### Tier 0 — Proto Contracts + Proto Fix

**Detail:** `tiers/tier-0-proto-contracts.md`

Fix `SetArmedRequest.force` field removal before codegen. Confirm buf lint passes.

**Exit gate:** `buf lint` passes; `SetArmedRequest.force` field is gone.

---

### Tier 1 — Pure Domain Logic [parallel ok: Go + TS simultaneously]

**Detail:** `tiers/tier-1-pure-domain-logic.md`

Go: gomavlib codec wrapper + message decode → TelemetryEvent (18 receive families).
TS: all resolver functions + sampleFromEvent shim. No sockets, no goroutines, no Redis.

**Exit gate:** `go test -race ./internal/codec/...` passes with golden byte vectors; `vitest ./frontend/src/logic/...` passes with known-answer vectors; NaN never returned from any resolver.

---

### Tier 2 — Parity Apparatus + Test Vectors [parallel ok with Tier 1]

**Detail:** `tiers/tier-2-parity-apparatus.md`

Go capability matrix (18 receive + 11 send + framing cases). TS known-answer fixture tables. Golden byte migration from flight-path-hud.

**Exit gate:** Matrix exists with all case IDs populated; TS resolver tests reference named fixture files.

---

### Tier 3 — Codegen Infrastructure

**Detail:** `tiers/tier-3-codegen.md`

`buf.gen.yaml` → Go stubs in `internal/gen/`, TS client in `frontend/src/gen/`. CI gates. Commit generated files.

**Exit gate:** `buf generate` produces Go and TS stubs; `buf lint` passes; `buf breaking` wired in CI.

---

### Tier 4 — Bridge Core (pure fold, injected clock)

**Detail:** `tiers/tier-4-bridge-core.md`

Vehicle fold, route table, fleet:active SET. Docker Compose scaffolding with multi-stage SITL Dockerfile **[parallel ok with pure code]**.

**Exit gate:** Pure fold tests pass with fixed-clock inputs; `time.Now()` never called in fold; Docker Compose validates; SITL Dockerfile builds.

---

### Tier 5 — Transport Adapter + Live Bridge

**Detail:** `tiers/tier-5-transport-live.md`

UDP transport, Redis client, GCS heartbeat goroutine, goroutine supervision, /healthz. First live SITL connection.

**Gate:** Tier 4 fold + Docker Compose complete.

**Exit gate:** `docker-compose up` → VEHICLE_DISCOVERED in logs; GCS heartbeat confirmed via failsafe test; goroutine count stable over 60s.

---

### Tier 6 — First Connect Services + Track Layer + Telemetry Log

**Detail:** `tiers/tier-6-connect-services.md`

FleetService, TelemetryService, track layer (MAVLink path), TrackService. Frontend adapter context + TelemetryLog component.

**Gate:** Tier 5 live bridge complete.

**Exit gate:** Browser shows live telemetry log; each row names its MAVLink source; MockAdapter renders same component with fixture data; TrackEvent stream has MAVLink vehicle tracks.

---

### Tier 7 — Read Protocol Transactions

**Detail:** `tiers/tier-7-read-transactions.md`

Parameter read/list folds + mission download fold (pure). ParameterService, MissionService. ParametersPanel (read-only), MissionPanel.

**Gate:** Tier 6 services live.

**Exit gate:** ARMING_CHECK readable from live ArduCopter; 5-waypoint mission round-trips cleanly; all capability matrix rows for read transactions complete.

---

### Tier 8 — Write Protocol Transactions

**Detail:** `tiers/tier-8-write-transactions.md`

Command registry first. Then ordered by risk: 8a message interval → 8b param write → 8c mission upload → 8d mode change → 8e arm/disarm → MapPanel (required before 8f) → 8f guided reposition → 8g guided workflow lifecycle.

**Gate:** Tier 7 read transactions complete; command registry wired.

**Exit gate:** Armed copter in GUIDED, map click → copter moves to target; full guided workflow lifecycle runs against SITL.

---

### Tier 9 — Additional Protocol Adapters

**Detail:** `tiers/tier-9-protocol-adapters.md`

ADS-B (MAVLink ADSB_VEHICLE + dump1090/Beast) and Meshtastic adapters. Both normalize to Track proto. Map renders all sources without importing MAVLink.

**Gate:** Tier 6 track layer live; hardware available.

---

### Tier 10 — UI Composition

**Detail:** `tiers/tier-10-ui-composition.md`

Base component modules → trusted instrument tier → composed layout tier. Instruments not scheduled until resolvers proven at logic + service layer.

**Gate:** All Tier 8 write transactions complete; all instruments have live data sources.

---

## UI Information Architecture (designed now, built in Tier 10)

### Base component modules

| Component | Data | Trust requirement |
|---|---|---|
| `TelemetryValue` | name, value, unit, source, age | Resolver returns non-null with named source |
| `TelemetryLog` | list of `TelemetryValue` rows | Tier 6 complete |
| `VehicleChip` | sysId, type icon, armed/mode, freshness dot | `HeartbeatState` from Tier 5 |
| `CommandButton` | label, eligibility gate, loading state, result | Write transaction fold from Tier 8 |
| `SourceBadge` | primary / fallback / missing + message name | Resolver `source` field |
| `FreshnessRing` | last-seen age as fill or color | `freshness.ts` from Tier 1 |

### Trusted instrument tier (one instrument per resolver)

| Instrument | Resolver | Prerequisite |
|---|---|---|
| `AttitudeIndicator` | `resolveAttitude` | pitch ladder + roll arc |
| `HeadingIndicator` | `resolveHeading` | fallback chain proven |
| `FlightStateDisplay` | `resolveFlightPath2d` | NED sign flip tested; VFR_HUD path exempt from sign flip |
| `PredictiveTrajectory` | `resolvePredictiveTrajectory(event, vehicleType)` | CTRV model + vehicle-type-conditional stall gate tested |
| `FlightPathRecorder` | `accumulateTrack` | ENU projection tested |
| `AltitudeTape` | `resolvePosition.altM` | Position resolver with fallback tested |

### Composed layout tier

| Layout | Composes | Schedule |
|---|---|---|
| `PrimaryFlightDisplay` | `AttitudeIndicator` + `HeadingIndicator` | After both instruments pass SITL |
| `HudPanel` | PFD + `FlightStateDisplay` + `PredictiveTrajectory` | After all instruments validated |
| `OperatorPanel` | `VehicleChip` + `CommandButton` set + `ParametersPanel` | After Tier 8 write transactions |
| `NodeView` | `HudPanel` + `OperatorPanel` + `MapPanel` | After all above |

Note: `MapPanel` and `FleetView` are built during Tier 8 (before 8f), not here.

---

## What Is Deliberately Out of Scope

| Item | Reason |
|---|---|
| Supabase / OIDC live auth | `NoopAuthProvider` for SITL; real auth introduced when first external operator onboards |
| WebRTC / WFB-NG video | First-class sidecar; does not block any navigation tier |
| Calibration workflows | Requires command-driven state machines + physical prompts + reboot handling; post vehicle-config milestone |
| PID tuning | Depends on vehicle configuration milestone |
| Electron wrapper | Browser-first per ADR-0002 |
| gRPC-native browser transport | ADR defers; Connect serves as bridge |
| Meshtastic relay implementation | Adapter boundary defined (ADR-0004); waits on Tier 9 + hardware |
| mTLS between services | TLS termination at proxy satisfies ADR-0005 for now; mTLS is a hardening step |
| MISSION_CLEAR_ALL UI | Protocol defined; wire command in Tier 8; surface in mission panel post-Tier 10 |
