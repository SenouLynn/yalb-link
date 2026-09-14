---
id: T-051
title: Record explicit offline map preparation and initial package scope
status: done
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

The operator selected Mission Planner-style explicit prefetch and a bounded
initial offline basemap scope on 2026-09-14.

## Outcome

Two accepted ADRs capture the workflow and package boundaries without selecting
an unverified provider or storage format.

## Scope

Documentation and planning links only. Preserve active T-030 implementation.

## Acceptance criteria

- [x] Record explicit area selection/download and operator-initiated refresh.
- [x] Record one region, one basemap and complete package replacement scope.
- [x] Link decisions and distinguish them from T-030 offline startup acceptance.

## Verification

Review against the operator request, resolve changed-document Markdown links,
run `git diff --check` and `./scripts/kanban check`.

## Open questions

Provider, package format and detail/storage bounds remain implementation discovery.

## Notes

No tile downloads or runtime changes are authorized by this documentation card.

Completed 2026-09-14: ADRs 0008 and 0009 accepted and linked. Relative links,
whitespace checks and board validation pass. No runtime or hardware checks
performed; active T-030 files were left untouched.
