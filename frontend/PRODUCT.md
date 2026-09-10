# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

A solo builder-operator — the same person writes the code and flies the
aircraft. Today the setting is a desk: indoors, SITL or bench hardware, mouse
and keyboard, a large screen, controlled light. Field operation on a laptop is
expected later, so the design must stay field-survivable (contrast and
hit-target sizes that hold up in sunlight, with gloves, one-handed) without
paying for that today at the desk.

A second, distinct audience uses the same screen: the developer, who needs to
see feed internals and raw wire values that an operator must never be shown.
See Capabilities and Constraints — this is a two-tier requirement, not a
persona nicety.

## Product Purpose

A ground control station for MAVLink vehicles: receive telemetry, fold it into
per-vehicle state, and present flight data an operator can act on. Success is
an operator who can answer "what is out there, where is it, and is it healthy"
without assembling the answer themselves, and who can command a vehicle through
a path that tells them whether the command landed.

## Positioning

Fleet-first multi-node command and control. The fleet picture is the outer view
and a single vehicle is the detail reached from it — not a single-vehicle app
with a fleet list bolted on. The question the interface answers first is
contextual: where is IT in relation to ME, or to THAT. MissionPlanner and
QGroundControl are both better single-vehicle configurators and neither is
built around this, which is the whole reason this exists.

## Operating Context

- Desk-bound today: SITL (Copter and Plane via Docker Compose), bench hardware,
  recorded-flight replay. Field-bound later.
- One viewport, no page scroll. Panels scroll internally; a body scrollbar
  narrows the viewport, resizes every column, and makes the map jump.
- Three interchangeable data sources drive the same display: live SSE
  (`/api/events`), deterministic fixtures (`?source=mock`), and paged replay of
  a recording (`?source=replay&recording=<id>`). Any surface must be presentable
  under all three.
- The UI is unsettled by intent. The final shape of the screen is not yet known,
  so the ability to add and remove information is a working requirement of this
  stage of development, not only a shipped feature.

## Capabilities and Constraints

**Confirmed capabilities**

- Per-vehicle fold over 13 MAVLink fleet/telemetry families, with deterministic
  discovery, loss, recovery, and freshness semantics.
- Fleet roster and fleet map; per-vehicle detail with artificial horizon,
  heading tape, altitude with datum, speed, climb, power, link health, and a
  MapLibre position/track map with a freshness-gated five-second prediction.
- Every reading carries provenance: the MAVLink message it came from and how old
  that message is, drained visibly against a TTL. This is an existing property
  of the display and future work preserves it, though it is not the product's
  positioning claim.
- Guarded arm/disarm as an addressed, acknowledged transaction, with timeout,
  cancellation, delivery-uncertain states, and an operator-attested resolution
  for ambiguous outcomes behind a bounded stale-ACK quarantine.
- Read-only onboard mission download; bounded SQLite recording with explicit
  lifecycle and deterministic replay.

**Panel visibility is a two-tier capability.** Operators toggle real panels.
Developers additionally get a debug/inspect tier — raw wire values, feed
internals, decoder state — that operators never see. These are two different
audiences with two different information ceilings on one screen, and the
distinction must survive into the component and state design.

**Architectural constraints (standing instruction to the codebase)**

- Components stay modular, state stays modular, logic stays isolated, so
  anything can be composed at will.
- Backend integration is relegated to a layer the front end does not need to
  care about.
- Some higher-level component abstractions pre-compose information that
  categorically belongs together. These are the operator's real units of
  attention and the owner attends to them directly.

**Undecided**

- Whether operator panel/layout choices persist across reload.
- The final composition of the screen.

## Brand Commitments

Name: YALB-GCS.

Binding references, stated by the owner:

- **DearImGui** (the C++ immediate-mode GUI library) is the design-philosophy
  reference. Its priority on immediate access to data and to important levers
  matches the owner's own.
- **`~/Desktop/flight-path-hud`** is this project's direct ancestor and the
  source of design authority. Considerable UI consideration was done there.
  Two designed artifacts exist in it: the `hud-ui` instrument package and the
  `apps/gcs` application shell.
- **The current post-port UI in this repository is explicitly not a reference.**
  The owner's instruction is zero design inspiration from it. The instrument
  palette in `src/ui/display.css` is inherited ancestor work and is not covered
  by that exclusion; the workspace, pane, and fleet chrome written after the
  port is what the exclusion is about.

## Evidence on Hand

- Deterministic mock fixtures (`?source=mock`) and recorded-flight replay — a
  real, driveable display without hardware.
- SITL harnesses for Copter and Plane.
- 14 Storybook stories covering the existing component vocabulary.
- Wire/semantic contracts and generated protobuf types under `src/gen/`.
- Ancestor implementation at `~/Desktop/flight-path-hud`
  (`apps/gcs/src/index.css`, `packages/hud-ui/src/hud.css`).
- No real-world field flight logs, customers, users other than the owner, or
  deployment history. Future work must not imply any.

## Product Principles

1. **Fleet before vehicle.** The outer question is what is out there and where;
   a single vehicle is a detail view reached from that answer.
2. **Data and levers within reach.** This is a monitoring, command, and control
   surface. Burying a value or a control one interaction deeper is a cost paid
   every flight, not a tidiness win.
3. **Say where it came from and how old it is.** A number without provenance is
   not trustworthy enough to fly on.
4. **Compose, don't hardcode.** The screen's final shape is unknown, so
   information must be addable and removable — and operator reach and developer
   reach are different ceilings on the same screen.
5. **Commanding is exceptional.** Observation is the baseline; every write path
   announces its own uncertainty rather than pretending to succeed.

## Accessibility & Inclusion

No formal standard has been set. One product-specific requirement is confirmed:
because field operation on a laptop is expected, contrast ratios and
hit-target sizes must remain field-survivable — legible in direct sunlight,
operable with gloves — even while the design optimizes for a desk today.
