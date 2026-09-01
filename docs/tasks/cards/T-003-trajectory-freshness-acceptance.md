---
id: T-003
title: Demonstrate absent and stale trajectory posture
status: ready
priority: 0
owner: unassigned
depends_on: none
---

## Motivation and evidence

The root README explicitly includes absent- and stale-data posture in the next
demonstrable outcome. It also states the invariant that stale telemetry must not
drive an instrument or number; the map projection needs the same live evidence.

## Outcome

The live browser demonstrably suppresses trajectory projection when its source
position, heading, or velocity is absent or stale.

## Scope

- In: absence, staleness, and recovery of the inputs used by the prediction.
- Out: changing the repository-wide freshness policy or adding new prediction
  inputs.

## Acceptance criteria

- [ ] An absent required input does not render a predicted trajectory.
- [ ] A stale required input removes or suppresses the predicted trajectory.
- [ ] Recovery with fresh inputs restores the prediction without a reload.
- [ ] Any defect found is fixed or represented by a new board card.
- [ ] The repeatable procedure and observed result are recorded in the relevant
      runbook or executable check.

## Verification

```sh
cd frontend && pnpm vitest run src/map src/logic
```

Manual: interrupt and restore the relevant live telemetry while observing the
map's trajectory overlay.

## Open questions

- What is the most repeatable live mechanism for making each input stale without
  stopping unrelated telemetry?

## Notes

None.
