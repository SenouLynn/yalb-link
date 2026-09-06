# Task 06 — Implement the standalone worksheet

**Outcome:** a usable Vite/React page for the tested sizing journeys, separate
from the GCS frontend. Dependency: [05](05-http-boundary.md).

## Work

- Present the supported book-based design methods from the [handoff](README.md)
  through the selected user journeys. Show chapter references and adaptations in
  equation details; keep unimplemented book analyses visibly outside current coverage.
- Use compact aligned fields, collapsible sections, readable numeric output and
  an always-visible results/requirements summary. Borrow Dear ImGui's directness
  while using accessible browser controls.
- Present entry choices in the user's language: span first, weight/performance
  first, weight/size first, existing design. Explanations and next actions follow
  the selected journey; do not force every user through one wizard.
- Show input/derived roles. A derived value stays readable/copyable and explains
  its drivers. “Use as input” invokes the tested driver swap from Task 04.
- Keep maximum span separate from actual span. Provide an explicit “Use maximum”
  action. Do not recopy the maximum into actual span on every edit.
- Separate draft text from committed inputs. Blank, `-`, and unfinished decimals
  are editing states. Enter/blur commits; Escape restores. Invalid commits retain
  the draft and display a local explanation; empty input never becomes zero.
- Unit selection preserves the underlying physical quantity. Display rounding
  does not change saved/evaluated values. Go owns physical conversions/validation;
  browser formatting is presentation only.
- Preserve previous results for comparison but label their input revision. During
  requests, distinguish pending from current; discard stale responses. Handle
  unavailable backend and retry without losing the user's draft.
  Use Task 04's fresh evaluation identities across undo/redo, history branches
  and draft loading; cancellation alone does not establish response freshness.
- Expose equation/substitution traces and assumptions through progressive detail.
  Avoid routine confirmation dialogs. A violated requirement does not disable
  unrelated panels or prevent saving a clearly identified draft.
- Keep a single canonical design state; UI components contain no aerodynamic
  formulas. Choose draft persistence/export mechanics when implementing, then test.
- Establish the minimal versioned snapshot contract before saving the first draft.
  Include schema version, relevant model/equation revisions, authoritative inputs,
  units, driver roles, requirements, cases and provenance. Preserve unfinished
  editor text separately from committed values; cached results are nonauthoritative.
  Keep storage I/O outside the Go core and use Go to validate physical content.
- Validate a loaded snapshot before replacing the current candidate. Define
  explicit supported migrations or reject incompatible schema versions without
  data loss. On model revision changes, invalidate cached results/evidence as
  appropriate and reevaluate supported inputs; show unsupported results as unknown.
  Tasks 07–09 must extend this contract with compatibility checks as needed;
  Task 11 reuses it for external handoff.

## Acceptance checks

- Browser tests complete span-first and both initial weight-first journeys, with
  equivalent physical designs yielding equivalent outputs.
- Keyboard-only editing, focus after validation, explicit driver swaps,
  undo/redo, units, draft retention and required/preferred checks work.
- A failed/out-of-order request never replaces current results or erases input.
- Browser tests delay a response across edit → undo → different edit, redo and
  draft loading. Only the current evaluation can update current results.
- Save/reopen complete and invalid/incomplete drafts, preserving physical inputs,
  roles, requirements, provenance and unfinished text. Load an older model revision
  and verify reevaluation without displaying cached outputs as current. Malformed
  snapshots and unsupported schema versions leave the current draft intact;
  any supported migration has a fixture from its prior schema.
- Tests cover descriptive invalid-input recovery and the Task 04 infeasible-wing
  example. State text communicates status without relying on color.
- Show Task 04's controlling cases, partial bounds and aggregate requirement
  status; browser coverage includes a missing required case and a preferred case.
- Conventional, V-tail and flying-wing selection works; missing handling analysis
  is clearly unknown and does not block wing sizing.
- Frontend typecheck, lint, build and relevant UI tests pass. Only claim behavior
  exercised by the checks; static markup tests do not establish browser behavior.

The first useful sizing release ends here. Do not claim motor selection or
handling prediction is complete merely because those sections appear in the UI.
