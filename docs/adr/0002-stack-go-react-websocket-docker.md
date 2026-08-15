# 0002 — Stack: Go Backend, React Frontend, Connect (Buf), Docker

**Status:** Accepted

## Context

The GCS requires:
- A concurrent pipeline handling multiple MAVLink transport links, service protocols, and vehicle state machines simultaneously
- A TAK-style situational awareness UI with real-time data-stream-coupled components, geospatial map layers, and composable, incrementally-built panels
- Fleet management and multi-node/multi-protocol capabilities from day one — not bolted on later
- Platform agnosticism (Linux, macOS, Windows)
- SITL integration from day one for hermetic testing without hardware
- Agentic inbound adapters — autonomous agents driving the domain through the same interface as a human operator

Reference implementations (QGroundControl, Mission Planner) were surveyed. Neither is being replicated; patterns are derived and failures are explicitly avoided. See `docs/research/qgc-missionplanner-analysis.md`.

---

## Decision

### Backend: Go

Go's goroutine/channel concurrency model maps directly to the hexagonal pipeline:

```
link goroutine      → chan []byte     → frame codec goroutine
frame codec         → chan Frame      → message codec goroutine
message codec       → chan Message    → router (select loop)
router              → dispatch        → service protocol goroutines (one each)
vehicle model       → atomic snapshot → outbound adapters
```

Each layer boundary is a typed channel — the port. Each goroutine is replaceable with a mock for testing. Context cancellation provides clean shutdown. Go interfaces are small and composable, making ports explicit and easy to satisfy with test doubles.

Go was chosen over Elixir (also a strong fit via OTP/GenServer) due to familiarity and MAVLink ecosystem availability.

### Frontend: React

React is chosen over Vue and Electron for the initial implementation:

- **Ecosystem** — Mapbox GL JS, Deck.gl, and Cesium own the geospatial space; they have first-class React bindings
- **Explicit data flow** — Vue's magic proxy reactivity obscures which streams drive which components; in a system where isolation and idempotency matter, explicit is better
- **Composability** — component model fits the incremental, scoped UI units design; each component subscribes to exactly the data streams it needs
- **Electron is a deployment wrapper, not an architecture** — ship as a web app first; wrap in Electron later if desktop filesystem access or offline capability is needed

State management: **React Query (TanStack Query)** for all server-derived state — parameters, missions, vehicle snapshots, command results. It owns the cache lifecycle, loading and error states, and stale-while-revalidate behavior for slow-changing data. Real-time telemetry streams feed into the React Query cache via `queryClient.setQueryData()` so components always read from one consistent store regardless of whether data arrived from a one-shot fetch or a Connect stream. See ADR-0003 for detail.

### Transport between backend and frontend: Connect (Buf)

Connect is chosen over plain WebSocket and gRPC-Web:

- **Typed contracts** — `.proto` files are the source of truth; `rules_buf` in Bazel generates Go server stubs and TypeScript client code from the same schema. No manual type sync across the boundary.
- **No proxy required** — unlike gRPC-Web, Connect speaks plain HTTP/1.1 or HTTP/2 natively in the browser. No Envoy, no sidecar.
- **First-class streaming** — server streaming maps directly to telemetry push (vehicle state, telemetry events, fleet updates). The React frontend subscribes to typed streams.
- **Schema evolution is enforced** — protobuf backward-compatibility rules prevent silent API drift.
- **Bazel integration is clean** — `rules_buf` + `buf.gen.yaml` fits naturally alongside the MAVLink XML → Go codegen pipeline.

WebSocket is retained for the SITL test harness specifically — see ADR-0003.

### Deployment: Docker

```
docker-compose (local dev / CI)
  ardupilot-sitl   ← MAVLink UDP:14550
  redis            ← :6379  (fleet state, telemetry pub/sub, replay streams)
  gcs-backend      ← Connect (HTTP) :8080  /  WebSocket :8081 (SITL harness)
  gcs-frontend     ← HTTP :3000
```

SITL as a container from day one means integration tests are `docker-compose up && bazel test //...` with no hardware dependency anywhere in the loop. This is the hermetic checkpoint boundary described in ADR-0001.

Platform agnosticism is achieved through Docker — the same compose file runs on Linux, macOS, and Windows without accommodation in application code.

### North star reference: TAK

Team Awareness Kit (TAK) is the architectural reference for the UI and federation model:
- Each UI "unit" is an independent component with its own data subscription (tracks, video, telemetry, chat)
- Multi-node federation: multiple backend instances relay vehicle state; clients subscribe to filtered streams
- The key divergence from TAK: we add *commanding* — bidirectional stateful exchanges with acks and retries — which is what makes the service protocol layer non-trivial

### WASM

Deferred. Potential future use: offloading the MAVLink frame codec to the browser for a zero-backend deployment mode. Not in scope until the core pipeline is stable.

---

## Consequences

**Accepted costs:**
- Go requires explicit supervision patterns (no OTP); goroutine lifecycle management is manual
- Connect + protobuf adds codegen step and `.proto` discipline; schema must be kept in sync with MAVLink semantics
- React adds JS ecosystem complexity (node_modules, bundler, TypeScript config)
- Redis adds an infrastructure dependency; must be treated as ephemeral (fleet state is rebuilt from live MAVLink, not persisted across restarts as ground truth)

**Expected benefits:**
- Goroutine-per-layer maps cleanly to ports and adapters; each layer is independently testable
- Docker compose gives hermetic SITL integration from commit one
- React + geospatial libraries give TAK-style map UI without building primitives
- Platform agnosticism requires no per-OS code paths
- Fleet and multi-protocol are first-class from the first line of code
