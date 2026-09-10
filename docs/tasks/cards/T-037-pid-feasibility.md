---
id: T-037
title: Assess isolated PID tuning feasibility for the target firmware
status: backlog
priority: 3
owner: unassigned
depends_on: T-033, T-036
---

## Motivation and evidence

Two-person tuning is desired after validated read/write foundations. See the [operator usage plan](../operator-usage-plan.md) for current evidence and scenario definitions.

## Outcome

Assess isolated PID tuning feasibility for the target firmware with explicit evidence and remaining limits.

## Scope

Research exact firmware parameter eligibility and design a bounded ground/SITL experiment; no live airborne edits.

## Acceptance criteria

- [ ] Identify supported values, limits, armed-update eligibility, application timing and persistence with versioned evidence.
- [ ] Specify baseline/change/readback audit and aligned response measurement, one edit at a time.
- [ ] Independent tuning gate, pilot/operator roles, stop/recovery procedure and unknown-outcome behavior are defined.
- [ ] Produce separate experiment and later field-trial cards; feasibility is not asserted without evidence.

## Verification

Review E2E-08 against exact firmware sources and controlled ground/SITL findings when available; record unknowns and run board validation.

## Open questions

Whether chosen PID values can safely and effectively update while armed; second operator identity/deployment needs.

## Notes

Planned, not executed. Dependencies sequence outcomes, not proof of hardware
compatibility. Refine implementation commands and scenario bounds before claim.
