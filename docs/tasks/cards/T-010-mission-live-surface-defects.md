---
id: T-010
title: Correct mission surfaces that unit tests cannot reach
status: done
priority: 0
owner: unassigned
depends_on: T-007, T-009
---

## Motivation and evidence

T-007 and T-009 are green — `pnpm typecheck`, `pnpm lint`, and 29 files / 270
frontend tests pass, as do `./internal/mission/...`, `./internal/codec/...`,
`./internal/bridge/...`, and `./cmd/gcs/...`. The suites cannot reach three
defects because each lives exactly where the test boundary stops.

- `activeMissionSequence` (`frontend/src/mission/model.ts:54`) highlights the
  active item only while `MISSION_CURRENT` is inside the telemetry TTL, and
  `internal/codec/message.go:42` decodes message 42. But `bridge.DefaultRates`
  (`internal/bridge/rates.go:46`) never requests it. The README states that a
  connected vehicle streams without external setup, so nothing else asks. The
  "Active" chip is unreachable on a live link regardless of firmware defaults.
- `buildStyle` (`frontend/src/map/MapPanel.tsx:34`) declares no `glyphs`, and
  `mission-labels` (`MapPanel.tsx:96`) is a symbol layer with `text-field`.
  MapLibre cannot rasterise text without a glyph source, so the numbered
  waypoints do not render. It is the style's first symbol layer, so nothing
  before this batch needed glyphs. `MapPanel.test.ts` covers `missionFeatures`,
  which is pure geometry and passes either way.
- `commandName`/`frameName` (`model.ts:46`) return `MavCmd[value]` and
  `MavFrame[value]`. These are TS reverse lookups over generated numeric enums,
  so any wire value this build does not enumerate yields `undefined` while the
  signature promises `string`. `MAV_CMD_NAV_SPLINE_WAYPOINT` (82) and
  `MAV_CMD_DO_SET_CAM_TRIGG_DIST` (206) are absent from
  `proto/gcs/v1/types.proto` and both appear in ordinary ArduPilot missions.
  The list renders `#3 undefined` and the map reason reads `command undefined
  is non-positional`.

Two of the three are conditions T-008 asserts directly, so accepting first
would spend a live SITL session rediscovering them.

## Outcome

The mission display's active-sequence highlight, numbered map points, and item
labels are correct against a real link and a real browser, and each has a
regression test that fails without the fix.

## Scope

- In: the `MISSION_CURRENT` acquisition rate, glyph-free sequence labels on the
  map, unknown-enum labelling, and tests for each.
- Out: live SITL acceptance (T-008), a mock mission fixture (T-011), retry
  policy, adding MAVLink commands to the proto enum, mission mutation.

## Acceptance criteria

- [x] The rate policy requests `MISSION_CURRENT`, so a discovered vehicle
      streams it without external setup, and a test fails if it is dropped.
- [x] Mission sequence numbers render without the style declaring `glyphs`, and
      a test fails if any layer needs a glyph source the style does not provide.
- [x] A command or frame value absent from the generated enum renders its wire
      value rather than `undefined`, in both the list and the omission reason.
- [x] No new external network dependency is introduced for map labelling.

## Verification

```sh
go test -race ./internal/bridge/... ./internal/mission/...
cd frontend && pnpm typecheck && pnpm lint
cd frontend && pnpm vitest run src/mission src/map src/ui
./scripts/kanban check
```

## Open questions

None. Sequence labels use DOM markers rather than a glyph server: adding a
public font endpoint would be a second prototyping network dependency and a
costly-to-reverse choice needing an ADR, and the repository has deliberately
deferred tile and asset infrastructure.

## Notes

Discovered while taking stock of the T-004..T-009 batch before live acceptance.
Complete before promoting T-008.

Implementation notes:

- `MISSION_CURRENT` joins `DefaultRates` at 1 Hz, comfortably inside the 5 s
  `TELEMETRY_TTL_MS` the highlight is gated on.
  `TestDefaultRatesRequestsMissionCurrent` pins it by the display feature that
  needs it rather than by list position, because the pre-existing
  `TestRateRequestsOnDiscovery` pins the whole policy and would be updated
  mechanically by anyone reordering it.
- Sequence numbers moved from a `symbol` layer to `maplibregl.Marker` DOM
  overlays, reusing the pattern `createVehicleMarkerElement` already
  established. A glyph endpoint was rejected: it is a second public network
  dependency beside the basemap, it breaks the deferred offline-imagery story,
  and it would need an ADR. `flightLayers()` and `buildStyle` are now exported
  so `MapPanel.test.ts` can assert the style never asks for text it cannot
  draw, which is the invariant that was violated rather than the specific
  layer.
- `commandName`/`frameName` route through `nameOrValue`, which keeps the wire
  value when the generated enum has no name for it. TypeScript forbids writing
  an unenumerated literal as a `MavCmd`, so the tests cast — that models
  `fromJson`, which is the only place such a value can enter.
- All three fixes were mutation-checked: reverting each one individually fails
  its test. The reverted enum lookup produced exactly
  `command undefined is non-positional`, and the reverted layer list produced
  `expected [ 'mission-labels' ] to deeply equal []`.
- Verified: `go build ./...`, `go vet`, `go test -race ./internal/bridge/...
  ./internal/mission/...`, the full Go suite excluding `internal/recording`,
  `pnpm typecheck`, `pnpm lint`, `pnpm vitest run` (29 files, 274 tests, up
  from 270), `git diff --check`, and `./scripts/kanban check`.
  `internal/recording` was skipped: `TestDurationLimitDoesNotRequireAnotherEvent`
  fails on most runs on this machine independently of any change here.
  `golangci-lint` is not installed and the installed Bazel is 8.3.1 against a
  pinned 8.7.0, so neither gate was run; no BUILD files changed.
- The map change is not covered by browser automation, which remains out of
  scope. The glyph invariant is asserted against the style specification, not
  against rendered output.
