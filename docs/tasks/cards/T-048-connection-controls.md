---
id: T-048
title: Choose connections and diagnose acquisition from the UI
status: done
priority: 1
owner: unassigned
depends_on: T-046, T-047
---

## Motivation and evidence

The current LIVE indicator establishes browser stream connectivity, not serial
access or aircraft reachability. T-045/T-046/T-047 delivered a complete,
tested backend surface — the ten-plus-`AMBIGUOUS` state machine, the
`acquisition` SSE event and the `/api/connections[...]` HTTP API in
[the connection contract](../connection-contract.md) — with no frontend
consumer yet: `frontend/src/stream/events.ts` has no `acquisition`
`StreamEvent` variant, and nothing in `frontend/src` calls
`/api/connections*`. This card is pure frontend work; no backend or contract
change is in scope.

Required by [ADR 0006](../../adr/0006-operator-connection-readiness.md).

## Outcome

An operator can select/save/connect/release a device and understand
acquisition status from an empty fleet or vehicle workspace, using the
existing backend contract and UI primitives.

## Scope

**In:**

- A typed client module (e.g. `frontend/src/connections/api.ts`) for
  `GET /api/connections`, `GET /api/connections/devices`,
  `POST /api/connections/connect`, `POST /api/connections/disconnect`,
  `GET/POST /api/connections/profiles`, `DELETE /api/connections/profiles/{id}`,
  following `frontend/src/commands/arm.ts`'s strict-JSON-body/typed-HTTP-error
  convention (§7 and §11 of the contract for status codes and error shapes).
- The `acquisition` `StreamEvent` variant in `frontend/src/stream/events.ts`
  (§6) and its case in `frontend/src/stream/parse.ts` — plain JSON per the
  contract's snake-case wire fields (`state`, `device`, `settings`,
  `opened_at_ms`, `last_frame_at_ms`, `vehicle_keys`, `detailed_error`), not a
  protobuf schema like the existing `fleet`/`telemetry`/`command` cases.
- A connection state fold, keyed by connection id and independent of
  `frontend/src/fleet/state.ts`'s vehicle fold, bootstrapped from
  `GET /api/connections` and kept current from `acquisition` events (late
  subscriber gets current state, matching `FleetEvent`/`TelemetryEvent`
  bootstrap behavior already relied on elsewhere).
- A connection-controls surface (device/profile picker, connect/disconnect,
  save/delete profile) built from existing primitives
  (`Field`/`Lever`/`Group`/`Row`/`Chip`/`Note`/`Confirm` in
  `frontend/src/ui/primitives.tsx`) reachable both from `FleetOverview`'s rail
  with zero discovered vehicles, and from the vehicle workspace via a new
  `connections` entry in `frontend/src/workspace/registry.ts`'s `vehicle`
  section.
- A compact, persistent connection summary distinct from `LinkRows`'s existing
  "Source" row (`frontend/src/ui/StatusBar.tsx`), which continues to report
  only browser/backend stream connectivity.
- Mock fixtures for every §8 state plus representative connect failures
  (403/404/409/500), usable by tests and a new
  `frontend/stories/Connections.stories.tsx` Storybook entry.

**Out:** any backend/contract change, T-049's recovery/retry-timing behavior,
and real hardware acceptance (T-028/T-031/T-032).

## Acceptance criteria

- [x] Connection controls are reachable with zero discovered vehicles (from
      `FleetOverview`) and from vehicle detail, without requiring a selected
      vehicle to exist.
- [x] Device and profile choices show the identifying metadata the backend
      returns (`description`, `serial_number`, `manufacturer`, `vid`, `pid`)
      and re-render when `GET /api/connections/devices` changes; nothing is
      labeled a radio or a specific peripheral beyond what the backend reports.
- [x] Every `Status.State` in the contract's §8 table (`DEVICE_MISSING`,
      `AMBIGUOUS`, `IDLE`, `OPENING`, `ACCESS_FAILED`,
      `OPEN_AWAITING_TRAFFIC`, `REPORTING`, `INTERRUPTED`, `DEVICE_LOST`,
      `TRANSPORT_FAILED`, `RELEASED`) renders a distinct, supported label and
      tone — no two states collapse to the same operator-visible text, and
      `OPEN_AWAITING_TRAFFIC` renders as a normal resting state, not a warning.
- [x] `DEVICE_MISSING` and `AMBIGUOUS` present an explicit selection action
      (never an automatic choice); `ACCESS_FAILED`/`TRANSPORT_FAILED` and a
      409/404 connect response render the backend's own `detailed_error`/error
      text verbatim, never a guessed cause (no invented baud/wrong-device/
      powered-off claims, matching §11).
- [x] Disconnect is reachable from both surfaces, calls
      `POST /api/connections/disconnect`, and the UI stops presenting the
      connection as auto-retrying afterward (`RELEASED`) until the operator
      reconnects — recognizable as the MissionPlanner hand-off path from
      ADR 0006.
- [x] Save/update/delete profile round-trips through
      `GET/POST/DELETE /api/connections/profiles` and a saved profile survives
      a page reload (re-fetched from the backend, not cached client-side).
- [x] Connection-controls state and vehicle/fleet state are demonstrably
      independent both ways: selecting a different vehicle, or a vehicle
      transitioning `VEHICLE_LOST`/`VEHICLE_DISCOVERED`, leaves connection
      state unchanged, and connecting/disconnecting/reselecting a device does
      not alter unrelated vehicle fold state.
- [x] Operator-tier connection status requires no developer-tier panel to
      interpret (plain state labels, not raw wire values).

## Verification

```sh
cd frontend
pnpm test -- src/connections src/stream src/workspace src/fleet
pnpm typecheck
pnpm lint
pnpm build-storybook
```

Add fixture-driven tests for the state fold (bootstrap-from-list, late
subscriber, out-of-order/duplicate `acquisition` events) and for the HTTP
client (mirrors `frontend/src/commands/arm.test.ts`'s strict-body and
typed-error-decoding style, covering 403/404/409/500). Cover the controls
surface for each §8 state, both reachability points (empty fleet, vehicle
detail), and the profile CRUD round trip using mock fixtures — no real backend
required for these. Where browser tooling is available, exercise the empty
fleet and vehicle-detail surfaces with keyboard-only navigation and check the
built Storybook entries render in headless Chrome without runtime exceptions
(same procedure as T-038's acceptance).

Separately, exercise `GET/POST /api/connections*` against a real backend
(SITL or the T-046 PTY harness) for at least one connect → reporting →
disconnect cycle and one `AMBIGUOUS`/`DEVICE_MISSING` selection, to confirm
the frontend's assumptions about response shape match the live backend, not
only its own fixtures. Record this pass and any mismatches found in the
Notes; it is exploratory confirmation, not a substitute for the fixture-driven
suite above.

Finish with `./scripts/kanban check`.

## Open questions

Exact visual composition (dialog vs. inline panel, tab vs. accordion) follows
the established panel system and is an implementation choice, not a blocker.
Whether the compact summary lives inside `LinkRows` or as a sibling panel is
similarly open — either satisfies the acceptance criteria above as long as the
two facts (browser connectivity vs. acquisition state) stay visually distinct.

## Notes

Shaped to ready 2026-09-23, after T-046 and T-047 landed the full backend
surface this depends on.

**Implemented and verified 2026-09-23.**

`frontend/src/connections/` (new): `types.ts` (wire types + defensive
`parse*` functions, null-on-anything-unrecognised like `stream/parse.ts`),
`api.ts` (typed client + `ConnectionHTTPError`, mirroring
`commands/arm.ts`'s convention), `state.ts` (the id-keyed fold, independent
of `fleet/state.ts`), `labels.ts` (the one state→label/tone table), `useConnections.ts`
(the controller hook: fold + device/profile inventories + actions +
per-id `pending`), `ConnectionsBar.tsx` (compact `Glance` + popover, built
from existing `Field`/`Lever`/`Group`/`Row`/`Note`/`Glance` primitives — the
scope list's `Chip`/`Confirm` weren't needed), and `fixtures.ts` (one status
per §8 state, for tests and Storybook). `stream/events.ts`/`parse.ts` gained
the `acquisition` variant as plain JSON, per §6. `system.css` gained
`.connections`/`.connections__popover` (no `box-shadow`: `system.test.ts`'s
Only-Overlays-Float gate permits exactly two existing uses, and this isn't
either — a border against `--panel-deep` reads fine without one).

**Composition deviated from the scope note, discovered while building, not
guessed up front:** `workspace/registry.ts`'s panel system only renders
inside `VehiclePane`'s `SelectedFlightDisplay`, i.e. only once a vehicle is
selected — so a `connections` registry entry could not have satisfied
"reachable ... without requiring a selected vehicle" at all. `ConnectionsBar`
is instead mounted once in `FlightDisplay`'s `WorkspaceShell` `meta` slot,
which renders unconditionally for both the fleet and vehicle sections
regardless of selection — one mount point instead of two, and it is what
actually satisfies that criterion. No registry change was needed or made.

**Three real wire-shape bugs found and fixed by reading the actual Go types
before trusting the contract doc's summary, then confirmed live:**
`internal/connection/types.go`'s `Status.Device`/`Status.Settings`/
`OpenedAtMs`/`LastFrameAtMs` carry no `omitempty` and are always serialized
as their zero values (`device:{"id":""}`, `settings:{"baud_rate":0}`,
timestamps `0`) rather than omitted for e.g. `DEVICE_MISSING` — fixed by
treating an empty device id, a non-positive baud, and a zero timestamp as
absent in `types.ts`'s parsers (each now has a regression test built from
the exact payload confirmed below). `POST /api/connections/disconnect`
returns `{"disconnected":true}`, not a `Status` — `api.ts`'s `disconnect()`
now returns `void`; the fold picks up `RELEASED` from the `acquisition`
event the disconnect provokes, not from its own response. `Profile`'s wire
response nests `device:{id:...}` (`profile.go`) while the POST *request*
body takes a flat `device_id` (`http.go`'s decode struct) — two different
shapes on the same field name; `parseProfile` now reads the nested form.

**Real-backend confirmation pass** (`go build ./cmd/gcs`, run locally with
`GCS_MAVLINK_UDP_BIND=` and `GCS_COMMANDS_ENABLED=false`, macOS arm64,
Go 1.25.0): `GET /api/connections`(empty)/`/devices`(real local serial
candidates)/`/profiles`; a full `POST .../connect` (ad hoc device) →
`OPEN_AWAITING_TRAFFIC` → `POST .../disconnect` → `RELEASED` cycle against a
real local serial port, cross-checked against a live `/api/events` SSE
subscription showing the matching `event: acquisition` frames at each
transition; a profile save → `GET /api/connections/profiles` → delete round
trip. This is what surfaced the three bugs above — none were visible from
the fixture-driven suite alone, which only ever fed the parsers shapes this
session had itself assumed were correct. **Not exercised**, and remaining
for T-028/T-031 hardware acceptance or a future SITL/PTY pass: a real
`REPORTING` transition (needs actual MAVLink traffic, not a bare serial
port) and a genuine `AMBIGUOUS` case (needs two devices sharing one
identity) — both are covered structurally by fixture-driven component tests
instead. Keyboard reachability relies on native `<button>`/`<select>`
semantics (the same primitives already used everywhere else in this UI); no
separate dedicated keyboard-navigation pass was run.

**Executed:** `pnpm typecheck`, `pnpm lint`, `pnpm test` (469 tests: the same
4 pre-existing failures recorded on other cards, unrelated to this change —
confirmed by diffing against `git stash`; 0 new failures), `pnpm
build-storybook`, `pnpm build`. The six `Connections` stories (including two
with a `storybook/test` `play` step opening the popover) were driven in
headless Chrome via CDP (`--use-gl=angle --use-angle=swiftshader`) with
`Runtime.exceptionThrown`/console-error subscriptions: zero exceptions,
zero console errors, and `innerText` matched the expected rows for every
state combination exercised. `./scripts/kanban check` passes.
