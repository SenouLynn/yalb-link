# Tier 9 — Additional Protocol Adapters

## Overview

The MAVLink track path is already live from Tier 6. This tier adds ADS-B and Meshtastic adapters that normalize to the same `Track` proto, making the map protocol-agnostic. Both require hardware or external services; they gate on availability, not on earlier tiers completing.

## Dependencies

- Tier 6 track layer live (the `Track` proto, `track:events` stream, and TrackService are the stable interface)
- ADS-B: hardware receiver (e.g., RTL-SDR + dump1090) or MAVLink ADSB_VEHICLE feed from vehicle
- Meshtastic: Meshtastic node accessible over TCP/serial

## Chapters

---

### Chapter 1: ADS-B Adapter

**Goal:** Normalize ADS-B aircraft to `Track` protos and publish to `track:events`.

**Two source paths:**

**Source A — MAVLink ADSB_VEHICLE (#246):**
- Already arriving on the MAVLink receive path if the vehicle has an onboard detect-and-avoid module
- Add `ADSB_VEHICLE` to the decode switch in the codec (it is already in the ardupilotmega dialect)
- Normalize to `Track` with `source = TRACK_SOURCE_ADSB` and `AdsbDetail` populated (ICAO, callsign, squawk, emitter category)

**Source B — dump1090 Beast protocol over TCP:**
- Connect to `dump1090` at configured TCP address
- Parse Beast binary frames (Mode S/ADS-B)
- Normalize to same `Track` proto format

**Deduplication:** when both sources report the same ICAO address, deduplicate by ICAO. Last-writer-wins with a 1s debounce window is sufficient for ground station use.

**Track nodeId format:** `adsb-<icao_hex>` — stable across reconnects.

---

### Chapter 2: Meshtastic Adapter

**Goal:** Demultiplex Meshtastic mesh packets into three separate data paths.

**Connection:** single TCP or serial connection to a Meshtastic node.

**Demux by PortNum:**

| PortNum | Value | Path |
|---|---|---|
| TEXT_MESSAGE_APP | 1 | → ChatService |
| POSITION_APP | 3 | → TrackService (node position) |
| TELEMETRY_APP | 67 | → TrackService (device telemetry, battery) |
| Custom (256+) | negotiated | → MAVLink transport adapter (relay path) |

**`Track` output for POSITION_APP:**
- `source = TRACK_SOURCE_MESHTASTIC`
- `MeshtasticDetail`: node ID, SNR, hop count, long name, short name
- `is_mavlink_relay`: true if the node is configured as a MAVLink relay

**Track nodeId format:** `mesh-<node_id_hex>`

**ChatService:** out of scope for this tier — stub the interface; implement in a later milestone.

**MAVLink relay path:** when a node is a relay (carrying MAVLink over Meshtastic radio), its packets feed into the MAVLink transport adapter chain. This is deployment-negotiated; document the PortNum used in `docker-compose.yml` as an env var.

---

### Chapter 3: Map Protocol Isolation Verification

**Goal:** Confirm that the map renders all three source types without coupling to any protocol package.

**Test:** Run three simultaneous sources (MAVLink SITL, ADS-B mock, Meshtastic mock) and verify:
- All three appear as distinct markers on MapPanel
- Removing one source does not affect the others
- Adding a new adapter requires: new proto detail variant, new adapter file, zero map code changes

---

## Tier Exit Gate

- ADS-B adapter normalizes ICAO aircraft to `Track` proto; visible on MapPanel
- Meshtastic position nodes appear as `Track` entries on MapPanel
- ICAO deduplication works when both MAVLink and dump1090 report the same aircraft
- Map code imports zero MAVLink, ADS-B, or Meshtastic packages
- Protocol-agnostic verification: three source types visible simultaneously on map
