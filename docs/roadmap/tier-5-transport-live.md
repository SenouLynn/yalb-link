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
1. Capture 10s of traffic with one vehicle connected, then with two. The GCS heartbeat
   count must not scale with vehicle count.
2. Confirm the period: heartbeats from (255, 190) arrive at 1 Hz, not gomavlib's 5s
   default.

**What is *not* validated here, and why.** The original procedure opened with
`HeartbeatDisable: true` → send `MAV_CMD_COMPONENT_ARM_DISARM` → expect
STATUSTEXT "GCS Failsafe" or COMMAND_ACK DENIED, then the same command with the heartbeat
running → expect ACCEPTED. Both steps require an arm command, which is Tier 8e and the
highest-risk write in the roadmap. **This tier cannot prove the failsafe claim, and it is
the one gate here that Principle 3 genuinely does block** — arming changes vehicle state,
unlike the requests in Chapter 7. The assertion moved to Tier 8e, where the command
already exists and costs nothing extra. What Tier 5 proves is cardinality and rate; what
Tier 8e proves is that the rate is sufficient to hold off the failsafe.

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

### Chapter 7: Discovery Request Round Trip

**Goal:** Ask each vehicle, once it is discovered, to tell us what it is and to start
sending telemetry. Without this the link carries heartbeats and nothing else — proven, not
predicted, by the 2026-08-19 run.

**Authority:** ADR-0010. It classifies these frames as read-side, which is why they land
here and not in Tier 8. ADR-0007 §1 already required the capability half of this round trip
"at discovery", and `../reference/codec-capability-matrix.md:165,168` already assigned it to
Tier 5 — it simply had no chapter. Numbered 7 to leave Chapters 4–6 and the As Built table
stable; in execution order it belongs immediately after Chapter 2, because it is triggered
by the same first HEARTBEAT the router folds.

**File:** `internal/bridge/request.go`

**Trigger.** The `VEHICLE_DISCOVERED` and `VEHICLE_RECOVERED` events the fold already emits.
Recovery matters as much as discovery: a vehicle that dropped and came back has a new
gomavlib channel and, on a real radio, may have rebooted. Keying only on discovery is the
same `sync.Once` mistake the router loop was built to avoid.

**What is sent**, addressed to the link the heartbeat arrived on — `WriteTo(LinkID, msg)`,
never broadcast, per the outbound-targeting decision in `order-of-operations.md`:

| Command | Payload | Why |
|---|---|---|
| `COMMAND_LONG` cmd 512 | `param1 = 148` (`AUTOPILOT_VERSION`) | ADR-0007 §1 capability bitmap; source of `flight_sw_version` and `uid`/`uid2`, which ADR-0009 §2's cache predicate is keyed on |
| `COMMAND_LONG` cmd 512 | `param1 = 435` (`AVAILABLE_MODES`) | ADR-0007 §2 mode layer (a). Absent on Plane 4.6, which is the point of the split firmware pin — it falls back to layer (b) and that is success, not failure |
| `COMMAND_LONG` cmd 511 × 9 | `param1 = <msgid>`, `param2 = <interval_us>` | The **periodic** families of the priority receive set |

**The nine, and the three that are excluded.** Request intervals for `SYS_STATUS`(1),
`GPS_RAW_INT`(24), `ATTITUDE`(30), `GLOBAL_POSITION_INT`(33), `MISSION_CURRENT`(42),
`NAV_CONTROLLER_OUTPUT`(62), `VFR_HUD`(74), `BATTERY_STATUS`(147), `EKF_STATUS_REPORT`(193).

Do **not** request `RADIO_STATUS`(109), `STATUSTEXT`(253) or `HOME_POSITION`(242).
`RADIO_STATUS` is injected by the SiK radio firmware rather than produced by the autopilot,
so there is nothing to rate — and it does not appear over a SITL UDP link at all, which is
why no gate may require it against SITL. The other two are event-driven. ADR-0010 §2 states
the reasons; they are repeated here because the failure mode is a later reader "correcting"
nine back to twelve.

**Rates.** Pick them here, then verify against SITL rather than asserting them. The binding
constraint is the uplink budget the heartbeat-cardinality decision already protects: stream
rates and heartbeat duplication draw on the same scarce direction, and `RADIO_STATUS.txbuf`
is the signal that reports it. A defensible starting point is 4 Hz for `ATTITUDE` and
`GLOBAL_POSITION_INT`, 1 Hz for the rest, adjusted once measured.

**No command registry.** Fire-and-observe, per ADR-0010 §3. No in-flight entry, no ACK
correlation, no audit event, no role check. The success signal is telemetry arriving; the
failure signal is telemetry not arriving. `COMMAND_ACK` for 511/512 decodes to a
`ProtocolEvent` and is dropped, because `vehicle.Event` has no transaction member until
Tier 7 adds one — so a refusal and an ignored request look identical here. Both are
corrected by the re-send, and neither is silent: the absence is visible in the log.

**Re-send on a timer.** The link is UDP. A lost request leaves a healthy vehicle
permanently silent, which is indistinguishable from the bug this chapter fixes. Re-send
while the vehicle is known and telemetry has not been seen; stop once it has. gomavlib's
own stream-request module re-sends every 30s unconditionally — matching that order of
magnitude is sensible, and stopping on success is the improvement.

**Tests.** `internal/bridge/harness_test.go` already drains the outbound side of the
`net.Pipe`, so asserting on what was written needs a capturing drain rather than a
discarding one. Assert: the three request kinds go out on discovery; they go out again on
`VEHICLE_RECOVERED`; they are addressed to the originating link and not broadcast; the
re-send fires when no telemetry follows and stops when it does; and the nine/three split is
pinned by a table test, so shortening the exclusion list fails a test rather than a field
flight.

---

## Tier Exit Gate

- `docker-compose up` → SITL-1 heartbeat visible in backend logs with `VEHICLE_DISCOVERED`
- `redis-cli SUBSCRIBE telemetry:1` shows live `TelemetryEvent` proto bytes
- `redis-cli XREAD COUNT 10 STREAMS fleet:events 0` shows `VEHICLE_DISCOVERED` event
- `redis-cli SMEMBERS fleet:active` returns `["1"]`
- **Telemetry events in the log within seconds of `VEHICLE_DISCOVERED`** — not heartbeats
  only. This is the line the 2026-08-19 run failed for six minutes; it is the gate for
  Chapter 7 and the precondition for every Tier 6 exit criterion
- `AUTOPILOT_VERSION` received and folded: `flight_sw_version` and `uid` present in vehicle
  state for each SITL instance
- GCS heartbeat cardinality validated: the per-second count from (255, 190) does not scale
  with the number of connected vehicles, and the period is 1 Hz. **The failsafe assertion is
  Tier 8e's** — see Chapter 3
- `/healthz` returns `{"redis":"ok","sitl_link":"up",...}`
- `go test -race ./...` passes
- Goroutine count stable over 60-second SITL run (verify with `pprof` or log-level goroutine dump)

---

## As Built (2026-08-19) — Chapters 1, 2, 3 only

Partial. `internal/bridge` plus `cmd/gcs` cover the receive path; Chapters 4, 5 and 6
(Redis client, Redis init, real `/healthz`) are not started, and Chapter 7 (the discovery
request round trip) was written *because* of what building 1–3 revealed. The gate is not a `make`
target yet — it becomes one when Chapter 4 lands and there is something for
`redis-cli SUBSCRIBE` to show.

`docs/changelog/CHANGELOG.md` records the reasoning. The corrections that change how a
reader should use the chapters above:

| Chapter | The plan said | As built |
|---|---|---|
| 1 | `internal/transport/udp.go` over a raw `net.PacketConn`, exposing `InboundPacket`/`OutboundPacket` channels | **No such package.** gomavlib owns the socket via `EndpointConf` and takes an endpoint, not a byte stream. `cmd/gcs` builds `gomavlib.EndpointUDPServer` from `codec.ResolveBind` and passes it to `codec.NewNode`; `codec.FrameSource` is the transport port. `EndpointCustom` is not an escape hatch — it is a single stream and cannot represent N UDP peers on one bound port |
| 1 | Bind resolution reads the env var directly | `codec.ResolveBind(os.LookupEnv(...))` — unset means the default address, explicitly empty means no socket, and `cmd/gcs` logs which it got |
| 2 | One goroutine per vehicle, `sync.Once`-guarded, cancelled on `VEHICLE_LOST` | One router loop holding `map[routes.Key]vehicle.State`. The `sync.Once` guard is actively wrong now that `VEHICLE_RECOVERED` exists: it would deny a returning vehicle its goroutine forever |
| 2 | Pipeline terminates in a Redis publisher | Terminates in a `bridge.Sink`. `LogSink` for first light, Redis sink at Chapter 4. A sink error stops `Run` rather than being logged and continued |
| 2 | Fold input assembled from `codec.DecodedMessage` + `SrcAddr` | `vehicle.Inbound`, and `SrcAddr` is the gomavlib channel label. `EventFrame` carries no peer address; the label is `udp:<host>:<port>` per peer, which is the right granularity and also works on serial |
| 2 | (not mentioned) | Frames from our own `(255, 190)` identity are dropped before the fold, or they become a phantom vehicle 255 in `fleet:active` |
| 2 | (not mentioned) | A 1s sweep tick calls `vehicle.Expire` and `routes.Evict`. Without it the fold's TTL check can never fire, because a silent vehicle produces no folds |
| 3 | "configure, do not write" | Correct and already done — `codec.NewNode` sets `HeartbeatPeriod`, `OutSystemID` and `OutComponentID`. Nothing was added here. The **validation procedure has since been split**: cardinality and rate are provable here and stay; the failsafe assertion needed an arm command and moved to Tier 8e. It is still unrun |
| 6 | `/healthz` returns redis/sitl_link/last_heartbeat_ms | Still the Tier 4 liveness stub returning 200. Deliberate: a green `/healthz` claiming to have checked a Redis that does not exist yet is worse than one that visibly does not |
| 7 | (did not exist) | **Added after first light.** The chapter that makes telemetry arrive at all — ADR-0010 §2. Not built; the receive path was assembled first and is what surfaced the need for it |

### Resolved: nothing streams — ADR-0010

Run against the pinned `Copter-4.7.0` SITL, the backend held contact for six minutes and
received **heartbeats only** — one `VEHICLE_DISCOVERED`, zero `VEHICLE_LOST`, zero
telemetry events. `Tools/autotest/default_params/copter.parm` sets no `SR0_*`/`SR1_*`
stream rates and `codec.NewNode` leaves `StreamRequestEnable` false. Nothing had asked the
vehicle to send anything.

This was an **ordering defect**, not a bug: Tier 6 exits on "browser shows live telemetry
log" and Tier 7 on "ARMING_CHECK readable from live ArduCopter", and the stream-request
path both depend on was scheduled at Tier 8a. **Closed by ADR-0010 and Chapter 7 above.**

Two things had to be corrected before the choice was obvious, and both are worth keeping:

**The premise was false.** This file, `tier-1-pure-domain-logic.md` and the comment on
`StreamRequestEnable` all asserted that SITL streams telemetry unprompted, so Tiers 5–6
would look healthy and only field bring-up would be surprised. That is what made deferring
to Tier 8a survivable. SITL streams nothing either. Per ADR-0008 §3 the run outranks the
prose; all three have been corrected.

**Principle 3 was worded wrong, not crossed.** Requesting data is not a write. The repo had
already accepted this without saying so: ADR-0007 schedules an outbound `COMMAND_LONG`
cmd 512 at discovery, the capability matrix assigns it to **Tier 5**, and nobody asked for
a carve-out. Tier 8's own text says of cmd 511 "Reversible. Affects only telemetry rate. No
vehicle state change." ADR-0010 writes the boundary down.

Why the three candidate resolutions were not taken:

1. **`SR0_*` in the SITL parameter overlay** — rejected, and the reason inverted once the
   premise fell. It was rejected originally for making SITL diverge from a field vehicle in
   the property under test. With neither streaming unprompted, an overlay would now
   *create* that divergence: SITL would stream without being asked and a field vehicle would
   not. It would make Tier 6 pass and a field flight fail, which is exactly what
   `StreamRequestEnable: false` was set to prevent.
2. **Pull `SET_MESSAGE_INTERVAL` forward from Tier 8a** — superseded by something better.
   Tier 8a was not early enough to move; it was *misfiled*, because it rides `COMMAND_LONG`
   and everything on `COMMAND_LONG` had been filed under writes. Chapter 7 does not pull a
   write forward. It builds the outbound request path that ADR-0007 already required at
   Tier 5 and that no tier owned, and sends cmd 511 through it alongside cmd 512. Tier 8a
   keeps the operator-facing RPC, which is the part that genuinely needs a registry and an
   audit trail.
3. **`StreamRequestEnable: true`** — declined on the merits. It works: gomavlib substitutes
   4 Hz when `StreamRequestFrequency` is unset (`node.go:214`) and sends seven
   `REQUEST_DATA_STREAM` per ArduPilot heartbeat, re-sent every 30s. But `REQUEST_DATA_STREAM`
   (#66) is deprecated and is a *distinct message family*, so it would move the
   `len(SendFamilies) == 11` pin to admit a deprecated message permanently; it gives no
   per-message control, which makes the nine/three split in Chapter 7 inexpressible; and its
   confirmation event `EventStreamRequested` is one `internal/bridge/bridge.go` currently
   ignores, so it would be the only outbound path in the system with no observable trace.

**What this cost, and the cheaper thing that was available.** The defect was found by
running the system. It had also been found by reading it, on 2026-08-17, as finding N4 of
`tier-0-3-adversarial-review.md`, which closed with "Decide now and put the decision in
Tier 5's gate, so first hardware bring-up is not a surprise." It was not decided, and there
was no mechanism that would notice. A finding recorded in a review with no owner and no gate
behaves exactly like the unowned questions ADR-0009 §6 was written about.
