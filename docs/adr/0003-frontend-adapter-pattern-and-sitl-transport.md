# 0003 — Frontend Adapter Pattern and SITL Transport

**Status:** Accepted

## Context

The frontend must be modular and composable for two distinct reasons:

1. **Iteration speed** — UI units are built incrementally and re-composed without breaking unrelated panels. New telemetry views, map overlays, or command surfaces can be added without touching existing components.

2. **Adapter-driven testing** — each UI component must be testable against multiple data sources: production Connect streams, SITL-derived data, recorded telemetry replay, and pure mock generators. The component should not know or care which adapter is behind it.

This is the frontend expression of the same ports-and-adapters principle applied to the backend pipeline.

---

## Decision

### Frontend components are data-source-agnostic

Each component defines the shape of data it needs via a typed hook interface — its "port." The adapter that satisfies the port is injected at composition time, not hardcoded in the component.

```
┌─────────────────────────────────────────────┐
│  React Component                            │
│  (pure, renders from hook return value)     │
└────────────────┬────────────────────────────┘
                 │ useVehicleTelemetry(id)
                 ▼
┌─────────────────────────────────────────────┐
│  Adapter (injected via context/provider)    │
│                                             │
│  ConnectAdapter   → production backend      │
│  WebSocketAdapter → SITL test harness       │
│  MockAdapter      → unit tests / Storybook  │
│  ReplayAdapter    → recorded telemetry      │
└─────────────────────────────────────────────┘
```

A component that renders attitude, battery, and GPS position receives those values from a hook. Which adapter backs that hook — real Connect stream, SITL WebSocket, or static mock fixture — is determined by the provider wrapping the component tree. The component has no import from any adapter.

### WebSocket for the SITL test harness

WebSocket is retained as the transport for the SITL control and observation surface:

- **SITL is a test adapter, not a production concern** — it does not belong on the Connect API surface. Keeping it on a separate WebSocket endpoint enforces that boundary.
- **Test harnesses drive scenarios over WebSocket** — inject GPS positions, trigger failsafes, simulate packet loss. The backend SITL adapter exposes these controls without polluting the production proto schema.
- **Frontend integration tests swap the provider** — in CI, the `ConnectAdapter` is replaced with a `WebSocketAdapter` pointed at the SITL harness. The components under test are identical to production.

### Composition model

UI units are scoped to a single data stream or a closely related group of streams. A unit knows its data contract and nothing else. Composition happens at the layout/page layer, not inside components.

```
<FlightDashboard>
  <AttitudeIndicator />      ← subscribes to attitude stream
  <BatteryStatus />          ← subscribes to battery stream
  <MapOverlay vehicleId />   ← subscribes to position stream
  <CommandPanel vehicleId /> ← issues commands via command port
</FlightDashboard>
```

Each of these works standalone in Storybook against a `MockAdapter`. All of them together work in integration against `WebSocketAdapter` (SITL) or `ConnectAdapter` (production).

### State management

**React Query (TanStack Query)** is the single state layer for all server-derived data. The division of responsibility:

| Concern | Mechanism |
|---|---|
| Parameters, missions, vehicle list | React Query `useQuery` — fetch once, cache, invalidate on mutation |
| Command results | React Query `useMutation` — optimistic update, rollback on failure |
| Real-time telemetry (attitude, position, battery) | Connect server stream → `queryClient.setQueryData()` per vehicle |
| UI-only ephemeral state (panel visibility, map viewport) | `useState` / `useReducer` — local to the component that owns it |

This means every component reads from the same React Query cache regardless of how the data arrived — one-shot fetch or streaming update. No parallel stores, no sync problems.

**Redis layer (backend, not frontend)**

Redis sits between the MAVLink pipeline and the Connect API server on the backend. It is not a frontend concern but it shapes what the frontend receives:

- **Pub/Sub** — the MAVLink vehicle model publishes state updates to Redis channels; the Connect streaming handler subscribes and pushes to connected frontend clients. Multiple backend instances stay consistent without direct coupling.
- **Last-known state** — new frontend clients (page reload, second tab) receive current fleet state immediately from Redis rather than waiting for the next heartbeat cycle.
- **Redis Streams** — telemetry history stored as append-only streams (`XADD`). Enables the `ReplayAdapter` to replay a recorded flight by reading from the stream. Also the observation surface for SITL integration tests (assert vehicle state by reading the stream).
- **Redis is ephemeral** — fleet state is rebuilt from live MAVLink on reconnect. Redis is a cache and fan-out bus, not the system of record.

```
MAVLink pipeline → vehicle model → PUBLISH redis channel
                                         ↓
                               Connect API (SUBSCRIBE)
                                         ↓
                               React Query cache (setQueryData)
                                         ↓
                               React component (renders)
```

---

## Consequences

**Accepted costs:**
- Provider injection adds a layer of indirection that must be documented and enforced by convention
- Each component's hook interface is a contract that must be kept in sync with both the proto schema and mock fixtures
- More upfront structure than wiring components directly to a WebSocket

**Expected benefits:**
- Every component is independently testable in Storybook with zero backend dependency
- Integration tests run against SITL with no code changes to any component
- New data streams are added by writing a new hook and adapter implementation — existing components are untouched
- UI iteration is fast: swap the adapter, compose new layout, no plumbing changes
- The same composability that enables testing enables building operator-specific layouts (e.g., a simplified view for a specific vehicle type or mission profile)
