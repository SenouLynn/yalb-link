---
id: T-044
title: Record the operator connection and recovery milestone
status: done
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

The operator accepted the Vehicle UI as sufficient to move forward and requested
an ADR capturing the gap between a working demonstration and real use on
2026-09-14. Connection setup, persistence, diagnostics and recovery must be
considered as complete operator journeys.

## Outcome

An accepted milestone decision with explicit implementation gaps and acceptance
conditions, linked from the operator plan and ADR index.

## Scope

Documentation only; no transport implementation or hardware acceptance claim.

## Acceptance criteria

- [x] ADR records the decision, context, consequences and verification point.
- [x] Bench, field, MissionPlanner handoff and interruption journeys are covered.
- [x] Existing tasks and unresolved implementation choices remain explicit.

## Verification

Review the ADR against the operator usage plan; check relative Markdown links
in changed documents; run `git diff --check` and `./scripts/kanban check`.

## Open questions

Implementation choices are recorded in the ADR, not settled by this card.

## Notes

Separate documentation outcome; does not change T-014 ownership or evidence.

Completed 2026-09-14: ADR 0006 accepted and linked from the ADR index and
operator plan. Reviewed against the established workflows; relative Markdown
links resolve, `git diff --check` passes, and the board validates. No runtime
implementation or hardware verification was performed.
