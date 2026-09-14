---
id: T-045
title: Define the local connection service contract
status: ready
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

Make ADR 0006 implementable across the operating-system, Go backend and browser boundaries before adding serial controls.

Required by [ADR 0006](../../adr/0006-operator-connection-readiness.md).

## Outcome

A reviewed connection state/API contract and initial host support boundary that backend and frontend work can implement independently.

## Scope

Specify device inventory, connection identity, status and control operations, errors, event/bootstrap behavior, and ownership. Inspect existing transport and HTTP/event conventions. No runtime implementation or new write capabilities.

## Acceptance criteria

- [ ] Document initial supported host scope and native serial versus managed-adapter direction, distinguishing assumptions from confirmed target evidence.
- [ ] Define inventory, connect/disconnect and status interfaces, including how connections relate to multiple reporting vehicles.
- [ ] Distinguish backend availability, device presence, port access, valid MAVLink, vehicle liveness and reading freshness; specify empty, busy, silent and ambiguous-device cases.
- [ ] Specify explicit-disconnect behavior, startup connection policy, bounded retry behavior and browser/backend restart semantics; define measurable recovery expectations before implementation.
- [ ] Map the contract to the bench, field and MissionPlanner journeys in ADR 0006; record costly-to-reverse choices in an ADR if settled.

## Verification

Review against ADR 0006 and current Go transport/HTTP and frontend stream code. Walk each normal and failure scenario through the proposed states; check links and run ./scripts/kanban check.

## Open questions

Supported host and device evidence is collected by T-027. Unknown hardware facts must stay explicit; independent interface discovery can proceed.

## Notes

Planned, not implemented or accepted. Refine verification commands and scenario
bounds before promoting implementation work to ready.
