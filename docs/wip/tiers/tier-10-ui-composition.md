# Tier 10 — UI Composition

## Overview

Instrument composition after telemetry data sources are proven end-to-end. The pattern is module-first: smallest trusted atomic unit, then instruments, then composed layouts. No instrument is scheduled until its resolver is validated at the logic layer and its data source is proven live. MapPanel and FleetView are already built in Tier 8.

## Dependencies

- All Tier 8 write transactions complete
- All Tier 1 resolvers proven with known-answer tests
- Live SITL data confirming each instrument's data source

## Chapters

---

### Chapter 1: Base Component Modules

**Goal:** The atomic display primitives used across all instruments and panels.

**Components:**

| Component | Data | Notes |
|---|---|---|
| `TelemetryValue` | name, value, unit, source, age | Resolver returns non-null with named source; displays "—" when null |
| `VehicleChip` | sysId, type icon, armed/mode, freshness dot | Data from HeartbeatState |
| `CommandButton` | label, eligibility gate, loading state, result | Disabled when gate unmet; loading spinner during in-flight command |
| `SourceBadge` | primary / fallback / missing + message name | Color-coded; shows full MAVLink message name on hover |
| `FreshnessRing` | last-seen age as fill or color | Uses `freshness.ts` directly |

**Rule:** These components are display-only. They take flat props — no hooks, no async inside. Hooks live in parent containers.

---

### Chapter 2: Trusted Instrument Tier

**Goal:** One instrument per resolver. Each instrument is independently deployable and independently testable.

**Instruments:**

| Instrument | Resolver | Prerequisite |
|---|---|---|
| `AttitudeIndicator` | `resolveAttitude` | Pitch ladder + roll arc SVG; validated against SITL attitude stream |
| `HeadingIndicator` | `resolveHeading` | Compass rose or tape; fallback chain visible via SourceBadge |
| `FlightStateDisplay` | `resolveFlightPath2d` | Speed, track, climb rate, FPA; NED sign verified live |
| `PredictiveTrajectory` | `resolvePredictiveTrajectory` | Overlaid on MapPanel; vehicle-type-conditional stall gate |
| `FlightPathRecorder` | `accumulateTrack` | Track polyline (already in MapPanel from Tier 8); this instrument adds HUD overlay |
| `AltitudeTape` | `resolvePosition.altM` | Vertical tape or digital readout |

**Each instrument must:**
- Accept `TelemetrySample` as prop (not raw proto)
- Show `SourceBadge` when not using primary source
- Show `FreshnessRing` when data is stale
- Return a visually empty state (not crash) when resolver returns null

---

### Chapter 3: Composed Layout Tier

**Goal:** Assemble instruments into operational display layouts.

**Layouts:**

| Layout | Composes | Schedule |
|---|---|---|
| `PrimaryFlightDisplay` | `AttitudeIndicator` + `HeadingIndicator` | After both instruments pass live SITL |
| `HudPanel` | PFD + `FlightStateDisplay` + `PredictiveTrajectory` | After all PFD instruments validated |
| `OperatorPanel` | `VehicleChip` + `CommandButton` set + `ParametersPanel` + `MissionPanel` | After Tier 8 complete |
| `NodeView` | `HudPanel` + `OperatorPanel` + `MapPanel` | After all above |

**`NodeView`** is the full single-vehicle operator view. It is the composition target — everything feeds into it.

**`FleetView`** (already built in Tier 8) is the multi-vehicle overview. In Tier 10, it gains the ability to expand into `NodeView` when a vehicle is selected.

---

### Chapter 4: Regression Gate

**Goal:** Confirm that composition does not introduce regressions in instruments that were already working.

**Checks:**
- Each instrument renders correctly in isolation with MockAdapter (no live SITL required)
- Composed layouts render without errors when any single data source is missing or stale
- No resolver is called with undefined props (TypeScript strict null checks must pass)
- `pnpm tsc --noEmit` passes
- All Tier 1 vitest tests still pass

---

## Tier Exit Gate

- All six instruments render live SITL data correctly
- NED sign flip visually confirmed: descending vehicle shows negative climb rate
- Stall gate visually confirmed: copter in hover shows trajectory cone; plane below stall speed shows no cone
- `NodeView` renders with all panels populated from live ArduCopter SITL
- All instruments handle null resolver output gracefully (visually empty, not crashed)
- `FleetView` + vehicle select → `NodeView` transition works
- MockAdapter renders full `NodeView` with fixture data (no network required for demos)
