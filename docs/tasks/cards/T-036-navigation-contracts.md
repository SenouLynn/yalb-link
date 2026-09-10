---
id: T-036
title: Split QuadPlane navigation into action-specific contracts
status: backlog
priority: 3
owner: unassigned
depends_on: T-035
---

## Motivation and evidence

Laptop lift-off, go-to, next waypoint and RTH are the eventual goal. See the [operator usage plan](../operator-usage-plan.md) for current evidence and scenario definitions.

## Outcome

Split QuadPlane navigation into action-specific contracts with explicit evidence and remaining limits.

## Scope

Define four separately reviewable action cards and required SITL evidence; no commands implemented or sent here.

## Acceptance criteria

- [ ] Each action specifies modes, transitions, prerequisites, intent confirmation and observed completion.
- [ ] Separate gates and RC handoff are explicit; ACK is distinguished from aircraft outcome.
- [ ] Link loss, rejection, late ACK and ambiguity are covered without blind retries.

## Verification

Review E2E-07 per action against the exact firmware documentation/target SITL experiment plan; run board validation.

## Open questions

Meaning of each action in QuadPlane modes and operator takeover policy.

## Notes

Planned, not executed. Dependencies sequence outcomes, not proof of hardware
compatibility. Refine implementation commands and scenario bounds before claim.
