---
id: T-048
title: Choose connections and diagnose acquisition from the UI
status: backlog
priority: 1
owner: unassigned
depends_on: T-046, T-047
---

## Motivation and evidence

The current LIVE indicator establishes browser stream connectivity, not serial access or aircraft reachability.

Required by [ADR 0006](../../adr/0006-operator-connection-readiness.md).

## Outcome

An operator can select/save/connect/release a device and understand acquisition status from an empty fleet or vehicle workspace.

## Scope

Device/settings picker, saved profiles, connection controls, compact persistent summary and detailed actionable status using existing UI primitives. Keep connection and vehicle identity distinct.

## Acceptance criteria

- [ ] Connection controls are reachable without any discovered vehicle and from vehicle detail.
- [ ] Device choices expose available identifying information and refresh when hardware inventory changes.
- [ ] Opening a port, waiting for telemetry, reporting vehicles and stale readings are clearly distinguished from browser/backend connectivity.
- [ ] Missing/busy/access-failed and silent-port cases show supported explanations and actions, including releasing the port for MissionPlanner.
- [ ] Mock scenarios and real backend integration cover selection, persistence, connection failure, disconnect and return without disrupting vehicle context.

## Verification

Pin relevant frontend tests and browser procedure after T-045. Run lint/typecheck and applicable frontend checks; exercise empty fleet, vehicle detail and keyboard controls; run ./scripts/kanban check.

## Open questions

Exact composition follows the established panel system; operator status must not require developer diagnostics.

## Notes

Planned, not implemented or accepted. Refine verification commands and scenario
bounds before promoting implementation work to ready.
