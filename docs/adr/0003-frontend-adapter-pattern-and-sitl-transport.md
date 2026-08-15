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

- **Zustand** for local UI state (panel open/closed, selected vehicle, map viewport)
- **Connect streaming hooks** (`useServerStream`) for telemetry — data flows into components, not into a global store
- Telemetry is not stored globally; it is consumed at the point of rendering. Persistence (logs, replay) is a backend concern, not a frontend store concern.

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
