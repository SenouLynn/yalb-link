---
id: T-011
title: Demonstrate mission inspection without a backend
status: done
priority: 0
owner: unassigned
depends_on: T-007, T-010
---

## Motivation and evidence

Every other display surface can be demonstrated with no backend. `MockEventSource`
(`frontend/src/stream/mock.ts`) exists so "the display can be demonstrated and
tested without a backend, and so a failing instrument can be reproduced from a
fixed input", and the README points at `?source=mock` for exactly that.

The mission panel is the one surface that breaks the pattern. `downloadMission`
(`frontend/src/mission/client.ts:7`) always fetches
`/api/vehicles/{sys}/{comp}/mission`, so under `?source=mock` there is no
backend to answer and under `?source=replay` the live coordinator has no route
and returns `mission_no_route`. Seeing the mission list, the map route, the
numbered points, or the active-sequence chip therefore requires the full Compose
stack plus a mission loaded by an external ground station.

That is why the T-004..T-009 batch has been reviewed only as code: there is no
way to look at what it renders.

## Outcome

`?source=mock` shows a complete mission — ordered list, map route, numbered
points, an omitted non-positional item, and an active-sequence highlight — with
no backend running.

## Scope

- In: a fixture mission snapshot, source-dependent mission loading, a
  `MISSION_CURRENT` frame in the mock script, and tests.
- Out: mission in recordings or replay (the recording schema carries no mission),
  live SITL acceptance (T-008), mission mutation.

## Acceptance criteria

- [x] `?source=mock` renders a mission list, a map route, and numbered points
      with no backend process running.
- [x] The fixture includes at least one item omitted from map geometry, so the
      "Not mapped" explanation is demonstrable.
- [x] The mock script drives the active-sequence highlight through the same
      `MISSION_CURRENT` freshness rule as a live vehicle.
- [x] `?source=live` still fetches over HTTP and is unaffected.
- [x] The fixture is not reachable from the live source, so a mock mission can
      never be mistaken for a downloaded one.

## Verification

```sh
cd frontend && pnpm typecheck && pnpm lint
cd frontend && pnpm vitest run src/mission src/map src/ui src/stream
cd frontend && pnpm dev --port 3001   # then open /?source=mock
./scripts/kanban check
```

## Open questions

None. Replay is deliberately excluded: recordings store stream events, and a
mission snapshot is a synchronous HTTP read that was never recorded, so serving
one under `?source=replay` would invent data the recording does not contain.

## Notes

Raised while taking stock of the T-004..T-009 batch. This is what makes the
mission UI reviewable before and independently of a live SITL session.

Implementation notes:

- `mission/fixtures.ts` holds the snapshot; `mission/source.ts` is the single
  switch that decides where a mission comes from. Live and replay both use the
  HTTP client, so the fixture cannot reach a display showing real telemetry —
  a commanded route an operator might act on has to have come from the vehicle,
  and `source.test.ts` asserts that both ways round.
- The fixture deliberately contains two `MAV_FRAME_MISSION` items, so the
  "Not mapped" posture is demonstrable rather than theoretical.
- `MISSION_CURRENT` now streams in the mock script on every cycle, walking the
  active index through the mission. One frame at startup would go stale inside
  `TELEMETRY_TTL_MS` and the highlight would vanish, which is the same defect
  T-010 fixed on the live path.
- Verified in a real browser, not only in tests: headless Chrome over the
  DevTools protocol against `pnpm dev --port 3001` at `/?source=mock`, clicking
  "Download mission". Observed `Complete · 6 items`, all six items listed with
  command, frame, coordinates, parameters and autocontinue, one `Active` chip,
  both "Not mapped" explanations, map sequence markers `0 1 3 4`, the
  "Commanded mission" key, and a clean console.

Defect found by that browser run and fixed here:

- `useMission` returned the whole `MissionViewState`, which was spread into
  `MissionPanel`. `MissionViewState.key` is a `VehicleKey`, so React consumed
  it as a reconciliation key, dropped the prop, and logged an error on every
  render. No server-rendered test saw it.
- Fixed structurally with `missionPanelState`, which narrows the state to what
  the panel renders, so the mistake is no longer representable at any call
  site. A first attempt asserted on React's console warning instead; it was
  removed after a mutation check showed it passing with the bug reinstated —
  React deduplicates that warning per component type, so earlier tests in the
  same file had already consumed it and the test could never fail.
- Not fixed here: the map centres on the vehicle at zoom 16 and never fits the
  mission, so items 1, 3 and 4 sat clipped at the viewport edge. Map viewport
  behaviour was explicitly out of scope for T-007 and T-009; raised as T-012.
- Verified: `pnpm typecheck`, `pnpm lint`, `pnpm vitest run` (30 files, 281
  tests, up from 274), the browser session above, and `./scripts/kanban check`.
