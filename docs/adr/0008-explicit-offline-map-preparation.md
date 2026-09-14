# ADR 0008: Prepare offline maps through explicit area selection and prefetch

Date: 2026-09-14

Status: Accepted — operator workflow; implementation remains pending.

## Context

[ADR 0006](0006-operator-connection-readiness.md) requires usable offline
observation. T-030 addresses local startup and honest behavior when imagery is
absent; it does not implement saved maps. The operator now wants deliberate
preparation of map coverage before going to the field.

Mission Planner documents selecting a rectangle and prefetching its imagery.
QGroundControl documents named offline sets with minimum and maximum zoom
selection. The operator chose the Mission Planner interaction as the reference:
select an area and download it, with minimal setup ceremony. This is a product
preference, not a claim about why QGroundControl was designed that way.

## Decision

Provide an explicit **select area → prefetch → ready for offline use** workflow.
The operator selects a geographic rectangle on the map and deliberately starts
its download. Selection must have a visible interaction; Mission Planner's
keyboard gesture need not be copied or become the only way to select an area.

Show the selected extent, the supported detail and an estimated download/storage
cost before starting. Use a documented useful detail default rather than require
minimum/maximum zoom configuration or a named-set management flow. Exact detail
and size limits require measurement and remain open.

Report progress, cancellation, failure and verified completion. An incomplete
first download must not be described as offline-ready. Retain the selected
coverage across application/browser restarts and make its extent and available
detail discoverable. Beyond that extent or detail, explicitly report unavailable
coverage while telemetry and mission overlays remain usable.

Once prepared, the installed map is usable locally in both fleet and vehicle
views without a separate internet-dependent step. Preparation and refresh are
operator-initiated. Do not automatically select a county/state from a coordinate,
expand coverage as the vehicle moves, or refresh packages in the background.
The operator can deliberately replace the area or refresh its map data.

Map acquisition is independent of aircraft connection and must be available
before hardware is attached. It does not write to the aircraft.

## Consequences

This keeps preparation deliberate and offline use routine. It adds download and
coverage status, but avoids mandatory multi-set administration and technical
zoom controls in the initial flow. The operator must prepare the area while a
source is available; ordinary map browsing is not evidence of complete coverage.

[ADR 0009](0009-bounded-offline-map-packages.md) defines the initial package scope.
T-030 remains a separate prerequisite capability: the observer still works when
no package exists, including when online tile requests fail. This decision does
not expand or claim completion of its active implementation.

## Verification point

With declared source, extent, detail and size bounds, select and prefetch an
area through the UI, then restart the application with internet disconnected
and an empty browser cache. Verify coverage in fleet and vehicle maps throughout
the declared detail range. Exercise cancellation, download failure, insufficient
space, restart during preparation and requests outside coverage. Verify progress
and readiness match stored data, and no download begins without operator action.

These procedures are planned, not executed. Runtime work requires shaped task
cards under the normal board workflow.

## References

- [Mission Planner prefetch workflow](https://ardupilot.org/planner/docs/common-planning-a-mission-with-waypoints-and-events.html)
- [QGroundControl offline map sets](https://docs.qgroundcontrol.com/Stable_V5.0/en/qgc-user-guide/settings_view/offline_maps.html)
