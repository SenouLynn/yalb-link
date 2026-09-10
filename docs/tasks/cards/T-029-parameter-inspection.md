---
id: T-029
title: Inspect a complete read-only parameter snapshot
status: backlog
priority: 1
owner: unassigned
depends_on: T-028
---

## Motivation and evidence

Support channel-mapping and configuration inspection before any editing surface. See the [operator usage plan](../operator-usage-plan.md) for current evidence and scenario definitions.

## Outcome

Inspect a complete read-only parameter snapshot with explicit evidence and remaining limits.

## Scope

Add a bounded addressed parameter read coordinator and inspection UI; no setters or editable controls.

## Acceptance criteria

- [ ] E2E-02 compares names/types/values to an independent baseline.
- [ ] Incomplete transfers, stale snapshots and vehicle switches are explicit; duplicates and timeouts are tested.
- [ ] Outbound capture contains only allowed acquisition traffic and no parameter mutation.

## Verification

Run focused coordinator/API/UI tests; execute E2E-02 in target SITL and USB bench; record counts, discrepancies and recovery.

## Open questions

Parameter protocol/type support and completion/retry policy for exact firmware.

## Notes

Planned, not executed. Dependencies sequence outcomes, not proof of hardware
compatibility. Refine implementation commands and scenario bounds before claim.
