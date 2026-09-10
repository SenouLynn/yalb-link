---
id: T-038
title: Add an isolated Storybook component workshop
status: done
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

The operator requested Storybook to tune existing granular components in isolation.
React/Vite and prop-driven instruments and mission panels already support this.

## Outcome

A local component workshop using production CSS and deterministic fixtures.

## Scope

Storybook configuration, instrument/readout/mission/workspace stories and runbook.
An isolated layout shell retains its labelled map placeholder. Full frontend
stories render the real FlightDisplay with real maps and mock data. Connected
command/replay scenarios and figure-eight/snake profiles are deferred; this is UI tooling, not SITL acceptance.

## Acceptance criteria

- [x] Development and static build commands work with the existing frontend.
- [x] Instruments expose editable values and live/stale/unavailable states.
- [x] Missions cover idle/loading/error/empty/populated states.
- [x] Workspace stories demonstrate interactive pane controls and narrow/wide layouts.
- [x] Shared production CSS and deterministic fixtures require no backend.
- [x] Launch and verification instructions are documented.
- [x] Catalog progresses from components to panels, layout composition, and the actual frontend.
- [x] Real frontend stories support fleet/vehicle navigation, map, mission download, pane toggles, and narrow/wide review.

## Verification

`cd frontend && pnpm typecheck && pnpm lint && pnpm test && pnpm build-storybook && pnpm build`

Start Storybook and verify story index and rendered stories where browser tooling
is available. Run `./scripts/kanban check`.

## Open questions

None.

## Notes

Completed 2026-09-10 with Storybook 10.6.0 and 55 stories. The catalog progresses
through Components, Panels, Composition, and Frontend. Full frontend stories
reuse production FlightDisplay and actual maps; the animated story uses the
existing mock stream with cleanup. Frozen states permit precise visual work.

Story/config typecheck, ESLint, all 351 frontend tests, production build and
static Storybook build passed. Builds report bundle-size advisories. TypeScript
server resolves Frontend.stories.tsx to tsconfig.storybook.json with no semantic
diagnostics after adding the project to the frontend solution references.

Headless Chrome with software WebGL checked all 55 story entries without
runtime exceptions (the single-vehicle selector is intentionally empty).
Checked stale attitude withholding, manager Controls, 390px viewport, pane
toggles/all-hidden prompt, local mission clear/download, real frontend
fleet/vehicle navigation, mock mission download, and changing animated readings.
Desktop, narrow frontend, and standalone map screenshots were inspected.

The user's black-map report exposed the standalone story's missing pane--map
container: the real map relies on that ancestor for full-height layout. Added
it and verified a 968x600 map shell, panel and canvas with visible basemap and
mission markers. Preview-only CSS restores document scrolling for isolated long
panels; production workspace CSS remains shared and unchanged.

Storybook runs on localhost:6007 because 6006 belongs to another project.
No backend or live SITL was exercised. Basemap imagery uses public services.
Bazel source targets remain unchanged; Bazel was not rerun for this tooling work.
