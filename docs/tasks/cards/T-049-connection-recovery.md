---
id: T-049
title: Recover active observation across device and aircraft interruptions
status: backlog
priority: 1
owner: unassigned
depends_on: T-023, T-046, T-047
---

## Motivation and evidence

Source-change repair alone does not cover ground USB removal, brief aircraft outages, intentional handoff or stale retained snapshots.

Required by [ADR 0006](../../adr/0006-operator-connection-readiness.md).

## Outcome

An active observation connection resumes acquisition after supported interruptions without service restart or false freshness.

## Scope

Connection retry/cancellation, reacquisition and lifecycle integration. Cover aircraft silence with ground port open separately from local device removal; actual hardware acceptance remains T-028/T-031.

## Acceptance criteria

- [ ] Short and long aircraft interruptions resume requested telemetry, including gaps that never trigger vehicle loss.
- [ ] Returning identifiable ground hardware resumes an active connection within predeclared bounds; ambiguous replacement requires selection.
- [ ] Intentional disconnect cancels pending retries and releases the port until the operator reconnects.
- [ ] Vehicle context/history survives; new traffic alone refreshes readings, and retained mission snapshots are not presented as newly verified.
- [ ] Backend/browser interruption and different returning vehicle identities follow the contract without mixing state or unbounded retry traffic.

## Verification

Define timing bounds and fault procedures before implementation. Use applicable target SITL and controlled serial faults, focused race tests and browser integration; record limitations; run ./scripts/kanban check.

## Open questions

Reboot detection and snapshot revalidation policy are specified in T-045. T-023 remains the independently executable source-change repair.

## Notes

Planned, not implemented or accepted. Refine verification commands and scenario
bounds before promoting implementation work to ready.
