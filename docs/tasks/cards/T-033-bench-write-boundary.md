---
id: T-033
title: Decide the isolated Bench configuration boundary
status: backlog
priority: 3
owner: unassigned
depends_on: T-029
---

## Motivation and evidence

Configuration writes need independent logic, planning and testing. See the [operator usage plan](../operator-usage-plan.md) for current evidence and scenario definitions.

## Outcome

Decide the isolated Bench configuration boundary with explicit evidence and remaining limits.

## Scope

Produce an ADR and a single-parameter ground-only implementation card; do not implement mutations here.

## Acceptance criteria

- [ ] Compare separate app/process with isolated service deployment and specify default-off policy.
- [ ] Define preview, target/baseline checks, validation, readback, persistence/reboot and unknown-outcome recovery.
- [ ] Tests and negative cross-capability checks are specified; existing arm gate cannot enable Bench writes.

## Verification

Review the proposed boundary against E2E-05 and ADR 0004; trace interrupted writes and restart outcomes; run board validation.

## Open questions

Packaging and exact supported parameter metadata; no generic arbitrary setter commitment.

## Notes

Planned, not executed. Dependencies sequence outcomes, not proof of hardware
compatibility. Refine implementation commands and scenario bounds before claim.
