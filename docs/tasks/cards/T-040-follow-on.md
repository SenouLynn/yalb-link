---
id: T-040
title: Consolidate remaining typography and target-size literals
status: backlog
priority: 3
owner: unassigned
depends_on: none
---

## Motivation and evidence

T-014 moved focus and shadow vocabulary to tokens. system.css still repeats
tracking values and 28px targets; the original round-two plan explicitly left
that wider token audit for a follow-on.

## Outcome

Repeated target sizes and tracking follow semantic tokens without changing
SVG/map geometry or adding type steps.

## Scope

- In: the outcome above, deferred from T-014.
- Out: offline tiles (T-030), telemetry acquisition (T-023), persistence and new commands.

## Acceptance criteria

- [ ] Inventory repeated spacing/tracking/target literals and document geometry exceptions.
- [ ] Use existing tokens or introduce only justified semantic tokens.
- [ ] Verify unchanged target sizes and readability in desktop/stacked screenshots.

## Verification

```sh
cd frontend && pnpm vitest run src/ui/system.test.ts && pnpm lint && pnpm typecheck
```

## Open questions

Implementation details to shape before claiming; no product decision is implied.

## Notes

Tracked by the reviewed Impeccable round-two plan; not implemented by T-014.
