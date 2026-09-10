---
id: T-025
title: Separate ephemeral validation artifacts from durable documentation
status: in_progress
priority: 2
owner: codex-docs
depends_on: none
---

## Motivation and evidence

Validation captures accumulate under runbooks and look like required project inputs.
The user requested moving the entire evidence directory under docs/temp and
making its purpose, lifecycle, and optional role clear.

## Outcome

Readers can distinguish temporary run output from maintained validation guidance.

## Acceptance criteria

- [ ] Existing evidence is preserved under docs/temp/evidence.
- [ ] Documentation explains capture, summary, promotion, and cleanup expectations.
- [ ] References use the new location; builds and tests do not require temp artifacts.

## Verification

Run `./scripts/kanban check` and `git diff --check`. Check local Markdown links,
compare moved file contents, and search for obsolete paths and build dependencies.

## Notes

Documentation-only work; existing fleet implementation and staged changes are
outside this card. Existing captures are retained for this organizational step.
