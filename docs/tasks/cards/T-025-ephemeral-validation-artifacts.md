---
id: T-025
title: Separate ephemeral validation artifacts from durable documentation
status: done
priority: 2
owner: unassigned
depends_on: none
---

## Motivation and evidence

Validation captures accumulate under runbooks and look like required project inputs.
The user requested moving the entire evidence directory under docs/temp and
making its purpose, lifecycle, and optional role clear.

## Outcome

Readers can distinguish temporary run output from maintained validation guidance.

## Acceptance criteria

- [x] Existing evidence is preserved under docs/temp/evidence.
- [x] Documentation explains capture, summary, promotion, and cleanup expectations.
- [x] References use the new location; builds and tests do not require temp artifacts.

## Verification

Run `./scripts/kanban check` and `git diff --check`. Check local Markdown links,
compare moved file contents, and search for obsolete paths and build dependencies.

## Notes

Documentation-only work; existing fleet implementation and staged changes are
outside this card. Existing captures are retained for this organizational step.

Verification passed: all local Markdown links resolve, moved files matched
SHA-256 before documentation updates, reusable Python helpers parse, no old
evidence paths remain, and board/whitespace checks pass. Build and test entry
points do not reference the temporary directory. Historical run READMEs were
also preserved as maintained runbooks; reusable mission helpers were promoted
to scripts/validation. No runtime changes or live SITL runs were needed.
