---
id: T-022
title: Navigate from a live fleet overview into a vehicle workspace
status: done
priority: 1
owner: unassigned
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

- [x] Live and mock pages open on Fleet. Existing replay URLs still open the
      vehicle workspace; Fleet remains reachable during replay. Source badge
      and replay transport have one persistent home above both views.
- [x] At 1440×900 a compact left roster accompanies a map filling the remaining
      space. At 768×1024 a bounded roster band and map remain reachable with no
      horizontal document overflow. Compare with the operator's fleet screenshot.
- [x] Roster contains each known full system:component identity in stable order,
      vehicle-link and browser-connection state, position, altitude with datum,
      ground speed and freshness. Missing/stale readings use existing gates;
      arrival order cannot change an explicit selected identity.
- [x] Fresh positions appear as independently identified markers. At the 5 s
      position TTL (allow the next 250 ms clock tick), remove the current marker
      and disable centering for that vehicle while keeping its roster entry.
      A lost vehicle remains inspectable with explicit lost posture. Recovery
      restores the marker without navigation or reload; never place absence at 0,0.
- [x] Initially fit the first nonempty set of fresh positions once. Later
      telemetry or new vehicles do not repeatedly reset the user's camera.
      Fit fleet includes all current fresh positions with padding; Center vehicle
      targets that vehicle. Zero positions leaves a clear empty map state; one
      position uses a bounded zoom. Fit is disabled with no valid positions.
- [x] A labelled roster Open action opens the exact vehicle workspace; Back to
      fleet returns to the overview. Map markers identify vehicles and can open
      them; equivalent keyboard-accessible roster actions are always available.
      Hidden/inactive view controls leave the focus order and accessibility tree.
- [x] Navigation does not restart the stream or replay, reset fleet tracks, or
      lose pane visibility preferences. Retain a pending/completed mission when
      visiting Fleet and reopening the same selected vehicle; switching A → B
      or removing selection still cancels and isolates requests as in T-009.
      Clear unsubmitted arm confirmation on leaving a vehicle view; retain
      submitted transaction evidence and resolution gates for the correct identity.
- [x] Keep at most one active MapLibre context across view navigation. Camera
      state may be explicitly saved/restored when maps unmount; document that
      behavior separately from T-017's no-remount guarantee for pane toggles.
      View navigation restores the fleet camera; replay reset clears old tracks.
- [x] Demonstrate two live SITL identities, separate markers and selection,
      per-vehicle interruption/recovery, backend disconnect/reconnect, and replay.
      Record actual conditions and evidence; fixture checks are supplemental.

## Verification

```sh
cd frontend && pnpm typecheck && pnpm lint && pnpm vitest run && pnpm build
./scripts/kanban check
```

Executed on 2026-09-10: typecheck, lint, 39 files / 351 tests, and build all
exit 0; `go test -race ./...` exits 0 with no Go change here; `./scripts/kanban
check` and `git diff --check` exit 0. Bazel and golangci-lint remain unrunnable
on this machine and were not counted.

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

Completed execution evidence: [T-022 run](../../runbooks/validation/t022.md);
raw captures were optional under T-025 and have been pruned.

Two live SITL identities ran with distinct home locations, because the Compose
defaults coincide: Copter `1:1` at 37.7749,-122.4194 and Plane `2:1` at
37.7799,-122.4144, set through a scratchpad Compose override. T-021 is closed,
so recording used the default `GCS_RECORDING_DB_PATH` rather than T-017's `/tmp`
workaround. `GCS_COMMANDS_ENABLED=false` in that stack, so the arm-confirmation
reset on leaving a vehicle view is covered by the lifecycle test rather than
live; nothing was armed and no vehicle flew.

Acceptance found and fixed a camera defect: `framePoints` detected a
single-point extent with `west === east`, but `(lon + 360) % 360` does not
round-trip, so for roughly a sixth of longitudes the east edge came back west of
the west edge and MapLibre framed the whole globe. `Center vehicle` on Copter
`1:1` jumped to its antipode at zoom 1.008 while the same action on Plane `2:1`
behaved. The arc is now clamped and an extent below 1e-9° is framed as a point,
with regression tests in `camera.test.ts`. `camera.ts` is shared with T-012's
mission framing, so single-item missions carried the same defect; T-013 and
T-014 remain the owners of further reference parity and mission vocabulary.

Camera save/restore across view navigation is deliberate and distinct from
T-017's no-remount guarantee for pane toggles: the fleet map unmounts when a
vehicle opens and its camera is restored on return, measured as an unchanged
marker position, while pane toggles still keep one canvas alive.

Created from the operator's screenshots and explicit request to add a fleet-view
card. Fleet navigation and freshness are the first deliverable; the reference's
additional roster actions can be shaped separately after this is demonstrated.
T-020 is the next selected execution chunk before further map/UI expansion,
but is not an artificial dependency of this card. T-015 remains the next
telemetry-inspection slice; T-013 owns broader reference parity.
