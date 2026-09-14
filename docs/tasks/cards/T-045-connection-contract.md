---
id: T-045
title: Define the local connection service contract
status: done
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

- [x] Document initial supported host scope and native serial versus managed-adapter direction, distinguishing assumptions from confirmed target evidence.
- [x] Define inventory, connect/disconnect and status interfaces, including how connections relate to multiple reporting vehicles.
- [x] Distinguish backend availability, device presence, port access, valid MAVLink, vehicle liveness and reading freshness; specify empty, busy, silent and ambiguous-device cases.
- [x] Specify explicit-disconnect behavior, startup connection policy, bounded retry behavior and browser/backend restart semantics; define measurable recovery expectations before implementation.
- [x] Map the contract to the bench, field and MissionPlanner journeys in ADR 0006; record costly-to-reverse choices in an ADR if settled.

## Verification

Review against ADR 0006 and current Go transport/HTTP and frontend stream code. Walk each normal and failure scenario through the proposed states; check links and run ./scripts/kanban check.

## Open questions

Supported host and device evidence is collected by T-027. Unknown hardware facts must stay explicit; independent interface discovery can proceed.

## Notes

Planned, not implemented or accepted. Refine verification commands and scenario
bounds before promoting implementation work to ready.

## Resolution — 2026-09-14

Delivered as [docs/tasks/connection-contract.md](../connection-contract.md):
`Device`/`Settings`/`Inventory`/`Status`/`Manager` interface sketches for
[T-046](T-046-serial-acquisition.md); the ten-state connection machine
(`DEVICE_MISSING`, `AMBIGUOUS`, `IDLE`, `OPENING`, `ACCESS_FAILED`,
`OPEN_AWAITING_TRAFFIC`, `REPORTING`, `INTERRUPTED`, `DEVICE_LOST`,
`RELEASED`) separating device presence, port access and valid-MAVLink evidence
from the existing vehicle-liveness and freshness layers; the
`POST /api/connections/connect|disconnect` + `GET /api/connections[/devices|/profiles]`
surface, following `internal/command`/`internal/recording`'s action-verb
convention; a new `acquisition` SSE event kept namespace-distinct from the
frontend's existing transport-only `'connection'` `StreamEvent` kind; and a
settled auto-reconnect-unless-explicitly-released startup policy, resolving
the item ADR 0006 had left open. [ADR 0007](../../adr/0007-native-serial-acquisition.md)
records the native-in-process-serial decision as costly-to-reverse, separate
from this document per the acceptance criterion.

One item stayed an explicitly open engineering question for T-046 rather than
being settled here: how a serial `Device`'s frames reach the existing single-
`Source` `bridge.Bridge`, since `codec.NewNode` takes a fixed endpoint list at
`Initialize`. Two directions are named in the contract (§5); neither is
mandated.

`docs/tasks/operator-usage-plan.md` and the ADR index
(`docs/adr/README.md`) were updated to link both new documents. No runtime
code changed. `./scripts/kanban check` passes.
