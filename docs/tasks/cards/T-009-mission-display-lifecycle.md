---
id: T-009
title: Harden mission display lifecycle before live acceptance
status: done
priority: 0
owner: unassigned
depends_on: T-007
---

## Motivation and evidence

`FlightDisplay` returns before calling `useMission` while no vehicle is
selected, then calls it after the first heartbeat selects a vehicle. Because
the component remains mounted in `App`, that ordinary live transition changes
the hook count between renders and violates React's hook-order invariant. The
current server-render tests render each fleet state independently, so they do
not exercise the transition. Mission GeoJSON also always includes a line even
when fewer than two positional items exist, despite a GeoJSON `LineString`
requiring at least two positions.

## Outcome

The mission display remains valid while vehicles appear, disappear, and change,
and empty or single-point missions produce valid map geometry.

## Scope

- In: stable hook ownership across fleet lifecycle transitions, a rerender
  regression test, valid zero/one/many-point mission GeoJSON, and pure geometry
  tests.
- Out: live SITL interoperability, mission protocol changes, map viewport
  behavior, retries, mutation, fences, and rallies.

## Acceptance criteria

- [x] Rendering no vehicle and then the first discovered vehicle does not
      change the hooks called by any mounted component.
- [x] Switching selection or returning to the no-vehicle display cancels its
      mission request and cannot display that vehicle's snapshot under another
      identity.
- [x] Mission feature collections omit the line below two positional points
      while retaining every valid point marker.
- [x] A regression test exercises component rerender across the no-vehicle and
      selected-vehicle states; static one-shot rendering is not the only proof.

## Verification

```sh
cd frontend && pnpm typecheck
cd frontend && pnpm lint
cd frontend && pnpm vitest run src/mission src/map src/ui
./scripts/kanban check
```

## Open questions

None. Keep mission state in a component whose mounted lifetime requires a
selected vehicle, rather than calling a stateful hook conditionally.

## Notes

This is a pre-merge defect card discovered while reviewing `e0a67b1`. Complete
it before promoting T-008 or fast-forwarding `feat/next` into `main`.

Implementation notes:

- Mission state now belongs to `SelectedFlightDisplay`, which mounts only when
  a selected vehicle exists. The parent can move between its empty and selected
  children without conditionally invoking a hook.
- A Happy DOM regression rerenders one persistent React root from no vehicle to
  a selected vehicle and back. It also starts a request and proves that changing
  the selected identity aborts it and resets the visible mission state.
- Mission GeoJSON retains zero or one point marker but adds its line only once
  two positional items exist.
- Verified with `pnpm typecheck`, `pnpm lint`, and `pnpm vitest run src/mission
  src/map src/ui` (10 files, 79 tests).
