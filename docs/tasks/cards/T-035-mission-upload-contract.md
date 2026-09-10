---
id: T-035
title: Specify grounded mission upload and verification
status: backlog
priority: 3
owner: unassigned
depends_on: T-034
---

## Motivation and evidence

Uploading a mission is a distinct mutation from drafting or executing it. See the [operator usage plan](../operator-usage-plan.md) for current evidence and scenario definitions.

## Outcome

Specify grounded mission upload and verification with explicit evidence and remaining limits.

## Scope

Design the addressed upload transaction and its own capability gate, then scope an implementation card.

## Acceptance criteria

- [ ] Define preconditions, explicit confirmation and exact ordered download comparison.
- [ ] Specify partial transfer, rejection, timeout, uncertain onboard state and reconnect behavior.
- [ ] No implicit clear/start/set-current or navigation capability; E2E-06 failure tests are scoped.

## Verification

Walk protocol traces for success and interruption using exact target firmware behavior; review contract and test plan; run board validation.

## Open questions

Firmware-supported mission semantics and transaction recovery policy.

## Notes

Planned, not executed. Dependencies sequence outcomes, not proof of hardware
compatibility. Refine implementation commands and scenario bounds before claim.
