---
id: T-022
title: Navigate from a live fleet overview into a vehicle workspace
status: in_progress
priority: 1
owner: codex
depends_on: T-017
---

## Motivation and evidence

The operator supplied fleet and vehicle screenshots after T-017. The fleet
reference has a narrow roster beside a dominant shared map; opening a vehicle
leads into the denser vehicle workspace. This is a distinct navigation level,
not another panel inside the selected vehicle's workspace.

Reference: sibling `flight-path-hud` at `29426a9`,
`apps/gcs/src/App.tsx`, `fleet/FleetView.tsx` and `NodeView.tsx`;
see [reference review](../../reference/ui-reference-review.md). The supplied
screenshots are conversation evidence; they have not been copied into the repo.
YALB's `App.tsx` already owns the stream, clock and full `FleetState`, including
ordered identities, per-vehicle tracks and selection. It currently renders only
`FlightDisplay`; `MapPanel` is a single-vehicle adapter with automatic centering.

## Outcome

An operator can locate the connected fleet on one map, inspect a compact roster,
open one vehicle's existing workspace, and return without losing the feed or
mixing vehicle state.

## Scope

- In: fleet/vehicle navigation, compact roster, multi-vehicle position map,
  explicit Fit fleet and Center vehicle actions, responsive layout, freshness
  and lifecycle validation on the existing SSE and replay paths.
- Out: fleet mission downloads/routes, fleet arm controls, per-vehicle map
  visibility toggles, aggregate predictions/trails, video, guided commands,
  new backend endpoints, telemetry inspection, URL routing or persisted layout.
  Existing mission inspection remains in the vehicle workspace.

## Acceptance criteria

- [ ] Live and mock pages open on Fleet. Existing replay URLs still open the
      vehicle workspace; Fleet remains reachable during replay. Source badge
      and replay transport have one persistent home above both views.
- [ ] At 1440×900 a compact left roster accompanies a map filling the remaining
      space. At 768×1024 a bounded roster band and map remain reachable with no
      horizontal document overflow. Compare with the operator's fleet screenshot.
- [ ] Roster contains each known full system:component identity in stable order,
      vehicle-link and browser-connection state, position, altitude with datum,
      ground speed and freshness. Missing/stale readings use existing gates;
      arrival order cannot change an explicit selected identity.
- [ ] Fresh positions appear as independently identified markers. At the 5 s
      position TTL (allow the next 250 ms clock tick), remove the current marker
      and disable centering for that vehicle while keeping its roster entry.
      A lost vehicle remains inspectable with explicit lost posture. Recovery
      restores the marker without navigation or reload; never place absence at 0,0.
- [ ] Initially fit the first nonempty set of fresh positions once. Later
      telemetry or new vehicles do not repeatedly reset the user's camera.
      Fit fleet includes all current fresh positions with padding; Center vehicle
      targets that vehicle. Zero positions leaves a clear empty map state; one
      position uses a bounded zoom. Fit is disabled with no valid positions.
- [ ] A labelled roster Open action opens the exact vehicle workspace; Back to
      fleet returns to the overview. Map markers identify vehicles and can open
      them; equivalent keyboard-accessible roster actions are always available.
      Hidden/inactive view controls leave the focus order and accessibility tree.
- [ ] Navigation does not restart the stream or replay, reset fleet tracks, or
      lose pane visibility preferences. Retain a pending/completed mission when
      visiting Fleet and reopening the same selected vehicle; switching A → B
      or removing selection still cancels and isolates requests as in T-009.
      Clear unsubmitted arm confirmation on leaving a vehicle view; retain
      submitted transaction evidence and resolution gates for the correct identity.
- [ ] Keep at most one active MapLibre context across view navigation. Camera
      state may be explicitly saved/restored when maps unmount; document that
      behavior separately from T-017's no-remount guarantee for pane toggles.
      View navigation restores the fleet camera; replay reset clears old tracks.
- [ ] Demonstrate two live SITL identities, separate markers and selection,
      per-vehicle interruption/recovery, backend disconnect/reconnect, and replay.
      Record actual conditions and evidence; fixture checks are supplemental.

## Verification

```sh
cd frontend && pnpm typecheck && pnpm lint && pnpm vitest run && pnpm build
./scripts/kanban check
```

Use happy-dom/createRoot lifecycle tests in the existing style: stream setup and
cleanup (including StrictMode), navigation during mission request, A → B → A,
arm confirmation reset, freshness clock and replay reset. Test map bounds for
zero/one/multiple positions, including longitude wraparound.

Browser: both target sizes; measure roster scroll reachability and viewport
bounds; verify map resizing, focus and camera restoration. Use disarmed Copter
and Plane in Compose on the normal binary MAVLink/backend feed. Their default
homes coincide: set distinct simulator home locations before judging distinct
markers, record those coordinates, and do not inject synthetic vehicle positions.
Pause one simulator for >5 s while the other remains live; require only that
vehicle's position to become unavailable. Resume and verify marker recovery.
Use the T-017 writable recording override until T-021 fixes Compose storage.
Capture/replay the session and document any replay-specific limitations.

## Open questions

None for this first slice. Extracting a shared basemap adapter versus a dedicated
fleet map is an implementation choice; do not give the existing single-vehicle
MapPanel fleet state or carry its follow behavior into the overview.

## Notes

Created from the operator's screenshots and explicit request to add a fleet-view
card. Fleet navigation and freshness are the first deliverable; the reference's
additional roster actions can be shaped separately after this is demonstrated.
T-020 is the next selected execution chunk before further map/UI expansion,
but is not an artificial dependency of this card. T-015 remains the next
telemetry-inspection slice; T-013 owns broader reference parity.
