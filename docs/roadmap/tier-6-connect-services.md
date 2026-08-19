# Tier 6 — First Connect Services + Track Layer + Telemetry Log

## Overview

Wire the first Connect/gRPC services and put live telemetry data in a browser. Two parallel tracks: Go services (FleetService, TelemetryService, track layer, TrackService) and the frontend adapter context plus the TelemetryLog debug component. The track layer ships here — not Tier 9 — because MapPanel requires it and MapPanel gates guided reposition validation in Tier 8.

## Dependencies

- Tier 5 live bridge complete (VEHICLE_DISCOVERED visible in Redis)
- **Tier 5 Chapter 7 complete — the discovery request round trip.** Every exit criterion
  below needs telemetry, and an ArduPilot link sends none until asked. This dependency was
  missing until 2026-08-19, when a six-minute SITL run produced heartbeats and nothing
  else; ADR-0010 closes it. `RADIO_STATUS` is the one priority family that will *not*
  appear over SITL — the SiK radio injects it and there is no radio here — so no criterion
  below may require it
- Tier 3 generated stubs available in `internal/gen/` and `frontend/src/gen/`

## Chapters

---

### Chapter 1: FleetService

**Goal:** Implement `WatchFleet` and `ListVehicles` from `services.proto`.

**File:** `internal/services/fleet.go`

**`WatchFleet` bootstrap sequence:**
1. `SMEMBERS fleet:active` → list of known sysIds
2. For each sysId: `HGETALL vehicle:state:<sysId>` → deserialize to `VehicleSnapshot`
3. Emit synthetic `FleetEvent{Type: FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, Snapshot: snapshot}` for each
4. Switch to `XREADGROUP fleet:events gcs-backend <consumer> > COUNT 100` for live events
5. Buffer events received during the HGETALL scan phase; deliver them after the synthetic batch

**Why the buffer step matters:** between step 1 and step 4, a VEHICLE_LOST event may arrive on the stream. If delivered before the synthetic VEHICLE_DISCOVERED, the client sees a lost event for a vehicle it never knew about. Buffering and replaying after the snapshot batch ensures correct ordering.

**`ListVehicles`:** `SMEMBERS fleet:active` → `HGETALL vehicle:state:<sysId>` per member → return list of `VehicleSnapshot` protos.

---

### Chapter 2: TelemetryService

**Goal:** Stream live telemetry to clients.

**File:** `internal/services/telemetry.go`

**`StreamTelemetry(vehicleId)`:**
- Subscribe to Redis Pub/Sub channel `telemetry:<sysId>`
- Deserialize each message from proto bytes → `TelemetryEvent`
- Stream to client until context cancelled or client disconnects

**`GetSnapshot(vehicleId)`:**
- `HGETALL vehicle:state:<sysId>` → deserialize → return `VehicleSnapshot`
- Returns `NOT_FOUND` status if sysId not in `fleet:active`

---

### Chapter 3: Track Layer — MAVLink Path

**Goal:** Normalize VehicleSnapshot → Track and publish to Redis so TrackService can consume it.

**File:** `internal/adapters/track.go`

**On every VehicleSnapshot update (triggered by vehicle fold):**
1. Normalize to `Track` proto with `source = TRACK_SOURCE_MAVLINK` and `MavlinkDetail` populated
2. `XADD track:events` with serialized Track proto
3. `HSET track:state:<nodeId>` with Track proto fields
4. `EXPIRE track:state:<nodeId> 300` (rolling TTL)

**Track nodeId:** `mavlink-<sysId>` — unique, human-readable, stable across reconnects for the same vehicle.

**Why here and not Tier 9:** MapPanel needs `TrackService.WatchTracks`. MapPanel is needed in Tier 8 to click a position for guided reposition. Building the track layer in Tier 9 would create a blocker.

---

### Chapter 4: TrackService

**Goal:** Bootstrap and stream track data.

**File:** `internal/services/track.go`

**`WatchTracks` bootstrap sequence:**
1. Scan `track:state:*` keys (or maintain a `track:active` set for O(1) — preferred)
2. `HGETALL track:state:<nodeId>` for each → emit synthetic `TrackEvent{Type: TRACK_EVENT_TYPE_UPDATED}`
3. Switch to `XREADGROUP track:events gcs-backend <consumer> >` for live updates

**Protocol-agnostic:** TrackService returns `Track` protos. Map consumers do not import MAVLink packages. Adding ADS-B or Meshtastic in Tier 9 adds a new adapter — zero changes to TrackService or the map.

---

### Chapter 5: Connect Server Wiring

**Goal:** Wire all services into the Connect HTTP server in `cmd/gcs/main.go`.

**Pattern:**
```go
mux := http.NewServeMux()
mux.Handle(gcsv1connect.NewFleetServiceHandler(fleetSvc))
mux.Handle(gcsv1connect.NewTelemetryServiceHandler(telemetrySvc))
mux.Handle(gcsv1connect.NewTrackServiceHandler(trackSvc))

srv := &http.Server{
    Addr:    ":8080",
    Handler: h2c.NewHandler(mux, &http2.Server{}),
}
```

**h2c:** Connect requires HTTP/2 or HTTP/1.1 with upgrades. Use `golang.org/x/net/http2/h2c` for cleartext HTTP/2 (TLS is terminated at the nginx proxy).

**CORS:** Configure Connect's CORS handler for the Vite dev server origin (`http://localhost:3000`). In Docker, both services are on the same network so CORS may not be needed, but the Vite dev proxy should be configured regardless.

---

### Chapter 6: Frontend Adapter Context

**Goal:** An abstraction layer between components and the Connect transport. Components call hooks; hooks call the adapter; the adapter is swappable between real Connect and a mock.

**Files:**
```
frontend/src/adapters/
  types.ts         GcsAdapter interface
  connect.ts       ConnectAdapter — wraps generated @connectrpc/connect-web client
  mock.ts          MockAdapter — static TelemetryEvent fixtures; no network
  context.tsx      AdapterProvider + useAdapter() hook
```

**`GcsAdapter` interface:**
```typescript
interface GcsAdapter {
  watchFleet(): AsyncIterable<FleetEvent>;
  listVehicles(): Promise<VehicleSnapshot[]>;
  streamTelemetry(vehicleId: string): AsyncIterable<TelemetryEvent>;
  getSnapshot(vehicleId: string): Promise<VehicleSnapshot>;
  watchTracks(): AsyncIterable<TrackEvent>;
}
```

**`MockAdapter`:** Returns fixture data from `frontend/src/adapters/__fixtures__/`. No network calls. Used in tests and Storybook (if added later). Supports the same component rendering as ConnectAdapter.

**`useVehicleTelemetry(vehicleId)` hook:** subscribes to `streamTelemetry`, buffers last N events, exposes as a React state value.

---

### Chapter 7: TelemetryLog Component

**Goal:** A debug component that shows live telemetry rows. No SVG, no math — just the data. This proves the full pipeline: SITL → UDP → fold → Redis → Connect → browser.

**File:** `frontend/src/components/debug/TelemetryLog.tsx`

**Layout:** flat table with columns: `variable | value | unit | source | age`

**Behavior:**
- `source` column: name of the MAVLink message and dialect (e.g., "ATTITUDE (common)", "EKF_STATUS_REPORT (ardupilotmega)")
- Color coding: primary source = green, fallback = yellow, missing/stale = grey
- `age` column: last-seen delta in ms, updates at 1 Hz
- Battery voltage row must be visible
- EKF flags row must be visible (raw hex is acceptable at this stage)

**Data source:** `useVehicleTelemetry(vehicleId)` → run each sample through all Tier 1 resolvers → render resolver outputs

**Freshness:** row goes grey when `isFresh(lastSeenMs, nowMs, 5000)` returns false (5s TTL). Entire log greys when SITL is stopped.

---

## Tier Exit Gate

- Browser shows live TelemetryLog for ArduCopter SITL
- Each row names its MAVLink source message and dialect
- Battery voltage visible as a row
- EKF flags visible as a row
- Freshness goes grey when SITL is stopped
- MockAdapter renders the same TelemetryLog with fixture data (no network)
- `redis-cli XREAD COUNT 5 STREAMS track:events 0` shows TrackEvent entries with MAVLink source
- Map-layer consumer can render track data without importing any MAVLink package
