---
id: T-027
title: Capture the Plane 4.6.x QuadPlane acceptance profile
status: ready
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

Record exact firmware/build, board/frame, host and link configuration so compatibility claims have a target. See the [operator usage plan](../operator-usage-plan.md) for current evidence and scenario definitions.

## Outcome

Capture the Plane 4.6.x QuadPlane acceptance profile with explicit evidence and remaining limits.

## Scope

Capture controller identity and baseline parameters using the available established tool; record unknowns explicitly. Choose a repeatable target SITL configuration or document any simulation mismatch. Define E2E-01 through E2E-04 tolerances before execution.

## Acceptance criteria

- [ ] Profile distinguishes observed hardware facts from assumptions and names the gaps only hardware can close.
- [ ] Relevant Plane/QuadPlane SITL checks and a reproducible setup are specified; Copter 4.7 is not treated as equivalent.

## Verification

Manually inspect controller identification and exported baseline; review version and frame evidence against the proposed SITL setup. No application writes or flights.

## Open questions

Exact firmware patch/build, host OS, radio settings and USB power availability require operator/device evidence.

## Notes

Planned, not executed. Dependencies sequence outcomes, not proof of hardware
compatibility. Refine implementation commands and scenario bounds before claim.
