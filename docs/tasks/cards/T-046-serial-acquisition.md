---
id: T-046
title: Discover and open local serial devices through the backend
status: backlog
priority: 1
owner: unassigned
depends_on: T-045
---

## Motivation and evidence

The executable currently opens UDP only; an attached USB controller or radio cannot be selected through the application.

Required by [ADR 0006](../../adr/0006-operator-connection-readiness.md).

## Outcome

The local backend exposes device inventory and controlled serial acquisition using the agreed connection contract.

## Scope

OS device enumeration and metadata, refresh/removal, port settings, open/close, MAVLink ingress and addressed acquisition traffic. Preserve the UDP path. No profile persistence or frontend controls.

## Acceptance criteria

- [ ] Inventory reports available device identity metadata without claiming a generic serial device is a radio.
- [ ] Connect opens the selected device/settings and routes MAVLink through the normal bridge; disconnect releases it.
- [ ] Missing, inaccessible, busy and silent devices produce distinct supported states/errors without inventing a cause.
- [ ] Acquisition allows only the intended observer traffic when operator commands are disabled.
- [ ] A controlled serial test source exercises framing, port closure, error handling and multiple vehicle identities; hardware limitations are recorded.

## Verification

After contract selection, pin focused Go package commands and a repeatable pseudo-terminal or platform-equivalent procedure. Capture allowed outbound traffic; run ./scripts/kanban check.

## Open questions

Serial library/adapter, supported OS enumeration and permission handling follow T-045; actual device acceptance remains T-028/T-031.

## Notes

Planned, not implemented or accepted. Refine verification commands and scenario
bounds before promoting implementation work to ready.
