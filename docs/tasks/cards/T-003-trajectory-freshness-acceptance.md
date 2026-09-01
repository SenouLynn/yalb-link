---
id: T-003
title: Demonstrate absent and stale trajectory posture
status: done
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

- [x] An absent required input does not render a predicted trajectory.
- [x] A stale required input removes or suppresses the predicted trajectory.
- [x] Recovery with fresh inputs restores the prediction without a reload.
- [x] Any defect found is fixed or represented by a new board card.
- [x] The repeatable procedure and observed result are recorded in the relevant
      runbook or executable check.

## Verification

```sh
cd frontend && pnpm vitest run src/map src/logic
```

Manual: interrupt and restore the relevant live telemetry while observing the
map's trajectory overlay.

## Open questions

None. A controlled publisher can omit and resume one family; on live SITL,
`MAV_CMD_SET_MESSAGE_INTERVAL` isolates the same family without stopping
unrelated telemetry.

## Notes

Accepted on 2026-09-01 in one live browser session. With position absent, the
map had zero track and trajectory points while heading and speed remained live.
Fresh position produced an 11-coordinate prediction; stale position removed
it while attitude and VFR HUD remained 0.1 seconds old; fresh position restored
the same 11-coordinate prediction without a reload. No defect was found.
