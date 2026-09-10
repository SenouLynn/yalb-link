---
id: T-041
title: Pin workspace inventory and view-menu interactions
status: backlog
priority: 2
owner: unassigned
depends_on: none
---

## Motivation and evidence

T-014 adds keyboard tab/panel regression coverage; registry defaults, fixed
panels, slotOccupied, toggleable, and Views outside-click dismissal still lack
dedicated focused tests. FleetOverview interactions likewise rely on browser smoke.

## Outcome

Focused regression tests protect registry visibility and view-menu behavior
without duplicating implementation details.

## Scope

- In: the outcome above, deferred from T-014.
- Out: offline tiles (T-030), telemetry acquisition (T-023), persistence and new commands.

## Acceptance criteria

- [ ] Test user-visible default/fixed panel rules and hide/restore across slots.
- [ ] Test Views Escape and outside-pointer dismissal, including focus behavior.
- [ ] Test fleet open/center controls for fresh, missing-position and LOST entries.

## Verification

```sh
cd frontend && pnpm test && pnpm lint && pnpm typecheck
```

## Open questions

Implementation details to shape before claiming; no product decision is implied.

## Notes

Tracked by the reviewed Impeccable round-two plan; not implemented by T-014.
