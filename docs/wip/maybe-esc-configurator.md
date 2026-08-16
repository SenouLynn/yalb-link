# ESC Configurator — Consideration Doc

**Status:** Active consideration — standalone-first approach  
**Decision:** Build standalone, decouple entirely from GCS transport, integrate transport linkages later

---

## Core Constraint (Discovered in Testing)

The FC cannot simultaneously act as a flight controller and serve ESC configuration passthrough. When ArduPilot enters BLHeli passthrough mode, it hands the serial connection to the ESC and drops normal FC behavior. This means ESC configuration and active GCS use are mutually exclusive by nature — a standalone tool is the architecturally honest answer, not an embedded panel.

---

## What This Is

A standalone ESC parameter configurator for BLHeli_S, BLHeli_32, AM32, and BlueJay ESCs. Separate lifecycle from the GCS — user closes/disconnects GCS, opens ESC configurator, configures ESCs, returns to GCS.

Typical workflow: open configurator → connect to FC via USB serial → activate ESC passthrough → read ESC parameters → edit (motor direction, timing advance, demag, brake-on-stop, etc.) → write → verify → disconnect → reopen GCS.

---

## Standalone Architecture

```
Browser (standalone React app)
        │  Web Serial API (direct, no backend)
        ▼
FC via USB ─── BLHeli 4-way interface ──► ESC (BLHeli_32 / AM32)
```

No Go backend. No MAVLink tunnel. No GCS dependency. The browser talks directly to the FC's USB serial port via the Web Serial API, and the FC's `AP_BLHeli` passthrough forwards raw bytes to the ESC. All protocol logic lives in the React app.

This also means: **no prerequisite on the GCS MAVLink stack**. Can be built right now.

---

## Open-Source Baseline

**github.com/mathiasvr/esc-configurator** — MIT licensed.

- Stack: Vanilla JS + Web Serial API (Chrome/Edge)
- Coverage: BLHeli_S, BLHeli_32, AM32, BlueJay
- What's reusable: all protocol decoders, EEPROM layout tables (firmware-version-keyed), parameter schemas, flash verification logic, UI patterns

Options for "owning" this:
1. **Fork and modernize** — take the MIT codebase, rewrite in React/TypeScript, keep the protocol layer verbatim
2. **Port protocols, build fresh UI** — extract decoder logic, write a clean React/Vite app from scratch with our own design language
3. **Thin wrapper/fork** — minimal changes to the existing vanilla JS app, just brand and host it

Option 2 is recommended: the protocol decoders are the hard part and the MIT code gives them to us; the UI is simple enough to own cleanly in React/TypeScript to match the broader stack.

---

## Effort Estimate (Standalone)

| Layer | Effort |
|---|---|
| React/Vite project scaffold | ~0.5 day |
| Web Serial API transport wrapper | ~1 day |
| BLHeli/AM32 4-way protocol decoders (ported to TypeScript from MIT source) | ~3–4 days |
| EEPROM layout tables (version-keyed, borrowed from esc-configurator) | ~1–2 days |
| Parameter read/write UI | ~2–3 days |
| Multi-ESC detection and selection | ~1 day |
| **Phase A total (params only)** | **~1.5–2 weeks** |
| Firmware flash support | +3–4 weeks (Phase B) |

Significantly simpler than the embedded approach — no Go backend, no MAVLink tunnel, no proto changes.

---

## What's Still Hard

1. **Hardware-only testing.** No SITL equivalent for ESCs. Must test against real BLHeli_32 or AM32 hardware.
2. **Firmware versioning.** EEPROM layouts differ by firmware version. The esc-configurator lookup tables cover this; we inherit them.
3. **Flash operations are destructive.** Phase B — a botched flash bricks an ESC. Keep this out of Phase A entirely.
4. **Web Serial is Chrome/Edge only.** Not a blocker for field use (this is a tech-forward tool), but worth noting. Electron wrapping resolves this if needed.

---

## Transport Linkage (Future, Phase C)

Once the GCS MAVLink stack is functional (Tier 5 of order-of-operations-v1), we can revisit integrating the ESC configurator as a GCS panel. At that point, the transport swaps:

```
Before: Browser → Web Serial API → FC USB
After:  Browser → WebSocket → Go backend → MAVLink SERIAL_CONTROL → FC → ESC
```

The protocol decoder layer (TypeScript) is reusable unchanged. Only the transport adapter changes. Designing the standalone version with a clean `SerialTransport` interface abstraction now makes Phase C a swap, not a rewrite.

---

## Recommended Next Step

Scaffold a `tools/esc-configurator/` workspace in this repo (or a sibling repo). React + Vite + TypeScript + Web Serial API. No shared dependencies with `frontend/` — separate build, separate dev server, separate deployment. Start with the 4-way protocol decoder layer and a basic connect/read-params flow.
