# Tier 4 — Bridge Core (pure fold, injected clock)

## Overview

The vehicle model as a pure fold with no goroutines, no Redis, and no sockets. The fold is the heart of the bridge: it takes a current state, an incoming message, and a timestamp, and returns a new state plus any events to emit. The route table that maps sysId → return address is also built here. In parallel, Docker Compose infrastructure is scaffolded so it is ready when Tier 5 needs a live SITL.

## Dependencies

- Tier 1 codec (decode path) complete
- Tier 3 generated proto stubs available (`internal/gen/gcs/v1/`)
- Docker and Docker Compose installed (for the parallel scaffolding track)

## Chapters

---

### Chapter 1: Vehicle State Type

**Goal:** Define the in-memory vehicle state that the fold operates on.

**File:** `internal/vehicle/state.go`

**Type:**
```go
type VehicleState struct {
    SysID        uint8
    CompID       uint8
    Snapshot     *gcsv1.VehicleSnapshot   // proto; updated every fold
    LastSeenMs   int64
    LastExpireMs int64                    // EXPIRE throttle: only call Redis EXPIRE when nowMs - lastExpireMs > 30_000
    Armed        bool
    CustomMode   uint32
    BaseMode     uint8
    SystemStatus uint8
    VehicleType  uint8                    // MAV_TYPE from HEARTBEAT; used by trajectory resolver
    EkfFlags     uint16                   // from EKF_STATUS_REPORT; gates arm button
    // Rate bucket state for per-family freshness tracking
    LastMsgSeenMs map[uint32]int64        // msgID → last received ms
    // Source conflict detection
    LastSrcAddr  string                   // "<srcIP>:<srcPort>" of last packet; multi-source warning if changes
}
```

**VehicleSnapshot:** this is the proto message. The fold updates it in place (or returns a new copy — choose one, document it). It is the canonical serializable snapshot of vehicle state.

---

### Chapter 2: The Fold Function

**Goal:** A pure function that advances vehicle state given a single decoded message and a clock value.

**File:** `internal/vehicle/fold.go`

**Signature:**
```go
func Fold(state VehicleState, msg *codec.DecodedMessage, nowMs int64) (VehicleState, []Event)
```

**`codec.DecodedMessage`** carries:
- The `*TelemetryEvent` proto (from Tier 1 decoder)
- `SysID`, `CompID` from the MAVLink frame header
- `SrcAddr string` — `"<IP>:<port>"` recorded by transport layer

**Events emitted by fold:**

| Condition | Event |
|---|---|
| First HEARTBEAT from a (sysId, compId) pair | `FleetEvent{Type: VEHICLE_DISCOVERED}` |
| `nowMs - state.LastSeenMs > 60_000` and state was known | `FleetEvent{Type: VEHICLE_LOST}` |
| SrcAddr changes for an already-known sysId | `WarningEvent{Type: SOURCE_CONFLICT}` |
| Any message received | `TelemetryEvent` (pass-through to Redis publish channel) |

**HEARTBEAT handling:**
- Extract `custom_mode`, `base_mode`, `system_status`, `type` (MAV_TYPE), `autopilot`
- `armed = base_mode & MAV_MODE_FLAG_SAFETY_ARMED (128) != 0`
- Update `state.VehicleType` on first HEARTBEAT only (type should not change mid-flight)

**EKF_STATUS_REPORT handling:**
- Update `state.EkfFlags` from `flags` field
- EKF_UNINITIALIZED bit (10, value 1024) must be clear for arm to be allowed
- EKF_ATTITUDE bit (0, value 1) must be set for arm to be allowed

**TTL expiry:**
- Check on every fold call: `if nowMs - state.LastSeenMs > 60_000 { emit VEHICLE_LOST }`
- Set `state.LastSeenMs = nowMs` on every received message from that vehicle

**`time.Now()` must never appear in this file.** The fold receives time as a parameter. Tests use fixed timestamps.

---

### Chapter 3: Route Table

**Goal:** Track the return address for each vehicle so outbound commands reach the right socket destination.

**File:** `internal/routes/table.go`

**Entry:**
```go
type RouteEntry struct {
    SysID      uint8
    CompID     uint8
    SrcIP      string
    SrcPort    int
    LastSeenMs int64
}
```

**Operations:**
- `Upsert(entry RouteEntry, nowMs int64)` — set or update; record first-seen srcIP/port
- `Lookup(sysID, compID uint8) (RouteEntry, bool)` — O(1) map lookup
- `Evict(nowMs int64)` — remove entries where `nowMs - entry.LastSeenMs > 60_000`; called lazily on Lookup, not on a timer

**No background goroutine.** Eviction happens lazily on Lookup. This keeps the route table pure and testable without concurrency.

**Concurrent access:** the table is accessed from the transport receive goroutine and the command send goroutine. Protect with `sync.RWMutex`.

---

### Chapter 4: fleet:active Set Integration Point

**Goal:** Define the Redis set operations that the vehicle model will call in Tier 5 when fold emits VEHICLE_DISCOVERED / VEHICLE_LOST.

**This chapter is forward-definition only** — the actual Redis calls happen in Tier 5. Define the interface here so Tier 5 can implement it.

**Interface:**
```go
type FleetRegistry interface {
    AddVehicle(sysID uint8) error   // SADD fleet:active <sysId>
    RemoveVehicle(sysID uint8) error // SREM fleet:active <sysId>
}
```

**NopFleetRegistry** (for Tier 4 tests): implements the interface, does nothing.

---

### Chapter 5: Docker Compose Scaffolding [parallel ok]

**Goal:** A working `docker-compose.yml` that can start SITL-1 + Redis + backend stub. Does not need the real backend binary yet — just validates that the compose topology is correct.

**File:** `docker-compose.yml` (repo root)

**Services:**

| Service | Image/Build | Ports (host:container) | Notes |
|---|---|---|---|
| `ardupilot-sitl-1` | `build: ./docker/sitl` | `14550:14550/udp`, `5760:5760` | SYSID=1; `--out=udp:gcs-backend:14550` |
| `ardupilot-sitl-2` | same image | `14560:14550/udp`, `5761:5760` | SYSID=2; profile: `multi-sitl` |
| `ardupilot-sitl-3` | same image | `14570:14550/udp`, `5762:5760` | SYSID=3; profile: `multi-sitl` |
| `redis` | `redis:7-alpine` | `6379:6379` | AOF persistence; no auth in dev |
| `gcs-backend` | `build: .` | `8080:8080` | `GCS_MAVLINK_UDP_BIND=0.0.0.0:14550` |
| `gcs-frontend` | `build: ./frontend` | `3000:3000` | Vite; proxies `/api` → `backend:8080` |
| `nginx` | `nginx:alpine` | `443:443`, `80:80` | TLS termination → `backend:8080` |

**Key routing rule:** SITL instances 2 and 3 send MAVLink to `gcs-backend:14550` inside the Docker network. Host-published ports 14560/14570 are for external tooling (mavproxy, wireshark) only. The GCS single socket at 14550 receives all three instances, discriminated by sysId.

**SITL env vars per instance:**
```yaml
environment:
  - SYSID_THISMAV=1
  - ARDUPILOT_HOME=37.7749,-122.4194,10,0
command: >
  ArduCopter -S --model=+ --speedup=1
  --sysid=1
  --out=udp:gcs-backend:14550
  --home=37.7749,-122.4194,10,0
```

---

### Chapter 6: SITL Dockerfile

**Goal:** A multi-stage Dockerfile that produces a minimal SITL runtime image from a pinned ArduPilot commit SHA.

**File:** `docker/sitl/Dockerfile`

**Strategy:** Multi-stage build.
- Builder stage: `FROM ubuntu:22.04` — install build deps, clone ArduPilot at pinned tag, build `ArduCopter.elf` via `waf`
- Runtime stage: `FROM ubuntu:22.04` — copy binary + required shared libs only

**Pinning:** Use a specific release tag, e.g., `ARG ARDUPILOT_TAG=ArduCopter-4.6.0`. Never use a floating branch.

**Build command:**
```dockerfile
RUN git clone --depth=1 --branch=${ARDUPILOT_TAG} https://github.com/ArduPilot/ardupilot.git /ardupilot && \
    cd /ardupilot && \
    git submodule update --init --recursive && \
    ./waf configure --board sitl && \
    ./waf copter
```

**Note:** First build is slow (10–20 min). Layer caching is essential — the clone+build layer should only rebuild when `ARDUPILOT_TAG` changes. Use Docker BuildKit.

**CI gate:** SITL Dockerfile build is a slow gate — run only in CI, not in developer inner loop. Local developers use a pre-pulled image from a registry if available.

---

## Tier Exit Gate

- `go test -race ./internal/vehicle/...` passes with fixed-clock inputs — `time.Now()` never called
- `go test -race ./internal/routes/...` passes
- Sequence wrap/fallback tested (sysId collision, srcAddr change emits exactly one warning)
- Source-conflict detection: two different srcAddrs for same sysId emits WARNING event, not crash
- `docker-compose config` validates without error
- `docker-compose up redis` starts Redis cleanly
- SITL Dockerfile builds successfully (slow gate — CI only; local developer can skip with `SKIP_SITL_BUILD=1`)
