# 0004 — Multi-Protocol Track Layer

**Status:** Accepted

## Context

LoRa/Meshtastic is not a single-purpose protocol. It is a **carrier** that can simultaneously transport:
- MAVLink frames (vehicle telemetry and commands relayed over LoRa)
- Native Meshtastic payloads (position, device telemetry, environment sensors, text messages)
- Custom binary payloads (future extension)

Meshtastic nodes can be any entity type: aerial/ground/surface vehicles, human operators in the field, static sensor installations (weather, acoustic), or mesh infrastructure (routers, repeaters). The same mesh network that provides a long-range MAVLink link to a drone also carries position reports from field operators and temperature readings from a fixed sensor node.

ADS-B aircraft transponders, TAK CoT contacts, and future protocols (NMEA external GPS, custom sensors) present the same problem: multiple protocols producing positioned entities that all need to appear on the same situational awareness map.

The existing `VehicleId` / `FleetService` model is MAVLink-specific and cannot absorb these without polluting the MAVLink domain with protocol-agnostic concerns.

---

## Decision

### Two-layer fleet model

**Track layer** (protocol-agnostic) — used by the map and SA display:
- Every positioned entity becomes a `Track` regardless of origin protocol
- `NodeId` (id + `NodeProtocol` discriminator) identifies entities at this layer
- All adapters normalise their domain events into `Track` before publishing to Redis
- `TrackService.WatchTracks` is the map layer's only data source; it has no protocol imports
- Filtering by protocol, type, or node ID is done at the subscription level

**MAVLink domain layer** (protocol-specific) — used by the operator panel:
- `VehicleId`, `TelemetryService`, `CommandService`, `ParameterService`, `MissionService`, `CalibrationService` remain MAVLink-specific
- `MavlinkDetail` in a `Track` carries a `VehicleId` so the map can link to the operator panel for a selected vehicle
- No cross-contamination: the track layer does not import MAVLink types; the MAVLink services do not import track types

### LoRa/Meshtastic as a multiplexed transport adapter

The Meshtastic adapter is a **multiplexer**, not a single-protocol handler:

```
Meshtastic radio connection
    ↓ decode MeshPacket
    ├── PortNum 1  (TEXT_MESSAGE)  → ChatService (Redis pub/sub)
    ├── PortNum 3  (POSITION)      → TrackService (Track update)
    ├── PortNum 67 (TELEMETRY)     → TrackService (MeshtasticDetail update)
    └── PortNum N  (custom/MAVLink)→ MAVLink transport adapter (byte stream)
```

When a Meshtastic node is relaying MAVLink frames:
- It appears as a `Track` with `MeshtasticDetail.is_mavlink_relay = true`
- It also appears as a `LinkStatus` entry in `FleetService` with `uri: "meshtastic://<node_id>"`
- The vehicle behind it appears as a separate `Track` via the MAVLink adapter
- A single vehicle may be reachable on multiple links simultaneously; the multi-link manager selects primary

### Chat as a cross-protocol Redis bus

All text messaging is normalised into `ChatMessage` (with `ChatOrigin` discriminator) and published to Redis pub/sub channels. Sources:
- GCS operator (typed in UI)
- Meshtastic text messages (PortNum 1)
- TAK CoT GeoChat
- System alerts from GCS backend

Chat history is stored in Redis Streams (`chat:history:<channel>`) for replay. New clients receive history before live messages.

### ADS-B

Produced by either:
1. MAVLink `ADSB_VEHICLE` (#246) — received from an onboard detect-and-avoid system
2. External ADS-B receiver adapter (dump1090/Beast format over TCP)

Both paths produce a `Track` with `AdsbDetail` and `TrackType.AIRCRAFT`. The source adapter is transparent to the map.

### Redis as the normalisation bus

All adapters publish to the same Redis structures regardless of protocol:
- `PUBLISH track:events` — `TrackEvent` for live stream subscribers
- `HSET track:state:<node_id>` — last-known track for cold-start
- `XADD track:history` — append-only telemetry log for replay
- `PUBLISH chat:<channel>` — real-time chat fan-out
- `XADD chat:history:<channel>` — persistent message history

---

## New services

| Service | Purpose |
|---|---|
| `TrackService` | Map layer — all positioned entities, all protocols |
| `MeshService` | Meshtastic node management, raw packet access, relay config |
| `ChatService` | Cross-protocol text messaging with Redis-backed history |

Existing MAVLink services (`FleetService`, `TelemetryService`, `CommandService`, etc.) are unchanged.

---

## Consequences

**Accepted costs:**
- Two identity systems coexist: `VehicleId` (MAVLink) and `NodeId` (track layer). Cross-referencing requires the `MavlinkDetail.vehicle_id` field.
- The Meshtastic adapter is the most complex adapter: it multiplexes one connection into three dispatch paths (chat, track, MAVLink transport).
- ADS-B from two sources (MAVLink + external receiver) may produce duplicate tracks for the same aircraft; deduplication by ICAO address required at the track layer.

**Expected benefits:**
- The map has zero protocol dependencies — adding a new protocol requires only a new adapter and does not touch the frontend
- Chat, position, and telemetry from all protocols flow through Redis with the same key schema — replay, logging, and test harness work uniformly
- Meshtastic's native protobuf format aligns with our stack; adapter code is minimal
- Fleet management scales naturally: a 20-vehicle MAVLink fleet, 15 field operators on Meshtastic, and ADS-B airspace awareness all appear on the same map without architectural changes
