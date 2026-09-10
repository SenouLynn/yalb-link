---
id: T-032
title: Accept RC-piloted field telemetry observation
status: backlog
priority: 1
owner: unassigned
depends_on: T-031
---

## Motivation and evidence

Validate the primary real-world use before expanding write capabilities. See the [operator usage plan](../operator-usage-plan.md) for current evidence and scenario definitions.

## Outcome

Accept RC-piloted field telemetry observation with explicit evidence and remaining limits.

## Scope

RC pilot controls aircraft; MissionPlanner prepares mission; application observes and records.

## Acceptance criteria

- [ ] E2E-04 expectations and flight procedure are agreed before the run.
- [ ] Required state and mission progress agree with independent autopilot evidence within declared bounds.
- [ ] No application operator mutations occur; limits, failures and replay findings are recorded.

## Verification

Review target SITL and ground evidence first; execute E2E-04 when hardware and pilot are available, then compare logs/backend/UI and replay. Disruptive failure tests remain on ground or SITL.

## Open questions

Field availability, pilot procedure and required state gaps, including whether T-016 must precede acceptance.

## Notes

Planned, not executed. Dependencies sequence outcomes, not proof of hardware
compatibility. Refine implementation commands and scenario bounds before claim.
