# Documentation

Documentation describes the repository as it exists. Git history preserves how
it got there.

Keep only these document classes:

| Location | Purpose |
|---|---|
| `README.md` | Current capabilities, known gaps, and the next demonstrable outcome |
| `docs/runbooks/` | Commands that have been run and can be repeated |
| `docs/reference/` | Tables or contracts checked against code or tests |
| `docs/adr/` | Small records for accepted decisions that are costly to reverse |
| `docs/changelog/` | User-visible changes once releases exist |
| `docs/tasks/` | Planned outcomes, sequencing, ownership, and execution context |

Rules:

1. Prefer an executable check, test, schema, or configuration file over prose.
2. State current behavior. Put superseded reasoning in the commit message, not a
   source comment or an amendment appended to a document.
3. Shape future work in `docs/tasks/`. Mark hypotheses and unsettled choices
   explicitly; a task may explore an interface or implementation approach, but
   does not make it accepted architecture before its implementing slice begins.
4. A factual claim names its local evidence. If it depends on upstream behavior,
   record the upstream version and a reproducible check.
5. Comments explain a current invariant or a non-obvious constraint. They do not
   narrate previous plans, review findings, or tier numbers.
6. Generated files are never edited by hand.
7. Delete documents whose decision is encoded elsewhere or whose plan has been
   superseded. Completed task cards may remain through a milestone review, but
   do not retain them as an indefinite in-tree archive.

Before merging a documentation change, run the commands it mentions and search
for broken local links and references to deleted files.
