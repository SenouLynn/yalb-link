---
id: T-034
title: Shape offline waypoint drafting independently of upload
status: backlog
priority: 3
owner: unassigned
depends_on: T-032
---

## Motivation and evidence

MissionPlanner remains sufficient until field reads are accepted. See the [operator usage plan](../operator-usage-plan.md) for current evidence and scenario definitions.

## Outcome

Shape offline waypoint drafting independently of upload with explicit evidence and remaining limits.

## Scope

Define a local draft/editor slice with no vehicle writes.

## Acceptance criteria

- [ ] Specify coordinates, altitude datum, supported item types and offline geographic context.
- [ ] Draft persistence and vehicle separation are explicit.
- [ ] Create a small implementation card with E2E-06 draft acceptance; no upload/start side effect.

## Verification

Review an offline draft workflow and edge cases against the target mission representation; run board validation.

## Open questions

Imagery needs and supported QuadPlane mission item subset.

## Notes

Planned, not executed. Dependencies sequence outcomes, not proof of hardware
compatibility. Refine implementation commands and scenario bounds before claim.
