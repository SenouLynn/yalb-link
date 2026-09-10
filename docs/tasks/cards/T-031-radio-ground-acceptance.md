---
id: T-031
title: Accept powered ground observation over the telemetry pair
status: backlog
priority: 1
owner: unassigned
depends_on: T-028, T-030
---

## Motivation and evidence

USB controller success does not prove LR900 radio behavior. See the [operator usage plan](../operator-usage-plan.md) for current evidence and scenario definitions.

## Outcome

Accept powered ground observation over the telemetry pair with explicit evidence and remaining limits.

## Scope

Measure E2E-03 on powered ground hardware after applicable SITL checks; no flight or operator writes.

## Acceptance criteria

- [ ] Record actual radio/serial settings and observed rates, loss and recovery latency.
- [ ] Required readings remain within predeclared freshness bounds at measured capacity; missing families are explicit.
- [ ] Radio/USB interruptions, independent truth comparison and offline recording/replay pass or have precise blockers.

## Verification

Execute powered ground E2E-03 with controlled link interruptions; compare timestamped received telemetry to backend/UI; retain a runbook and results.

## Open questions

Required stream budget and acceptable freshness on this radio; shape policy changes as separate cards if measurements require them.

## Notes

Planned, not executed. Dependencies sequence outcomes, not proof of hardware
compatibility. Refine implementation commands and scenario bounds before claim.
