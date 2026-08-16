# Tier 5 — Transport Adapter + Live Bridge

## Overview

Connect the pure fold to a live UDP socket, wire Redis, and configure the GCS heartbeat (gomavlib emits it — Chapter 3). This is the first tier with goroutines of our own in the codebase. The pipeline goes: UDP socket → frame codec → message decoder → router → vehicle fold → Redis. Every boundary is a typed channel with an explicit buffer. Shutdown is coordinated; there are zero goroutine leaks past context cancellation.

## Dependencies

- Tier 4 vehicle fold and route table complete
- Tier 1 codec complete
- Docker Compose working (Redis accessible at localhost:6379 or inside compose network)
- gomavlib v3 in go.mod

## Chapters

---

### Chapter 1: UDP Transport

**Goal:** A `net.PacketConn` reader/writer that exposes typed channels and respects context cancellation.

**File:** `internal/transport/udp.go`

**Interface:**
```go
type UDPTransport struct {
    recv chan InboundPacket   // buffer: 512
    send chan OutboundPacket  // buffer: 64
}

type InboundPacket struct {
    Data    []byte
    SrcAddr net.Addr
}

type OutboundPacket struct {
    Data    []byte
    DstAddr net.Addr
}
```

**Bind address:** `GCS_MAVLINK_UDP_BIND` env var; default `0.0.0.0:14550`. If env var is empty string, skip socket creation entirely (test mode — no error).

**Read loop:**
- `conn.ReadFrom(buf)` → copy `buf[:n]` into new slice (do not retain buf) → push to `recv` channel
- On context cancellation: `conn.Close()` → read loop exits on error → close `recv` channel
- Sender address recorded with each packet and passed downstream for route table population

**Write path:**
- Goroutine drains `send` channel → `conn.WriteTo(pkt.Data, pkt.DstAddr)`
- `WriteTo` errors logged but not fatal (vehicle may have disconnected)
- On context cancellation: drain `send` channel, exit

**Buffer rationale:** 512 recv buffer accommodates bursts from multiple SITL instances. 64 send buffer is sufficient since command sends are low-rate.

---

### Chapter 2: Pipeline Assembly

**Goal:** Wire all stages into a single goroutine pipeline in `cmd/gcs/main.go`.

**File:** `cmd/gcs/main.go`

**Pipeline stages:**
```
UDPTransport.recv
  → frameDecoder (gomavlib frame parse; extracts sysId, compId, raw message)
  → messageDecoder (Tier 1 codec; emits TelemetryEvent proto)
  → router (dispatches to per-vehicle fold goroutine by sysId)
  → vehicleFold (pure fold + event emission)
  → Redis publisher (telemetry Pub/Sub + fleet/track Streams)
```

**Per-vehicle goroutines:**
- One goroutine per discovered vehicle, spawned on first HEARTBEAT
- Each goroutine holds the fold state for that vehicle
- Context: child of the main context; cancelled on VEHICLE_LOST or main shutdown
- Uses `sync.Once` guard keyed by sysId to prevent duplicate spawn

**errgroup coordination:**
```go
g, ctx := errgroup.WithContext(mainCtx)
g.Go(transport.RunRecv)
g.Go(transport.RunSend)
g.Go(router.Run)
g.Go(redisPublisher.Run)
// per-vehicle goroutines added dynamically via g.Go(vehicle.Run)
if err := g.Wait(); err != nil { log.Fatal(err) }
```

**Shutdown order:**
1. Cancel main context
2. Transport closes UDP socket
3. Recv channel closed → router exits
4. Router exits → per-vehicle goroutine contexts cancelled
5. Per-vehicle goroutines exit → Redis publisher sees closed channels
6. errgroup.Wait() returns

---

### Chapter 3: GCS Heartbeat — configure, do not write

**Goal:** Emit `HEARTBEAT` at 1 Hz on each link. Without it, ArduPilot's GCS failsafe
fires within a few seconds and silently rejects guided commands.

**There is no heartbeat file to write.** gomavlib already does this. Verified against
v3.3.5 `node.go`: `HeartbeatDisable` defaults to *false*, `HeartbeatPeriod` defaults
to 5s, `HeartbeatSystemType` defaults to `6` (MAV_TYPE_GCS) and
`HeartbeatAutopilotType` to `0` (MAV_AUTOPILOT_GENERIC). The only change needed is
the rate:

```go
node := &gomavlib.Node{
    // ...
    HeartbeatDisable: false,
    HeartbeatPeriod:  time.Second,
    OutSystemID:      255,
    OutComponentID:   190,
}
```

**Per link, not per vehicle.** The earlier plan called for a goroutine per discovered
vehicle with a `sync.Once` guard. That is wrong twice. It double-emits, because the
library's heartbeat is already running unless disabled. And the cardinality is wrong:
HEARTBEAT is a node-level broadcast announcing *this GCS* on a link, not a message
addressed to a peer. Twenty vehicles on the shared `0.0.0.0:14550` socket would
produce twenty identical frames per second from (255,190) — on a 57.6 kbps SiK link
roughly 400 B/s of pure duplication in the scarce direction, inflating
`RADIO_STATUS.txbuf`, which is the back-pressure signal `fleet.proto` documents as
the most actionable field.

gomavlib's per-channel heartbeat already has the right cardinality. Use it.

**Validation procedure (required for exit gate):**
1. `HeartbeatDisable: true` → send `MAV_CMD_COMPONENT_ARM_DISARM` → confirm STATUSTEXT
   "GCS Failsafe" or COMMAND_ACK with result DENIED
2. `HeartbeatDisable: false, HeartbeatPeriod: time.Second` → send the same command →
   confirm COMMAND_ACK ACCEPTED
3. Capture 10s of traffic with one vehicle connected, then with two. The GCS heartbeat
   count must not scale with vehicle count.

---

### Chapter 4: Redis Client

**Goal:** A typed Redis client that wraps the go-redis library and exposes only the operations needed by this system.

**File:** `internal/redis/client.go`

**Operations:**
```go
type Client interface {
    // Telemetry Pub/Sub (ephemeral)
    Publish(ctx context.Context, channel string, msg proto.Message) error
    Subscribe(ctx context.Context, channel string) (<-chan proto.Message, error)

    // Streams (persistent)
    XAdd(ctx context.Context, stream string, fields map[string]string) error
    XReadGroup(ctx context.Context, stream, group, consumer string) (<-chan StreamEntry, error)
    XGroupCreateMkStream(ctx context.Context, stream, group string) error // idempotent

    // Hash (vehicle state, params)
    HSet(ctx context.Context, key string, fields map[string]interface{}) error
    HGetAll(ctx context.Context, key string) (map[string]string, error)
    Expire(ctx context.Context, key string, ttl time.Duration) error

    // Set (fleet:active)
    SAdd(ctx context.Context, key string, members ...string) error
    SRem(ctx context.Context, key string, members ...string) error
    SMembers(ctx context.Context, key string) ([]string, error)
}
```

**MAXLEN enforcement:** `XAdd` should use `XADD MAXLEN ~ 10000` for fleet/track streams and `MAXLEN ~ 50000` for audit stream. Use `~` (approximate trimming) for performance.

**EXPIRE throttling:** implemented in the vehicle model, not here. The Redis client `Expire` method is unconditional; the caller decides whether to call it.

---

### Chapter 5: Redis Init Sequence

**Goal:** Ensure consumer groups exist before any XREADGROUP call. Must run at service startup, before serving any requests.

**File:** `internal/redis/init.go`

**Sequence:**
```go
func InitStreams(ctx context.Context, client *redis.Client) error {
    streams := []string{"fleet:events", "track:events", "audit:events"}
    for _, stream := range streams {
        err := client.XGroupCreateMkStream(ctx, stream, "gcs-backend", "$").Err()
        if err != nil && !isBusyGroupError(err) {
            return fmt.Errorf("init stream %s: %w", stream, err)
        }
        // BUSYGROUP = group already exists; idempotent, not an error
    }
    return nil
}
```

**`isBusyGroupError`:** checks if the error string contains "BUSYGROUP Consumer Group name already exists".

**MKSTREAM:** creates the stream itself if it does not exist yet. Safe to call repeatedly.

---

### Chapter 6: Health Endpoint

**Goal:** A `/healthz` HTTP endpoint for Docker Compose health checks and monitoring.

**File:** `internal/health/handler.go`

**Response:**
```json
{
  "redis": "ok",
  "sitl_link": "up",
  "last_heartbeat_ms": 1234567890
}
```

**`sitl_link`:** `"up"` if at least one vehicle heartbeat was received within the last 5 seconds; `"down"` otherwise.

**`last_heartbeat_ms`:** epoch ms of last HEARTBEAT from any vehicle; 0 if none received.

**Redis check:** attempt a `PING`; if it fails, `"redis": "error"`.

**Wire into `cmd/gcs/main.go`:** `http.HandleFunc("/healthz", health.Handler)` on a separate port (e.g., 8081) or on 8080 alongside the Connect server.

---

## Tier Exit Gate

- `docker-compose up` → SITL-1 heartbeat visible in backend logs with `VEHICLE_DISCOVERED`
- `redis-cli SUBSCRIBE telemetry:1` shows live `TelemetryEvent` proto bytes
- `redis-cli XREAD COUNT 10 STREAMS fleet:events 0` shows `VEHICLE_DISCOVERED` event
- `redis-cli SMEMBERS fleet:active` returns `["1"]`
- GCS heartbeat validated: arm command accepted when heartbeat running; STATUSTEXT "GCS Failsafe" when heartbeat stopped
- `/healthz` returns `{"redis":"ok","sitl_link":"up",...}`
- `go test -race ./...` passes
- Goroutine count stable over 60-second SITL run (verify with `pprof` or log-level goroutine dump)
