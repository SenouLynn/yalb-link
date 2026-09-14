---
id: T-047
title: Persist recognizable local connection profiles
status: backlog
priority: 1
owner: unassigned
depends_on: T-045
---

## Motivation and evidence

Bench controller and Field radio settings must survive browser and application restarts without silently opening a replacement device.

Required by [ADR 0006](../../adr/0006-operator-connection-readiness.md).

## Outcome

Backend-owned local profiles retain settings and resolve saved devices with explicit missing or ambiguous outcomes.

## Scope

Profile storage/API, device matching and declared startup policy. Profiles select acquisition paths and never grant command capabilities.

## Acceptance criteria

- [ ] Profiles retain a recognizable name, device matching information and communication settings across browser/backend restart.
- [ ] Stable identity is preferred when available; changed port names, missing identity and ambiguous matches follow documented behavior.
- [ ] A different device occupying the saved port is not silently accepted as the saved device.
- [ ] Invalid or unreadable stored settings produce an actionable state; saving failures are reported.
- [ ] Remembering a profile does not itself override intentional disconnect or enable commands.

## Verification

Pin focused storage/API checks after T-045. Exercise save/reload, failed writes, changed port name, missing device and ambiguous replacement with controlled inventory fixtures; run ./scripts/kanban check.

## Open questions

Storage location/schema and identity fallback follow the declared host scope and T-045; do not assume every adapter exposes a serial number.

## Notes

Planned, not implemented or accepted. Refine verification commands and scenario
bounds before promoting implementation work to ready.
