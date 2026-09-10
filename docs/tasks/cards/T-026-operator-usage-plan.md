---
id: T-026
title: Capture operator workflows and stage isolated capabilities
status: done
priority: 0
owner: unassigned
depends_on: none
---

## Motivation and evidence

The operator described a Plane 4.6.x tricopter QuadPlane, USB bench use,
radio field observation, and later navigation and PID writes. The README
currently prioritizes simulation display work without this usage sequence.

## Outcome

A living usage plan, unexecuted end-to-end scenario register, and granular
follow-up cards guide the next work without claiming hardware acceptance.

## Acceptance criteria

- [x] Record hardware assumptions, capability ordering, and independent write gates.
- [x] Separate existing evidence from intended USB, offline radio, navigation and PID scenarios.
- [x] Link the plan from the README and scope the next cards with verification.

## Verification

Run `./scripts/kanban check` and `git diff --check`; check changed Markdown local
links and review the plan against the operator request and current code.
No runtime changes or hardware tests are part of this planning task.

## Notes

Use a WIP in docs/tasks because application boundaries remain unsettled.

Verification: board validation, whitespace checks and changed Markdown file-link
checks passed. Current UDP entry point and codec primitives were inspected.
No hardware, SITL or runtime tests were executed; all new scenarios remain planned.
