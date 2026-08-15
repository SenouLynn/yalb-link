# GCS Reference Architecture Analysis: QGroundControl & Mission Planner

## Purpose

Pre-implementation survey of two mature GCS implementations to extract proven patterns and identify antipatterns before designing our own stack. We are not replicating either — most functionality can be compared, derived, or reimplemented from these examples.

---

## QGroundControl

**Stack:** C++20, Qt6/QML, CMake, cross-platform (Windows/macOS/Linux/Android/iOS)

### Layer decomposition (what they got right)

```
LinkInterface (serial/UDP/TCP, per-thread)
  → MAVLinkProtocol (framing, CRC, routing)
    → MultiVehicleManager (vehicle discovery via HEARTBEAT)
      → Vehicle (domain model)
        → FirmwarePlugin (ArduPilot vs PX4 adapter)
          → QML UI
```

Clean transport → protocol → domain separation. Each link runs on its own thread.

### FirmwarePlugin — the pattern worth lifting

ArduPilot and PX4 share MAVLink but diverge on flight mode enumerations, command behaviors, and parameter naming. QGC isolates all firmware-specific logic behind a `FirmwarePlugin` interface. The `Vehicle` class stays clean. This is a textbook adapter. We want this.

### FactSystem — metadata-driven parameters

Parameters described as typed metadata (range, units, docs, enum values). UI is generated from the schema, not hardcoded. When an agent or operator asks about a parameter, the answer comes from the schema. We want this.

### What they got wrong

- **Singletons everywhere** — `MAVLinkProtocol`, `LinkManager`, `QGCApplication` are all global. Cannot mock, cannot unit test in isolation, cannot run multiple instances.
- **MAVLink not abstracted** — `Vehicle` assumes MAVLink at its core. Swapping protocols touches the domain.
- **Mutable signal-driven state** — `Vehicle` state updated in-place via Qt signals. No audit trail, no replay, hard to debug transitions.
- **Multi-vehicle bolted on** — `MultiVehicleManager` was added later. Fleet is a second-class citizen.

---

## Mission Planner

**Stack:** C#/.NET, WinForms, Windows-only

### CurrentState — the pattern worth lifting

Single aggregated projection per vehicle. All incoming telemetry streams (attitude, GPS, battery, mode) merge into one `CurrentState` object. All UI reads from it. The concept is right; the execution leaked into the UI layer. In our design this is an immutable snapshot replaced atomically, not mutated in place.

### Dual extensibility — worth noting

Plugins via `IPlugin` DLL interface + IronPython scripting. Two extension surfaces for two different audiences (developers vs. power users/agents). The scripting angle maps directly to our agentic inbound adapter.

### MAVLink code generation

Both QGC and MP generate typed message structs from the MAVLink XML definitions rather than handwriting them. We do the same — Bazel `genrule` reading `common.xml`, emitting Go structs.

### What they got wrong

- **Windows-only, WinForms** — platform lock-in from day one.
- **Multi-vehicle crashes** — GitHub issue #658. Fleet was never a design constraint.
- **God object** — `MAVLinkInterface.cs` handles framing, parsing, routing, retries, and state updates. No seams.
- **UI bleed** — WinForms event handlers call protocol code directly.

---

## Shared failure modes (both projects)

| Problem | Consequence | Our mitigation |
|---|---|---|
| Singletons | Untestable, un-mockable | Dependency injection; interfaces as ports |
| Mutable vehicle state | No audit trail, hard to debug | Immutable snapshots, event log |
| Protocol not abstracted | MAVLink baked into domain | Protocol handling isolated in adapters |
| Multi-vehicle afterthought | Fleet operations brittle | sysid is a first-class dimension from day 1 |
| UI coupled to domain | Can't test domain without runtime | Domain has no UI imports; inbound/outbound ports only |

---

## What both validate

Both converged on the same layer decomposition independently:

```
Transport → Frame codec → Message routing → Vehicle model → Domain/UI
```

Different names, same cuts. The architecture is correct.
