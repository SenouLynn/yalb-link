---
id: T-028
title: Demonstrate USB controller observation
status: backlog
priority: 1
owner: unassigned
depends_on: T-027, T-048, T-049, T-050
---

## Motivation and evidence

Make E2E-01 work through the normal backend and browser. See the [operator usage plan](../operator-usage-plan.md) for current evidence and scenario definitions.

## Outcome

Demonstrate USB controller observation with explicit evidence and remaining limits.

## Scope

Accept the implemented connection journey from T-045 through T-050 on the target USB controller. Cover first selection, saved settings, hardware attached before/after launch, unplug/replug and explicit port release/reacquisition for MissionPlanner. Keep operator commands disabled. A manually configured adapter experiment alone does not satisfy ADR 0006.

## Acceptance criteria

- [ ] Applicable target SITL ingestion/recovery checks pass before controller acceptance.
- [ ] Battery-free USB connection and unplug/replug produce correct identity, freshness and recovery without mutation traffic.
- [ ] Device access and startup procedure are recorded with unsupported readings.

## Verification

Run relevant Go transport/bridge tests; exercise E2E-01 on the captured target and record MAVLink, backend and UI evidence.

## Open questions

Target device identity, USB-only readings and ground procedure require T-027 evidence. Connection service choices belong to T-045; implementation belongs to its dependent cards.

## Notes

Planned, not executed. Dependencies sequence outcomes, not proof of hardware
compatibility. Refine implementation commands and scenario bounds before claim.
