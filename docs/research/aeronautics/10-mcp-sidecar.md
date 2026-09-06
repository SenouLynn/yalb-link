# Task 10 — Optional MCP access to the same core

**Outcome:** an agent can inspect and execute the calculator's supported equations
and curated workflows with the same validity checks as the UI.
Dependency: [04](04-workflow-engine.md); expose later models only when implemented.

## Work

- Preserve the [handoff's book-method mappings](README.md), source metadata and
  RC adaptations in discovery and results. An agent sees the same implemented
  subset and unsupported-method states as the worksheet.
- At implementation time, verify current MCP SDK/transport guidance and select
  an appropriate Go adapter. Keep protocol types and lifecycle out of the core.
- Expose discovery of supported equation and workflow revisions, evaluation,
  requirement explanations, and bounded sensitivity/batch evaluation as useful.
  Choose concrete tool names/schemas in this task, not before its implementation.
- “Blessed patterns” means curated, versioned, tested workflows with input roles,
  assumptions, applicability, examples and expected outcomes. It does not mean
  certified aerodynamic designs or privileged bypasses around validation.
- Return structured values, units, sources, traces and issues. Preserve the same
  driver/requirement/evidence distinctions as HTTP and direct calls.
- Keep ordinary calculation stateless where practical. Candidate persistence or
  mutation, if added, must be explicit and separate from evaluation.
- Bound batch work and honor cancellation; use deterministic ordering with any
  worker pool. An MCP caller cannot bypass missing evidence or nonconvergence.

## Acceptance checks

- Protocol integration exercises discovery, an actual call, malformed/unsupported
  input, domain failure and cancellation.
- The same golden inputs give equivalent results through direct Go, HTTP and MCP
  boundaries where those adapters exist.
- No formula/workflow is independently reimplemented in the sidecar.
- Listed patterns have test-backed examples and model revisions.
- Sidecar startup and shutdown can run independently of the UI/GCS. Document
  only the configuration and transport actually implemented and tested.
