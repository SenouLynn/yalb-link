# Temporary validation artifacts

This directory holds ephemeral working material from validation: screenshots,
JSON observations, logs, and run-specific helpers. They explain what a reviewer
saw during a particular run; they are not maintained application inputs, test
fixtures, visual baselines, or a required archive.

`evidence/` contains the former runbook evidence folders, retained intact during
this organizational move. Existing captures may travel with this change; this
is not a requirement to commit future captures or retain them indefinitely.
Written acceptance results and reusable procedures live in
[runbooks](../runbooks/README.md) and [task cards](../tasks/README.md).

## Capture and review flow

1. Define the scenario, expected result, and verification in the task card.
2. Write run output to `docs/temp/evidence/<task-id>/<run-id>/`, using a date or
   timestamp for the run ID. Give files descriptive names.
3. Include a short run README: purpose, task, date, code revision, environment,
   commands, expected versus observed results, limitations, and file meanings.
   Distinguish live SITL observations from synthetic fixtures and unexecuted checks.
4. Record the lasting result, reproduction procedure, and any discovered defect
   in the task card or maintained runbook. Mark capture links as optional; the
   written conclusion must remain understandable without them.
5. After review or when superseded, prune the run folder. Remove its optional
   links when pruning. No retention guarantee applies; do not delete another
   active task's captures before its review is complete.

Builds, tests, CI, application behavior, and required verification must work
without this directory's contents. Repeating validation generates fresh output;
comparing a historical capture is an optional inspection, not a required gate.
Promote reusable helpers to `scripts/`, regression inputs to the appropriate test
fixtures, and required procedures to `docs/runbooks/` before depending on them.
Never point a required test or tool at this temporary directory.

Keep captures scoped to the claim being reviewed. Do not add credentials,
private recordings, or unrelated dumps. Stage only captures deliberately needed
for the current review; routine run output does not need to be committed.
