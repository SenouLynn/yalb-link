---
id: T-028
title: Demonstrate USB controller observation
status: backlog
priority: 1
owner: unassigned
depends_on: T-027, T-023
---

## Motivation and evidence

Make E2E-01 work through the normal backend and browser. See the [operator usage plan](../operator-usage-plan.md) for current evidence and scenario definitions.

## Outcome

Demonstrate USB controller observation with explicit evidence and remaining limits.

## Scope

Select a narrow serial ingress or documented local serial-to-UDP adapter experiment; verify the choice before expanding configuration. Keep operator commands disabled.

## Acceptance criteria

- [ ] Applicable target SITL ingestion/recovery checks pass before controller acceptance.
- [ ] Battery-free USB connection and unplug/replug produce correct identity, freshness and recovery without mutation traffic.
- [ ] Device access and startup procedure are recorded with unsupported readings.

## Verification

Run relevant Go transport/bridge tests; exercise E2E-01 on the captured target and record MAVLink, backend and UI evidence.

## Open questions

Native serial versus local bridge; serial device selection and baud configuration.

## Notes

Planned, not executed. Dependencies sequence outcomes, not proof of hardware
compatibility. Refine implementation commands and scenario bounds before claim.
