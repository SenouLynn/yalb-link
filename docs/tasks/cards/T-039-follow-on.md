---
id: T-039
title: Keep fleet and vehicle maps resident in the panel registry
status: backlog
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

FleetOverview still composes shell slots outside the registry and hand-rolls
fleet rows. FlightDisplay conditionally mounts FleetOverview and the selected
MapPanel; camera refs compensate for losing the MapLibre context. These are
explicitly deferred from T-014, not accepted as fixed.

## Outcome

Fleet panels use registry mounts and shared Row vocabulary. Switching between
fleet and vehicle views preserves maps and their cameras without save/restore
plumbing.

## Scope

- In: the outcome above, deferred from T-014.
- Out: offline tiles (T-030), telemetry acquisition (T-023), persistence and new commands.

## Acceptance criteria

- [ ] Add fleet registry composition without nested scroll regions.
- [ ] Keep both map instances resident across fleet/detail navigation; dispose only when their owner ends.
- [ ] Replace roster dl rows with shared Row; remove duplicate row/layout CSS.
- [ ] Pin navigation, selected-vehicle changes, camera preservation and pending missions with lifecycle tests.

## Verification

```sh
cd frontend && pnpm test && pnpm lint && pnpm typecheck
# Browser: navigate fleet → vehicle → fleet repeatedly; preserve cameras and requests.
```

## Open questions

Implementation details to shape before claiming; no product decision is implied.

## Notes

Tracked by the reviewed Impeccable round-two plan; not implemented by T-014.
